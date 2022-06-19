package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/madflojo/tasks"

	"github.com/spf13/cobra"
)

var automationExecutionId string

const ApiMax = 50
const maxRecords = int64(ApiMax)

type Trackomate struct {
	SSMCommand
	maxRecords          int64
	reportChan          *chan string
	scheduler           *tasks.Scheduler
	parentSchedulerId   string
	childrenSchedulerId string
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
		t := Trackomate{SSMCommand{}, maxRecords, &reportChan, tasks.New(), "", ""}
		t.conf()
		t.thingDo()
	},
}

func (trackomate *Trackomate) scheduleParent() string {
	// Add a task
	parentCheckId, err := trackomate.scheduler.Add(&tasks.Task{
		Interval: 2 * time.Second,
		TaskFunc: func() error {
			succeeded := trackomate.checkParent()
			if !succeeded {
				*trackomate.reportChan <- "Failed"
				_, err := fmt.Fprintf(os.Stdout, "[%s]: Faled!\n", automationExecutionId)
				exitOnError(err)
			} else {
				*trackomate.reportChan <- "Succeeded"
				_, err := fmt.Fprintf(os.Stdout, "[%s]: Success!\n", automationExecutionId)
				exitOnError(err)
			}
			return nil
		},
	})
	exitOnError(err)
	return parentCheckId
}

func (trackomate *Trackomate) checkParent() bool {
	filters := getParent()

	input := ssm.DescribeAutomationExecutionsInput{
		Filters:    filters,
		MaxResults: &trackomate.maxRecords,
	}
	res, serviceError := trackomate.svc.DescribeAutomationExecutions(&input)
	exitOnError(serviceError)
	if len(res.AutomationExecutionMetadataList) == 0 {
		exitOnError(&SesameError{msg: "No results for execution id."})
	} else {
		for _, item := range res.AutomationExecutionMetadataList {

			_, err := fmt.Fprintf(os.Stdout, "Parent document: %s [%s]\n", *item.AutomationExecutionStatus, *item.DocumentName)
			if err != nil {
				exitOnError(err)
			}
			isSuccess := trackomate.isSuccess(item)
			isFailed := trackomate.isFailed(item)
			if isFailed || isSuccess {
				if trackomate.parentSchedulerId != "" {
					trackomate.scheduler.Del(trackomate.parentSchedulerId)
				}
			}
			return isSuccess
		}
	}

	return false
}

func (trackomate *Trackomate) isSuccess(item *ssm.AutomationExecutionMetadata) bool {
	isSuccess := *item.AutomationExecutionStatus == ssm.AutomationExecutionStatusSuccess
	return isSuccess
}

func (trackomate *Trackomate) getStatusColor(item *ssm.AutomationExecutionMetadata) string {
	if trackomate.isSuccess(item) {
		return "\033[32m" + *item.AutomationExecutionStatus + "\033[0m"
	} else {
		return "\033[31m" + *item.AutomationExecutionStatus + "\033[0m"
	}
}

func (trackomate *Trackomate) isFailed(item *ssm.AutomationExecutionMetadata) bool {
	isFailed :=
		*item.AutomationExecutionStatus == ssm.AutomationExecutionStatusFailed ||
			*item.AutomationExecutionStatus == ssm.AutomationExecutionStatusTimedOut
	return isFailed
}
func (trackomate *Trackomate) scheduleChildren() string {
	childCheckId, err := trackomate.scheduler.Add(&tasks.Task{
		Interval: time.Duration(2 * time.Second),
		TaskFunc: func() error {
			trackomate.checkChildren(automationExecutionId)
			*trackomate.reportChan <- "child ran"
			return nil
		},
	})
	exitOnError(err)
	return childCheckId
}

func (trackomate *Trackomate) checkChildren(executionId string) {
	childFilters := getFirstLevelChildren(executionId)
	childrenInput := ssm.DescribeAutomationExecutionsInput{
		Filters:    childFilters,
		MaxResults: &trackomate.maxRecords,
	}
	resChildren, childrenServiceError := trackomate.svc.DescribeAutomationExecutions(&childrenInput)
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
			name := trackomate.getManagedInstanceTagValue(item, "Name")
			outs := item.Outputs
			for s, k := range outs {
				fmt.Fprintf(os.Stdout, "%s:%v", s, k)
			}
			if trackomate.isSuccess(item) {
				execs.succeeded = append(execs.succeeded, *item.Target)
				//TODO: May or may not have children run commands
			} else if trackomate.isFailed(item) {
				execs.failed = append(execs.failed, *item.Target)
			} else {
				execs.allComplete = false
				execs.incomplete = append(execs.incomplete, *item.Target)
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
		}

		if execs.allComplete {
			trackomate.scheduler.Del(trackomate.childrenSchedulerId)
			*trackomate.reportChan <- "DONE"
		}
	}
}

func (trackomate *Trackomate) getManagedInstanceTagValue(item *ssm.AutomationExecutionMetadata, tagName string) string {
	managedInstance := "ManagedInstance"
	tagList := ssm.ListTagsForResourceInput{
		ResourceId:   item.Target,
		ResourceType: &managedInstance,
	}
	tags, tagError := trackomate.svc.ListTagsForResource(&tagList)
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
	// if non-success, go async
	//   async -
	//     check parent in 2 sec loop
	//     check children in 2 sec loop
	//     report status from either parent or child
	//     exit on failure

	succeeded := trackomate.checkParent()
	if !succeeded {
		trackomate.parentSchedulerId = trackomate.scheduleParent()
		trackomate.childrenSchedulerId = trackomate.scheduleChildren()
	}
	// non-existent would have aborted, we have a valid id!
	if succeeded {
		_, err := fmt.Fprintf(os.Stdout, "PARENT: automation-id=[%s]: Success!\n", automationExecutionId)
		exitOnError(err)
		trackomate.checkChildren(automationExecutionId)
	}

	if !succeeded {
		// Start the Scheduler

		defer trackomate.scheduler.Stop()
		x := 20
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

func getParent() []*ssm.AutomationExecutionFilter {
	key := "ExecutionId"
	filters := []*ssm.AutomationExecutionFilter{
		{
			Key: &key,
			Values: []*string{
				&automationExecutionId,
			},
		},
	}
	return filters
}

func getFirstLevelChildren(executionId string) []*ssm.AutomationExecutionFilter {
	key := "ParentExecutionId"
	filters := []*ssm.AutomationExecutionFilter{
		{
			Key: &key,
			Values: []*string{
				&executionId,
			},
		},
	}
	return filters
}

func init() {
	rootCmd.AddCommand(trackomateCmd)

	trackomateCmd.Flags().StringVarP(&automationExecutionId, "id", "i", "", "Provide the AutomationExecutionId from an ssm start-automation-execution command")

	err := trackomateCmd.MarkFlagRequired("id")
	if err != nil {
		return
	}

}
