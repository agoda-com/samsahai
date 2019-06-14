package queue

import (
	"context"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/runtime/log"

	"github.com/agoda-com/samsahai/internal"
	"github.com/agoda-com/samsahai/internal/apis/env/v1beta1"
	"github.com/agoda-com/samsahai/internal/config"
)

const (
	LabelSelector = "managed-by=samsahai"
)

type controller struct {
	//client     client.Client
	restClient rest.Interface
	namespace  string
	log        logr.Logger
}

var _ internal.QueueController = &controller{}

func NewUpgradeQueue(namespace, name, repository, version string) *v1beta1.Queue {
	return &v1beta1.Queue{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"managed-by": "samsahai",
			},
		},
		Spec: v1beta1.QueueSpec{
			Name:       name,
			Repository: repository,
			Version:    version,
			NoOfRetry:  0,
			Type:       v1beta1.UPGRADE,
		},
		Status: v1beta1.QueueStatus{},
	}
}

func NewWithClient(namespace string, restClient rest.Interface) internal.QueueController {
	c := &controller{
		namespace:  namespace,
		restClient: restClient,
		log:        log.Log,
	}
	return c
}

func New(namespace string, cfg *rest.Config) internal.QueueController {
	log := log.Log.WithName("new queue controller")

	// register types at the scheme builder
	if err := v1beta1.AddToScheme(scheme.Scheme); err != nil {
		log.Error(err, "cannot addtoscheme")
		return nil
	}

	// create rest client
	restClient, err := rest.UnversionedRESTClientFor(config.GetRESTConfg(cfg, &v1beta1.SchemeGroupVersion))
	if err != nil {
		log.Error(err, "cannot create unversioned restclient")
		return nil
	}

	return NewWithClient(namespace, restClient)
}

func (c *controller) Add(q *v1beta1.Queue) error {
	return c.add(context.TODO(), q, false)
}

func (c *controller) AddTop(q *v1beta1.Queue) error {
	return c.add(context.TODO(), q, true)
}

func (c *controller) Size() int {
	list, err := c.List(nil)
	if err != nil {
		c.log.Error(err, "cannot list queue")
		return 0
	}
	return len(list.Items)
}

func (c *controller) First() (*v1beta1.Queue, error) {
	list, err := c.List(nil)
	if err != nil {
		c.log.Error(err, "cannot list queue")
		return nil, err
	}
	list.Sort()
	return list.First(), nil
}

func (c *controller) Remove(q *v1beta1.Queue) error {
	return c.Delete(q.Name, nil)
}

func (c *controller) RemoveAll() error {
	return c.DeleteCollection(nil, nil)
}

func (c *controller) add(ctx context.Context, queue *v1beta1.Queue, atTop bool) error {
	queueList, err := c.List(nil)
	if err != nil {
		return err
	}

	var pQueue *v1beta1.Queue
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

	for _, q := range removingList {
		if err := c.Delete(q.Name, nil); err != nil {
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

		if _, err := c.Update(pQueue); err != nil {
			return err
		}
	} else {
		// queue not exist
		if atTop {
			queue.Spec.NoOfOrder = queueList.TopQueueOrder()
		} else {
			queue.Spec.NoOfOrder = queueList.LastQueueOrder()
		}

		queue.Status.CreatedAt = &now
		queue.Status.UpdatedAt = &now

		if _, err := c.Create(queue); err != nil {
			return err
		}
	}

	return nil
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
