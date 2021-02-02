package queue

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/pkg/errors"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	s2hv1 "github.com/agoda-com/samsahai/api/v1"
	samsahairpc "github.com/agoda-com/samsahai/pkg/samsahai/rpc"
)

func (c *controller) createPullRequestQueueHistory(ctx context.Context, prQueue *s2hv1.PullRequestQueue) error {
	prQueueLabels := getPullRequestQueueLabels(c.teamName, prQueue.Spec.BundleName, prQueue.Spec.PRNumber)

	if err := c.deletePullRequestQueueHistoryOutOfRange(ctx); err != nil {
		return err
	}

	history := &s2hv1.PullRequestQueueHistory{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateHistoryName(prQueue.Name, prQueue.CreationTimestamp, prQueue.Spec.NoOfRetry),
			Namespace: c.namespace,
			Labels:    prQueueLabels,
		},
		Spec: s2hv1.PullRequestQueueHistorySpec{
			PullRequestQueue: &s2hv1.PullRequestQueue{
				Spec:   prQueue.Spec,
				Status: prQueue.Status,
			},
		},
	}

	if err := c.client.Create(ctx, history); err != nil && !k8serrors.IsAlreadyExists(err) {
		return errors.Wrapf(err,
			"cannot create pull request queue history of %s", prQueue.Name)
	}

	return nil
}

func (c *controller) deletePullRequestQueueHistoryOutOfRange(ctx context.Context) error {
	prQueueHists := s2hv1.PullRequestQueueHistoryList{}
	if err := c.client.List(ctx, &prQueueHists, &client.ListOptions{Namespace: c.namespace}); err != nil {
		if k8serrors.IsNotFound(err) {
			return nil
		}

		return errors.Wrapf(err, "cannot list pull request queue histories of namespace: %s", c.namespace)
	}

	prConfig, err := c.s2hClient.GetPullRequestConfig(ctx, &samsahairpc.TeamWithBundleName{TeamName: c.teamName})
	if err != nil {
		return err
	}

	// parse max stored pull request queue histories in day to time duration
	maxHistDays := int(prConfig.MaxHistoryDays)
	maxHistDuration, err := time.ParseDuration(strconv.Itoa(maxHistDays*24) + "h")
	if err != nil {
		logger.Error(err, fmt.Sprintf("cannot parse time duration of %d", maxHistDays))
		return nil
	}

	prQueueHists.SortDESC()
	now := metav1.Now()
	for i := len(prQueueHists.Items) - 1; i > 0; i-- {
		if now.Sub(prQueueHists.Items[i].CreationTimestamp.Time) >= maxHistDuration {
			if err := c.client.Delete(ctx, &prQueueHists.Items[i]); err != nil {
				if k8serrors.IsNotFound(err) {
					continue
				}

				logger.Error(err, fmt.Sprintf("cannot delete pull request queue histories %s", prQueueHists.Items[i].Name))
				return errors.Wrapf(err, "cannot delete pull request queue histories %s", prQueueHists.Items[i].Name)
			}
			continue
		}

		break
	}

	return nil
}

func generateHistoryName(prQueueName string, startTime metav1.Time, retryCount int) string {
	return fmt.Sprintf("%s-%s-%d", prQueueName, startTime.Format("20060102-150405"), retryCount)
}
