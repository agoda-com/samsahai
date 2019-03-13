package command

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var SamsahaiCmd = &cobra.Command{
	Use:   "samsahai",
	Short: "Samsahai is a command line for create Blue/Green enviroment",
	Long:  "Samsahai is a command line for create Blue/Green enviroment",
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
}
