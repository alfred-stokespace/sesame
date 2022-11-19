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
	"strings"
	"time"
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

		if err := g.SetKeybinding("", gocui.KeyCtrlQ, gocui.ModNone, quit); err != nil {
			log.Panicln(err)
		}

		if err := g.SetKeybinding("", gocui.KeyCtrlR, gocui.ModNone, refresh); err != nil {
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
var gal = Gallery{SSMCommand{}, []UsefullyNamed{}, ""}
var sideSelectedNum = 0

type UsefullyNamed struct {
	InstanceId string
	Name       string
	Status     string
	TagList    []*ssm.Tag
	Everything ssm.InstanceInformation
}

type Gallery struct {
	SSMCommand
	Instances      []UsefullyNamed
	TimeOfRetrieve string
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

func (gallery *Gallery) thingDoWithTarget(g *gocui.Gui, inventoryView io.ReadWriter, footer io.ReadWriter) error {
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
	gallery.Instances = []UsefullyNamed{}
	serviceError := gallery.svc.DescribeInstanceInformationPages(input, func(page *ssm.DescribeInstanceInformationOutput, lastPage bool) bool {
		pageNum++
		gallery.TimeOfRetrieve = time.Now().String()
		for _, value := range page.InstanceInformationList {

			tagInput := &ssm.ListTagsForResourceInput{
				ResourceId:   value.InstanceId,
				ResourceType: value.ResourceType,
			}
			listTagsForResourceOutput, listTagServiceError := gallery.svc.ListTagsForResource(tagInput)
			if listTagServiceError != nil {
				panic(listTagServiceError)
			}

			aNamedThing := UsefullyNamed{InstanceId: *value.InstanceId, Status: *value.PingStatus, Everything: *value}

			aNamedThing.TagList = listTagsForResourceOutput.TagList
			for _, tagV := range listTagsForResourceOutput.TagList {
				if bestNameTag == *tagV.Key {
					aNamedThing.Name = *tagV.Value
				}
			}
			gallery.Instances = append(gallery.Instances, aNamedThing)
		}
		return !lastPage
	})
	if serviceError != nil {
		return serviceError
	}

	if pageNum == 0 {
		return &SesameError{msg: "No results for tag filter."}
	}

	sort.Slice(gallery.Instances, func(i, j int) bool {
		if gallery.Instances[i].Name == "" && gallery.Instances[j].Name != "" {
			return false
		} else if gallery.Instances[i].Name != "" && gallery.Instances[j].Name == "" {
			return true
		} else if gallery.Instances[i].Name == "" && gallery.Instances[j].Name == "" {
			return true
		}

		return gallery.Instances[i].Name < gallery.Instances[j].Name
	})
	for _, value := range gallery.Instances {

		var pict = "\033[31;1m!\033[0m"
		if value.Status == "Online" {
			pict = "\033[32;1m^\033[0m"
		}
		_, err := fmt.Fprintln(inventoryView, pict+" "+value.InstanceId+"("+value.Name+")")
		if err != nil {
			panic(err)
		}
	}
	err := gallery.printFooter(footer)
	if err != nil {
		panic(err)
	}
	err = changeMainView(g, nil)
	if err != nil {
		panic(err)
	}
	return nil
}

func (gallery *Gallery) printFooter(footer io.ReadWriter) error {
	_, err := fmt.Fprintf(footer, "Total instance count: %d @(%s)\n", len(gallery.Instances), gallery.TimeOfRetrieve)
	if err == nil {
		fmt.Fprintln(footer, "Ctrl+R => Refresh gallery")
		fmt.Fprintln(footer, "Ctrl+Q => Quit")
		fmt.Fprintln(footer, "UpArrow/DownArrow => Navigate gallery up/down")
	}
	return err
}

func layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()
	const sideWidth = 40
	if maxX <= 0 {
		maxX = sideWidth * 2
	}
	if maxY <= 0 {
		maxY = maxX
	}
	footerHeight := 5
	minusFooter := maxY - footerHeight
	if minusFooter < 0 {
		minusFooter = maxY
	}

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

		v.Editable = false
		v.Wrap = true
	}
	footer, err := g.SetView("footer", -1, maxY-5, maxX, maxY)
	if err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}

		footer.Editable = false
		footer.Wrap = true
	}
	if _, err := g.SetCurrentView("side"); err != nil {
		return err
	}

	if len(gal.Instances) == 0 {
		side.Clear()
		return gal.thingDoWithTarget(g, side, footer)
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
		return changeMainView(g, v)
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
		return changeMainView(g, v)
	}
	return nil
}

func changeMainView(g *gocui.Gui, v *gocui.View) error {
	var b strings.Builder

	if v, err := g.View("main"); err == nil {
		v.Clear()
		if sideSelectedNum >= len(gal.Instances) {
			return nil
		}
		if sideSelectedNum < 0 {
			sideSelectedNum = 0
		}
		fmt.Fprintf(&b, "PING STATUS: %s (%s)\n", gal.Instances[sideSelectedNum].Status, gal.Instances[sideSelectedNum].Everything.LastPingDateTime)
		for _, t := range gal.Instances[sideSelectedNum].TagList {
			fmt.Fprintf(&b, "%s:\t%s\n", *t.Key, *t.Value)
		}
		_, erri := fmt.Fprintf(v, "%s", b.String())
		if erri == nil {
			return erri
		}
	} else {
		return err
	}
	return nil
}

func refresh(g *gocui.Gui, v *gocui.View) error {
	if v, err := g.View("side"); err == nil {
		v.Clear()
		if f, err := g.View("footer"); err == nil {
			f.Clear()
			return gal.thingDoWithTarget(g, v, f)
		}
	} else {
		return err
	}
	return nil
}
