package cmd

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/jroimartin/gocui"
	"github.com/spf13/cobra"
	"io"
	"log"
	"sort"
	"strconv"
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

		g.Cursor = true

		g.SetManagerFunc(layout)

		if err := g.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, quit); err != nil {
			log.Panicln(err)
		}

		if err := g.SetKeybinding("side", gocui.KeyArrowDown, gocui.ModNone, cursorDown); err != nil {
			log.Panicln(err)
		}
		if err := g.SetKeybinding("side", gocui.KeyArrowUp, gocui.ModNone, cursorUp); err != nil {
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
var bestNameTag string
var gal = Gallery{SSMCommand{}, []UsefullyNamed{}}
var sideSelectedNum = 0

type UsefullyNamed struct {
	InstanceId string
	Name       string
	Status     string
	TagList    []*ssm.Tag
}

type Gallery struct {
	SSMCommand
	instances []UsefullyNamed
}

func init() {

	gallerateCmd.Flags().StringVarP(&filterTag, "filterTag", "t", "", "Provide a Tag key:value to filter the gallery.")
	gallerateCmd.Flags().StringVarP(&bestNameTag, "bestNameTag", "n", "", "Provide a Tag key name that has the best value for a UI friendly name.")
	err := gallerateCmd.MarkFlagRequired("filterTag")
	if err != nil {
		exitOnError(err)
	}
	err = gallerateCmd.MarkFlagRequired("bestNameTag")
	if err != nil {
		exitOnError(err)
	}
	rootCmd.AddCommand(gallerateCmd)
	gal.conf()
}

func (gallery *Gallery) thingDoWithTarget(ior io.ReadWriter, ior2 io.ReadWriter) error {
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

	pageNum := 0
	gallery.instances = []UsefullyNamed{}
	serviceError := gallery.svc.DescribeInstanceInformationPages(input, func(page *ssm.DescribeInstanceInformationOutput, lastPage bool) bool {
		pageNum++
		for _, value := range page.InstanceInformationList {

			tagInput := &ssm.ListTagsForResourceInput{
				ResourceId:   value.InstanceId,
				ResourceType: value.ResourceType,
			}
			listTagsForResourceOutput, listTagServiceError := gallery.svc.ListTagsForResource(tagInput)
			if listTagServiceError != nil {
				panic(listTagServiceError)
			}

			aNamedThing := UsefullyNamed{InstanceId: *value.InstanceId}

			aNamedThing.TagList = listTagsForResourceOutput.TagList
			for _, tagV := range listTagsForResourceOutput.TagList {
				if bestNameTag == *tagV.Key {
					aNamedThing.Name = *tagV.Value
				}
			}
			gallery.instances = append(gallery.instances, aNamedThing)
		}
		return !lastPage
	})
	if serviceError != nil {
		return serviceError
	}

	if pageNum == 0 {
		return &SesameError{msg: "No results for tag filter."}
	}

	sort.Slice(gallery.instances, func(i, j int) bool {
		if gallery.instances[i].Name == "" && gallery.instances[j].Name != "" {
			return false
		} else if gallery.instances[i].Name != "" && gallery.instances[j].Name == "" {
			return true
		} else if gallery.instances[i].Name == "" && gallery.instances[j].Name == "" {
			return true
		}

		return gallery.instances[i].Name < gallery.instances[j].Name
	})
	for _, value := range gallery.instances {
		_, err := fmt.Fprintln(ior, value.InstanceId+"("+value.Name+")")
		if err != nil {
			panic(err)
		}
	}
	_, err := fmt.Fprintln(ior2, "Total instance count: "+strconv.Itoa(len(gallery.instances)))
	if err != nil {
		panic(err)
	}
	return nil
}

func layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()
	footerHeight := 5
	minusFooter := maxY - footerHeight
	if minusFooter < 0 {
		minusFooter = maxY
	}
	const sideWidth = 40
	side, err := g.SetView("side", -1, -1, sideWidth, minusFooter)
	if err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		side.Highlight = true
		side.SelBgColor = gocui.ColorGreen
		side.SelFgColor = gocui.ColorBlack
		side.Editable = false
		if err != nil {
			fmt.Fprintln(side, err.Error())
		}
	}
	if v, err := g.SetView("main", sideWidth, -1, maxX, minusFooter); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}

		fmt.Fprintf(v, "%s", "Hello")
		v.Editable = false
		v.Wrap = true
	}
	footer, err := g.SetView("footer", -1, maxY-5, maxX, maxY)
	if err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}

		fmt.Fprintf(footer, "%s", "Footer")
		footer.Editable = false
		footer.Wrap = true
	}
	if _, err := g.SetCurrentView("side"); err != nil {
		return err
	}

	if len(gal.instances) == 0 {
		return gal.thingDoWithTarget(side, footer)
	}

	return nil
}

func quit(g *gocui.Gui, v *gocui.View) error {
	return gocui.ErrQuit
}

func cursorDown(g *gocui.Gui, v *gocui.View) error {
	sideSelectedNum++
	if v != nil {
		cx, cy := v.Cursor()
		if err := v.SetCursor(cx, cy+1); err != nil {
			ox, oy := v.Origin()
			if err := v.SetOrigin(ox, oy+1); err != nil {
				return err
			}
		}
	}
	return nil
}

func cursorUp(g *gocui.Gui, v *gocui.View) error {
	sideSelectedNum--
	if v != nil {
		ox, oy := v.Origin()
		cx, cy := v.Cursor()
		if err := v.SetCursor(cx, cy-1); err != nil && oy > 0 {
			if err := v.SetOrigin(ox, oy-1); err != nil {
				return err
			}
		}
	}
	return nil
}
