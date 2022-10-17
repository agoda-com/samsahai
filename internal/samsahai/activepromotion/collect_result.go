package activepromotion

import (
	"context"

	corev1 "k8s.io/api/core/v1"

	s2hv1 "github.com/agoda-com/samsahai/api/v1"
	"github.com/agoda-com/samsahai/internal/errors"
	"github.com/agoda-com/samsahai/internal/queue"
)

func (c *controller) collectResult(ctx context.Context, atpComp *s2hv1.ActivePromotion) error {
	teamName := atpComp.Name
	targetNs := c.getTargetNamespace(atpComp)
	q, err := queue.EnsurePreActiveComponents(c.client, teamName, targetNs, atpComp.Spec.SkipTestRunner)
	if err != nil {
		return errors.Wrapf(err, "cannot ensure pre-active components, namespace %s", targetNs)
	}

	if !atpComp.IsActivePromotionCanceled() && !atpComp.Status.IsTimeout {
		// to save pre-active queue after pre-active queue finished
		q, err = c.ensurePreActiveComponentsTested(teamName, targetNs, atpComp.Spec.SkipTestRunner)
		if err != nil {
			return errors.Wrapf(err, "cannot ensure pre-active components finished, namespace %s", targetNs)
		}
	}

	if q != nil {
		atpComp.Status.SetPreActiveQueue(q.Status)
	}

	if len(q.Status.ImageMissingList) > 0 {
		atpComp.Status.SetCondition(s2hv1.ActivePromotionCondVerified, corev1.ConditionTrue,
			"Image missing")
	}

	if atpComp.IsActivePromotionFailure() || atpComp.IsActivePromotionCanceled() {
		logger.Debug("destroying pre-active environment due to failure or cancel",
			"team", teamName, "namespace", targetNs)
		atpComp.Status.SetCondition(s2hv1.ActivePromotionCondResultCollected, corev1.ConditionTrue,
			"Result has been collected")
		atpComp.Status.SetCondition(s2hv1.ActivePromotionCondActivePromoted, corev1.ConditionFalse,
			c.getActivePromotionVerificationReason(atpComp))
		atpComp.SetState(s2hv1.ActivePromotionDestroyingPreActive, "Destroying pre-active environment")

		return nil
	}

	if atpComp.Spec.SwitchBeforeDemote {
		// or can remove
		//atpComp.Status.SetCondition(s2hv1.ActivePromotionCondActivePromotionStarted, corev1.ConditionTrue,
		//	"Active promotion an active environment has been started")
		atpComp.SetState(s2hv1.ActivePromotionActiveEnvironment, "Promoting an active environment")
	} else {
		atpComp.Status.SetCondition(s2hv1.ActivePromotionCondActiveDemotionStarted, corev1.ConditionTrue,
			"Active demotion has been started")
		atpComp.SetState(s2hv1.ActivePromotionDemoting, "Demoting an active environment")
	}

	return nil
}

func (c *controller) getActivePromotionVerificationReason(atpComp *s2hv1.ActivePromotion) string {
	if atpComp.Status.IsTimeout {
		return "Active promotion has been timeout"
	}

	for _, cond := range atpComp.Status.Conditions {
		if cond.Type == s2hv1.ActivePromotionCondVerified {
			return cond.Message
		}
	}

	return "Active environment has not been promoted"
}
