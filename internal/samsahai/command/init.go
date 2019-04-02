package command

import (
	"fmt"
	"os"

	"github.com/agoda-com/samsahai/internal/samsahai/command/send"

	"github.com/spf13/cobra"
)

// SamsahaiCmd is the base command
var SamsahaiCmd = &cobra.Command{
	Use:   "samsahai",
	Short: "Samsahai is a command line for create Blue/Green environment",
	Long:  "Samsahai is a command line for create Blue/Green environment",
}

// Execute executes the current command.
func Execute() {
	if err := SamsahaiCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	addCommands()
}

func addCommands() {
	SamsahaiCmd.AddCommand(versionCmd())
	SamsahaiCmd.AddCommand(send.Cmd())
}
