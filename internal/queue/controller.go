package queue

import (
	"context"
	"time"

	"github.com/pkg/errors"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/agoda-com/samsahai/api/v1beta1"
	"github.com/agoda-com/samsahai/internal"
	s2hlog "github.com/agoda-com/samsahai/internal/log"
)

var logger = s2hlog.Log.WithName(CtrlName)

const CtrlName = "queue-ctrl"

type controller struct {
	client    client.Client
	namespace string
}

var _ internal.QueueController = &controller{}

func NewUpgradeQueue(teamName, namespace, name, repository, version string) *v1beta1.Queue {
	qLabels := internal.GetDefaultLabels(teamName)
	qLabels["app"] = name
	qLabels["component"] = name
	return &v1beta1.Queue{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    qLabels,
		},
		Spec: v1beta1.QueueSpec{
			Name:       name,
			TeamName:   teamName,
			Repository: repository,
			Version:    version,
			Type:       v1beta1.QueueTypeUpgrade,
		},
		Status: v1beta1.QueueStatus{},
	}
}

// New returns QueueController
func New(ns string, runtimeClient client.Client) internal.QueueController {
	c := &controller{
		namespace: ns,
		client:    runtimeClient,
	}
	return c
}

func (c *controller) Add(q *v1beta1.Queue) error {
	return c.add(context.TODO(), q, false)
}

func (c *controller) AddTop(q *v1beta1.Queue) error {
	return c.add(context.TODO(), q, true)
}

func (c *controller) Size() int {
	list, err := c.list(nil)
	if err != nil {
		logger.Error(err, "cannot list queue")
		return 0
	}
	return len(list.Items)
}

func (c *controller) First() (*v1beta1.Queue, error) {
	list, err := c.list(nil)
	if err != nil {
		logger.Error(err, "cannot list queue")
		return nil, err
	}

	q := list.First()
	c.resetQueueOrderWithCurrentQueue(list, q)
	if err := c.updateQueueList(list); err != nil {
		return nil, err
	}

	if q == nil {
		return nil, nil
	}

	if q.Spec.NextProcessAt == nil || time.Now().After(q.Spec.NextProcessAt.Time) {
		return q, nil
	}

	return nil, nil
}

func (c *controller) Remove(q *v1beta1.Queue) error {
	return c.client.Delete(context.TODO(), q)
}

func (c *controller) RemoveAllQueues() error {

	return c.client.DeleteAllOf(context.TODO(), &v1beta1.Queue{}, client.InNamespace(c.namespace))
}

func (c *controller) add(ctx context.Context, queue *v1beta1.Queue, atTop bool) error {
	queueList, err := c.list(nil)
	if err != nil {
		return err
	}

	if isMatch, err := c.isMatchWithStableComponent(ctx, queue); err != nil {
		return err
	} else if isMatch {
		return nil
	}

	pQueue := &v1beta1.Queue{}
	isAlreadyInQueue := false
	for i, q := range queueList.Items {
		if q.IsSame(queue) {
			isAlreadyInQueue = true
			pQueue = &queueList.Items[i]
			break
		}
	}

	// remove duplicate component
	removingList := c.removeSimilar(queue, queueList)

	for i := range removingList {
		if err := c.client.Delete(context.TODO(), &removingList[i]); err != nil {
			return err
		}
	}

	queueList.Sort()

	now := metav1.Now()
	if isAlreadyInQueue {
		isQueueOnTop := pQueue.IsSame(&queueList.Items[0])

		// move queue to the top
		if atTop && !isQueueOnTop {
			pQueue.Spec.NoOfOrder = queueList.TopQueueOrder()
		}

		if err = c.client.Update(context.TODO(), pQueue); err != nil {
			return err
		}
	} else {
		// queue not exist
		if atTop {
			queue.Spec.NoOfOrder = queueList.TopQueueOrder()
		} else {
			queue.Spec.NoOfOrder = queueList.LastQueueOrder()
		}

		queue.Status.State = v1beta1.Waiting
		queue.Status.CreatedAt = &now
		queue.Status.UpdatedAt = &now

		if err = c.client.Create(context.TODO(), queue); err != nil {
			return err
		}
	}

	return nil
}

func (c *controller) isMatchWithStableComponent(ctx context.Context, q *v1beta1.Queue) (isMatch bool, err error) {
	stableComp := &v1beta1.StableComponent{}
	err = c.client.Get(ctx, types.NamespacedName{Namespace: c.namespace, Name: q.GetName()}, stableComp)
	if err != nil && k8serrors.IsNotFound(err) {
		return false, nil
	} else if err != nil {
		return
	}

	isMatch = stableComp.Spec.Repository == q.Spec.Repository &&
		stableComp.Spec.Version == q.Spec.Version

	return
}

// removeSimilar removes similar queue (same `name` from queue) from QueueList
func (c *controller) removeSimilar(queue *v1beta1.Queue, list *v1beta1.QueueList) []v1beta1.Queue {
	var items []v1beta1.Queue
	var removing []v1beta1.Queue
	var hasSameQueue = false

	for _, q := range list.Items {
		if !hasSameQueue && q.IsSame(queue) {
			// only add one `queue` to items
			hasSameQueue = true
		} else if q.Spec.Name == queue.Spec.Name {
			// remove all the name with `queue`
			removing = append(removing, q)
			continue
		}
		items = append(items, q)
	}
	list.Items = items
	return removing
}

func (c *controller) list(opts *client.ListOptions) (list *v1beta1.QueueList, err error) {
	list = &v1beta1.QueueList{}
	if opts == nil {
		opts = &client.ListOptions{Namespace: c.namespace}
	}
	if err = c.client.List(context.Background(), list, opts); err != nil {
		return
	}
	return list, nil
}

