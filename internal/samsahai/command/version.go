package command

import (
	"fmt"

	"github.com/agoda-com/samsahai/internal/samsahai"
	"github.com/spf13/cobra"
)

func getVersion() string {
	var branch = ""
	if samsahai.GitBranch != "master" {
		branch = fmt.Sprintf(" git:%s-%s%s branch:%s", samsahai.GitCommit, samsahai.GitState, branch, samsahai.GitBranch)
	}
	return fmt.Sprintf("v%s%s", samsahai.Version, branch)
}

func getAppName() string {
	return samsahai.AppName
}

func versionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "version",
		Aliases: []string{"v"},
		Short:   "show version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(samsahai.AppName, getVersion())
		},
	}
	return cmd
}
