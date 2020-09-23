package queue

import (
	"context"

	"github.com/pkg/errors"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	s2hv1beta1 "github.com/agoda-com/samsahai/api/v1beta1"
)

func (c *controller) listPullRequestQueues(opts *client.ListOptions, namespace string) (list *s2hv1beta1.PullRequestQueueList, err error) {
	list = &s2hv1beta1.PullRequestQueueList{}
	if opts == nil {
		opts = &client.ListOptions{Namespace: namespace}
	}
	if err = c.client.List(context.Background(), list, opts); err != nil {
		return list, errors.Wrapf(err, "cannot list pull request queues with options: %+v", opts)
	}
	return list, nil
}

func (c *controller) updatePullRequestQueue(ctx context.Context, prQueue *s2hv1beta1.PullRequestQueue) error {
	if err := c.client.Update(ctx, prQueue); err != nil {
		return errors.Wrapf(err, "cannot update pull request queue %s", prQueue.Name)
	}

	return nil
}

func (c *controller) deletePullRequestQueue(ctx context.Context, prQueue *s2hv1beta1.PullRequestQueue) error {
	logger.Info("deleting pull request queue",
		"component", prQueue.Spec.ComponentName, "prNumber", prQueue.Spec.PRNumber)

	if err := c.client.Delete(ctx, prQueue); err != nil {
		if k8serrors.IsNotFound(err) {
			return nil
		}

		return errors.Wrapf(err, "cannot delete pull request queue %s", prQueue.Name)
	}

	return nil
}
