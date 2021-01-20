package activepromotion

import (
	"context"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	s2hv1 "github.com/agoda-com/samsahai/api/v1"
)

func (c *controller) manageQueue(ctx context.Context, currentAtpComp *s2hv1.ActivePromotion) (
	skipReconcile bool, err error) {

	runningAtpComps := s2hv1.ActivePromotionList{}
	listOpts := client.ListOptions{LabelSelector: labels.SelectorFromSet(c.getStateLabel(stateRunning))}
	if err = c.client.List(ctx, &runningAtpComps, &listOpts); err != nil {
		err = errors.Wrap(err, "cannot list activepromotions")
		return
	}

	concurrentAtp := c.configs.ActivePromotion.Concurrences
	if len(runningAtpComps.Items) >= concurrentAtp {
		return
	}

	waitingAtpComps := s2hv1.ActivePromotionList{}
	listOpts = client.ListOptions{LabelSelector: labels.SelectorFromSet(c.getStateLabel(stateWaiting))}
	if err = c.client.List(ctx, &waitingAtpComps, &listOpts); err != nil {
		err = errors.Wrap(err, "cannot list activepromotions")
		return
	}

	// there is no new queue
	if len(waitingAtpComps.Items) == 0 {
		return false, nil
	}

	waitingAtpComps.SortASC()

	if concurrentAtp-len(runningAtpComps.Items) > 0 {
		logger.Info("start active promotion process", "team", waitingAtpComps.Items[0].Name)

		c.addFinalizer(&waitingAtpComps.Items[0])
		waitingAtpComps.Items[0].SetState(s2hv1.ActivePromotionCreatingPreActive,
			"Creating pre-active environment")
		waitingAtpComps.Items[0].Status.SetCondition(s2hv1.ActivePromotionCondStarted, corev1.ConditionTrue,
			"Active promotion has been started")
		c.appendStateLabel(&waitingAtpComps.Items[0], stateRunning)
		if err = c.updateActivePromotion(ctx, &waitingAtpComps.Items[0]); err != nil {
			return
		}

		// should not continue the process due to current active promotion component has been updated
		if waitingAtpComps.Items[0].Name == currentAtpComp.Name {
			skipReconcile = true
			return
		}
	}

	return
}

func (c *controller) checkRetryQueue(ctx context.Context, atpComp *s2hv1.ActivePromotion) (
	skipReconcile bool, err error) {

	if atpComp.Status.State == s2hv1.ActivePromotionFinished && atpComp.IsActivePromotionFailure() {
		maxRetry := c.getMaxActivePromotionRetry(atpComp.Name)
		if atpComp.Spec.NoOfRetry < maxRetry {
			atpComp.Spec.NoOfRetry++
			if err = c.setRetryQueue(ctx, atpComp); err != nil {
				return
			}

			skipReconcile = true
			return
		}
	}

	return
}

func (c *controller) setRetryQueue(ctx context.Context, atpComp *s2hv1.ActivePromotion) error {
	now := metav1.Now()
	atpComp.Status = s2hv1.ActivePromotionStatus{
		StartedAt: &now,
		UpdatedAt: &now,
	}

	return c.updateActivePromotion(ctx, atpComp)
}
