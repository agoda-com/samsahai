package activepromotion

import (
	"context"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	s2hv1beta1 "github.com/agoda-com/samsahai/pkg/apis/env/v1beta1"
)

func (c *controller) manageQueue(ctx context.Context) (bool, error) {
	atpComps := s2hv1beta1.ActivePromotionList{}
	if err := c.client.List(ctx, &client.ListOptions{}, &atpComps); err != nil {
		if k8serrors.IsNotFound(err) {
			return false, nil
		}

		return false, errors.Wrap(err, "cannot list activepromotions")
	}

	atpComps.SortASC()

	concurrentAtp := c.configs.ActivePromotion.Concurrences
	for i, pcount := 0, 0; i < len(atpComps.Items) && pcount < concurrentAtp; i, pcount = i+1, pcount+1 {
		if atpComps.Items[i].Status.State == s2hv1beta1.ActivePromotionWaiting {
			logger.Info("start active promotion process", "team", atpComps.Items[i].Name)
			logger.Debug("start creating pre-active environment", "team", atpComps.Items[i].Name)

			c.addFinalizer(&atpComps.Items[i])
			atpComps.Items[i].SetState(s2hv1beta1.ActivePromotionCreatingPreActive,
				"Creating pre-active environment")
			atpComps.Items[i].Status.SetCondition(s2hv1beta1.ActivePromotionCondStarted, corev1.ConditionTrue,
				"Active promotion has been started")
			if err := c.updateActivePromotion(ctx, &atpComps.Items[i]); err != nil {
				return false, err
			}
			return true, nil
		}
	}

	return false, nil
}
