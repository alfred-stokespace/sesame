package cmd

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	ec2 "github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"math"
	"os"
	"strings"
	"time"

	"github.com/madflojo/tasks"

	"github.com/spf13/cobra"
)

var automationExecutionId string
var maxPollCount int

const DefaultPendingPollCount = 40
const ApiMax = 50
const maxRecords = int32(ApiMax)

type Trackomate struct {
	SSMCommand
	maxRecords            int32
	reportChan            *chan string
	scheduler             *tasks.Scheduler
	parentSchedulerId     string
	childrenSchedulerId   string
	automationExecutionId string
	maxPollCount          int
}

// trackomateCmd represents the trackomate command
var trackomateCmd = &cobra.Command{
	Use:   "trackomate",
	Short: "Track a start-automation-execution ",
	Long:  `Track for limited amount of time progress on all hosts`,
	Args:  ValidateArgsFunc(),
	Run: func(cmd *cobra.Command, args []string) {
		_, err := fmt.Fprintf(os.Stderr, "trackomate called: [id=%s]\n", automationExecutionId)
		if err != nil {
			panic(err)
		}
		if len(automationExecutionId) == 0 {
			exitOnError(&SesameError{msg: "id cannot be empty "})
		}

		reportChan := make(chan string, 10)
		t := Trackomate{SSMCommand{}, maxRecords, &reportChan, tasks.New(), "", "", automationExecutionId, maxPollCount}
		t.conf()
		t.thingDo()
	},
}

func (trackomate *Trackomate) scheduleParent() string {
	// Add a task
	parentCheckId, err := trackomate.scheduler.Add(&tasks.Task{
		Interval: 2 * time.Second,
		TaskFunc: func() error {
			endState := trackomate.checkParent()
			if endState.IsEndState {
				if endState.IsEndStateSuccess {
					*trackomate.reportChan <- "Succeeded"
					_, err := fmt.Fprintf(os.Stdout, "[%s]: Success!\n", trackomate.automationExecutionId)
					exitOnError(err)
				} else {
					*trackomate.reportChan <- "Failed"
					_, err := fmt.Fprintf(os.Stdout, "[%s]: Faled!\n", trackomate.automationExecutionId)
					exitOnError(err)
				}
			}
			return nil
		},
	})
	exitOnError(err)
	return parentCheckId
}

type ComplexStatus struct {
	IsEndState        bool
	IsEndStateSuccess bool
	InternalStatus    string
}

func (trackomate *Trackomate) checkParent() ComplexStatus {
	filters := trackomate.getParent()

	input := ssm.DescribeAutomationExecutionsInput{
		Filters:    filters,
		MaxResults: &trackomate.maxRecords,
	}
	res, serviceError := trackomate.svc.DescribeAutomationExecutions(context.Background(), &input)
	exitOnError(serviceError)
	cState := ComplexStatus{IsEndState: true, IsEndStateSuccess: false}
	if len(res.AutomationExecutionMetadataList) == 0 {
		exitOnError(&SesameError{msg: "No results for execution id."})
	} else {
		for _, item := range res.AutomationExecutionMetadataList {

			_, err := fmt.Fprintf(os.Stdout, "Parent document: %s [%s]\n", item.AutomationExecutionStatus, *item.DocumentName)
			if err != nil {
				exitOnError(err)
			}

			isCompleted, isSuccess := trackomate.isCompletedStatus(item)

			cState := ComplexStatus{IsEndState: isCompleted, IsEndStateSuccess: isSuccess}

			if isCompleted {
				if trackomate.parentSchedulerId != "" {
					trackomate.scheduler.Del(trackomate.parentSchedulerId)
				}
				cState.IsEndState = true
				cState.IsEndStateSuccess = isSuccess
			} else if trackomate.isPendingStatus(item) {
				cState.InternalStatus = string(item.AutomationExecutionStatus)
				cState.IsEndState = false
			} else {
				// this should only happen if AWS adds a status we didn't account for, or we have a bug!
				cState.IsEndState = true
				cState.IsEndStateSuccess = false
			}
			return cState
		}
	}

	return cState
}

