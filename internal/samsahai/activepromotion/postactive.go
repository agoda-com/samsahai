package activepromotion

import (
	"context"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	s2hv1 "github.com/agoda-com/samsahai/api/v1"
	"github.com/agoda-com/samsahai/internal"
	"github.com/agoda-com/samsahai/internal/util/outdated"
)

func (c *controller) runPostActive(ctx context.Context, atpComp *s2hv1.ActivePromotion) error {
	if atpComp.Status.ActivePromotionHistoryName == "" {
		if err := c.setOutdatedDuration(ctx, atpComp); err != nil {
			return err
		}

		histName, err := c.createActivePromotionHistory(ctx, atpComp)
		if err != nil && !k8serrors.IsAlreadyExists(err) {
			return err
		}
		atpComp.Status.SetActivePromotionHistoryName(histName)

		if err := c.sendReport(ctx, atpComp); err != nil {
			return err
		}

		logger.Debug("active promotion history has been created",
			"team", atpComp.Name, "status", atpComp.Status.Result, "name", histName)
		logger.Debug("active promotion report has been sent",
			"team", atpComp.Name, "status", atpComp.Status.Result)

		if err := c.updateActivePromotion(ctx, atpComp); err != nil {
			return err
		}

		return nil
	}

	if err := c.updateActivePromotionHistory(ctx, atpComp.Status.ActivePromotionHistoryName, atpComp); err != nil {
		return err
	}

	return nil
}

func (c *controller) sendReport(ctx context.Context, atpComp *s2hv1.ActivePromotion) error {
	currentNs := c.getTargetNamespace(atpComp)
	if atpComp.Status.Result != s2hv1.ActivePromotionSuccess {
		currentNs = atpComp.Status.PreviousActiveNamespace
		if atpComp.Status.DemotionStatus == s2hv1.ActivePromotionDemotionFailure {
			currentNs = ""
		}
	}

	teamComp, err := c.getTeam(ctx, atpComp.Name)
	if err != nil {
		return err
	}

	if err := c.s2hCtrl.LoadTeamSecret(teamComp); err != nil {
		logger.Error(err, "cannot load team secret", "team", teamComp.Name)
		return err
	}

	runs := atpComp.Spec.NoOfRetry + 1
	atpRpt := internal.NewActivePromotionReporter(
		atpComp.Status,
		c.configs,
		atpComp.Name,
		currentNs,
		runs,
		internal.WithActivePromotionOptCredential(teamComp.Status.Used.Credential),
	)
	c.s2hCtrl.NotifyActivePromotionReport(atpRpt)

	return nil
}

func (c *controller) setOutdatedDuration(ctx context.Context, atpComp *s2hv1.ActivePromotion) error {
	configCtrl := c.s2hCtrl.GetConfigController()
	config, err := configCtrl.Get(atpComp.Name)
	if err != nil {
		return err
	}

	targetNs := c.getTargetNamespace(atpComp)
	if atpComp.Status.Result != s2hv1.ActivePromotionSuccess {
		targetNs = atpComp.Status.PreviousActiveNamespace
	}

	stableCompList := &s2hv1.StableComponentList{}
	if targetNs != "" {
		err = c.client.List(ctx, stableCompList, &client.ListOptions{Namespace: targetNs})
		if err != nil {
			return err
		}
	}

	var currentActiveComps = make(map[string]s2hv1.StableComponent)
	for _, stableComp := range stableCompList.Items {
		currentActiveComps[stableComp.Name] = s2hv1.StableComponent{
			Spec: stableComp.Spec,
		}
	}

	teamComp, err := c.getTeam(ctx, atpComp.Name)
	if err != nil {
		return err
	}

	if len(currentActiveComps) == 0 {
		currentActiveComps = teamComp.Status.ActiveComponents
	}
	desiredCompsImageCreatedTime := teamComp.Status.DesiredComponentImageCreatedTime
	o := outdated.New(&config.Status.Used, desiredCompsImageCreatedTime, currentActiveComps)
	atpStatus := &atpComp.Status
	o.SetOutdatedDuration(atpStatus)
	return nil
}
