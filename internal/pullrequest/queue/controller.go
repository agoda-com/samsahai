package queue

import (
	"context"
	"net/http"
	"time"

	"github.com/pkg/errors"
	"github.com/twitchtv/twirp"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crctrl "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	s2hv1 "github.com/agoda-com/samsahai/api/v1"
	"github.com/agoda-com/samsahai/internal"
	s2herrors "github.com/agoda-com/samsahai/internal/errors"
	s2hlog "github.com/agoda-com/samsahai/internal/log"
	"github.com/agoda-com/samsahai/internal/util/stringutils"
	samsahairpc "github.com/agoda-com/samsahai/pkg/samsahai/rpc"
)

var logger = s2hlog.Log.WithName(CtrlName)

const (
	CtrlName = "pull-request-queue-ctrl"

	pullRequestQueueFinalizer = "pullrequestqueue.finalizers.samsahai.io"
)

type controller struct {
	teamName  string
	client    client.Client
	scheme    *apiruntime.Scheme
	namespace string
	authToken string
	s2hClient samsahairpc.RPC
}

var _ internal.QueueController = &controller{}
var _ reconcile.Reconciler = &controller{}

type Option func(*controller)

func WithClient(client client.Client) Option {
	return func(c *controller) {
		c.client = client
	}
}

func NewPullRequestQueue(teamName, namespace, bundleName, prNumber, commitSHA string,
	comps []*s2hv1.QueueComponent) *s2hv1.PullRequestQueue {

	qLabels := getPullRequestQueueLabels(teamName, bundleName, prNumber)
	prQueueName := internal.GenPullRequestBundleName(bundleName, prNumber)

	return &s2hv1.PullRequestQueue{
		ObjectMeta: metav1.ObjectMeta{
			Name:      prQueueName,
			Namespace: namespace,
			Labels:    qLabels,
		},
		Spec: s2hv1.PullRequestQueueSpec{
			TeamName:           teamName,
			BundleName:         bundleName,
			PRNumber:           prNumber,
			CommitSHA:          commitSHA,
			Components:         comps,
			UpcomingCommitSHA:  commitSHA,
			UpcomingComponents: comps,
		},
		Status: s2hv1.PullRequestQueueStatus{},
	}
}

