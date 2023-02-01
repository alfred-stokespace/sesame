package cmd

import (
	"bytes"
	"context"
	"fmt"
	"github.com/Heraclitus/sesame/cmd/sesame/automation"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/jroimartin/gocui"
	"github.com/madflojo/tasks"
	"github.com/spf13/cobra"
	"io"
	"log"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"
)

const centerViewName = "center"
const sideViewName = "side"
const mainViewName = "main"
const bottomViewName = "footer"
const ssmInvokeHelperShell = "github-based-automation-helper.sh"
const defaultGitBasedAutomation = "repo origin/a/branch/in/repo ssm-param-name-holding-gh-ssh-key \"who && ls -ltra && sleep 10\""

var (
	active     = 0
	firstFocus = true
	viewArr    = []string{sideViewName, mainViewName, bottomViewName, centerViewName}
)

var filterTag string
var filterTagName string
var filterTagValue string
var bestNameTag string
var automationDocumentName string
var helperBashFilePathAndName string
var automationParameterValues string
var gal = Gallery{SSMCommand{}, []UsefullyNamed{}, "", "", "", ""}
var sideSelectedNum = 0
var ssmExecutionItemNum = 0
var automationLibSearchPath = "./"
var automationLibs []automation.AutomationLib
var ssmAutomationParams = SSMAutomationParameters{"", "", "", ""}

type UsefullyNamed struct {
	InstanceId string
	Name       string
	Status     string
	TagList    []types.Tag
	Everything types.InstanceInformation
}

type Gallery struct {
	SSMCommand
	Instances        []UsefullyNamed
	TimeOfRetrieve   string
	openSsmSessionTo string
	trackomateOn     string
	ssmCommandString string
}

type SSMAutomationParameters struct {
	repoName          string
	cmd               string
	branchName        string
	ghSshKeyParamName string
}