// Which ones are considered success?
// - From services/ssm/types/enums.go
//AutomationExecutionStatusPending                        AutomationExecutionStatus = "Pending"
//AutomationExecutionStatusInprogress                     AutomationExecutionStatus = "InProgress"
//AutomationExecutionStatusWaiting                        AutomationExecutionStatus = "Waiting"
//AutomationExecutionStatusSuccess                        AutomationExecutionStatus = "Success"
//AutomationExecutionStatusTimedout                       AutomationExecutionStatus = "TimedOut"
//AutomationExecutionStatusCancelling                     AutomationExecutionStatus = "Cancelling"
//AutomationExecutionStatusCancelled                      AutomationExecutionStatus = "Cancelled"
//AutomationExecutionStatusFailed                         AutomationExecutionStatus = "Failed"
//AutomationExecutionStatusPendingApproval                AutomationExecutionStatus = "PendingApproval"
//AutomationExecutionStatusApproved                       AutomationExecutionStatus = "Approved"
//AutomationExecutionStatusRejected                       AutomationExecutionStatus = "Rejected"
//AutomationExecutionStatusScheduled                      AutomationExecutionStatus = "Scheduled"
//AutomationExecutionStatusRunbookInprogress              AutomationExecutionStatus = "RunbookInProgress"
//AutomationExecutionStatusPendingChangeCalendarOverride  AutomationExecutionStatus = "PendingChangeCalendarOverride"
//AutomationExecutionStatusChangeCalendarOverrideApproved AutomationExecutionStatus = "ChangeCalendarOverrideApproved"
//AutomationExecutionStatusChangeCalendarOverrideRejected AutomationExecutionStatus = "ChangeCalendarOverrideRejected"
//AutomationExecutionStatusCompletedWithSuccess           AutomationExecutionStatus = "CompletedWithSuccess"
//AutomationExecutionStatusCompletedWithFailure           AutomationExecutionStatus = "CompletedWithFailure"
//
// AWS Doesn't seem to have methods for categorizing statuses, so we have add that.
//             /- Succeeded =====================================
//            /           - AutomationExecutionStatusCompletedWithSuccess
// completed =            - AutomationExecutionStatusSuccess
//             \- Failed    ======================================
//                        - AutomationExecutionStatusTimedout
//                        - AutomationExecutionStatusCancelled
//                        - AutomationExecutionStatusFailed
//                        - AutomationExecutionStatusRejected
//                        - AutomationExecutionStatusCompletedWithFailure
//
// pending =   ======================
//           - AutomationExecutionStatusPending
//           - AutomationExecutionStatusInprogress
//           - AutomationExecutionStatusWaiting
//           - AutomationExecutionStatusCancelling
//           - AutomationExecutionStatusPendingApproval
//           - AutomationExecutionStatusApproved
//           - AutomationExecutionStatusScheduled
//           - AutomationExecutionStatusRunbookInprogress
//           - AutomationExecutionStatusPendingChangeCalendarOverride
//           - AutomationExecutionStatusChangeCalendarOverrideApproved
//           - AutomationExecutionStatusChangeCalendarOverrideRejected
//

