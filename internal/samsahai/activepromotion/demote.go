package activepromotion

import (
	"context"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	s2herrors "github.com/agoda-com/samsahai/internal/errors"
	"github.com/agoda-com/samsahai/internal/queue"
	s2hv1beta1 "github.com/agoda-com/samsahai/pkg/apis/env/v1beta1"
)

func (c *controller) demoteActiveEnvironment(ctx context.Context, atpComp *s2hv1beta1.ActivePromotion) error {
	if err := c.checkDemotionTimeout(ctx, atpComp); err != nil {
		if s2herrors.IsLoadingConfiguration(err) {
			return s2herrors.ErrEnsureActiveDemoted
		}

		return err
	}

	teamName := atpComp.Name
	prevNs := atpComp.Status.PreviousActiveNamespace

	if prevNs != "" {
		if err := c.ensureQueueActiveDemoted(teamName, prevNs); err != nil {
			return err
		}
	}

	atpComp.Status.SetDemotionStatus(s2hv1beta1.ActivePromotionDemotionSuccess)
	atpComp.Status.SetCondition(s2hv1beta1.ActivePromotionCondActiveDemoted, corev1.ConditionTrue,
		"Demoted an active environment successfully")
	atpComp.SetState(s2hv1beta1.ActivePromotionActiveEnvironment, "Promoting an active environment")

	return nil
}

func (c *controller) ensureQueueActiveDemoted(teamName, ns string) error {
	q, err := queue.EnsureDemoteFromActiveComponents(c.client, teamName, ns)
	if err != nil {
		return errors.Wrapf(err, "cannot ensure environment demoted from active components, namespace %s", ns)
	}

	if q.Status.State == s2hv1beta1.Finished {
		return nil
	}

	return s2herrors.ErrEnsureActiveDemoted
}

func (c *controller) checkDemotionTimeout(ctx context.Context, atpComp *s2hv1beta1.ActivePromotion) error {
	isTimeout, err := c.isTimeoutFromConfig(atpComp, timeoutActiveDemotion)
	if err != nil {
		return err
	}

	if isTimeout {
		// destroy active environment when demotion timeout due to active is not working
		if atpComp.Status.DestroyTime == nil {
			now := metav1.Now()
			atpComp.Status.SetDemotionStatus(s2hv1beta1.ActivePromotionDemotionFailure)
			atpComp.Status.SetDestroyTime(now)

			if err := c.updateActivePromotion(ctx, atpComp); err != nil {
				return err
			}
			return s2herrors.ErrActiveDemotionTimeout
		}

		if err := c.destroyActiveEnvironment(ctx, atpComp, atpComp.Status.DestroyTime); err != nil {
			if s2herrors.IsDeletingReleases(err) || s2herrors.IsEnsuringNamespaceDestroyed(err) {
				return s2herrors.ErrEnsureActiveDemoted
			}
			return err
		}

		teamName := atpComp.Name
		prevNs := atpComp.Status.PreviousActiveNamespace
		logger.Debug("active demotion has been timeout, deleted active environment",
			"team", teamName, "namespace", prevNs)
		atpComp.Status.SetCondition(s2hv1beta1.ActivePromotionCondActiveDemoted, corev1.ConditionFalse,
			"Demoted an active environment timeout, active environment has been deleted")
		atpComp.SetState(s2hv1beta1.ActivePromotionActiveEnvironment, "Demoted active environment timeout")

		if err := c.updateActivePromotion(ctx, atpComp); err != nil {
			return err
		}

		return s2herrors.ErrActiveDemotionTimeout
	}

	return nil
}
