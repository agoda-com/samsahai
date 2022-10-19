package activepromotion

import (
	"context"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	s2hv1 "github.com/agoda-com/samsahai/api/v1"
	s2herrors "github.com/agoda-com/samsahai/internal/errors"
	"github.com/agoda-com/samsahai/internal/queue"
)

func (c *controller) demoteActiveEnvironment(ctx context.Context, atpComp *s2hv1.ActivePromotion) error {
	if err := c.checkDemotionTimeout(ctx, atpComp); err != nil {
		return err
	}

	teamName := atpComp.Name
	prevNs := atpComp.Status.PreviousActiveNamespace

	if prevNs != "" {
		if err := c.ensureQueueActiveDemoted(teamName, prevNs); err != nil {
			if !s2herrors.IsErrReleaseFailed(err) {
				return err
			}

			// destroy active environment if release failed
			if err := c.destroyActiveEnvironment(ctx, atpComp, atpComp.Status.DestroyedTime); err != nil {
				if !s2herrors.IsDeletingReleases(err) && !s2herrors.IsEnsuringNamespaceDestroyed(err) {
					return err
				}
			}

			teamName := atpComp.Name
			prevNs := atpComp.Status.PreviousActiveNamespace
			logger.Debug("failed to demote active environment, deleted active environment",
				"team", teamName, "namespace", prevNs)
			atpComp.Status.SetCondition(s2hv1.ActivePromotionCondActiveDemoted, corev1.ConditionFalse,
				"Failed to demote active environment, active environment has been deleted")

			if *atpComp.Spec.NoDowntimeGuarantee {
				atpComp.SetState(s2hv1.ActivePromotionDestroyingPreviousActive,
					"Failed to demote active environment")
				logger.Info("Demote failed, and start destroying an active environment")
				return nil
			}

			atpComp.SetState(s2hv1.ActivePromotionActiveEnvironment, "Failed to demote active environment")
			logger.Info("Demote failed, and start promoting an active environment")

			return nil
		}
	}

	atpComp.Status.SetDemotionStatus(s2hv1.ActivePromotionDemotionSuccess)
	atpComp.Status.SetCondition(s2hv1.ActivePromotionCondActiveDemoted, corev1.ConditionTrue,
		"Demoted an active environment successfully")

	if *atpComp.Spec.NoDowntimeGuarantee {
		atpComp.SetState(s2hv1.ActivePromotionDestroyingPreviousActive,
			"Destroying the previous active environment")
		logger.Info("Demote successfully, and start destroying the previous active environment")
		return nil
	}

	atpComp.SetState(s2hv1.ActivePromotionActiveEnvironment, "Promoting an active environment")
	logger.Info("Demote successfully, and start promoting an active environment")

	return nil
}

func (c *controller) ensureQueueActiveDemoted(teamName, ns string) error {
	q, err := queue.EnsureDemoteFromActiveComponents(c.client, teamName, ns)
	if err != nil {
		return errors.Wrapf(err, "cannot ensure environment demoted from active components, namespace %s", ns)
	}

	if q.Status.State == s2hv1.Finished {
		if !q.IsDeploySuccess() {
			return s2herrors.ErrReleaseFailed
		}

		return nil
	}

	return s2herrors.ErrEnsureActiveDemoted
}

func (c *controller) checkDemotionTimeout(ctx context.Context, atpComp *s2hv1.ActivePromotion) error {
	isTimeout, err := c.isTimeoutFromConfig(atpComp, timeoutActiveDemotion)
	if err != nil {
		return err
	}

	if isTimeout {
		// destroy active environment when demotion timeout due to active is not working
		if atpComp.Status.DestroyedTime == nil {
			now := metav1.Now()
			atpComp.Status.SetDemotionStatus(s2hv1.ActivePromotionDemotionFailure)
			atpComp.Status.SetDestroyedTime(now)

			if err := c.updateActivePromotion(ctx, atpComp); err != nil {
				return err
			}
			return s2herrors.ErrActiveDemotionTimeout
		}

		if err := c.destroyActiveEnvironment(ctx, atpComp, atpComp.Status.DestroyedTime); err != nil {
			if !s2herrors.IsDeletingReleases(err) && !s2herrors.IsEnsuringNamespaceDestroyed(err) {
				return err
			}
		}

		teamName := atpComp.Name
		prevNs := atpComp.Status.PreviousActiveNamespace
		logger.Debug("active demotion has been timeout, deleted active environment",
			"team", teamName, "namespace", prevNs)
		atpComp.Status.SetCondition(s2hv1.ActivePromotionCondActiveDemoted, corev1.ConditionFalse,
			"Demoted an active environment timeout, active environment has been deleted")

		if *atpComp.Spec.NoDowntimeGuarantee {
			atpComp.SetState(s2hv1.ActivePromotionDestroyingPreviousActive, "Demoted active environment timeout")
			logger.Info("Demote timeout, and start destroying an active environment")
			return s2herrors.ErrActiveDemotionTimeout
		}

		atpComp.SetState(s2hv1.ActivePromotionActiveEnvironment, "Demoted active environment timeout")
		logger.Info("Demote timeout, and start destroying an active environment")

		if err := c.updateActivePromotion(ctx, atpComp); err != nil {
			return err
		}

		return s2herrors.ErrActiveDemotionTimeout
	}

	return nil
}