func (trackomate *Trackomate) isCompletedStatus(item types.AutomationExecutionMetadata) (bool, bool) {
	isCompleted := false
	isSuccess := false
	switch status := item.AutomationExecutionStatus; status {
	case types.AutomationExecutionStatusCompletedWithSuccess:
		{
			isCompleted = true
			isSuccess = true
		}
	case types.AutomationExecutionStatusSuccess:
		{
			isCompleted = true
			isSuccess = true
		}
	case types.AutomationExecutionStatusTimedout:
		{
			isCompleted = true
			isSuccess = false
		}
	case types.AutomationExecutionStatusCancelled:
		{
			isCompleted = true
			isSuccess = false
		}
	case types.AutomationExecutionStatusFailed:
		{
			isCompleted = true
			isSuccess = false
		}
	case types.AutomationExecutionStatusRejected:
		{
			isCompleted = true
			isSuccess = false
		}
	case types.AutomationExecutionStatusCompletedWithFailure:
		{
			isCompleted = true
			isSuccess = false
		}
	}
	return isCompleted, isSuccess
}

func (trackomate *Trackomate) isPendingStatus(item types.AutomationExecutionMetadata) bool {
	isPending := false
	switch status := item.AutomationExecutionStatus; status {
	case types.AutomationExecutionStatusPending:
		{
			isPending = true
		}
	case types.AutomationExecutionStatusInprogress:
		{
			isPending = true
		}
	case types.AutomationExecutionStatusWaiting:
		{
			isPending = true
		}
	case types.AutomationExecutionStatusCancelling:
		{
			isPending = true
		}
	case types.AutomationExecutionStatusPendingApproval:
		{
			isPending = true
		}
	case types.AutomationExecutionStatusApproved:
		{
			isPending = true
		}
	case types.AutomationExecutionStatusRunbookInprogress:
		{
			isPending = true
		}
	case types.AutomationExecutionStatusPendingChangeCalendarOverride:
		{
			isPending = true
		}
	case types.AutomationExecutionStatusChangeCalendarOverrideApproved:
		{
			isPending = true
		}
	case types.AutomationExecutionStatusChangeCalendarOverrideRejected:
		{
			isPending = true
		}
	case types.AutomationExecutionStatusScheduled:
		{
			isPending = true
		}
	}
	return isPending
}

func (trackomate *Trackomate) getStatusColor(item types.AutomationExecutionMetadata) string {
	isCompleted, isSuccess := trackomate.isCompletedStatus(item)
	if !isCompleted {
		// yellow
		return "\033[33m" + string(item.AutomationExecutionStatus) + "\033[0m"
	}
	if isSuccess {
		// green
		return "\033[32m" + string(item.AutomationExecutionStatus) + "\033[0m"
	} else {
		// red
		return "\033[31m" + string(item.AutomationExecutionStatus) + "\033[0m"
	}
}

func (trackomate *Trackomate) scheduleChildren() string {
	childCheckId, err := trackomate.scheduler.Add(&tasks.Task{
		Interval: time.Duration(2 * time.Second),
		TaskFunc: func() error {
			trackomate.checkChildren(trackomate.automationExecutionId)
			*trackomate.reportChan <- "child ran"
			return nil
		},
	})
	exitOnError(err)
	return childCheckId
}