// New returns QueueController
func New(
	teamName string,
	ns string,
	mgr manager.Manager,
	authToken string,
	s2hClient samsahairpc.RPC,
	options ...Option,
) internal.QueueController {

	c := &controller{
		teamName:  teamName,
		namespace: ns,
		authToken: authToken,
		s2hClient: s2hClient,
	}

	if mgr != nil {
		c.client = mgr.GetClient()
		c.scheme = mgr.GetScheme()
		if err := add(mgr, c); err != nil {
			logger.Error(err, "cannot add new controller to manager")
		}
	}

	for _, opt := range options {
		opt(c)
	}

	return c
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
	err = c.Watch(&source.Kind{Type: &s2hv1.PullRequestQueue{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

func (c *controller) addFinalizer(prQueue *s2hv1.PullRequestQueue) {
	if prQueue.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object.
		if !stringutils.ContainsString(prQueue.ObjectMeta.Finalizers, pullRequestQueueFinalizer) {
			prQueue.ObjectMeta.Finalizers = append(prQueue.ObjectMeta.Finalizers, pullRequestQueueFinalizer)
		}
	}
}

func (c *controller) removeFinalizerObject(ctx context.Context, prQueue *s2hv1.PullRequestQueue) error {
	err := c.client.Get(ctx, types.NamespacedName{
		Name:      prQueue.Name,
		Namespace: c.namespace,
	}, &s2hv1.PullRequestQueue{})
	if err != nil && k8serrors.IsNotFound(err) {
		return nil
	}

	if stringutils.ContainsString(prQueue.ObjectMeta.Finalizers, pullRequestQueueFinalizer) {
		prQueue.ObjectMeta.Finalizers = stringutils.RemoveString(prQueue.ObjectMeta.Finalizers, pullRequestQueueFinalizer)
		if err := c.updatePullRequestQueue(ctx, prQueue); err != nil {
			return errors.Wrapf(err, "cannot remove finalizer of pull request queue %s", prQueue.Name)
		}
	}

	return nil
}

func (c *controller) deleteFinalizerWhenFinished(ctx context.Context, prQueue *s2hv1.PullRequestQueue) (
	skipReconcile bool, err error) {

	if prQueue.Status.State == s2hv1.PullRequestQueueFinished {
		if prQueue.ObjectMeta.DeletionTimestamp.IsZero() {
			logger.Debug("process has been finished and pull request queue has been deleted",
				"team", c.teamName, "bundle", prQueue.Spec.BundleName,
				"prNumber", prQueue.Spec.PRNumber)

			if err = c.deletePullRequestQueue(ctx, prQueue); err != nil {
				return
			}

			// skip updating pull request queue due to pull request queue is being deleted
			return true, err
		}
	}

	if !prQueue.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is being deleted
		if stringutils.ContainsString(prQueue.ObjectMeta.Finalizers, pullRequestQueueFinalizer) {
			nsObj := &corev1.Namespace{}
			err = c.client.Get(ctx, types.NamespacedName{Name: c.namespace}, nsObj)
			if err != nil && k8serrors.IsNotFound(err) || nsObj.Status.Phase == corev1.NamespaceTerminating {
				if err = c.removeFinalizerObject(ctx, prQueue); err != nil {
					return
				}
				return true, nil
			}

			if prQueue.Status.State == "" ||
				prQueue.Status.State == s2hv1.PullRequestQueueWaiting ||
				prQueue.Status.State == s2hv1.PullRequestQueueFinished {

				if err = c.removeFinalizerObject(ctx, prQueue); err != nil {
					return
				}

				// pull request queue has been finished
				skipReconcile = true
				return
			}

			if prQueue.IsCanceled() {
				return
			}

			prQueue.Status.SetResult(s2hv1.PullRequestQueueCanceled)
			prQueue.SetState(s2hv1.PullRequestQueueCollecting)
			if err = c.updatePullRequestQueue(ctx, prQueue); err != nil {
				return
			}

			skipReconcile = true
			return
		}
	}

	return
}

func (c *controller) setup(prQueue *s2hv1.PullRequestQueue) {
	logger.Info("pull request queue has been created", "team", c.teamName,
		"bundle", prQueue.Spec.BundleName, "prNumber", prQueue.Spec.PRNumber)

	c.appendStateLabel(prQueue, stateWaiting)
	prQueue.SetState(s2hv1.PullRequestQueueWaiting)

	logger.Info("pull request is waiting in queue", "team", c.teamName,
		"bundle", prQueue.Spec.BundleName, "prNumber", prQueue.Spec.PRNumber)
}

const (
	stateWaiting = "waiting"
	stateRunning = "running"
)

func (c *controller) appendStateLabel(prQueue *s2hv1.PullRequestQueue, state string) {
	if prQueue.Labels == nil {
		prQueue.Labels = make(map[string]string)
	}

	prQueue.Labels["state"] = state
}

func (c *controller) getStateLabel(state string) map[string]string {
	return map[string]string{"state": state}
}

func (c *controller) managePullRequestQueue(ctx context.Context, currentPRQueue *s2hv1.PullRequestQueue) (
	skipReconcile bool, err error) {

	listOpts := client.ListOptions{
		Namespace:     currentPRQueue.Namespace,
		LabelSelector: labels.SelectorFromSet(c.getStateLabel(stateRunning)),
	}
	runningPRQueues, err := c.listPullRequestQueues(&listOpts, currentPRQueue.Namespace)
	if err != nil {
		err = errors.Wrapf(err, "cannot list pull request queues, team: %s, namespace: %s",
			c.teamName, c.namespace)
		return
	}

	prConfig, err := c.s2hClient.GetPullRequestConfig(ctx, &samsahairpc.TeamWithBundleName{
		TeamName:   c.teamName,
		BundleName: currentPRQueue.Spec.BundleName,
	})
	if err != nil {
		return
	}

	prQueueConcurrences := int(prConfig.Concurrences)
	if len(runningPRQueues.Items) >= prQueueConcurrences {
		return
	}

	listOpts = client.ListOptions{
		Namespace:     currentPRQueue.Namespace,
		LabelSelector: labels.SelectorFromSet(c.getStateLabel(stateWaiting)),
	}
	waitingPRQueues, err := c.listPullRequestQueues(&listOpts, currentPRQueue.Namespace)
	if err != nil {
		err = errors.Wrapf(err, "cannot list pull request queues, team: %s, namespace: %s",
			c.teamName, c.namespace)
		return
	}

	// there is no new queue
	if len(waitingPRQueues.Items) == 0 {
		return
	}

	waitingPRQueues.Sort()

	if prQueueConcurrences-len(runningPRQueues.Items) > 0 {
		logger.Info("start running pull request queue", "team", c.teamName,
			"bundle", waitingPRQueues.Items[0].Name, "prNumber", waitingPRQueues.Items[0].Spec.PRNumber)

		c.addFinalizer(&waitingPRQueues.Items[0])
		c.appendStateLabel(&waitingPRQueues.Items[0], stateRunning)

		waitingPRQueues.Items[0].SetState(s2hv1.PullRequestQueueEnvCreating)
		waitingPRQueues.Items[0].Status.SetCondition(s2hv1.PullRequestQueueCondStarted, corev1.ConditionTrue,
			"Pull request queue has been started")
		waitingPRQueues.Items[0].Spec.Components = waitingPRQueues.Items[0].Spec.UpcomingComponents
		waitingPRQueues.Items[0].Spec.CommitSHA = waitingPRQueues.Items[0].Spec.UpcomingCommitSHA

		if err = c.updatePullRequestQueue(ctx, &waitingPRQueues.Items[0]); err != nil {
			return
		}

		// should not continue the process due to current pull request queue has been updated
		if waitingPRQueues.Items[0].Name == currentPRQueue.Name {
			skipReconcile = true
			return
		}
	}

	return
}

// Reconcile reads that state of the cluster for a PullRequestQueue object and makes changes based on the state read
// and what is in the PullRequestQueue.Spec
// +kubebuilder:rbac:groups=env.samsahai.io,resources=pullrequestqueues,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=env.samsahai.io,resources=pullrequestqueues/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=env.samsahai.io,resources=queuehistories,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=env.samsahai.io,resources=queuehistories/status,verbs=get;update;patch
func (c *controller) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	ctx := context.TODO()

	prQueue := &s2hv1.PullRequestQueue{}
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

	// make request with samsahai controller
	headers := make(http.Header)
	headers.Set(internal.SamsahaiAuthHeader, c.authToken)
	ctx, err = twirp.WithHTTPRequestHeaders(ctx, headers)
	if err != nil {
		logger.Error(err, "cannot set request header")
	}

	if skipReconcile, err := c.deleteFinalizerWhenFinished(ctx, prQueue); err != nil || skipReconcile {
		if err != nil && !k8serrors.IsNotFound(err) {
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	}

	if skipReconcile, err := c.managePullRequestQueue(ctx, prQueue); err != nil || skipReconcile {
		if err != nil {
			return reconcile.Result{}, err
		}
		// current pull request has been updated, it will re-trigger the reconcile
		return reconcile.Result{}, nil
	}

	switch prQueue.Status.State {
	case "":
		c.setup(prQueue)

	case s2hv1.PullRequestQueueWaiting:
		return reconcile.Result{
			Requeue:      true,
			RequeueAfter: 2 * time.Second,
		}, nil

	case s2hv1.PullRequestQueueEnvCreating:
		if err := c.createPullRequestEnvironment(ctx, prQueue); err != nil && !k8serrors.IsAlreadyExists(err) {
			if s2herrors.IsNamespaceStillCreating(err) {
				return reconcile.Result{
					Requeue:      true,
					RequeueAfter: 2 * time.Second,
				}, nil
			}

			return reconcile.Result{}, errors.Wrapf(err, "cannot create pull request environment")
		}

	case s2hv1.PullRequestQueueDeploying:
		if err := c.ensurePullRequestComponentsDeploying(ctx, prQueue); err != nil {
			if s2herrors.IsEnsuringComponentDeployed(err) {
				return reconcile.Result{
					Requeue:      true,
					RequeueAfter: 2 * time.Second,
				}, nil
			}

			return reconcile.Result{}, err
		}

	case s2hv1.PullRequestQueueTesting:
		if err := c.ensurePullRequestComponentsTesting(ctx, prQueue); err != nil {
			if s2herrors.IsEnsuringComponentTested(err) {
				return reconcile.Result{
					Requeue:      true,
					RequeueAfter: 2 * time.Second,
				}, nil
			}

			return reconcile.Result{}, err
		}

	case s2hv1.PullRequestQueueCollecting:
		if err := c.collectPullRequestQueueResult(ctx, prQueue); err != nil {
			return reconcile.Result{}, errors.Wrapf(err, "cannot collect pull request queue result")
		}

	case s2hv1.PullRequestQueueEnvDestroying:
		if skipReconcile, err := c.destroyPullRequestEnvironment(ctx, prQueue); err != nil || skipReconcile {
			if err != nil {
				if s2herrors.IsNamespaceStillExists(err) {
					return reconcile.Result{
						Requeue:      true,
						RequeueAfter: 2 * time.Second,
					}, nil
				}

				return reconcile.Result{}, errors.Wrapf(err, "cannot destroy pull request environment")
			}
			return reconcile.Result{}, nil
		}
	}

	if err := c.updatePullRequestQueue(ctx, prQueue); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}