func init() {

	gallerateCmd.Flags().StringVarP(&filterTag, "filterTag", "t", "", "Provide a Tag key:value to filter the gallery.")
	gallerateCmd.Flags().StringVarP(&bestNameTag, "bestNameTag", "n", "", "Provide a Tag key name that has the best value for a UI friendly name.")
	gallerateCmd.Flags().StringVarP(&automationDocumentName, "autodocname", "a", "", "Provide an ssm automation document name for use in commanding. OPTIONAL")
	gallerateCmd.Flags().StringVarP(&ssmAutomationParams.ghSshKeyParamName, "autosshparamname", "g", "", "Provide an ssm parameter name containing a GitHub SSH Key w/repo permissions. OPTIONAL")
	gallerateCmd.Flags().StringVarP(&automationLibSearchPath, "libsearchpath", "l", "./", "Provide a path to search for library automations. OPTIONAL")
	gallerateCmd.Flags().StringVarP(&helperBashFilePathAndName, "helperBash", "b", ssmInvokeHelperShell, "Provide a local full-or-relative path invocation helper script. OPTIONAL")
	gallerateCmd.Flags().StringVarP(&automationParameterValues, "autoParams", "p", defaultGitBasedAutomation, "Provide parameters to pass to the helperBash script. DEFAULT IS EXAMPLE ONLY!")
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

		g.SelFgColor = gocui.ColorGreen
		g.Cursor = true

		g.SetManagerFunc(layout)

		if err := g.SetKeybinding("", gocui.KeyTab, gocui.ModNone, nextView); err != nil {
			log.Panicln(err)
		}

		if err := g.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, cancelQuit); err != nil {
			log.Panicln(err)
		}

		if err := g.SetKeybinding("", gocui.KeyCtrlQ, gocui.ModNone, quit); err != nil {
			log.Panicln(err)
		}

		if err := g.SetKeybinding("", gocui.KeyCtrlR, gocui.ModNone, refresh); err != nil {
			log.Panicln(err)
		}

		if err := g.SetKeybinding("", gocui.KeyCtrlS, gocui.ModNone, ssmStart); err != nil {
			log.Panicln(err)
		}

		if err := g.SetKeybinding("", gocui.KeyCtrlM, gocui.ModNone, commandATarget); err != nil {
			log.Panicln(err)
		}

		if err := g.SetKeybinding(sideViewName, gocui.KeyArrowDown, gocui.ModNone, cursorDown); err != nil {
			log.Panicln(err)
		}

		if err := g.SetKeybinding(mainViewName, gocui.KeyArrowDown, gocui.ModNone, cursorDown); err != nil {
			log.Panicln(err)
		}

		if err := g.SetKeybinding(sideViewName, gocui.KeyArrowUp, gocui.ModNone, cursorUp); err != nil {
			log.Panicln(err)
		}

		if err := g.SetKeybinding(mainViewName, gocui.KeyArrowUp, gocui.ModNone, cursorUp); err != nil {
			log.Panicln(err)
		}

		if err := g.SetKeybinding(centerViewName, gocui.KeyArrowUp, gocui.ModNone, cursorUp); err != nil {
			log.Panicln(err)
		}

		if err := g.SetKeybinding(centerViewName, gocui.KeyArrowDown, gocui.ModNone, cursorDown); err != nil {
			log.Panicln(err)
		}

		if err := g.MainLoop(); err != nil && err != gocui.ErrQuit {
			log.Panicln(err)
		}

		// clear the screen todo: this doesn't work for windows
		fmt.Print("\033[H\033[2J")

		// Defer close will catch unexpected exit and preserve the containing shell/terminal
		// this explicit close is needed for the work below to be able to use stdout/err in a meaningful way.
		g.Close()

		// in this case the user exited, we should not start up again
		if gal.openSsmSessionTo != "" {
			fmt.Println("Attempting SSM Session open")
			const insideThisProjectsStandardDockerContainer = "/usr/local/bin/ssmcli"

			if _, err := os.Stat(insideThisProjectsStandardDockerContainer); os.IsNotExist(err) {
				cmd := exec.Command("/usr/local/bin/aws", "ssm", "start-session", "--target", fmt.Sprintf("%s", gal.openSsmSessionTo))
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				cmd.Stdin = os.Stdin
				if err := cmd.Run(); err != nil {
					log.Fatal(err)
				}
			} else {
				cmd := exec.Command(insideThisProjectsStandardDockerContainer, "start-session", "--instance-id", fmt.Sprintf("%s", gal.openSsmSessionTo))
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				cmd.Stdin = os.Stdin
				if err := cmd.Run(); err != nil {
					log.Fatal(err)
				}
			}
			gal.openSsmSessionTo = ""
		}
		if gal.trackomateOn != "" && gal.ssmCommandString != "" {
			fmt.Println("Attempting SSM Automation Execution")

			args := strings.SplitN(gal.ssmCommandString, "\"", 2)
			subArgs := strings.Split(args[0], " ")
			allArgs := []string{automationDocumentName, gal.trackomateOn}
			for _, str := range subArgs {
				if str != "" {
					allArgs = append(allArgs, str)
				}
			}
			allArgs = append(allArgs, "\""+strings.TrimSpace(args[1]))
			for i, arg := range allArgs {
				fmt.Printf("%d, arg: %s\n", i, arg)
			}

			cmd := exec.Command(helperBashFilePathAndName, allArgs...)
			var outb, errb bytes.Buffer
			cmd.Stdout = &outb
			cmd.Stderr = &errb
			if err := cmd.Run(); err != nil {
				log.Println("Gallerate-to-helper handoff failed!")
				log.Println("stdout: " + outb.String())
				log.Println("stderr: " + errb.String())
				log.Fatal(err)
			} else {
				ssmInvokeOut := outb.String()
				log.Println(ssmInvokeOut)
				log.Println("stderr: " + errb.String())

				ssmStartOut := strings.Split(ssmInvokeOut, " \"AutomationExecutionId\":")
				if len(ssmStartOut) != 2 {
					panic("Missing AutomationExecutionId, can't trackomate!")
				} else {
					searchForId := strings.Split(ssmStartOut[1], "\"")
					if len(searchForId) != 3 {
						panic("Missing AutomationExecutionId, can't trackomate!")
					} else {
						id := searchForId[1]
						reportChan := make(chan string, 10)
						t := Trackomate{SSMCommand{}, maxRecords, &reportChan, tasks.New(), "", "", id}
						t.conf()
						t.thingDo()
					}
				}
			}
		}
	},
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

	side, err := g.SetView(sideViewName, -1, -1, sideWidth, minusFooter)
	if err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		side.Highlight = true
		side.SelBgColor = gocui.ColorGreen
		side.SelFgColor = gocui.ColorBlack
		side.Editable = false
		_, printErr := fmt.Fprintf(side, "Initilizing...")
		if printErr != nil {
			_, _ = fmt.Fprintln(side, err.Error())
		}
	}
	if v, err := g.SetView(mainViewName, sideWidth, -1, maxX, minusFooter); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}

		v.Editable = false
		v.Wrap = true
		_, _ = fmt.Fprintf(v, "Initializing...")
	}
	footer, err := g.SetView(bottomViewName, -1, maxY-5, maxX, maxY)
	if err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}

		footer.Editable = false
		footer.Wrap = true
		_, _ = fmt.Fprintf(footer, "Initilizing...")
	}

	if firstFocus {
		if _, err := g.SetCurrentView(sideViewName); err != nil {
			return err
		}
		active++
		firstFocus = false
	}

	if len(gal.Instances) == 0 {
		go backGroundUpdate(g)
	}

	return nil
}

