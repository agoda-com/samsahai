package compare

import "github.com/spf13/cobra"

func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "compare",
		Short: "compare [version-components]",
	}

	cmd.AddCommand(versionComponentsCmd())
	return cmd
}