func (trackomate *Trackomate) checkChildren(executionId string) {
	childFilters := getFirstLevelChildren(executionId)
	childrenInput := &ssm.DescribeAutomationExecutionsInput{
		Filters:    childFilters,
		MaxResults: &trackomate.maxRecords,
	}
	resChildren, childrenServiceError := trackomate.svc.DescribeAutomationExecutions(context.Background(), childrenInput)
	exitOnError(childrenServiceError)
	if len(resChildren.AutomationExecutionMetadataList) == 0 {
		exitOnError(&SesameError{msg: "No results for execution id."})
	} else {
		type Executions struct {
			allComplete bool
			succeeded   []string
			failed      []string
			incomplete  []string
		}
		execs := Executions{}
		execs.allComplete = true
		for _, item := range resChildren.AutomationExecutionMetadataList {
			var name string
			if strings.HasPrefix(*item.Target, "mi-") {
				name = trackomate.getManagedInstanceTagValue(&item, "Name")
			} else {
				name = trackomate.getEC2InstanceTagValue(&item, "Name")
			}
			outs := item.Outputs
			for s, k := range outs {
				fmt.Fprintf(os.Stdout, "%s:%v", s, k)
			}
			isCompleted, isSuccess := trackomate.isCompletedStatus(item)
			if isCompleted {
				if isSuccess {
					execs.succeeded = append(execs.succeeded, *item.Target)
					//TODO: May or may not have children run commands
				} else {
					execs.failed = append(execs.failed, *item.Target)
				}
				fm := item.FailureMessage
				if fm == nil {
					none := ""
					fm = &none
				}
				_, err := fmt.Fprintf(os.Stdout, " CHILD: what [%s]:[%s] %s[%s] : %s\n", *item.DocumentName, trackomate.getStatusColor(item), name, *item.Target, *fm)
				if err != nil {
					panic(err)
				}
				trackomate.getStepExecutions(&item)
			} else {
				execs.allComplete = false
				execs.incomplete = append(execs.incomplete, *item.Target)
				trackomate.getStepExecutions(&item)
				_, err := fmt.Fprintf(os.Stdout, " CHILD: what [%s]:[%s] %s[%s] : %s\n", *item.DocumentName, trackomate.getStatusColor(item), name, *item.Target, "pending")
				if err != nil {
					panic(err)
				}
			}

		}

		if execs.allComplete {
			trackomate.scheduler.Del(trackomate.childrenSchedulerId)
			*trackomate.reportChan <- "DONE"
		}
	}
}

func (trackomate *Trackomate) getManagedInstanceTagValue(item *types.AutomationExecutionMetadata, tagName string) string {
	tagList := ssm.ListTagsForResourceInput{
		ResourceId:   item.Target,
		ResourceType: types.ResourceTypeForTaggingManagedInstance,
	}
	tags, tagError := trackomate.svc.ListTagsForResource(context.Background(), &tagList)
	exitOnError(tagError)
	var name = ""
	for _, v := range tags.TagList {
		if *v.Key == tagName {
			name = *v.Value
		}
	}
	return name
}

func (trackomate *Trackomate) thingDo() {

	// sync check if parent exists
	// if success, no reason to go async
	// if absent, error (exit)
	// if non-endstate, go async
	//   async -
	//     check parent in 2 sec loop
	//     check children in 2 sec loop
	//     report status from either parent or child
	//     exit on failure

	endState := trackomate.checkParent()
	if !endState.IsEndState {
		trackomate.parentSchedulerId = trackomate.scheduleParent()
		trackomate.childrenSchedulerId = trackomate.scheduleChildren()
	}

	if endState.IsEndState && endState.IsEndStateSuccess {
		_, err := fmt.Fprintf(os.Stdout, "PARENT: automation-id=[%s]: Success!\n", trackomate.automationExecutionId)
		exitOnError(err)
		trackomate.checkChildren(trackomate.automationExecutionId)
	}

	if !endState.IsEndState {
		// Start the Scheduler

		defer trackomate.scheduler.Stop()

		if trackomate.maxPollCount < 0 {
			trackomate.maxPollCount = math.MaxInt32
		}
		x := trackomate.maxPollCount
		for i := 1; i < x-1; i++ {
			if len(trackomate.scheduler.Tasks()) > 0 {
				fmt.Println("Checking..")
				c := trackomate.reportChan
				report := <-*c
				fmt.Printf("  REPORT: %s \n", report)
				if report == "DONE" {
					trackomate.scheduler.Stop()
					break
				}
			} else {
				fmt.Printf("  REPORT: Nothing scheduled, ending watch! \n")
				break
			}
		}
		fmt.Println("Stopping")
	}
}

