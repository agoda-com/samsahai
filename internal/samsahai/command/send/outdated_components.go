package send

import (
	"log"

	"github.com/agoda-com/samsahai/internal/samsahai/component"
	"github.com/agoda-com/samsahai/internal/samsahai/reporter"
	"github.com/spf13/cobra"
)

// outdatedComponentsArgs defines arguments of outdated-components command
type outdatedComponentsArgs struct {
	currentActiveValuesPath string
	newValuesPath           string
}

// outdatedComponents is global var for binding value
var outdatedComponents outdatedComponentsArgs

func outdatedComponentsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "outdated-components",
		Short: "send outdated components list",
		RunE:  sendOutdatedComponentCmd,
	}

	cmd.Flags().StringVar(&outdatedComponents.currentActiveValuesPath, "current-active-values-file", "", "Current active values file (Required)")
	cmd.Flags().StringVar(&outdatedComponents.newValuesPath, "new-values-file", "", "New values file (Required)")

	if err := cmd.MarkFlagRequired("current-active-values-file"); err != nil {
		log.Fatalf("current-active-values-file argument not found [%v]", err)
	}
	if err := cmd.MarkFlagRequired("new-values-file"); err != nil {
		log.Fatalf("new-values-file argument not found [%v]", err)
	}

	return cmd
}

// sendOutdatedComponentCmd runs when command is executed
func sendOutdatedComponentCmd(cmd *cobra.Command, args []string) error {
	components, err := getActiveComponentsFromValuesFile(outdatedComponents.currentActiveValuesPath, outdatedComponents.newValuesPath)
	if err != nil {
		return err
	}

	opts := []reporter.Option{
		reporter.NewOptionSubject(email.subject),
	}

	for _, r := range reporters {
		if err := sendOutdatedComponents(r, components, opts...); err != nil {
			return err
		}
	}

	return nil
}

func sendOutdatedComponents(r reporter.Reporter, components []component.OutdatedComponent, options ...reporter.Option) error {
	if err := r.SendOutdatedComponents(components, options...); err != nil {
		return err
	}

	return nil
}
