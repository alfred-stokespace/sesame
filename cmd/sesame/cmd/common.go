package cmd

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/spf13/cobra"
	"os"
	"runtime/debug"
)

func ValidateArgsFunc() func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			return fmt.Errorf("unexpected argument [%s]", args[0])
		}
		return nil
	}
}

func exitOnError(err error) {
	if err != nil {
		_, err := fmt.Fprintln(os.Stdout, err)
		if err != nil {
			panic(err)
		}
		os.Exit(1)
	}
}

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

type SSMCommand struct {
	svc    *ssm.Client
	svcEc2 *ec2.Client
}

func (ssmCommand *SSMCommand) conf() {
	conf, err := config.LoadDefaultConfig(context.Background())
	exitOnError(err)
	ssmCommand.svc = ssm.NewFromConfig(conf)
	ssmCommand.svcEc2 = ec2.NewFromConfig(conf)
}

func (ssmCommand *SSMCommand) thingDo() {
	os.Exit(200)
}
