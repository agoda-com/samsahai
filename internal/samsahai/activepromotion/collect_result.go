package activepromotion

import (
	"context"

	corev1 "k8s.io/api/core/v1"

	"github.com/agoda-com/samsahai/internal/errors"
	"github.com/agoda-com/samsahai/internal/queue"
	s2hv1beta1 "github.com/agoda-com/samsahai/pkg/apis/env/v1beta1"
)

func (c *controller) collectResult(ctx context.Context, atpComp *s2hv1beta1.ActivePromotion) error {
	teamName := atpComp.Name
	targetNs := atpComp.Status.TargetNamespace
	q, err := queue.EnsurePreActiveComponents(c.client, teamName, targetNs)
	if err != nil {
		return errors.Wrapf(err, "cannot ensure pre-active components, namespace %s", targetNs)
	}

	if !atpComp.IsActivePromotionCanceled() && !atpComp.Status.IsTimeout {
		// to save pre-active queue after pre-active queue finished
		q, err = c.ensurePreActiveComponentsTested(teamName, targetNs)
		if err != nil {
			return errors.Wrapf(err, "cannot ensure pre-active components tested, namespace %s", targetNs)
		}
	}

	if q != nil {
		atpComp.Status.SetPreActiveQueue(q.Status)
	}

	if atpComp.IsActivePromotionFailure() || atpComp.IsActivePromotionCanceled() {
		logger.Debug("destroying pre-active environment due to failure or cancel",
			"team", teamName, "namespace", targetNs)
		atpComp.Status.SetCondition(s2hv1beta1.ActivePromotionCondResultCollected, corev1.ConditionTrue,
			"Result has been collected")
		atpComp.Status.SetCondition(s2hv1beta1.ActivePromotionCondActivePromoted, corev1.ConditionFalse,
			"Active environment has not been promoted")
		atpComp.SetState(s2hv1beta1.ActivePromotionDestroyingPreActive, "Destroying pre-active environment")

		return nil
	}

	atpComp.Status.SetCondition(s2hv1beta1.ActivePromotionCondActiveDemotionStarted, corev1.ConditionTrue,
		"Active demotion has been started")
	atpComp.SetState(s2hv1beta1.ActivePromotionDemoting, "Demoting an active environment")

	return nil
}