func (trackomate *Trackomate) getStepExecutions(item *types.AutomationExecutionMetadata) {
	reverse := true
	stepsInput := ssm.DescribeAutomationStepExecutionsInput{
		AutomationExecutionId: item.AutomationExecutionId,
		ReverseOrder:          &reverse,
	}

	steps, err := trackomate.svc.DescribeAutomationStepExecutions(context.Background(), &stepsInput)
	exitOnError(err)
	if len(steps.StepExecutions) == 0 {
		return
	} else {
		for _, s := range steps.StepExecutions {
			fmt.Printf(" CHILD: StepName:%s, Status:%s, execId:%s\n", *s.StepName, s.StepStatus, *s.StepExecutionId)
		}
	}
	getStepInput := ssm.GetAutomationExecutionInput{
		AutomationExecutionId: item.AutomationExecutionId,
	}
	stepForCommand, notherErr := trackomate.svc.GetAutomationExecution(context.Background(), &getStepInput)
	exitOnError(notherErr)
	for _, se := range stepForCommand.AutomationExecution.StepExecutions {
		for _, commandId := range se.Outputs["CommandId"] {
			instanceIdInputWithFormating := se.Inputs["InstanceIds"]
			instanceIdInputs := strings.Replace(strings.Replace(instanceIdInputWithFormating, "\"]", "", -1), "[\"", "", -1)
			listCommandInput := ssm.ListCommandInvocationsInput{
				CommandId:  &commandId,
				InstanceId: &instanceIdInputs,
				Details:    true,
			}
			commandInvs, moreErr := trackomate.svc.ListCommandInvocations(context.Background(), &listCommandInput)
			exitOnError(moreErr)
			for _, commandInv := range commandInvs.CommandInvocations {
				for _, commandPlugins := range commandInv.CommandPlugins {
					if commandPlugins.Output != nil && *commandPlugins.Output == "" {
						fmt.Printf(" CHILD: [%s:%s]: output: -empty-\n", *commandPlugins.Name, commandId)
					} else if commandPlugins.Output != nil {
						fmt.Printf(" CHILD: [%s:%s]: output: \n\t%s\n", *commandPlugins.Name, commandId, strings.Replace(*commandPlugins.Output, "\n", "\n\t", -1))
					}
				}
			}
		}
	}
}

func (trackomate *Trackomate) getParent() []types.AutomationExecutionFilter {
	key := "ExecutionId"
	filters := []types.AutomationExecutionFilter{
		{
			Key: types.AutomationExecutionFilterKey(key),
			Values: []string{
				trackomate.automationExecutionId,
			},
		},
	}
	return filters
}

func (trackomate *Trackomate) getEC2InstanceTagValue(item *types.AutomationExecutionMetadata, tagName string) string {
	maxResults := int32(50)

	str := item.Target
	strVals := []string{*str}
	tagListFilter := ec2.DescribeTagsInput{
		Filters: []ec2types.Filter{{
			Name:   aws.String("resource-id"),
			Values: strVals,
		}},
		MaxResults: &maxResults,
	}
	tags, tagError := trackomate.svcEc2.DescribeTags(context.Background(), &tagListFilter)
	exitOnError(tagError)
	var name = ""
	for _, v := range tags.Tags {
		if *v.Key == tagName {
			name = *v.Value
		}
	}
	return name
}

func getFirstLevelChildren(executionId string) []types.AutomationExecutionFilter {
	key := "ParentExecutionId"
	filters := []types.AutomationExecutionFilter{
		{
			Key: types.AutomationExecutionFilterKey(key),
			Values: []string{
				executionId,
			},
		},
	}
	return filters
}

func init() {
	rootCmd.AddCommand(trackomateCmd)

	trackomateCmd.Flags().StringVarP(&automationExecutionId, "id", "i", "", "Provide the AutomationExecutionId from an ssm start-automation-execution command")
	trackomateCmd.Flags().IntVarP(&maxPollCount, "maxPollCount", "p", DefaultPendingPollCount, fmt.Sprintf("Provide a number of times to poll for pending tasks before giving up. (-1) will poll until overal terminal status reached."))

	err := trackomateCmd.MarkFlagRequired("id")
	if err != nil {
		return
	}

}
