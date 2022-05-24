package cmd

import (
	"fmt"
	"github.com/aws/aws-sdk-go/service/ssm"
	"os"

	"github.com/spf13/cobra"
)

const ApiMax = 50

var automationExecutionId string

type Trackomate struct {
	SSMCommand
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
		}
		if len(automationExecutionId) == 0 {
			exitOnError(&SesameError{msg: "id cannot be empty "})
		}

		t := Trackomate{SSMCommand{}}
		t.conf()
		t.thingDo()
	},
}

func (trackomate *Trackomate) thingDo() {
	checkParent(trackomate)
	checkChildren(trackomate)
}

func checkChildren(trackomate *Trackomate) {
	maxRecords := int64(ApiMax)
	childFilters := getFirstLevelChildren()
	childrenInput := ssm.DescribeAutomationExecutionsInput{
		Filters:    childFilters,
		MaxResults: &maxRecords,
	}
	resChildren, childrenServiceError := trackomate.svc.DescribeAutomationExecutions(&childrenInput)
	exitOnError(childrenServiceError)
	if len(resChildren.AutomationExecutionMetadataList) == 0 {
		exitOnError(&SesameError{msg: "No results for execution id."})
	} else {
		for _, item := range resChildren.AutomationExecutionMetadataList {

			_, err := fmt.Fprintln(os.Stdout, *item.AutomationExecutionStatus)
			if err != nil {
			}

		}
	}
}

func checkParent(trackomate *Trackomate) int64 {
	filters := getParent()
	maxRecords := int64(ApiMax)
	input := ssm.DescribeAutomationExecutionsInput{
		Filters:    filters,
		MaxResults: &maxRecords,
	}
	res, serviceError := trackomate.svc.DescribeAutomationExecutions(&input)
	exitOnError(serviceError)
	if len(res.AutomationExecutionMetadataList) == 0 {
		exitOnError(&SesameError{msg: "No results for execution id."})
	} else {
		for _, item := range res.AutomationExecutionMetadataList {

			_, err := fmt.Fprintln(os.Stdout, *item.AutomationExecutionStatus)
			if err != nil {
			}

		}
	}
	return maxRecords
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
