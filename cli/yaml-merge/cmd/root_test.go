package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestYamlMergeCommand(t *testing.T) {
	// GIVEN
	// Create a temporary test directory
	const version = "v20"
	tmpDir := filepath.Join(ReleasesDir, version, LatestDir)
	err := os.MkdirAll(tmpDir, os.ModePerm)
	assert.NoError(t, err)
	defer os.RemoveAll("releases")

	// Create test files
	downstreamFile := filepath.Join(tmpDir, PatchesDir, "ci.yml")
	upstreamFile := filepath.Join(tmpDir, KeycloakDir, "ci.yml")
	devFile := filepath.Join(tmpDir, "dev/ci.yml")
	err = os.MkdirAll(filepath.Dir(downstreamFile), os.ModePerm)
	assert.NoError(t, err)
	err = os.MkdirAll(filepath.Dir(upstreamFile), os.ModePerm)
	assert.NoError(t, err)
	//err = os.MkdirAll(filepath.Dir(devFile), os.ModePerm)
	//assert.NoError(t, err)
	os.WriteFile(downstreamFile, []byte("on: {}\njobs:\n  build: {}\n"), os.ModePerm)
	os.WriteFile(upstreamFile, []byte("jobs:\n  test: {}\n"), os.ModePerm)

	// Run the command
	args := []string{version}
	rootCmd.SetArgs(args)
	err = rootCmd.RunE(nil, args)
	assert.NoError(t, err)

	// Assert that the dev file was written correctly
	devData, err := os.ReadFile(devFile)
	assert.NoError(t, err)
	expectedDevData := "\"on\": {}\njobs:\n    build: {}\n    test: {}\n"
	assert.Equal(t, expectedDevData, string(devData))
}
