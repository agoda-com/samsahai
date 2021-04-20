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

func (c *controller) promoteActiveEnvironment(ctx context.Context, atpComp *s2hv1.ActivePromotion) error {
	teamName := atpComp.Name
	targetNs := c.getTargetNamespace(atpComp)
	prevNs := atpComp.Status.PreviousActiveNamespace

	if err := queue.DeletePreActiveQueue(c.client, targetNs); err != nil {
		return err
	}

	if err := c.ensureQueuePromotedToActive(teamName, targetNs); err != nil {
		if s2herrors.IsErrReleaseFailed(err) {
			atpComp.Status.SetResult(s2hv1.ActivePromotionFailure)
			atpComp.Status.SetCondition(s2hv1.ActivePromotionCondRollbackStarted, corev1.ConditionTrue,
				"Rollback process has been started due to cannot apply active values file")
			atpComp.SetState(s2hv1.ActivePromotionRollback,
				"Active promotion failed due to cannot apply active values file")
			return nil
		}

		return err
	}

	if err := c.ensureActiveEnvironmentPromoted(ctx, atpComp); err != nil {
		return err
	}

	if prevNs != "" && atpComp.Status.DestroyedTime == nil {
		logger.Debug("previous active namespace destroyed time has been set",
			"team", teamName, "namespace", prevNs)
		destroyedTime := metav1.Now().Add(atpComp.Spec.TearDownDuration.Duration)
		atpComp.Status.SetDestroyedTime(metav1.Time{Time: destroyedTime})
	}

	logger.Info("active environment has been promoted successfully",
		"team", teamName, "status", s2hv1.ActivePromotionSuccess, "namespace", targetNs)
	atpComp.Status.SetResult(s2hv1.ActivePromotionSuccess)
	atpComp.Status.SetCondition(s2hv1.ActivePromotionCondResultCollected, corev1.ConditionTrue,
		"Result has been collected, promoted successfully")
	atpComp.Status.SetCondition(s2hv1.ActivePromotionCondActivePromoted, corev1.ConditionTrue,
		"Active environment has been promoted")
	atpComp.SetState(s2hv1.ActivePromotionDestroyingPreviousActive,
		"Destroying the previous active environment")

	if err := c.runPostActive(ctx, atpComp); err != nil {
		return err
	}

	return nil
}

func (c *controller) ensureQueuePromotedToActive(teamName, ns string) error {
	q, err := queue.EnsurePromoteToActiveComponents(c.client, teamName, ns)
	if err != nil {
		return errors.Wrapf(err, "cannot ensure environment promoted to active components, namespace %s", ns)
	}

	if q.Status.State == s2hv1.Finished {
		if !q.IsDeploySuccess() {
			return s2herrors.ErrReleaseFailed
		}

		return nil
	}

	return s2herrors.ErrEnsureActivePromoted
}
