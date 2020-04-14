package activepromotion

import (
	"context"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	s2hv1beta1 "github.com/agoda-com/samsahai/api/v1beta1"
)

func (c *controller) manageQueue(ctx context.Context, currentAtpComp *s2hv1beta1.ActivePromotion) (bool, error) {
	runningAtpComps := s2hv1beta1.ActivePromotionList{}
	listOpts := client.ListOptions{LabelSelector: labels.SelectorFromSet(c.getStateLabel(stateRunning))}
	if err := c.client.List(ctx, &runningAtpComps, &listOpts); err != nil {
		return false, errors.Wrap(err, "cannot list activepromotions")
	}

	concurrentAtp := c.configs.ActivePromotion.Concurrences
	if len(runningAtpComps.Items) >= concurrentAtp {
		return false, nil
	}

	waitingAtpComps := s2hv1beta1.ActivePromotionList{}
	listOpts = client.ListOptions{LabelSelector: labels.SelectorFromSet(c.getStateLabel(stateWaiting))}
	if err := c.client.List(ctx, &waitingAtpComps, &listOpts); err != nil {
		return false, errors.Wrap(err, "cannot list activepromotions")
	}

	// there is no new queue
	if len(waitingAtpComps.Items) == 0 {
		return false, nil
	}

	waitingAtpComps.SortASC()

	if concurrentAtp-len(runningAtpComps.Items) > 0 {
		logger.Info("start active promotion process", "team", waitingAtpComps.Items[0].Name)

		c.addFinalizer(&waitingAtpComps.Items[0])
		waitingAtpComps.Items[0].SetState(s2hv1beta1.ActivePromotionCreatingPreActive,
			"Creating pre-active environment")
		waitingAtpComps.Items[0].Status.SetCondition(s2hv1beta1.ActivePromotionCondStarted, corev1.ConditionTrue,
			"Active promotion has been started")
		waitingAtpComps.Items[0].Labels = c.getStateLabel(stateRunning)
		if err := c.updateActivePromotion(ctx, &waitingAtpComps.Items[0]); err != nil {
			return false, err
		}

		// should not continue the process due to current active promotion component has been updated
		if waitingAtpComps.Items[0].Name == currentAtpComp.Name {
			return true, nil
		}
	}

	return false, nil
}