func (gallery *Gallery) thingDoWithTarget(g *gocui.Gui, inventoryView *gocui.View, footer *gocui.View) error {
	// Create our filter slice
	k := fmt.Sprintf("tag:%s", filterTagName)
	filters := []types.InstanceInformationStringFilter{
		{
			Key:    &k,
			Values: []string{filterTagValue},
		},
	}

	const ApiMin = 50
	maxRes := int32(ApiMin)
	input := &ssm.DescribeInstanceInformationInput{
		Filters:    filters,
		MaxResults: &maxRes,
	}

	pageNum := 0
	gallery.Instances = []UsefullyNamed{}
	pager := ssm.NewDescribeInstanceInformationPaginator(gallery.svc, input, func(o *ssm.DescribeInstanceInformationPaginatorOptions) {})

	for pager.HasMorePages() {
		page, err := pager.NextPage(context.Background())
		if err != nil {
			panic(err)
		}
		pageNum++
		gallery.TimeOfRetrieve = time.Now().String()
		for _, value := range page.InstanceInformationList {

			aNamedThing := UsefullyNamed{InstanceId: *value.InstanceId, Status: string(value.PingStatus), Everything: value}
			if value.ResourceType == types.ResourceTypeEc2Instance {
				if value.Name == nil {
					aNamedThing.Name = *value.InstanceId
				} else {
					aNamedThing.Name = *value.Name
				}
				aNamedThing.TagList = make([]types.Tag, 0)
				gallery.Instances = append(gallery.Instances, aNamedThing)
			} else if value.ResourceType == types.ResourceTypeManagedInstance {
				tagInput := &ssm.ListTagsForResourceInput{
					ResourceId:   value.InstanceId,
					ResourceType: types.ResourceTypeForTagging(value.ResourceType),
				}
				listTagsForResourceOutput, listTagServiceError := gallery.svc.ListTagsForResource(context.Background(), tagInput)
				if listTagServiceError != nil {
					panic(listTagServiceError)
				}

				aNamedThing.TagList = listTagsForResourceOutput.TagList
				for _, tagV := range listTagsForResourceOutput.TagList {
					if bestNameTag == *tagV.Key {
						aNamedThing.Name = *tagV.Value
					}
				}
				gallery.Instances = append(gallery.Instances, aNamedThing)
			}
		}
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
	inventoryView.Clear()
	for _, value := range gallery.Instances {

		deployLockedStatusSymbol := "○"
		for _, tagTuple := range value.TagList {
			if tagTuple.Key != nil && *tagTuple.Key == "DeployLocked" && tagTuple.Value != nil && *tagTuple.Value == "true" {
				deployLockedStatusSymbol = "◙"
			}
		}

		var pict = "\033[31;1m!\033[0m"
		if value.Status == "Online" {
			pict = "\033[32;1m^\033[0m"
		}
		pict = pict + deployLockedStatusSymbol
		_, err := fmt.Fprintln(inventoryView, pict+" "+value.InstanceId+"("+value.Name+")")
		if err != nil {
			panic(err)
		}
	}
	footer.Clear()
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
		_, _ = fmt.Fprintln(footer, "Ctrl+r => Refresh gallery | Ctrl+s => SSM Session Open | Ctrl+m/Enter => Command Target")
		_, _ = fmt.Fprintln(footer, "Ctrl+q => Quit            | Ctrl+c => Cancel/Quit      |")
		_, _ = fmt.Fprintln(footer, "Up ↑/Down ↓ => Navigate gallery")
	}
	return err
}