func (c *controller) SetLastOrder(q *v1beta1.Queue) error {
	queueList, err := c.list(nil)
	if err != nil {
		return err
	}

	q.Spec.NoOfOrder = queueList.LastQueueOrder()
	q.Status.Conditions = nil

	return c.client.Update(context.TODO(), q)
}

func (c *controller) SetReverifyQueueAtFirst(q *v1beta1.Queue) error {
	list, err := c.list(nil)
	if err != nil {
		return err
	}

	now := metav1.Now()
	q.Status = v1beta1.QueueStatus{
		CreatedAt:     &now,
		NoOfProcessed: q.Status.NoOfProcessed,
		State:         v1beta1.Waiting,
	}
	q.Spec.Type = v1beta1.QueueTypeReverify
	q.Spec.NoOfOrder = list.TopQueueOrder()
	return c.client.Update(context.TODO(), q)
}

func (c *controller) SetRetryQueue(q *v1beta1.Queue, noOfRetry int, nextAt time.Time) error {
	list, err := c.list(nil)
	if err != nil {
		return err
	}

	now := metav1.Now()
	q.Status = v1beta1.QueueStatus{
		CreatedAt:     &now,
		NoOfProcessed: q.Status.NoOfProcessed,
		State:         v1beta1.Waiting,
	}
	q.Spec.NextProcessAt = &metav1.Time{Time: nextAt}
	q.Spec.NoOfRetry = noOfRetry
	q.Spec.Type = v1beta1.QueueTypeUpgrade
	q.Spec.NoOfOrder = list.LastQueueOrder()
	return c.client.Update(context.TODO(), q)
}

func (c *controller) updateQueueList(ql *v1beta1.QueueList) error {
	for i := range ql.Items {
		if err := c.client.Update(context.TODO(), &ql.Items[i]); err != nil {
			return errors.Wrapf(err, "cannot update queue %s in %s", ql.Items[i].Name, ql.Items[i].Namespace)
		}
	}

	return nil
}

// resetQueueOrderWithCurrentQueue resets order of all queues to start with 1 respectively
func (c *controller) resetQueueOrderWithCurrentQueue(ql *v1beta1.QueueList, currentQueue *v1beta1.Queue) {
	ql.Sort()
	count := 2
	for i := range ql.Items {
		if ql.Items[i].Name == currentQueue.Name {
			ql.Items[i].Spec.NoOfOrder = 1
			continue
		}
		ql.Items[i].Spec.NoOfOrder = count
		count++
	}
}

// EnsurePreActiveComponents ensures that components with were deployed with `pre-active` config and tested
func EnsurePreActiveComponents(c client.Client, teamName, namespace string) (q *v1beta1.Queue, err error) {
	q = &v1beta1.Queue{
		ObjectMeta: metav1.ObjectMeta{
			Name:      internal.EnvPreActive,
			Namespace: namespace,
		},
		Spec: v1beta1.QueueSpec{
			Type:     v1beta1.QueueTypePreActive,
			TeamName: teamName,
		},
	}

	err = ensureQueue(context.TODO(), c, q)
	return
}

// EnsurePromoteToActiveComponents ensures that components were deployed with `active` config
func EnsurePromoteToActiveComponents(c client.Client, teamName, namespace string) (q *v1beta1.Queue, err error) {
	q = &v1beta1.Queue{
		ObjectMeta: metav1.ObjectMeta{
			Name:      internal.EnvActive,
			Namespace: namespace,
		},
		Spec: v1beta1.QueueSpec{
			Type:     v1beta1.QueueTypePromoteToActive,
			TeamName: teamName,
		},
	}
	err = ensureQueue(context.TODO(), c, q)
	return
}

// EnsureDemoteFromActiveComponents ensures that components were deployed without `active` config
func EnsureDemoteFromActiveComponents(c client.Client, teamName, namespace string) (q *v1beta1.Queue, err error) {
	q = &v1beta1.Queue{
		ObjectMeta: metav1.ObjectMeta{
			Name:      internal.EnvDeActive,
			Namespace: namespace,
		},
		Spec: v1beta1.QueueSpec{
			Type:     v1beta1.QueueTypeDemoteFromActive,
			TeamName: teamName,
		},
	}
	err = ensureQueue(context.TODO(), c, q)
	return
}

func DeletePreActiveQueue(c client.Client, ns string) error {
	return deleteQueue(c, ns, internal.EnvPreActive)
}

func DeletePromoteToActiveQueue(c client.Client, ns string) error {
	return deleteQueue(c, ns, internal.EnvActive)
}

func DeleteDemoteFromActiveQueue(c client.Client, ns string) error {
	return deleteQueue(c, ns, internal.EnvDeActive)
}

// deleteQueue removes Queue in target namespace by name
func deleteQueue(c client.Client, ns, name string) error {
	q := &v1beta1.Queue{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
	}
	err := c.Delete(context.TODO(), q)
	if err != nil && k8serrors.IsNotFound(err) {
		return nil
	}

	return errors.Wrap(err, "cannot delete queue")
}

func ensureQueue(ctx context.Context, c client.Client, q *v1beta1.Queue) (err error) {
	fetched := &v1beta1.Queue{}
	err = c.Get(ctx, types.NamespacedName{Namespace: q.Namespace, Name: q.Name}, fetched)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			// Create
			return c.Create(ctx, q)
		}
		return err
	}
	q.Spec = fetched.Spec
	q.Status = fetched.Status
	return nil
}
