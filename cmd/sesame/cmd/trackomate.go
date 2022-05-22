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
	MaxRecords int64
	reportChan *chan string
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
		t := Trackomate{SSMCommand{}, maxRecords, &reportChan}
		t.conf()
		t.thingDo()
	},
}

func (trackomate *Trackomate) checkParent(checkOnce bool) bool {
	filters := getParent()
	
	input := ssm.DescribeAutomationExecutionsInput{
		Filters:    filters,
		MaxResults: &trackomate.MaxRecords,
	}
	res, serviceError := trackomate.svc.DescribeAutomationExecutions(&input)
	exitOnError(serviceError)
	if len(res.AutomationExecutionMetadataList) == 0 {
		exitOnError(&SesameError{msg: "No results for execution id."})
	} else {
		for _, item := range res.AutomationExecutionMetadataList {

			_, err := fmt.Fprintln(os.Stdout, *item.AutomationExecutionStatus)
			if err != nil {
				isSuccess := *item.AutomationExecutionStatus != ssm.AutomationExecutionStatusSuccess  
				if isSuccess {
					return isSuccess
				} 
				if checkOnce {
					return isSuccess
				}
				// go async until end state or max timeout. 
			}

		}
	}

	return false
}

func (trackomate *Trackomate) checkChildren() {
	childFilters := getFirstLevelChildren()
	childrenInput := ssm.DescribeAutomationExecutionsInput{
		Filters:    childFilters,
		MaxResults: &trackomate.MaxRecords,
	}
	resChildren, childrenServiceError := trackomate.svc.DescribeAutomationExecutions(&childrenInput)
	exitOnError(childrenServiceError)
	if len(resChildren.AutomationExecutionMetadataList) == 0 {
		exitOnError(&SesameError{msg: "No results for execution id."})
	} else {
		for _, item := range resChildren.AutomationExecutionMetadataList {

			_, err := fmt.Fprintln(os.Stdout, *item.AutomationExecutionStatus)
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

	succeeded := trackomate.checkParent(true)
	// non-existent would have aborted, we have a valid id!
	if succeeded {
		_, err := fmt.Fprintf(os.Stdout, "[%s]: Success!\n", automationExecutionId)
		exitOnError(err)
	}
	
	// Start the Scheduler
	scheduler := tasks.New()
	defer scheduler.Stop()

	// Add a task
	parentCheckId, err := scheduler.Add(&tasks.Task{
		Interval: time.Duration(2 * time.Second),
		TaskFunc: func() error {
			succeeded := trackomate.checkParent(false)
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
	fmt.Printf("%s", parentCheckId)

	childCheckId, err := scheduler.Add(&tasks.Task{
		Interval: time.Duration(2 * time.Second),
		TaskFunc: func() error {
			trackomate.checkChildren()
			*trackomate.reportChan <- "child ran"
			return nil
		},
	})
	exitOnError(err)
	fmt.Printf("%s", childCheckId)

	x := 20
	for i := 1; i < x-1; i++ {
		fmt.Println("Checking..")
		c := trackomate.reportChan
		report := <-*c
		fmt.Printf("REPORT: %s \n", report)
	}
	fmt.Println("Stopping")
	scheduler.Del(parentCheckId)

    trackomate.checkChildren()
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
