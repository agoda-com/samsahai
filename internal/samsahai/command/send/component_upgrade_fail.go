package send

import (
	"log"
	"strings"

	"github.com/agoda-com/samsahai/internal/samsahai/component"
	"github.com/agoda-com/samsahai/internal/samsahai/reporter"

	"github.com/spf13/cobra"
)

// componentUpgradeFailArgs defines arguments of component-upgrade-fail command
type componentUpgradeFailArgs struct {
	component    string
	image        string
	serviceOwner string
	issueType    string
	valuesFile   string
	ciURL        string
	logsURL      string
	errorURL     string
}

// upgradeFail is global var for binding value
var upgradeFail componentUpgradeFailArgs

func componentUpgradeFailCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "component-upgrade-fail",
		Short: "send component upgrade fail details",
		RunE:  sendComponentUpgradeFailCmd,
	}

	cmd.Flags().StringVar(&upgradeFail.component, "component", "", "Component name (Required)")
	cmd.Flags().StringVar(&upgradeFail.image, "image", "", "Image of component (Required)")
	cmd.Flags().StringVar(&upgradeFail.serviceOwner, "service-owner", "", "Service owner (Required)")
	cmd.Flags().StringVar(&upgradeFail.issueType, "issue-type", "", "Issue type")
	cmd.Flags().StringVar(&upgradeFail.valuesFile, "values-file", "", "Values file URL")
	cmd.Flags().StringVar(&upgradeFail.ciURL, "ci-url", "", "Build URL")
	cmd.Flags().StringVar(&upgradeFail.logsURL, "logs-url", "", "Logs URL")
	cmd.Flags().StringVar(&upgradeFail.errorURL, "error-url", "", "Error URL")

	if err := cmd.MarkFlagRequired("component"); err != nil {
		log.Fatalf("component argument not found [%v]", err)
	}
	if err := cmd.MarkFlagRequired("image"); err != nil {
		log.Fatalf("image argument not found [%v]", err)
	}
	if err := cmd.MarkFlagRequired("service-owner"); err != nil {
		log.Fatalf("service-owner argument not found [%v]", err)
	}

	return cmd
}

// sendComponentUpgradeFailCmd runs when command is executed
func sendComponentUpgradeFailCmd(cmd *cobra.Command, args []string) error {
	component, err := createComponent(upgradeFail.component, upgradeFail.image)
	if err != nil {
		return err
	}

	optIssueType := reporter.NewOptionIssueType(upgradeFail.issueType)
	optValuesFile := reporter.NewOptionValuesFileURL(upgradeFail.valuesFile)
	optCIURL := reporter.NewOptionCIURL(upgradeFail.ciURL)
	optLogsURL := reporter.NewOptionLogsURL(upgradeFail.logsURL)
	optErrorURL := reporter.NewOptionErrorURL(upgradeFail.errorURL)

	if slack.enabled {
		slackCli, err := newSlackReporter()
		if err != nil {
			return err
		}

		if err := sendComponentUpgradeFail(slackCli, component, optIssueType, optValuesFile, optCIURL, optLogsURL, optErrorURL); err != nil {
			return err
		}
	}

	return nil
}

func sendComponentUpgradeFail(r reporter.Reporter, component *component.Component, options ...reporter.Option) error {
	if err := r.SendComponentUpgradeFail(component, upgradeFail.serviceOwner, options...); err != nil {
		return err
	}

	return nil
}

func createComponent(name, image string) (*component.Component, error) {
	imageTag := strings.Split(image, ":")
	if len(imageTag) != 2 {
		return nil, ErrWrongImageFormat
	}

	return component.NewComponent(
		name,
		imageTag[1],
		component.NewOptionImage(&component.Image{Repository: imageTag[0], Tag: imageTag[1]}),
	)
}
