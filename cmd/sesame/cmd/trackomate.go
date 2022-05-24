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
	maxRecords int64
	reportChan *chan string
	scheduler  *tasks.Scheduler
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
		t := Trackomate{SSMCommand{}, maxRecords, &reportChan, tasks.New()}
		t.conf()
		t.thingDo()
	},
}

func (trackomate *Trackomate) scheduleParent() string {
	// Add a task
	parentCheckId, err := trackomate.scheduler.Add(&tasks.Task{
		Interval: time.Duration(2 * time.Second),
		TaskFunc: func() error {
			succeeded := trackomate.checkParent()
			*trackomate.reportChan <- "Ran"
			// non-existent would have aborted, we have a valid id!
			if succeeded {
				_, err := fmt.Fprintf(os.Stdout, "[%s]: Success!\n", automationExecutionId)
				exitOnError(err)
			}
			return nil
		},
	})
	exitOnError(err)
	fmt.Printf(" parentCheckId: %s\n", parentCheckId)
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

			_, err := fmt.Fprintf(os.Stdout, "Parent status: %s\n", *item.AutomationExecutionStatus)
			if err != nil {
				exitOnError(err)
			}
			// go async until end state or max timeout.

			isSuccess := *item.AutomationExecutionStatus == ssm.AutomationExecutionStatusSuccess
			if !isSuccess {
				trackomate.scheduleParent()
				trackomate.scheduleChildren()
			}
			return isSuccess
		}
	}

	return false
}

func (trackomate *Trackomate) scheduleChildren() string {
	childCheckId, err := trackomate.scheduler.Add(&tasks.Task{
		Interval: time.Duration(2 * time.Second),
		TaskFunc: func() error {
			trackomate.checkChildren()
			*trackomate.reportChan <- "child ran"
			return nil
		},
	})
	exitOnError(err)
	fmt.Printf(" ChildCheckId: %s\n", childCheckId)
	return childCheckId
}

func (trackomate *Trackomate) checkChildren() {
	childFilters := getFirstLevelChildren()
	childrenInput := ssm.DescribeAutomationExecutionsInput{
		Filters:    childFilters,
		MaxResults: &trackomate.maxRecords,
	}
	resChildren, childrenServiceError := trackomate.svc.DescribeAutomationExecutions(&childrenInput)
	exitOnError(childrenServiceError)
	if len(resChildren.AutomationExecutionMetadataList) == 0 {
		exitOnError(&SesameError{msg: "No results for execution id."})
	} else {
		for _, item := range resChildren.AutomationExecutionMetadataList {

			_, err := fmt.Fprintf(os.Stdout, "CHILD: instanceId[%s], status: %s\n", *item.Target, *item.AutomationExecutionStatus)
			if err != nil {
				panic(err)
			}
		}
	}
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

	// TODO: checkonce means that checkParent has control over scheduling, you can't also do scheduling down below.
	succeeded := trackomate.checkParent()
	// non-existent would have aborted, we have a valid id!
	if succeeded {
		_, err := fmt.Fprintf(os.Stdout, "PARENT: automation-id=[%s]: Success!\n", automationExecutionId)
		exitOnError(err)
		trackomate.checkChildren()
	}

	if !succeeded {
		// Start the Scheduler

		defer trackomate.scheduler.Stop()
		x := 20
		for i := 1; i < x-1; i++ {
			fmt.Println("Checking..")
			c := trackomate.reportChan
			report := <-*c
			fmt.Printf("REPORT: %s \n", report)
		}
		fmt.Println("Stopping")
		//		trackomate.scheduler.Del(parentCheckId)
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

func getFirstLevelChildren() []*ssm.AutomationExecutionFilter {
	key := "ParentExecutionId"
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

func init() {
	rootCmd.AddCommand(trackomateCmd)

	trackomateCmd.Flags().StringVarP(&automationExecutionId, "id", "i", "", "Provide the AutomationExecutionId from an ssm start-automation-execution command")

	err := trackomateCmd.MarkFlagRequired("id")
	if err != nil {
		return
	}

}
