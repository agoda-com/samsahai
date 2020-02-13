package activepromotion

import (
	"context"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	s2hv1beta1 "github.com/agoda-com/samsahai/api/v1beta1"
	"github.com/agoda-com/samsahai/internal"
	"github.com/agoda-com/samsahai/internal/samsahai/exporter"
	"github.com/agoda-com/samsahai/internal/util/outdated"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type outdatedComponentTime struct {
	Component   *s2hv1beta1.ActivePromotion
	CreatedTime *metav1.Time
}

func (c *controller) runPostActive(ctx context.Context, atpComp *s2hv1beta1.ActivePromotion) error {
	o := outdatedComponentTime{}
	if atpComp.Status.ActivePromotionHistoryName == "" {
		if err := c.setOutdatedDuration(ctx, atpComp); err != nil {
			return err
		}
		atpHisList := &s2hv1beta1.ActivePromotionHistoryList{}
		if err := c.client.List(context.TODO(), atpHisList); err != nil {
			logger.Error(err, "cannot list all active promotion histories")
		} else {
			exporter.OutdatedComponentMetric.Reset()
			for _, obj := range o.getAllOutdatedComponent(*atpHisList) {
				exporter.SetOutdatedComponentMetric(obj.Component)
			}
			exporter.SetOutdatedComponentMetric(atpComp)
		}

		histName, err := c.createActivePromotionHistory(ctx, atpComp)
		if err != nil && !k8serrors.IsAlreadyExists(err) {
			return err
		}
		atpComp.Status.SetActivePromotionHistoryName(histName)

		if err := c.sendReport(ctx, atpComp); err != nil {
			return err
		}

		logger.Debug("activepromotionhistory has been created",
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

func (c *controller) sendReport(ctx context.Context, atpComp *s2hv1beta1.ActivePromotion) error {
	currentNs := atpComp.Status.TargetNamespace
	if atpComp.Status.Result != s2hv1beta1.ActivePromotionSuccess {
		currentNs = atpComp.Status.PreviousActiveNamespace
		if atpComp.Status.DemotionStatus == s2hv1beta1.ActivePromotionDemotionFailure {
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

	atpRpt := internal.NewActivePromotionReporter(
		&atpComp.Status,
		c.configs,
		atpComp.Name,
		currentNs,
		internal.WithCredential(teamComp.Spec.Credential),
	)
	return c.s2hCtrl.NotifyActivePromotion(atpRpt)
}

func (c *controller) setOutdatedDuration(ctx context.Context, atpComp *s2hv1beta1.ActivePromotion) error {
	teamName := atpComp.Name
	teamComp, err := c.getTeam(ctx, teamName)
	if err != nil {
		return err
	}

	configMgr, err := c.getTeamConfiguration(teamName)
	if err != nil {
		return err
	}

	atpNs := atpComp.Status.TargetNamespace
	if atpComp.Status.Result != s2hv1beta1.ActivePromotionSuccess {
		atpNs = atpComp.Status.PreviousActiveNamespace
	}

	stableCompList := &s2hv1beta1.StableComponentList{}
	err = c.client.List(ctx, stableCompList, &client.ListOptions{Namespace: atpNs})
	if err != nil {
		return err
	}

	desiredCompsImageCreatedTime := teamComp.Status.DesiredComponentImageCreatedTime
	stableComps := stableCompList.Items
	o := outdated.New(configMgr.Get(), desiredCompsImageCreatedTime, stableComps)
	atpStatus := &atpComp.Status
	o.SetOutdatedDuration(atpStatus)
	return nil
}

func (o *outdatedComponentTime) getAllOutdatedComponent(atpHisList s2hv1beta1.ActivePromotionHistoryList) map[string]outdatedComponentTime {
	oc := map[string]outdatedComponentTime{}
	for _, atpHistories := range atpHisList.Items {
		teamName := atpHistories.Spec.TeamName
		if teamName == "" {
			teamName = atpHistories.Labels["samsahai.io/teamname"]
		}
		if atpHistories.Spec.ActivePromotion == nil {
			continue
		}
		if atpHistories.Spec.ActivePromotion.Status.Result == s2hv1beta1.ActivePromotionCanceled {
			continue
		}
		itemCreateTime := atpHistories.CreationTimestamp
		if obj, ok := oc[teamName]; ok {
			if !obj.CreatedTime.Before(&itemCreateTime) {
				continue
			}
		}
		atpHistories.Spec.ActivePromotion.Name = teamName
		oc[teamName] = outdatedComponentTime{
			atpHistories.Spec.ActivePromotion,
			&itemCreateTime,
		}
	}
	return oc
}
