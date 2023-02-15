package automation

import (
	"errors"
	"fmt"
	"github.com/jroimartin/gocui"
	"gopkg.in/yaml.v3"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
)

const SsmDocType = "ssm-doc-type"
const BashType = "bash-type"

type AutomationLib struct {
	Metadata struct {
		Name        string
		Description string
		Labels      map[string]string `yaml:"labels"`
		Annotations map[string]string `yaml:"annotations"`
	}
}

func GetListOfAutomationLibraries(center *gocui.View, directoryToSearch string) []AutomationLib {
	files, err := ioutil.ReadDir(directoryToSearch)
	if err != nil {
		log.Println(err)
		return []AutomationLib{}
	}
	var libs []AutomationLib
	for _, file := range files {
		fileName := file.Name()
		if strings.HasSuffix(fileName, ".yaml") || strings.HasSuffix(fileName, ".yml") {
			f, err := os.Open(filepath.Join(directoryToSearch, fileName))
			if err != nil {
				log.Println(err)
			}
			dec := yaml.NewDecoder(f)
			for {
				lib := AutomationLib{}
				decErr := dec.Decode(&lib)
				if decErr != nil {
					if errors.Is(decErr, io.EOF) {
						break
					} else {
						log.Println(decErr)
						continue
					}
				}
				if lib.Metadata.Annotations != nil && lib.Metadata.Labels != nil {
					grabIt := false
					for labelK, labelV := range lib.Metadata.Labels {
						if strings.HasSuffix(labelK, "automation") {
							if strings.HasSuffix(labelV, BashType) {
								for annoK := range lib.Metadata.Annotations {
									if strings.HasSuffix(annoK, "bash") {
										fmt.Fprintf(center, "> %s: \"%s\"\n", lib.Metadata.Name, lib.Metadata.Description)
										grabIt = true
									}
								}
							} else if strings.HasSuffix(labelV, SsmDocType) {
								for annoK := range lib.Metadata.Annotations {
									if strings.Contains(annoK, "automation-doc-name") {
										fmt.Fprintf(center, "> %s: \"%s\"\n", lib.Metadata.Name, lib.Metadata.Description)
										grabIt = true
									}
								}
							}
						}
					}
					if grabIt {
						libs = append(libs, lib)
					}
				}
			}

			if err != nil {
				log.Println(err)
			}
		}
	}
	return libs
}
