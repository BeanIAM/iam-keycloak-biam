package cmd

//import (
//	"bytes"
//	"testing"
//
//	"github.com/spf13/cobra"
//	"github.com/stretchr/testify/assert"
//)
//
//func TestHelloCmd(t *testing.T) {
//	// Create a new Cobra command
//	cmd := &cobra.Command{
//		Use: "hello",
//		Run: func(cmd *cobra.Command, args []string) {
//			// This is where the command logic would go
//			name, _ := cmd.Flags().GetString("name")
//			greeting := "Hello, " + name + "!"
//			cmd.Print(greeting)
//		},
//	}
//
//	// Set up a buffer to capture the command output
//	buf := new(bytes.Buffer)
//	cmd.SetOutput(buf)
//
//	// Test the command with a "world" argument
//	cmd.SetArgs([]string{"--name", "world"})
//	err := cmd.Execute()
//	assert.NoError(t, err)
//	assert.Equal(t, "Hello, world!\n", buf.String())
//
//	// Test the command with a "Alice" argument
//	buf.Reset()
//	cmd.SetArgs([]string{"--name", "Alice"})
//	err = cmd.Execute()
//	assert.NoError(t, err)
//	assert.Equal(t, "Hello, Alice!\n", buf.String())
//}