func cancelQuit(g *gocui.Gui, v *gocui.View) error {
	found, verr := g.View(centerViewName)
	// The center view doesn't exist and we want to quit with CtrlC
	if verr == gocui.ErrUnknownView && found == nil {
		return gocui.ErrQuit
	}
	// something else happened
	if verr != gocui.ErrUnknownView {
		exitOnError(verr)
	}

	// the center exists and we want to exit out of it
	if found != nil {
		_ = g.DeleteView(centerViewName)
	}
	_, err := g.SetCurrentView(sideViewName)
	if err != nil {
		return err
	}
	return nil
}

func quit(g *gocui.Gui, v *gocui.View) error {
	return gocui.ErrQuit
}

func cursorDown(g *gocui.Gui, v *gocui.View) error {
	if v != nil {
		cx, cy := v.Cursor()
		if err := v.SetCursor(cx, cy+1); err != nil {
			ox, oy := v.Origin()
			if err := v.SetOrigin(ox, oy+1); err != nil {
				return err
			}
		}
		if v.Name() == sideViewName {
			sideSelectedNum++
			return changeMainView(g, v)
		} else if v.Name() == centerViewName {
			ssmExecutionItemNum++
		}
	}
	return nil
}

func cursorUp(g *gocui.Gui, v *gocui.View) error {
	if v != nil {
		ox, oy := v.Origin()
		cx, cy := v.Cursor()
		if err := v.SetCursor(cx, cy-1); err != nil && oy > 0 {
			if err := v.SetOrigin(ox, oy-1); err != nil {
				return err
			}
		}
		if v.Name() == sideViewName {
			sideSelectedNum--
			return changeMainView(g, v)
		} else if v.Name() == centerViewName {
			ssmExecutionItemNum--
		}
	}
	return nil
}

