package activepromotion

import (
	"context"

	"github.com/pkg/errors"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	s2hv1beta1 "github.com/agoda-com/samsahai/api/v1beta1"
	"github.com/agoda-com/samsahai/internal"
	s2herrors "github.com/agoda-com/samsahai/internal/errors"
	"github.com/agoda-com/samsahai/internal/samsahai/exporter"
	"github.com/agoda-com/samsahai/internal/util/stringutils"
)

func (c *controller) getTargetNamespace(atpComp *s2hv1beta1.ActivePromotion) string {
	if atpComp.Status.TargetNamespace == "" {
		logger.Warn("target namespace has not been set, getting namespace from team", "team",
			atpComp.Name)
		teamComp, err := c.getTeam(context.TODO(), atpComp.Name)
		if err != nil {
			logger.Error(err, "cannot pre-active namespace from team", "team", atpComp.Name)
			return ""
		}

		return teamComp.Status.Namespace.PreActive
	}

	return atpComp.Status.TargetNamespace
}

func (c *controller) updateActivePromotion(ctx context.Context, atpComp *s2hv1beta1.ActivePromotion) error {
	if err := c.client.Update(ctx, atpComp); err != nil {
		return errors.Wrapf(err, "cannot update activepromotion %s", atpComp.Name)
	}

	// Add metric activepromotion
	exporter.SetActivePromotionMetric(atpComp)

	return nil
}

func (c *controller) forceDeleteActivePromotion(ctx context.Context, atpComp *s2hv1beta1.ActivePromotion) error {
	if err := c.removeFinalizerObject(ctx, atpComp); err != nil {
		return err
	}

	if err := c.deleteActivePromotion(ctx, atpComp); err != nil {
		return err
	}

	return nil
}

func (c *controller) deleteActivePromotion(ctx context.Context, atpComp *s2hv1beta1.ActivePromotion) error {
	logger.Info("deleting activepromotion", "team", atpComp.Name)
	if err := c.client.Delete(ctx, atpComp); err != nil {
		if k8serrors.IsNotFound(err) {
			return nil
		}

		return errors.Wrapf(err, "cannot delete activepromotion %s", atpComp.Name)
	}

	return nil
}

func (c *controller) removeFinalizerObject(ctx context.Context, atpComp *s2hv1beta1.ActivePromotion) error {
	atpCompTmp := &s2hv1beta1.ActivePromotion{}
	err := c.client.Get(ctx, types.NamespacedName{Name: atpComp.Name}, atpCompTmp)
	if err != nil && k8serrors.IsNotFound(err) {
		return nil
	}

	if stringutils.ContainsString(atpComp.ObjectMeta.Finalizers, activePromotionFinalizer) {
		atpComp.ObjectMeta.Finalizers = stringutils.RemoveString(atpComp.ObjectMeta.Finalizers, activePromotionFinalizer)
		if err := c.updateActivePromotion(ctx, atpComp); err != nil {
			return errors.Wrapf(err, "cannot remove finalizer of activepromotion %s", atpComp.Name)
		}
	}

	return nil
}

func (c *controller) getTeam(ctx context.Context, teamName string) (*s2hv1beta1.Team, error) {
	teamComp := &s2hv1beta1.Team{}
	if err := c.client.Get(ctx, types.NamespacedName{Name: teamName}, teamComp); err != nil {
		return &s2hv1beta1.Team{}, err
	}

	return teamComp, nil
}

func (c *controller) getTeamConfiguration(teamName string) (internal.ConfigManager, error) {
	configMgr, ok := c.s2hCtrl.GetTeamConfigManager(teamName)
	if !ok {
		return nil, s2herrors.ErrLoadingConfiguration
	}
	if configMgr == nil {
		return nil, errors.Wrapf(s2herrors.ErrLoadConfiguration, "cannot load configuration for %s", teamName)
	}

	return configMgr, nil
}
