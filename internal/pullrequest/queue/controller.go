package queue

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/pkg/errors"
	"github.com/twitchtv/twirp"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crctrl "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	s2hv1beta1 "github.com/agoda-com/samsahai/api/v1beta1"
	"github.com/agoda-com/samsahai/internal"
	s2herrors "github.com/agoda-com/samsahai/internal/errors"
	s2hlog "github.com/agoda-com/samsahai/internal/log"
	samsahairpc "github.com/agoda-com/samsahai/pkg/samsahai/rpc"
)

var logger = s2hlog.Log.WithName(CtrlName)

const CtrlName = "pull-request-queue-ctrl"

type controller struct {
	teamName  string
	client    client.Client
	namespace string
	authToken string
	s2hClient samsahairpc.RPC
}

var _ internal.QueueController = &controller{}
var _ reconcile.Reconciler = &controller{}

func NewPullRequestQueue(teamName, namespace, componentName, prNumber string, comps []*s2hv1beta1.QueueComponent) *s2hv1beta1.PullRequestQueue {
	qLabels := getPullRequestQueueLabels(teamName, componentName, prNumber)
	prQueueName := internal.GenPullRequestComponentName(componentName, prNumber)

	return &s2hv1beta1.PullRequestQueue{
		ObjectMeta: metav1.ObjectMeta{
			Name:      prQueueName,
			Namespace: namespace,
			Labels:    qLabels,
		},
		Spec: s2hv1beta1.PullRequestQueueSpec{
			TeamName:          teamName,
			ComponentName:     componentName,
			PullRequestNumber: prNumber,
			Components:        comps,
		},
		Status: s2hv1beta1.PullRequestQueueStatus{},
	}
}

// New returns QueueController
func New(
	teamName string,
	ns string,
	mgr manager.Manager,
	authToken string,
	s2hClient samsahairpc.RPC,
) internal.QueueController {

	c := &controller{
		teamName:  teamName,
		namespace: ns,
		client:    mgr.GetClient(),
		authToken: authToken,
		s2hClient: s2hClient,
	}

	if err := add(mgr, c); err != nil {
		logger.Error(err, "cannot add new controller to manager")
	}

	return c
}

func (c *controller) Add(obj runtime.Object, priorityQueues []string) error {
	q, ok := obj.(*s2hv1beta1.PullRequestQueue)
	if !ok {
		return s2herrors.ErrParsingRuntimeObject
	}

	return c.add(context.TODO(), q, false)
}

