package cmd

import (
	"errors"
	"fmt"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	"log"
	"os"
	"path/filepath"
	"strings"
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

			// get upstream yaml path
			upstreamFile := strings.Replace(downstreamFile, downstreamFolder, upstreamFolder, 1)
			if _, err := os.Stat(upstreamFile); os.IsNotExist(err) {
				log.Println(fmt.Sprintf("File not found: %s", upstreamFile))
				continue
			}

			sourceFile, err := unmarshalYAMLFile(upstreamFile)
			if err != nil {
				log.Printf("Error parsing %q: %v", upstreamFile, err)
			}
			overrideFile, err := unmarshalYAMLFile(downstreamFile)
			if err != nil {
				log.Printf("Error parsing %q: %v", downstreamFile, err)
			}

			err = recursiveMerge(&overrideFile, &sourceFile)
			if err != nil {
				log.Printf("Error merging from %q to %q:\n %v", downstreamFile, upstreamFile, err)
			}

			targetPath := strings.Replace(downstreamFile, downstreamFolder, devFolder, 1)
			_, err = os.Stat(devFolder)
			if os.IsNotExist(err) {
				err = os.MkdirAll(devFolder, os.ModePerm)
				if err != nil {
					return err
				}
			}
			err = writeYamlNodeToFile(&sourceFile, targetPath)
			if err != nil {
				log.Printf("Error writing %q: %v", targetPath, err)
			}
		}
		return nil
	},
}

func nodesEqual(l, r *yaml.Node) bool {
	if l.Kind == yaml.ScalarNode && r.Kind == yaml.ScalarNode {
		return l.Value == r.Value
	}
	panic("equals on non-scalars not implemented!")
}

// recursiveMerge recursively merges two YAML nodes, keeping the order of the content in the "into" node. It checks if
// the two nodes are of the same kind, and if so, it merges the content of the "from" node into the "into" node by
// either appending it (if it's a sequence node) or merging it recursively (if it's a mapping node). If a key from the
// "from" node is not found in the "into" node, it is added to the end. If a different kind of node is encountered,
// an error is returned.
func recursiveMerge(from, into *yaml.Node) error {
	if from.Kind != into.Kind {
		return errors.New("cannot merge nodes of different kinds")
	}
	switch from.Kind {
	case yaml.MappingNode:
		for i := 0; i < len(from.Content); i += 2 {
			found := false
			for j := 0; j < len(into.Content); j += 2 {
				if nodesEqual(from.Content[i], into.Content[j]) {
					found = true
					if err := recursiveMerge(from.Content[i+1], into.Content[j+1]); err != nil {
						return errors.New("at key " + from.Content[i].Value + ": " + err.Error())
					}
					break
				}
			}
			if !found {
				into.Content = append(into.Content, from.Content[i:i+2]...)
			}
		}
	case yaml.SequenceNode:
		into.Content = append(into.Content, from.Content...)
	case yaml.DocumentNode:
		err := recursiveMerge(from.Content[0], into.Content[0])
		if err != nil {
			return err
		}
	default:
		return errors.New("can only merge mapping and sequence nodes")
	}
	return nil
}

func writeYamlNodeToFile(node *yaml.Node, filepath string) error {
	file, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Encode the YAML node to a []byte slice
	encodedYaml, err := yaml.Marshal(node)
	if err != nil {
		return err
	}

	// Write the encoded YAML to the file
	_, err = file.Write(encodedYaml)
	if err != nil {
		return err
	}

	return nil
}

func marshalYAMLFile(file *map[string]interface{}, path string) (err error) {
	data, err := yaml.Marshal(&file)
	if err != nil {
		return err
	}
	err = os.WriteFile(path, data, os.ModePerm)
	if err != nil {
		return err
	}
	return nil
}

// unmarshalYAMLFile reads the contents of a YAML file at the given file path and
// unmarshals the contents into a map of interface{} types.
func unmarshalYAMLFile(filePath string) (data yaml.Node, err error) {
	file, err := os.ReadFile(filePath)
	if err != nil {
		return yaml.Node{}, err
	}
	err = yaml.Unmarshal(file, &data)
	if err != nil {
		return yaml.Node{}, err
	}
	return data, nil
}

// findYAMLFiles recursively searches for YAML files in the given directory
// and its subdirectories, and returns a slice of file paths that match the
// ".yaml" or ".yml" file extension.
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

// mergeMaps merges the contents of two maps, with the values in the
// overrideFile taking precedence over those in the sourceFile.
func mergeMaps(sourceFile, overrideFile map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{})
	var keys []interface{}
	for k := range sourceFile {
		keys = append(keys, k)
	}
	for k, v := range sourceFile {
		out[k] = v
	}
	for k, v := range overrideFile {
		if v, ok := v.(map[string]interface{}); ok {
			if bv, ok := out[k]; ok {
				if bv, ok := bv.(map[string]interface{}); ok {
					out[k] = mergeMaps(bv, v)
					continue
				}
			}
		}
		out[k] = v
	}
	return out
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
