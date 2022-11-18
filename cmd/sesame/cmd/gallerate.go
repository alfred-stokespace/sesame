package cmd

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/jroimartin/gocui"
	"github.com/spf13/cobra"
	"io"
	"log"
	"strings"
)

var gallerateCmd = &cobra.Command{
	Use:   "gallerate",
	Short: "Walk through SSM like it was a gallery",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {

		if strings.Contains(filterTag, ":") || strings.HasSuffix(filterTag, ":") || strings.HasPrefix(filterTag, ":") {
			var parts = strings.Split(filterTag, ":")
			filterTagName = parts[0]
			filterTagValue = parts[1]
		} else {
			exitOnError(&SesameError{msg: "filterTag needs to be tagName:tagValue, e.g. CostCenter:FunTeam\nYou provided [" + filterTag + "]"})
		}

		g, err := gocui.NewGui(gocui.OutputNormal)
		if err != nil {
			log.Panicln(err)
		}
		defer g.Close()

		g.SetManagerFunc(layout)

		if err := g.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, quit); err != nil {
			log.Panicln(err)
		}

		if err := g.MainLoop(); err != nil && err != gocui.ErrQuit {
			log.Panicln(err)
		}
	},
}

var filterTag string
var filterTagName string
var filterTagValue string

type Gallery struct {
	SSMCommand
}

func init() {

	gallerateCmd.Flags().StringVarP(&filterTag, "filterTag", "t", "", "Provide a Tag key:value to filter the gallery.")
	err := gallerateCmd.MarkFlagRequired("filterTag")
	if err != nil {
		exitOnError(err)
	}
	rootCmd.AddCommand(gallerateCmd)
}

func (gallery *Gallery) thingDoWithTarget(ior io.ReadWriter) error {
	// Create our filter slice
	filters := []*ssm.InstanceInformationStringFilter{
		{
			Key:    aws.String(fmt.Sprintf("tag:%s", filterTagName)),
			Values: aws.StringSlice([]string{filterTagValue}),
		},
	}

	const ApiMin = 50
	maxRes := int64(ApiMin)
	input := &ssm.DescribeInstanceInformationInput{
		Filters:    filters,
		MaxResults: &maxRes,
	}
	res, serviceError := gallery.svc.DescribeInstanceInformation(input)
	if serviceError != nil {
		return serviceError
	}

	if len(res.InstanceInformationList) == 0 {
		return &SesameError{msg: "No results for tag filter."}
	} else {
		for _, item := range res.InstanceInformationList {

			_, err := fmt.Fprintln(ior, *item.InstanceId)
			if err != nil {
				panic(err)
			}
		}
	}
	return nil
}

func layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()
	if v, err := g.SetView("side", -1, -1, 30, maxY); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Highlight = true
		v.SelBgColor = gocui.ColorGreen
		v.SelFgColor = gocui.ColorBlack

		g := Gallery{SSMCommand{}}
		g.conf()
		err = g.thingDoWithTarget(v)
		if err != nil {
			fmt.Fprintln(v, err.Error())
		}
	}
	if v, err := g.SetView("main", 30, -1, maxX, maxY); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}

		fmt.Fprintf(v, "%s", "Hello")
		v.Editable = false
		v.Wrap = true
		if _, err := g.SetCurrentView("main"); err != nil {
			return err
		}
	}
	return nil
}

func quit(g *gocui.Gui, v *gocui.View) error {
	return gocui.ErrQuit
}
