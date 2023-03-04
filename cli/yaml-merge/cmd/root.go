/*
Copyright © 2023 Henry Pham minhhieu060799@gmail.com
*/
package cmd

import (
	"errors"
	"fmt"
	"gopkg.in/yaml.v3"
	"io"
	"os"

	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "yaml-merge",
	Short: "merge a yaml file with another",
	Long:  `When we run patch v20, it must merge ci.yml in the patches folder with ci.yml in the upstream keycloak folder and save the output result in the file with path releases/v20/latest/dev/.github/workflows/ci.yml.`,
	Args:  cobra.MaximumNArgs(0),
	// Uncomment the following line if your bare application
	// has an action associated with it:
	RunE: func(cmd *cobra.Command, args []string) error {
		mergedVersion, flagErr := cmd.Flags().GetString("version")
		if flagErr != nil {
			return flagErr
		}

		cwd, _ := os.Getwd()
		versionPath := fmt.Sprintf("%s/releases/%s/latest/patches", cwd, mergedVersion)
		targetPath := "./.github/workflows/ci.yml"
		sourcePath := fmt.Sprintf("%s/%s", versionPath, targetPath)

		if _, err := os.Stat(versionPath); os.IsNotExist(err) {
			return errors.New(fmt.Sprintf("version not found (%s)", versionPath))
		}

		sourceFile, err := readCIFile(sourcePath)
		if os.IsNotExist(err) {
			return errors.New(fmt.Sprintf("File not found (%s)", sourcePath))
		} else if err != nil {
			return err
		}

		if targetFile, err := readCIFile(targetPath); os.IsNotExist(err) {
			err := copyFile(sourcePath, targetPath)
			if err != nil {
				return err
			}
		} else {
			targetFile.merge(sourceFile)
			err := targetFile.writeCIFile(targetPath)
			if err != nil {
				return err
			}
		}
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

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return err
	}

	return nil
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
	rootCmd.Flags().StringP("version", "v", "", "Specify merged version")
}
