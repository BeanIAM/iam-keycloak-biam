package cmd

import (
	"errors"
	"fmt"
	"gopkg.in/yaml.v3"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

const (
	ReleasesDir = "releases"
	LatestDir   = "latest"
	PatchesDir  = "patches"
	KeycloakDir = "keycloak"
	DevDir      = "dev"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "yaml-merge",
	Short: "merge a yaml file with another",
	Long:  `When we run patch v20, it must merge ci.yml in the patches folder with ci.yml in the upstream keycloak folder and save the output result in the file with path releases/v20/latest/dev/.github/workflows/ci.yml.`,
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		versionPath := fmt.Sprintf("%s/%s", ReleasesDir, args[0])
		if _, err := os.Stat(versionPath); os.IsNotExist(err) {
			return errors.New(fmt.Sprintf("Version not found: %s", versionPath))
		}

		// args[0]: given version
		downstreamFolder := fmt.Sprintf("%s/%s/%s/%s", ReleasesDir, args[0], LatestDir, PatchesDir)
		upstreamFolder := fmt.Sprintf("%s/%s/%s/%s", ReleasesDir, args[0], LatestDir, KeycloakDir)
		devFolder := fmt.Sprintf("%s/%s/%s/%s", ReleasesDir, args[0], LatestDir, DevDir)

		downstreamFiles, err := findYAMLFiles(downstreamFolder)
		if err != nil {
			return nil
		}

		errorFile, err := os.OpenFile("error.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			log.Fatalf("Error: %s", err)
		}
		defer errorFile.Close()
		log.SetOutput(errorFile)

		for _, downstreamFile := range downstreamFiles {
			fmt.Printf("Merging file %s", downstreamFile)

			// Read upstream yaml file
			upstreamFile := strings.Replace(downstreamFile, downstreamFolder, upstreamFolder, 1)
			if _, err := os.Stat(upstreamFile); os.IsNotExist(err) {
				log.Println(fmt.Sprintf("File not found: %s", upstreamFile))
				continue
			}

			// read yaml files
			sourceFile, err := readWorkflowFile(upstreamFile)
			if err != nil {
				errMsg := fmt.Sprintf("Failed to read file %q: %v", upstreamFile, err)
				log.Println(errMsg)
				fmt.Println(errMsg)
				continue
			}
			targetFile, err := readWorkflowFile(downstreamFile)
			if err != nil {
				errMsg := fmt.Sprintf("Failed to read file %q: %v", upstreamFile, err)
				log.Println(errMsg)
				fmt.Println(errMsg)
				continue
			}
			sourceFile.merge(targetFile)

			devFile := strings.Replace(downstreamFile, downstreamFolder, devFolder, 1)
			err = sourceFile.writeCIFile(devFile)
			if err != nil {
				log.Printf("Failed to write file %q: %v", devFile, err)
			}
		}
		return nil
	},
}

func findYAMLFiles(dir string) ([]string, error) {
	var yamlFiles []string

	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if file.IsDir() {
			subdir := filepath.Join(dir, file.Name())
			subdirYAMLFiles, err := findYAMLFiles(subdir)
			if err != nil {
				return nil, err
			}
			yamlFiles = append(yamlFiles, subdirYAMLFiles...)
		} else {
			fileName := filepath.Ext(file.Name())
			if fileName == ".yaml" || fileName == ".yml" {
				yamlFiles = append(yamlFiles, filepath.Join(dir, file.Name()))
			}
		}
	}

	return yamlFiles, nil
}

func (f *Workflow) merge(targetFile Workflow) error {
	if targetFile.Name != "" {
		f.Name = targetFile.Name
	}

	if targetFile.RunName != "" {
		f.RunName = targetFile.RunName
	}
	//f.On = targetFile.On

	if f.Jobs == nil {
		f.Jobs = targetFile.Jobs
	} else {
		for key, value := range targetFile.Jobs {
			if _, ok := f.Jobs[key]; !ok {
				f.Jobs[key] = value
			} else {
				for k, v := range value {
					f.Jobs[key][k] = v
				}
			}
		}
	}
	return nil
}

type Workflow struct {
	Name    string                            `yaml:"name,omitempty"`
	RunName string                            `yaml:"run-name,omitempty"`
	On      map[string]interface{}            `yaml:"on"`
	Jobs    map[string]map[string]interface{} `yaml:"jobs"`
}

func readWorkflowFile(filePath string) (ciFile Workflow, err error) {
	fileBytes, err := os.ReadFile(filePath)
	if err != nil {
		return ciFile, err
	}
	parseErr := yaml.Unmarshal(fileBytes, &ciFile)
	if parseErr != nil {
		return ciFile, parseErr
	}
	return ciFile, nil
}

func (f *Workflow) writeCIFile(path string) error {
	data, err := yaml.Marshal(f)
	if err != nil {
		return err
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
	}
	err = os.WriteFile(path, data, 0644)
	if err != nil {
		return err
	}
	return nil
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.yaml-merge.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	//rootCmd.Flags().StringP("version", "v", "", "Specify merged version")
}
