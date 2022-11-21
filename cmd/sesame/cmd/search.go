package cmd

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/spf13/cobra"
	"os"
)

var nickname string
var tag string

type Search struct {
	SSMCommand
}

// searchCmd represents the search command
var searchCmd = &cobra.Command{
	Use:   "search",
	Short: "search for a single host from a nickname you provide",
	Long: `If you have a "Nickname" tag on your host search using that.

If you don't have the default tag name then you can provide it.`,
	Args: ValidateArgsFunc(),
	Run: func(cmd *cobra.Command, args []string) {
		_, err := fmt.Fprintf(os.Stderr, "search called: [%s: %s]\n", tag, nickname)
		if err != nil {
			panic(err)
		}
		if len(nickname) == 0 {
			exitOnError(fmt.Errorf("tag cannot be empty %s:%s", tag, nickname))
		}

		s := Search{SSMCommand{}}
		s.conf()
		s.thingDo()
	},
}

func (search *Search) thingDo() {
	// Create our filter slice
	filters := []types.InstanceInformationStringFilter{
		{
			Key:    aws.String(fmt.Sprintf("tag:%s", tag)),
			Values: []string{nickname},
		},
	}

	const ApiMin = 5
	maxRes := int32(ApiMin)
	input := &ssm.DescribeInstanceInformationInput{
		Filters:    filters,
		MaxResults: &maxRes,
	}
	res, serviceError := search.svc.DescribeInstanceInformation(context.Background(), input)
	exitOnError(serviceError)
	if len(res.InstanceInformationList) > 1 {
		exitOnError(&SesameError{msg: "Too many results for tag."})

	} else if len(res.InstanceInformationList) == 0 {
		exitOnError(&SesameError{msg: "No results for tag."})
	} else {
		for _, item := range res.InstanceInformationList {

			_, err := fmt.Fprintln(os.Stdout, *item.InstanceId)
			if err != nil {
				panic(err)
			}
		}
	}
}

func init() {
	rootCmd.AddCommand(searchCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// searchCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	searchCmd.Flags().StringVarP(&nickname, "nickname", "n", "", "Provide the value (or name) to search SSM hosts by tag value. See additional flag for your custom tag key.")
	searchCmd.Flags().StringVarP(&tag, "tag", "t", "Nickname", "Provide the value of a tag name to search SSM hosts by tag value.")

	err := searchCmd.MarkFlagRequired("nickname")
	if err != nil {
		return
	}

}
