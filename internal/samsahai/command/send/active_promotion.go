package send

import (
	"io/ioutil"
	"log"

	"github.com/agoda-com/samsahai/internal/samsahai/reporter"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	"github.com/agoda-com/samsahai/internal/samsahai/component"
	"github.com/agoda-com/samsahai/internal/samsahai/namespace/active"
	s "github.com/agoda-com/samsahai/internal/samsahai/reporter/slack"
)

type activePromotionArgs struct {
	status                  string
	currentActiveNamespace  string
	currentActiveValuesPath string
	newValuesPath           string
	serviceOwner            string
	showedDetail            bool
}

var atvPromotion activePromotionArgs

func activePromotionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "active-promotion",
		Short: "send active promotion status",
		RunE:  sendActivePromotionStatusCmd,
	}

	cmd.Flags().StringVar(&atvPromotion.status, "status", "", "Active promotion status [success/fail] (Required)")
	cmd.Flags().StringVar(&atvPromotion.currentActiveNamespace, "current-active-namespace", "", "Current active namespace (Required)")
	cmd.Flags().StringVar(&atvPromotion.currentActiveValuesPath, "current-active-values-file", "", "Current active values file")
	cmd.Flags().StringVar(&atvPromotion.newValuesPath, "new-values-file", "", "New values file")
	cmd.Flags().StringVar(&atvPromotion.serviceOwner, "service-owner", "", "Service owner (Required)")
	cmd.Flags().BoolVar(&atvPromotion.showedDetail, "show-detail", false, "Showed all current component details")

	if err := cmd.MarkFlagRequired("status"); err != nil {
		log.Fatalf("status argument not found [%v]", err)
	}
	if err := cmd.MarkFlagRequired("current-active-namespace"); err != nil {
		log.Fatalf("current-active-namespace argument not found [%v]", err)
	}
	if err := cmd.MarkFlagRequired("service-owner"); err != nil {
		log.Fatalf("service-owner argument not found [%v]", err)
	}

	return cmd
}

func sendActivePromotionStatusCmd(cmd *cobra.Command, args []string) error {
	if slack.enabled {
		if validated, err := validateSlackArgs(&slack); !validated {
			return err
		}

		components, err := getActiveComponentsFromValuesFile(atvPromotion.currentActiveValuesPath, atvPromotion.newValuesPath)
		if err != nil {
			return err
		}

		channels := getSlackChannels(slack.channels)
		slackCli := s.NewSlack(slack.accessToken, slack.username, channels)

		if err := sendActivePromotionStatus(slackCli, components); err != nil {
			return err
		}
	}

	return nil
}

func getActiveComponentsFromValuesFile(currentActiveValuesPath, newValuesPath string) ([]component.Component, error) {
	if currentActiveValuesPath == "" {
		return nil, nil
	}

	var (
		activeValues, newValues map[string]component.ValuesFile
		err                     error
	)

	activeValues, err = parseValuesfileToStruct(currentActiveValuesPath)
	if err != nil {
		return nil, err
	}

	if newValuesPath != "" {
		newValues, err = parseValuesfileToStruct(newValuesPath)
		if err != nil {
			return nil, err
		}
	}

	return active.GetCurrentActiveComponents(activeValues, newValues)
}

func sendActivePromotionStatus(r reporter.Reporter, components []component.Component) error {
	if err := r.SendActivePromotionStatus(atvPromotion.status, atvPromotion.currentActiveNamespace, atvPromotion.serviceOwner, components, atvPromotion.showedDetail); err != nil {
		return err
	}

	return nil
}

func parseValuesfileToStruct(valuesFilePath string) (map[string]component.ValuesFile, error) {
	b, err := ioutil.ReadFile(valuesFilePath)
	if err != nil {
		return nil, err
	}

	var values map[string]component.ValuesFile
	if err := yaml.Unmarshal(b, &values); err != nil {
		return nil, err
	}

	return values, nil
}