func changeMainView(g *gocui.Gui, v *gocui.View) error {
	var b strings.Builder

	if v, err := g.View(mainViewName); err == nil {
		v.Clear()
		v.Autoscroll = true
		if sideSelectedNum >= len(gal.Instances) {
			return nil
		}
		if sideSelectedNum < 0 {
			sideSelectedNum = 0
		}
		_, _ = fmt.Fprintf(&b, "PING STATUS: %s (%s)\n", gal.Instances[sideSelectedNum].Status, gal.Instances[sideSelectedNum].Everything.LastPingDateTime)
		for _, t := range gal.Instances[sideSelectedNum].TagList {
			_, _ = fmt.Fprintf(&b, "%s:\t%s\n", *t.Key, *t.Value)
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
	if v, err := g.View(sideViewName); err == nil {
		v.Clear()
		_, _ = fmt.Fprintln(v, "Refreshing...")
		if f, err := g.View(bottomViewName); err == nil {
			f.Clear()
			_, _ = fmt.Fprintln(f, "Refreshing...")
			go backGroundUpdate(g)
		}
	} else {
		return err
	}
	return nil
}

func ssmStart(g *gocui.Gui, v *gocui.View) error {
	// Before we issue the quit command lets capture which host the user selected and prepare for ssm
	if sideSelectedNum >= len(gal.Instances) {
		return nil
	}
	if sideSelectedNum < 0 {
		sideSelectedNum = 0
	}
	gal.openSsmSessionTo = gal.Instances[sideSelectedNum].InstanceId

	return gocui.ErrQuit
}

func commandATarget(g *gocui.Gui, v *gocui.View) error {
	maxX, maxY := g.Size()
	found, verr := g.View(centerViewName)
	if verr == gocui.ErrUnknownView {
		if found == nil {
			// let's capture which host the user selected and prepare for commanding
			if sideSelectedNum >= len(gal.Instances) {
				return nil
			}
			if sideSelectedNum < 0 {
				sideSelectedNum = 0
			}
			gal.trackomateOn = gal.Instances[sideSelectedNum].InstanceId

			xStart := maxX / 10
			yStart := maxY / 3
			center, err := g.SetView(centerViewName, xStart, yStart, xStart*9, yStart+5)
			if err != nil {
				if err != gocui.ErrUnknownView {
					return err
				}
				center.Highlight = true
				center.Wrap = true
				center.SelBgColor = gocui.ColorGreen
				center.SelFgColor = gocui.ColorBlack
				center.Editable = true
				center.Title = fmt.Sprintf("GitBranch Command this [%s], Ctrl+m/Enter to execute.\n", gal.Instances[sideSelectedNum].Name)
				if automationDocumentName == "" {
					_, printErr := fmt.Fprintf(center, "ERROR: you didn't provide an automation document name argument on startup.")
					if printErr != nil {
						_, _ = fmt.Fprintln(center, err.Error())
					}
				} else {
					automationLibs = automation.GetListOfAutomationLibraries(center, automationLibSearchPath)
					_, printErr := fmt.Fprintf(center, automationParameterValues)
					if printErr != nil {
						_, _ = fmt.Fprintln(center, err.Error())
					}
				}
				_, _ = g.SetCurrentView(centerViewName)
			}
		}
	} else {
		if found != nil && automationDocumentName != "" {
			cmdBuffer := found.Buffer()
			lines := strings.SplitN(cmdBuffer, "\n", -1)
			lineChosen := lines[ssmExecutionItemNum]
			const documentNamePrefix = "> "
			if strings.HasPrefix(lineChosen, documentNamePrefix) {
				for _, lib := range automationLibs {
					selectionPortion := strings.Replace(lineChosen, documentNamePrefix, "", 1)
					if strings.HasPrefix(selectionPortion, lib.Metadata.Name) {
						for k, v := range lib.Metadata.Annotations {
							if strings.HasSuffix(k, "bash") {
								ssmAutomationParams.cmd = v
							} else if strings.HasSuffix(k, "automation-repo-name") {
								ssmAutomationParams.repoName = v
							} else if strings.HasSuffix(k, "automation-branch-name") {
								ssmAutomationParams.branchName = v
							} else if strings.HasSuffix(k, "automation-gh-ssm-param-name") {
								ssmAutomationParams.ghSshKeyParamName = v
							}
						}
						if ssmAutomationParams.repoName == "" || ssmAutomationParams.branchName == "" || ssmAutomationParams.cmd == "" || ssmAutomationParams.ghSshKeyParamName == "" {
							panic("Desired automation library is missing either repoName, branchName, ssmGHParamName or the command to execute")
						} else {
							gal.ssmCommandString = ssmAutomationParams.repoName + " " + ssmAutomationParams.branchName + " " + ssmAutomationParams.ghSshKeyParamName + " " + ssmAutomationParams.cmd
						}
					}
				}
			} else {
				gal.ssmCommandString = lineChosen
			}
			return gocui.ErrQuit
		}
		if automationDocumentName == "" {
			_ = g.DeleteView(centerViewName)
			_, _ = g.SetCurrentView(sideViewName)
		}
	}

	return nil
}

func backGroundUpdate(g *gocui.Gui) {
	g.Update(func(g *gocui.Gui) error {
		if v, err := g.View(sideViewName); err == nil {
			if f, err := g.View(bottomViewName); err == nil {
				_ = gal.thingDoWithTarget(g, v, f)
			}
		} else {
			return err
		}
		return nil
	})
}

func nextView(g *gocui.Gui, v *gocui.View) error {
	nextIndex := (active + 1) % len(viewArr)
	name := viewArr[nextIndex]

	currentView := g.CurrentView()

	g.Cursor = false
	currentView.Highlight = false
	_, err := g.View(name)
	if err != nil {
		if name != centerViewName {
			// optional modal
			return err
		}
	}

	if v, err := g.SetCurrentView(name); err != nil {
		if name != centerViewName {
			// optional modal
			return err
		}
	} else {
		v.Highlight = true
	}

	if name == sideViewName || name == centerViewName {
		g.Cursor = true
		g.Highlight = true
	} else {
		g.Cursor = false
	}

	active = nextIndex
	return nil
}
