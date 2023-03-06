/*
Copyright © 2023 Henry Pham minhhieu060799@gmail.com
*/
package cmd

import (
	"errors"
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "yaml-merge",
	Short: "merge a yaml file with another",
	Long:  `When we run patch v20, it must merge ci.yml in the patches folder with ci.yml in the upstream keycloak folder and save the output result in the file with path releases/v20/latest/dev/.github/workflows/ci.yml.`,
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		const (
			releasesDir    = "./releases"
			latestDir      = "latest"
			patchesDir     = "patches"
			keycloakDir    = "keycloak"
			devDir         = "dev"
			workflowsDir   = ".github/workflows"
			ciYamlFilename = "ci.yml"
		)

		versionPath := fmt.Sprintf("%s/%s/%s/%s/%s/%s", releasesDir, args[0], latestDir, patchesDir, workflowsDir, ciYamlFilename)
		keycloakPath := fmt.Sprintf("%s/%s/%s/%s/%s/%s", releasesDir, args[0], latestDir, keycloakDir, workflowsDir, ciYamlFilename)
		targetPath := fmt.Sprintf("%s/%s/%s/%s/%s/%s", releasesDir, args[0], latestDir, devDir, workflowsDir, ciYamlFilename)

		if _, err := os.Stat(versionPath); os.IsNotExist(err) {
			return errors.New(fmt.Sprintf("Version not found (%s)", versionPath))
		}

		targetFile := CIFile{}
		for _, path := range []string{keycloakPath, versionPath} {
			file, err := readCIFile(path)
			if os.IsNotExist(err) {
				return fmt.Errorf("file not found (%s)", path)
			} else if err != nil {
				return err
			}
			targetFile.merge(file)
		}
		err := targetFile.writeCIFile(targetPath)
		if err != nil {
			return err
		}
		cmd.Println("Merge successfully")
		return nil
	},
}

func (f *CIFile) merge(srcFile CIFile) {
	f.Name = srcFile.Name
	f.RunName = srcFile.RunName
	f.On = srcFile.On

	if f.Jobs == nil {
		f.Jobs = srcFile.Jobs
	} else {
		for key, value := range srcFile.Jobs {
			if _, ok := f.Jobs[key]; !ok {
				f.Jobs[key] = value
			} else {
				for k, v := range value {
					f.Jobs[key][k] = v
				}
			}
		}
	}
}

type CIFile struct {
	Name    string                            `yaml:"name,omitempty"`
	RunName string                            `yaml:"run-name,omitempty"`
	On      map[string]map[string]interface{} `yaml:"on"`
	Jobs    map[string]map[string]interface{} `yaml:"jobs"`
}

func readCIFile(filePath string) (ciFile CIFile, err error) {
	fileBytes, err := os.ReadFile(filePath)
	if err != nil {
		return ciFile, err
	}
	parseErr := yaml.Unmarshal(fileBytes, &ciFile)
	if parseErr != nil {
		return CIFile{}, parseErr
	}
	return ciFile, nil
}

func (f *CIFile) writeCIFile(path string) error {
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
