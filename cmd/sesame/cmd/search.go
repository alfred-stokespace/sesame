package cmd

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/spf13/cobra"
	"os"
	"runtime/debug"
)

type SesameError struct {
	msg         string
	reportStack bool
}

func (m *SesameError) Error() string {
	if m.reportStack {
		debug.PrintStack()
	}
	return m.msg
}

var nickname string
var tag string

// searchCmd represents the search command
var searchCmd = &cobra.Command{
	Use:   "search",
	Short: "search for a single host from a nickname you provide",
	Long: `If you have a "Nickname" tag on your host search using that.

If you don't have the default tag name then you can provide it.`,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			return fmt.Errorf("unexpected argument [%s]", args[0])
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		_, err := fmt.Fprintf(os.Stderr, "search called: [%s: %s]\n", tag, nickname)
		if err != nil {
		}
		if len(nickname) == 0 {
			exitOnError(fmt.Errorf("tag cannot be empty %s:%s", tag, nickname))
		}

		config := &aws.Config{Region: aws.String(getAwsRegion())}
		sess, err := session.NewSession()
		exitOnError(err)
		svc := ssm.New(sess, config)

		// Create our filter slice
		filters := []*ssm.InstanceInformationStringFilter{
			{
				Key:    aws.String(fmt.Sprintf("tag:%s", tag)),
				Values: aws.StringSlice([]string{nickname}),
			},
		}

		const ApiMin = 5
		maxRes := int64(ApiMin)
		input := &ssm.DescribeInstanceInformationInput{
			Filters:    filters,
			MaxResults: &maxRes,
		}
		res, serviceError := svc.DescribeInstanceInformation(input)
		exitOnError(serviceError)
		if len(res.InstanceInformationList) > 1 {
			exitOnError(&SesameError{msg: "Too many results for tag."})

		} else if len(res.InstanceInformationList) == 0 {
			exitOnError(&SesameError{msg: "No results for tag."})
		} else {
			for _, item := range res.InstanceInformationList {

				_, err := fmt.Fprintln(os.Stdout, *item.InstanceId)
				if err != nil {
				}
			}
		}
	},
}

func exitOnError(err error) {
	if err != nil {
		_, err := fmt.Fprintln(os.Stdout, err)
		if err != nil {
		}
		os.Exit(1)
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

	err := rootCmd.MarkFlagRequired("nickname")
	if err != nil {
		return
	}

}
