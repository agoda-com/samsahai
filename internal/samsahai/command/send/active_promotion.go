package send

import (
	"log"

	"github.com/agoda-com/samsahai/internal/samsahai/reporter"

	"github.com/agoda-com/samsahai/internal/samsahai/component"
	"github.com/spf13/cobra"
)

// activePromotionArgs defines arguments of active-promotion command
type activePromotionArgs struct {
	status                  string
	currentActiveNamespace  string
	currentActiveValuesPath string
	newValuesPath           string
	serviceOwner            string
	showedDetail            bool
}

// atvPromotion is global var for binding value
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

// sendActivePromotionStatusCmd runs when command is executed
func sendActivePromotionStatusCmd(cmd *cobra.Command, args []string) error {
	components, err := getActiveComponentsFromValuesFile(atvPromotion.currentActiveValuesPath, atvPromotion.newValuesPath)
	if err != nil {
		return err
	}

	opts := []reporter.Option{
		reporter.NewOptionShowedDetails(atvPromotion.showedDetail),
		reporter.NewOptionSubject(email.subject),
	}

	for _, r := range reporters {
		if err := sendActivePromotionStatus(r, components, opts...); err != nil {
			return err
		}
	}

	return nil
}

func sendActivePromotionStatus(r reporter.Reporter, components []component.OutdatedComponent, options ...reporter.Option) error {
	if err := r.SendActivePromotionStatus(atvPromotion.status, atvPromotion.currentActiveNamespace, atvPromotion.serviceOwner, components, options...); err != nil {
		return err
	}

	return nil
}