func (c *controller) AddTop(obj runtime.Object) error {
	q, ok := obj.(*s2hv1beta1.PullRequestQueue)
	if !ok {
		return s2herrors.ErrParsingRuntimeObject
	}

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

func (c *controller) First() (runtime.Object, error) {
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

func (c *controller) Remove(obj runtime.Object) error {
	return c.client.Delete(context.TODO(), obj)
}

func (c *controller) RemoveAllQueues() error {
	return c.client.DeleteAllOf(context.TODO(), &s2hv1beta1.PullRequestQueue{}, client.InNamespace(c.namespace))
}

func (c *controller) add(ctx context.Context, queue *s2hv1beta1.PullRequestQueue, atTop bool) error {
	// TODO: pohfy, update here
	if err := c.client.Create(ctx, queue); err != nil {
		if k8serrors.IsAlreadyExists(err) {
			if err := c.client.Update(ctx, queue); err != nil {
				return err
			}

			return nil
		}
		return err
	}

	return nil
}

// queue always contains 1 component
func (c *controller) isMatchWithStableComponent(ctx context.Context, q *s2hv1beta1.Queue) (isMatch bool, err error) {
	if len(q.Spec.Components) == 0 {
		isMatch = true
		return
	}

	qComp := q.Spec.Components[0]
	stableComp := &s2hv1beta1.StableComponent{}
	err = c.client.Get(ctx, types.NamespacedName{Namespace: c.namespace, Name: qComp.Name}, stableComp)
	if err != nil && k8serrors.IsNotFound(err) {
		return false, nil
	} else if err != nil {
		return
	}

	isMatch = stableComp.Spec.Repository == qComp.Repository &&
		stableComp.Spec.Version == qComp.Version

	return
}

func (c *controller) getExistingQueueNumberInList(queue *s2hv1beta1.Queue, list *s2hv1beta1.QueueList) int {
	for i, q := range list.Items {
		for _, qComp := range q.Spec.Components {
			// queue already existed
			if qComp.Name == queue.Spec.Components[0].Name {
				return i
			}
		}
	}

	return -1
}

// removeAndUpdateSimilarQueue removes similar component/queue (same `component name` from queue) from QueueList
func (c *controller) removeAndUpdateSimilarQueue(queue *s2hv1beta1.Queue, list *s2hv1beta1.QueueList) (
	removing []s2hv1beta1.Queue, updating []s2hv1beta1.Queue) {

	updating = make([]s2hv1beta1.Queue, len(list.Items))
	var items []s2hv1beta1.Queue
	var containComp = false

	for i, q := range list.Items {
		if !containComp && q.ContainSameComponent(queue.Spec.Name, queue.Spec.Components[0]) {
			// only add one `queue` to items
			containComp = true
		} else {
			newComps := make([]*s2hv1beta1.QueueComponent, 0)
			for _, qComp := range q.Spec.Components {
				if qComp.Name == queue.Spec.Components[0].Name {
					continue
				}
				newComps = append(newComps, qComp)
			}

			if len(newComps) == 0 {
				removing = append(removing, q)
				continue
			}

			if len(newComps) != len(q.Spec.Components) {
				q.Spec.Components = newComps
				updating[i] = q
			}
		}

		items = append(items, q)
	}

	list.Items = items
	return
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

func (c *controller) SetLastOrder(obj runtime.Object) error {
	q, ok := obj.(*s2hv1beta1.Queue)
	if !ok {
		return s2herrors.ErrParsingRuntimeObject
	}

	queueList, err := c.list(nil)
	if err != nil {
		logger.Error(err, "cannot list queue")
		return err
	}

	q.Spec.NoOfOrder = queueList.LastQueueOrder()
	q.Status.Conditions = nil

	return c.client.Update(context.TODO(), q)
}

func (c *controller) SetReverifyQueueAtFirst(obj runtime.Object) error {
	q, ok := obj.(*s2hv1beta1.Queue)
	if !ok {
		return s2herrors.ErrParsingRuntimeObject
	}

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

func (c *controller) SetRetryQueue(obj runtime.Object, noOfRetry int, nextAt time.Time) error {
	q, ok := obj.(*s2hv1beta1.Queue)
	if !ok {
		return s2herrors.ErrParsingRuntimeObject
	}

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

func getQueueLabels(teamName, component string) map[string]string {
	qLabels := internal.GetDefaultLabels(teamName)
	qLabels["app"] = component
	qLabels["component"] = component

	return qLabels
}

func getPullRequestQueueLabels(teamName, component, prNumber string) map[string]string {
	qLabels := getQueueLabels(teamName, component)
	qLabels["pr-number"] = prNumber

	return qLabels
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := crctrl.New(CtrlName, mgr, crctrl.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to PullRequestQueue
	err = c.Watch(&source.Kind{Type: &s2hv1beta1.PullRequestQueue{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

const (
	stateWaiting = "waiting"
	stateRunning = "running"
)

func (c *controller) getStateLabel(state string) map[string]string {
	return map[string]string{"state": state}
}

func (c *controller) manageQueue(ctx context.Context, currentPRQueue *s2hv1beta1.PullRequestQueue) (bool, error) {
	runningPRQueues := s2hv1beta1.PullRequestQueueList{}
	listOpts := client.ListOptions{LabelSelector: labels.SelectorFromSet(c.getStateLabel(stateRunning))}
	if err := c.client.List(ctx, &runningPRQueues, &listOpts); err != nil {
		return false, errors.Wrap(err, "cannot list pullrequestqueues")
	}

	// TODO: pohfy, get concurrent pr queue from config
	concurrentPRQueue := 2
	if len(runningPRQueues.Items) >= concurrentPRQueue {
		return false, nil
	}

	waitingPRQueues := s2hv1beta1.PullRequestQueueList{}
	listOpts = client.ListOptions{LabelSelector: labels.SelectorFromSet(c.getStateLabel(stateWaiting))}
	if err := c.client.List(ctx, &waitingPRQueues, &listOpts); err != nil {
		return false, errors.Wrap(err, "cannot list pullrequestqueues")
	}

	// there is no new queue
	if len(waitingPRQueues.Items) == 0 {
		return false, nil
	}

	waitingPRQueues.SortASC()

	if concurrentPRQueue-len(runningPRQueues.Items) > 0 {
		logger.Info("start running pull request queue",
			"component", waitingPRQueues.Items[0].Name,
			"prNumber", waitingPRQueues.Items[0].Spec.PullRequestNumber)

		waitingPRQueues.Items[0].SetState(s2hv1beta1.PullRequestQueueRunning)
		// TODO: pohfy, set condition
		//runningPRQueues.Items[0].Status.SetCondition(s2hv1beta1.ActivePromotionCondStarted, corev1.ConditionTrue,
		//	"Active promotion has been started")
		waitingPRQueues.Items[0].Labels = c.getStateLabel(stateRunning)
		if err := c.updatePullRequestQueue(ctx, &waitingPRQueues.Items[0]); err != nil {
			return false, err
		}

		// should not continue the process due to current pull request queue has been updated
		if waitingPRQueues.Items[0].Name == currentPRQueue.Name {
			return true, nil
		}
	}

	return false, nil
}

func (c *controller) updatePullRequestQueue(ctx context.Context, prQueue *s2hv1beta1.PullRequestQueue) error {
	if err := c.client.Update(ctx, prQueue); err != nil {
		return errors.Wrapf(err, "cannot update pullrequestqueue %s", prQueue.Name)
	}

	return nil
}

func (c *controller) createPullRequestEnvironment(ctx context.Context, prQueue *s2hv1beta1.PullRequestQueue) error {
	prNamespace := fmt.Sprintf("%s%s-%s", internal.AppPrefix, c.teamName, prQueue.Name)
	_, err := c.s2hClient.CreatePullRequestEnvironment(ctx, &samsahairpc.TeamWithNamespace{
		TeamName:  c.teamName,
		Namespace: prNamespace,
	})
	if err != nil {
		return err
	}

	prQueue.SetPullRequestNamespace(prNamespace)

	// TODO: pohfy, check result, set condition and send to collecting state
	prQueue.SetResult(s2hv1beta1.PullRequestQueueSuccess)
	prQueue.SetState(s2hv1beta1.PullRequestQueueCollecting)

	return nil
}

func (c *controller) deletePullRequestQueue(ctx context.Context, prQueue *s2hv1beta1.PullRequestQueue) error {
	logger.Info("deleting pullrequestqueue",
		"component", prQueue.Spec.ComponentName, "prNumber", prQueue.Spec.PullRequestNumber)

	prNamespace := fmt.Sprintf("%s%s-%s", internal.AppPrefix, c.teamName, prQueue.Name)
	_, err := c.s2hClient.DestroyPullRequestEnvironment(ctx, &samsahairpc.TeamWithNamespace{
		TeamName:  c.teamName,
		Namespace: prNamespace,
	})
	if err != nil {
		return err
	}

	if err := c.client.Delete(ctx, prQueue); err != nil {
		if k8serrors.IsNotFound(err) {
			return nil
		}

		return errors.Wrapf(err, "cannot delete pullrequestqueue %s", prQueue.Name)
	}

	return nil
}

// Reconcile reads that state of the cluster for a PullRequestQueue object and makes changes based on the state read
// and what is in the PullRequestQueue.Spec
// +kubebuilder:rbac:groups=env.samsahai.io,resources=pullrequestqueues,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=env.samsahai.io,resources=pullrequestqueues/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=env.samsahai.io,resources=queuehistories,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=env.samsahai.io,resources=queuehistories/status,verbs=get;update;patch
func (c *controller) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	ctx := context.TODO()
	prQueue := &s2hv1beta1.PullRequestQueue{}
	err := c.client.Get(ctx, req.NamespacedName, prQueue)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			// Object not found, return. Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// TODO: pohfy, add finalizer, to check pr namespace, should be destroyed when got deletes

	if isTriggered, err := c.manageQueue(ctx, prQueue); err != nil || isTriggered {
		if err != nil {
			return reconcile.Result{}, err
		}
		// current pull request has been updated, it will re-trigger the reconcile
		return reconcile.Result{}, nil
	}

	// make request with samsahai controller
	headers := make(http.Header)
	headers.Set(internal.SamsahaiAuthHeader, c.authToken)
	ctx, err = twirp.WithHTTPRequestHeaders(ctx, headers)
	if err != nil {
		logger.Error(err, "cannot set request header")
	}

	switch prQueue.Status.State {
	case "":
		logger.Info("pull request queue has been created",
			"component", prQueue.Spec.ComponentName, "prNumber", prQueue.Spec.PullRequestNumber)
		prQueue.Labels = c.getStateLabel(stateWaiting)
		prQueue.SetState(s2hv1beta1.PullRequestQueueWaiting)
		logger.Info("pull request is waiting in queue",
			"component", prQueue.Spec.ComponentName, "prNumber", prQueue.Spec.PullRequestNumber)

	case s2hv1beta1.PullRequestQueueWaiting:
		return reconcile.Result{
			Requeue:      true,
			RequeueAfter: 2 * time.Second,
		}, nil

	case s2hv1beta1.PullRequestQueueRunning:
		// TODO: pohfy, check queue and start doing
		err = c.createPullRequestEnvironment(ctx, prQueue)
		if err != nil && !k8serrors.IsAlreadyExists(err) {
			if s2herrors.IsNamespaceStillCreating(err) {
				return reconcile.Result{
					Requeue:      true,
					RequeueAfter: 2 * time.Second,
				}, nil
			}

			return reconcile.Result{}, err
		}

	// TODO: pohfy, check queue in pr namespace is finished or not

	case s2hv1beta1.PullRequestQueueCollecting:
		prQueue.SetState(s2hv1beta1.PullRequestQueueFinished)

	case s2hv1beta1.PullRequestQueueFinished:
		// TODO: pohfy, recreate if failed
		if err := c.deletePullRequestQueue(ctx, prQueue); err != nil {
			return reconcile.Result{}, err
		}
	}

	if err := c.updatePullRequestQueue(ctx, prQueue); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}
