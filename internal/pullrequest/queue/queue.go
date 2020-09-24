package queue

import (
	"context"
	"time"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	s2hv1beta1 "github.com/agoda-com/samsahai/api/v1beta1"
	s2herrors "github.com/agoda-com/samsahai/internal/errors"
)

func (c *controller) Add(obj runtime.Object, priorityQueues []string) error {
	prQueue, ok := obj.(*s2hv1beta1.PullRequestQueue)
	if !ok {
		return s2herrors.ErrParsingRuntimeObject
	}

	return c.addQueue(context.TODO(), prQueue, false)
}

func (c *controller) AddTop(obj runtime.Object) error {
	prQueue, ok := obj.(*s2hv1beta1.PullRequestQueue)
	if !ok {
		return s2herrors.ErrParsingRuntimeObject
	}

	return c.addQueue(context.TODO(), prQueue, true)
}

func (c *controller) Size(namespace string) int {
	list, err := c.listPullRequestQueues(nil, namespace)
	if err != nil {
		logger.Error(err, "cannot list pull request queues")
		return 0
	}
	return len(list.Items)
}

func (c *controller) First(namespace string) (runtime.Object, error) {
	list, err := c.listPullRequestQueues(nil, namespace)
	if err != nil {
		logger.Error(err, "cannot list pull request queues")
		return nil, err
	}

	return list.First(), nil
}

func (c *controller) Remove(obj runtime.Object) error {
	return c.client.Delete(context.TODO(), obj)
}

func (c *controller) RemoveAllQueues(namespace string) error {
	return c.client.DeleteAllOf(context.TODO(), &s2hv1beta1.PullRequestQueue{}, client.InNamespace(namespace))
}

func (c *controller) SetLastOrder(obj runtime.Object) error {
	prQueue, ok := obj.(*s2hv1beta1.PullRequestQueue)
	if !ok {
		return s2herrors.ErrParsingRuntimeObject
	}

	queueList, err := c.listPullRequestQueues(nil, prQueue.Namespace)
	if err != nil {
		logger.Error(err, "cannot list pull request queues")
		return err
	}

	prQueue.Labels = c.getStateLabel(stateWaiting)
	prQueue.Spec.NoOfOrder = queueList.LastQueueOrder()

	createdAt := prQueue.Status.CreatedAt
	prQueue.Status = s2hv1beta1.PullRequestQueueStatus{
		CreatedAt: createdAt,
	}

	return c.client.Update(context.TODO(), prQueue)
}

func (c *controller) SetReverifyQueueAtFirst(obj runtime.Object) error {
	prQueue, ok := obj.(*s2hv1beta1.PullRequestQueue)
	if !ok {
		return s2herrors.ErrParsingRuntimeObject
	}

	list, err := c.listPullRequestQueues(nil, prQueue.Namespace)
	if err != nil {
		logger.Error(err, "cannot list pull request queues")
		return err
	}

	prQueue.Labels = c.getStateLabel(stateWaiting)
	prQueue.Spec.NoOfOrder = list.TopQueueOrder()

	createdAt := prQueue.Status.CreatedAt
	prQueue.Status = s2hv1beta1.PullRequestQueueStatus{
		CreatedAt: createdAt,
	}

	return c.client.Update(context.TODO(), prQueue)
}

func (c *controller) SetRetryQueue(obj runtime.Object, noOfRetry int, nextAt time.Time) error {
	prQueue, ok := obj.(*s2hv1beta1.PullRequestQueue)
	if !ok {
		return s2herrors.ErrParsingRuntimeObject
	}

	list, err := c.listPullRequestQueues(nil, prQueue.Namespace)
	if err != nil {
		logger.Error(err, "cannot list pull request queues")
		return err
	}

	now := metav1.Now()
	prQueue.Labels = c.getStateLabel(stateWaiting)
	prQueue.Status = s2hv1beta1.PullRequestQueueStatus{
		CreatedAt: &now,
		State:     s2hv1beta1.PullRequestQueueWaiting,
	}
	prQueue.Spec.NoOfRetry = noOfRetry
	prQueue.Spec.NoOfOrder = list.LastQueueOrder()
	return c.client.Update(context.TODO(), prQueue)
}

func (c *controller) addQueue(ctx context.Context, prQueue *s2hv1beta1.PullRequestQueue, atTop bool) error {
	c.resetQueueOrderWithRunningQueue(ctx, prQueue)

	prQueueList, err := c.listPullRequestQueues(nil, prQueue.Namespace)
	if err != nil {
		logger.Error(err, "cannot list pull request queues")
		return err
	}

	tmpPRQueue := &s2hv1beta1.PullRequestQueue{}
	err = c.client.Get(ctx, types.NamespacedName{
		Namespace: c.namespace,
		Name:      prQueue.Name,
	}, tmpPRQueue)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			// create pull request queue
			order := prQueueList.LastQueueOrder()
			prQueue.Spec.NoOfOrder = order
			if err := c.client.Create(ctx, prQueue); err != nil && !k8serrors.IsAlreadyExists(err) {
				return err
			}

			return nil
		}

		return err
	}

	currentOrder := tmpPRQueue.Spec.NoOfOrder
	currentRetry := tmpPRQueue.Spec.NoOfRetry

	// update pull request queue
	tmpPRQueue.Spec = prQueue.Spec
	tmpPRQueue.Spec.NoOfOrder = currentOrder
	tmpPRQueue.Spec.NoOfRetry = currentRetry
	if err := c.client.Update(ctx, tmpPRQueue); err != nil {
		return err
	}

	return nil
}

// resetQueueOrderWithRunningQueue resets order of all queues to start with 1 respectively
func (c *controller) resetQueueOrderWithRunningQueue(ctx context.Context, currentPRQueue *s2hv1beta1.PullRequestQueue) {
	allPRQueues, err := c.listPullRequestQueues(nil, currentPRQueue.Namespace)
	if err != nil {
		logger.Error(err, "cannot list pull request queues")
		return
	}

	listOpts := &client.ListOptions{LabelSelector: labels.SelectorFromSet(c.getStateLabel(stateRunning))}
	runningPRQueues, err := c.listPullRequestQueues(listOpts, currentPRQueue.Namespace)
	if err != nil {
		logger.Error(err, "cannot list running pull request queues")
		return
	}

	runningPRQueues.Sort()
	updateList := make([]s2hv1beta1.PullRequestQueue, 0)

	// set order for all running queues
	count := 1
	if runningPRQueues != nil {
		for _, runningQueue := range runningPRQueues.Items {
			for i := range allPRQueues.Items {
				if allPRQueues.Items[i].Name == runningQueue.Name {
					if allPRQueues.Items[i].Spec.NoOfOrder != count {
						allPRQueues.Items[i].Spec.NoOfOrder = count
						updateList = append(updateList, allPRQueues.Items[i])
					}

					count++
					break
				}
			}
		}
	}

	// set order for the rest
	for i := range allPRQueues.Items {
		found := false
		if runningPRQueues != nil {
			for _, runningQueue := range runningPRQueues.Items {
				if allPRQueues.Items[i].Name == runningQueue.Name {
					found = true
					break
				}
			}
		}

		if !found {
			if allPRQueues.Items[i].Spec.NoOfOrder != count {
				allPRQueues.Items[i].Spec.NoOfOrder = count
				updateList = append(updateList, allPRQueues.Items[i])
			}
			count++
		}
	}

	for i := range updateList {
		if err := c.updatePullRequestQueue(ctx, &updateList[i]); err != nil {
			logger.Error(err, "cannot update pull request queue order", "queue", updateList[i].Name)
		}
	}
}
