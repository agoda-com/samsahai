package queue

import (
	"context"
	"time"

	"github.com/pkg/errors"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	s2hv1beta1 "github.com/agoda-com/samsahai/api/v1beta1"
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

func NewUpgradeQueue(teamName, namespace, name, bundle string, comps []*s2hv1beta1.QueueComponent) *s2hv1beta1.Queue {
	qLabels := internal.GetDefaultLabels(teamName)
	qLabels["app"] = name
	qLabels["component"] = name
	return &s2hv1beta1.Queue{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    qLabels,
		},
		Spec: s2hv1beta1.QueueSpec{
			Name:       name,
			TeamName:   teamName,
			Bundle:     bundle,
			Components: comps,
			// TODO: pohfy, start remove here
			//Repository: repository,
			//Version:    version,
			// TODO: pohfy, end remove here
			Type: s2hv1beta1.QueueTypeUpgrade,
		},
		Status: s2hv1beta1.QueueStatus{},
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

func (c *controller) Add(q *s2hv1beta1.Queue) error {
	return c.add(context.TODO(), q, false)
}

func (c *controller) AddTop(q *s2hv1beta1.Queue) error {
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

func (c *controller) First() (*s2hv1beta1.Queue, error) {
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

func (c *controller) Remove(q *s2hv1beta1.Queue) error {
	return c.client.Delete(context.TODO(), q)
}

func (c *controller) RemoveAllQueues() error {

	return c.client.DeleteAllOf(context.TODO(), &s2hv1beta1.Queue{}, client.InNamespace(c.namespace))
}

// TODO: pohfy, check bundle and add to group instead
func (c *controller) add(ctx context.Context, queue *s2hv1beta1.Queue, atTop bool) error {
	queueList, err := c.list(nil)
	if err != nil {
		logger.Error(err, "cannot list queue")
		return err
	}

	if isMatch, err := c.isMatchWithStableComponent(ctx, queue); err != nil {
		return err
	} else if isMatch {
		return nil
	}

	bundleQueue := &s2hv1beta1.Queue{}
	pQueue := &s2hv1beta1.Queue{}
	isAlreadyInQueue := false
	isAlreadyInBundle := false
	for i, q := range queueList.Items {
		if q.IsInExistBundle(queue) {
			isAlreadyInBundle = true
			bundleQueue = &queueList.Items[i]
			break
		}

		// TODO: pohfy, need to be updated
		if q.IsSame(queue) {
			isAlreadyInQueue = true
			pQueue = &queueList.Items[i]
			break
		}
	}

	// remove duplicate component
	removingList := c.removeSimilar(queue, queueList)
	for i := range removingList {
		if err := c.client.Delete(ctx, &removingList[i]); err != nil {
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

		if err = c.client.Update(ctx, pQueue); err != nil {
			return err
		}
	} else {
		isNewQueue := true
		if isAlreadyInBundle {
			queue, isNewQueue = c.generateBundleQueue(queue, bundleQueue)
			// reset NoOfRetry/NextProcessAt if there is new component join the bundle
			queue.Spec.NoOfRetry = 0
			queue.Spec.NextProcessAt = nil
		}

		// queue not exist
		if atTop {
			queue.Spec.NoOfOrder = queueList.TopQueueOrder()
		} else {
			if isAlreadyInBundle {
				queue.Spec.NoOfOrder = bundleQueue.Spec.NoOfOrder
			} else {
				queue.Spec.NoOfOrder = queueList.LastQueueOrder()
			}
		}

		queue.Status.State = s2hv1beta1.Waiting
		queue.Status.CreatedAt = &now
		queue.Status.UpdatedAt = &now

		if isNewQueue {
			if err = c.client.Create(ctx, queue); err != nil {
				return err
			}

			// delete old queue if creating a new bundle queue
			if isAlreadyInBundle {
				if err = c.client.Delete(ctx, bundleQueue); err != nil {
					return err
				}
			}
			return nil
		}

		if err = c.client.Update(ctx, queue); err != nil {
			return err
		}
	}

	return nil
}

func (c *controller) isMatchWithStableComponent(ctx context.Context, q *s2hv1beta1.Queue) (isMatch bool, err error) {
	stableComp := &s2hv1beta1.StableComponent{}
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

func (c *controller) generateBundleQueue(targetQ *s2hv1beta1.Queue, existBundleQ *s2hv1beta1.Queue) (*s2hv1beta1.Queue, bool) {
	isNewBundle := false
	newBundleQueue := existBundleQ
	if existBundleQ.Name != existBundleQ.Spec.Bundle { // e.g. redis != db
		// create new bundle queue
		newBundleQueue = NewUpgradeQueue(existBundleQ.Spec.TeamName, existBundleQ.Namespace, existBundleQ.Spec.Bundle,
			existBundleQ.Spec.Bundle, existBundleQ.Spec.Components)
		isNewBundle = true
	}

	// append components if bundle exists
	for _, tcomp := range targetQ.Spec.Components {
		isFound := false
		for j, ecomp := range existBundleQ.Spec.Components {
			if ecomp.Name == tcomp.Name {
				isFound = true
				newBundleQueue.Spec.Components[j].Repository = tcomp.Repository
				newBundleQueue.Spec.Components[j].Version = tcomp.Version
				break
			}
		}
		if !isFound {
			newBundleQueue.Spec.Components = append(newBundleQueue.Spec.Components, tcomp)
		}
	}

	newBundleQueue.Spec.Components.Sort()

	return newBundleQueue, isNewBundle
}

// removeSimilar removes similar queue (same `name` from queue) from QueueList
func (c *controller) removeSimilar(queue *s2hv1beta1.Queue, list *s2hv1beta1.QueueList) []s2hv1beta1.Queue {
	var items []s2hv1beta1.Queue
	var removing []s2hv1beta1.Queue
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

func (c *controller) list(opts *client.ListOptions) (list *s2hv1beta1.QueueList, err error) {
	list = &s2hv1beta1.QueueList{}
	if opts == nil {
		opts = &client.ListOptions{Namespace: c.namespace}
	}
	if err = c.client.List(context.Background(), list, opts); err != nil {
		return list, errors.Wrapf(err, "cannot list queue with options: %+v", opts)
	}
	return list, nil
}

func (c *controller) SetLastOrder(q *s2hv1beta1.Queue) error {
	queueList, err := c.list(nil)
	if err != nil {
		logger.Error(err, "cannot list queue")
		return err
	}

	q.Spec.NoOfOrder = queueList.LastQueueOrder()
	q.Status.Conditions = nil

	return c.client.Update(context.TODO(), q)
}

func (c *controller) SetReverifyQueueAtFirst(q *s2hv1beta1.Queue) error {
	list, err := c.list(nil)
	if err != nil {
		logger.Error(err, "cannot list queue")
		return err
	}

	now := metav1.Now()
	q.Status = s2hv1beta1.QueueStatus{
		CreatedAt:     &now,
		NoOfProcessed: q.Status.NoOfProcessed,
		State:         s2hv1beta1.Waiting,
	}
	q.Spec.Type = s2hv1beta1.QueueTypeReverify
	q.Spec.NoOfOrder = list.TopQueueOrder()
	return c.client.Update(context.TODO(), q)
}

func (c *controller) SetRetryQueue(q *s2hv1beta1.Queue, noOfRetry int, nextAt time.Time) error {
	list, err := c.list(nil)
	if err != nil {
		logger.Error(err, "cannot list queue")
		return err
	}

	now := metav1.Now()
	q.Status = s2hv1beta1.QueueStatus{
		CreatedAt:     &now,
		NoOfProcessed: q.Status.NoOfProcessed,
		State:         s2hv1beta1.Waiting,
	}
	q.Spec.NextProcessAt = &metav1.Time{Time: nextAt}
	q.Spec.NoOfRetry = noOfRetry
	q.Spec.Type = s2hv1beta1.QueueTypeUpgrade
	q.Spec.NoOfOrder = list.LastQueueOrder()
	return c.client.Update(context.TODO(), q)
}

func (c *controller) updateQueueList(ql *s2hv1beta1.QueueList) error {
	for i := range ql.Items {
		if err := c.client.Update(context.TODO(), &ql.Items[i]); err != nil {
			logger.Error(err, "cannot update queue list", "queue", ql.Items[i].Name)
			return errors.Wrapf(err, "cannot update queue %s in %s", ql.Items[i].Name, ql.Items[i].Namespace)
		}
	}

	return nil
}

// resetQueueOrderWithCurrentQueue resets order of all queues to start with 1 respectively
func (c *controller) resetQueueOrderWithCurrentQueue(ql *s2hv1beta1.QueueList, currentQueue *s2hv1beta1.Queue) {
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
func EnsurePreActiveComponents(c client.Client, teamName, namespace string) (q *s2hv1beta1.Queue, err error) {
	q = &s2hv1beta1.Queue{
		ObjectMeta: metav1.ObjectMeta{
			Name:      string(s2hv1beta1.EnvPreActive),
			Namespace: namespace,
		},
		Spec: s2hv1beta1.QueueSpec{
			Type:     s2hv1beta1.QueueTypePreActive,
			TeamName: teamName,
		},
	}

	err = ensureQueue(context.TODO(), c, q)
	return
}

// EnsurePromoteToActiveComponents ensures that components were deployed with `active` config
func EnsurePromoteToActiveComponents(c client.Client, teamName, namespace string) (q *s2hv1beta1.Queue, err error) {
	q = &s2hv1beta1.Queue{
		ObjectMeta: metav1.ObjectMeta{
			Name:      string(s2hv1beta1.EnvActive),
			Namespace: namespace,
		},
		Spec: s2hv1beta1.QueueSpec{
			Type:     s2hv1beta1.QueueTypePromoteToActive,
			TeamName: teamName,
		},
	}
	err = ensureQueue(context.TODO(), c, q)
	return
}

// EnsureDemoteFromActiveComponents ensures that components were deployed without `active` config
func EnsureDemoteFromActiveComponents(c client.Client, teamName, namespace string) (q *s2hv1beta1.Queue, err error) {
	q = &s2hv1beta1.Queue{
		ObjectMeta: metav1.ObjectMeta{
			Name:      string(s2hv1beta1.EnvDeActive),
			Namespace: namespace,
		},
		Spec: s2hv1beta1.QueueSpec{
			Type:     s2hv1beta1.QueueTypeDemoteFromActive,
			TeamName: teamName,
		},
	}
	err = ensureQueue(context.TODO(), c, q)
	return
}

func DeletePreActiveQueue(c client.Client, ns string) error {
	return deleteQueue(c, ns, string(s2hv1beta1.EnvPreActive))
}

func DeletePromoteToActiveQueue(c client.Client, ns string) error {
	return deleteQueue(c, ns, string(s2hv1beta1.EnvActive))
}

func DeleteDemoteFromActiveQueue(c client.Client, ns string) error {
	return deleteQueue(c, ns, string(s2hv1beta1.EnvDeActive))
}

// deleteQueue removes Queue in target namespace by name
func deleteQueue(c client.Client, ns, name string) error {
	q := &s2hv1beta1.Queue{
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

func ensureQueue(ctx context.Context, c client.Client, q *s2hv1beta1.Queue) (err error) {
	fetched := &s2hv1beta1.Queue{}
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
