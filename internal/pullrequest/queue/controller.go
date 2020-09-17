package queue

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"github.com/twitchtv/twirp"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
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
	"github.com/agoda-com/samsahai/internal/queue"
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
	queueCtrl internal.QueueController
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
	client client.Client,
	authToken string,
	s2hClient samsahairpc.RPC,
) internal.QueueController {

	c := &controller{
		teamName:  teamName,
		namespace: ns,
		client:    client,
		authToken: authToken,
		s2hClient: s2hClient,
	}

	if err := add(mgr, c); err != nil {
		logger.Error(err, "cannot add new controller to manager")
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
	err = c.Watch(&source.Kind{Type: &s2hv1beta1.PullRequestQueue{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

func (c *controller) addFinalizer(prQueue *s2hv1beta1.PullRequestQueue) {
	if prQueue.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object.
		if !stringutils.ContainsString(prQueue.ObjectMeta.Finalizers, pullRequestQueueFinalizer) {
			prQueue.ObjectMeta.Finalizers = append(prQueue.ObjectMeta.Finalizers, pullRequestQueueFinalizer)
		}
	}
}

func (c *controller) removeFinalizerObject(ctx context.Context, prQueue *s2hv1beta1.PullRequestQueue) error {
	err := c.client.Get(ctx, types.NamespacedName{
		Name:      prQueue.Name,
		Namespace: c.namespace,
	}, &s2hv1beta1.PullRequestQueue{})
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

func (c *controller) deleteFinalizerWhenFinished(ctx context.Context, prQueue *s2hv1beta1.PullRequestQueue) (
	skipReconcile bool, err error) {

	if prQueue.Status.State == s2hv1beta1.PullRequestQueueFinished {
		if prQueue.ObjectMeta.DeletionTimestamp.IsZero() {
			logger.Debug("process has been finished and pull request queue has been deleted",
				"team", c.teamName, "component", prQueue.Spec.ComponentName,
				"prNumber", prQueue.Spec.PullRequestNumber)

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
			if prQueue.Status.State == "" ||
				prQueue.Status.State == s2hv1beta1.PullRequestQueueWaiting ||
				prQueue.Status.State == s2hv1beta1.PullRequestQueueFinished {

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

			prQueue.Status.SetResult(s2hv1beta1.PullRequestQueueCanceled)
			prQueue.SetState(s2hv1beta1.PullRequestQueueCollecting)
			if err = c.updatePullRequestQueue(ctx, prQueue); err != nil {
				return
			}

			skipReconcile = true
			return
		}
	}

	return
}

func (c *controller) setup(prQueue *s2hv1beta1.PullRequestQueue) {
	logger.Info("pull request queue has been created", "team", c.teamName,
		"component", prQueue.Spec.ComponentName, "prNumber", prQueue.Spec.PullRequestNumber)

	prQueue.Labels = c.getStateLabel(stateWaiting)
	prQueue.SetState(s2hv1beta1.PullRequestQueueWaiting)

	logger.Info("pull request is waiting in queue", "team", c.teamName,
		"component", prQueue.Spec.ComponentName, "prNumber", prQueue.Spec.PullRequestNumber)
}

const (
	stateWaiting = "waiting"
	stateRunning = "running"
)

func (c *controller) getStateLabel(state string) map[string]string {
	return map[string]string{"state": state}
}

func (c *controller) manageQueue(ctx context.Context, currentPRQueue *s2hv1beta1.PullRequestQueue) (
	skipReconcile bool, err error) {

	listOpts := client.ListOptions{LabelSelector: labels.SelectorFromSet(c.getStateLabel(stateRunning))}
	runningPRQueues, err := c.listPullRequestQueues(&listOpts, currentPRQueue.Namespace)
	if err != nil {
		err = errors.Wrap(err, "cannot list pull request queues")
		return
	}

	prConfig, err := c.s2hClient.GetPullRequestConfig(ctx, &samsahairpc.TeamName{Name: c.teamName})
	if err != nil {
		return
	}

	concurrentPRQueue := int(prConfig.Parallel)
	if len(runningPRQueues.Items) >= concurrentPRQueue {
		return
	}

	listOpts = client.ListOptions{LabelSelector: labels.SelectorFromSet(c.getStateLabel(stateWaiting))}
	waitingPRQueues, err := c.listPullRequestQueues(&listOpts, currentPRQueue.Namespace)
	if err != nil {
		err = errors.Wrap(err, "cannot list pull request queues")
		return
	}

	// there is no new queue
	if len(waitingPRQueues.Items) == 0 {
		return
	}

	waitingPRQueues.Sort()

	if concurrentPRQueue-len(runningPRQueues.Items) > 0 {
		logger.Info("start running pull request queue", "team", c.teamName,
			"component", waitingPRQueues.Items[0].Name, "prNumber", waitingPRQueues.Items[0].Spec.PullRequestNumber)

		c.addFinalizer(&waitingPRQueues.Items[0])
		waitingPRQueues.Items[0].SetState(s2hv1beta1.PullRequestQueueEnvCreating)
		waitingPRQueues.Items[0].Status.SetCondition(s2hv1beta1.PullRequestQueueCondStarted, corev1.ConditionTrue,
			"Pull request queue has been started")
		waitingPRQueues.Items[0].Labels = c.getStateLabel(stateRunning)
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

func (c *controller) ensurePullRequestNamespaceReady(ctx context.Context, ns string) error {
	prNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: ns,
		},
	}

	if err := c.client.Get(ctx, types.NamespacedName{Name: ns}, prNamespace); err != nil {
		return errors.Wrapf(err, "cannot get namespace %s", ns)
	}

	return nil
}

func (c *controller) createPullRequestEnvironment(ctx context.Context, prQueue *s2hv1beta1.PullRequestQueue) error {
	prNamespace := fmt.Sprintf("%s%s-%s", internal.AppPrefix, c.teamName, prQueue.Name)
	_, err := c.s2hClient.CreatePullRequestEnvironment(ctx, &samsahairpc.TeamWithPullRequest{
		TeamName:      c.teamName,
		Namespace:     prNamespace,
		ComponentName: prQueue.Spec.ComponentName,
	})
	if err != nil {
		return err
	}

	if err := c.ensurePullRequestNamespaceReady(ctx, prNamespace); err != nil {
		logger.Warn("cannot ensure pull request namespace created", "error", err.Error())
		return s2herrors.ErrTeamNamespaceStillCreating
	}

	prQueue.Status.SetPullRequestNamespace(prNamespace)
	prQueue.Status.SetCondition(s2hv1beta1.PullRequestQueueCondEnvCreated, corev1.ConditionTrue,
		"Pull request environment has been created")
	prQueue.SetState(s2hv1beta1.PullRequestQueueDeploying)

	return nil
}

func (c *controller) destroyPullRequestEnvironment(ctx context.Context, prQueue *s2hv1beta1.PullRequestQueue) (
	skipReconcile bool, err error) {

	prNamespace := prQueue.Status.PullRequestNamespace
	if err = queue.DeletePullRequestQueue(c.client, prNamespace, prQueue.Name); err != nil {
		return
	}

	_, err = c.s2hClient.DestroyPullRequestEnvironment(ctx, &samsahairpc.TeamWithNamespace{
		TeamName:  c.teamName,
		Namespace: prNamespace,
	})
	if err != nil {
		return
	}

	prQueue.Status.SetCondition(s2hv1beta1.PullRequestQueueCondEnvDestroyed, corev1.ConditionTrue,
		"Pull request environment has been destroyed")

	prConfig, err := c.s2hClient.GetPullRequestConfig(ctx, &samsahairpc.TeamName{Name: c.teamName})
	if err != nil {
		return
	}

	if prQueue.Status.Result == s2hv1beta1.PullRequestQueueFailure {
		maxRetryQueue := int(prConfig.MaxRetry)
		if prQueue.Spec.NoOfRetry < maxRetryQueue {
			prQueue.Spec.NoOfRetry++
			if err = c.SetRetryQueue(prQueue, prQueue.Spec.NoOfRetry, time.Now()); err != nil {
				err = errors.Wrapf(err, "cannot set retry pull request queue")
				return
			}

			c.resetQueueOrderWithRunningQueue(ctx, prQueue)
			skipReconcile = true
			return
		}
	}

	prQueue.SetState(s2hv1beta1.PullRequestQueueFinished)

	return
}

func (c *controller) getDeploymentQueue(ctx context.Context, queueName, prNamespace string) (*s2hv1beta1.Queue, error) {
	deployedQueue := &s2hv1beta1.Queue{}
	err := c.client.Get(ctx, types.NamespacedName{
		Namespace: prNamespace,
		Name:      queueName,
	}, deployedQueue)

	return deployedQueue, err
}

func (c *controller) updatePullRequestComponentDependenciesVersion(ctx context.Context, teamName, prCompName string,
	prComps *s2hv1beta1.QueueComponents) error {

	if prComps == nil {
		return nil
	}

	prDependencies, err := c.s2hClient.GetPullRequestComponentDependencies(ctx, &samsahairpc.TeamWithComponentName{
		TeamName:      teamName,
		ComponentName: prCompName,
	})
	if err != nil {
		return err
	}

	for _, prDep := range prDependencies.Dependencies {
		imgRepo := prDep.Image.Repository
		imgTag := prDep.Image.Tag
		depFound := false
		for i := range *prComps {
			if (*prComps)[i].Name == prDep.Name {
				depFound = true
				(*prComps)[i].Repository = imgRepo
				(*prComps)[i].Version = imgTag
			}
		}

		if !depFound {
			*prComps = append(*prComps, &s2hv1beta1.QueueComponent{
				Name:       prDep.Name,
				Repository: imgRepo,
				Version:    imgTag,
			})
		}
	}

	return nil
}

func (c *controller) ensurePullRequestComponentsDeploying(ctx context.Context, prQueue *s2hv1beta1.PullRequestQueue) error {
	prComps := prQueue.Spec.Components
	prNamespace := prQueue.Status.PullRequestNamespace

	err := c.updatePullRequestComponentDependenciesVersion(ctx, c.teamName, prQueue.Spec.ComponentName, &prComps)
	if err != nil {
		return err
	}

	deployedQueue, err := queue.EnsurePullRequestComponents(c.client, c.teamName, prNamespace, prQueue.Name, prComps)
	if err != nil {
		return errors.Wrapf(err, "cannot ensure pull request components, namespace %s", prNamespace)
	}

	if deployedQueue.Status.State == s2hv1beta1.Finished || // in case of queue state was finished without deploying
		(deployedQueue.Status.StartDeployTime != nil && deployedQueue.Status.State != s2hv1beta1.Creating) {
		if deployedQueue.IsDeploySuccess() {
			// in case successful deployment
			logger.Debug("components has been deployed successfully",
				"team", c.teamName, "component", prQueue.Spec.ComponentName,
				"prNumber", prQueue.Spec.PullRequestNumber)
			prQueue.Status.SetCondition(s2hv1beta1.PullRequestQueueCondDeployed, corev1.ConditionTrue,
				"Components have been deployed successfully")
			prQueue.SetState(s2hv1beta1.PullRequestQueueTesting)
		}

		// in case failure deployment
		prQueue.Status.SetResult(s2hv1beta1.PullRequestQueueFailure)
		prQueue.Status.SetCondition(s2hv1beta1.PullRequestQueueCondDeployed, corev1.ConditionFalse,
			"Deployment failed")
		prQueue.Status.SetCondition(s2hv1beta1.PullRequestQueueCondTested, corev1.ConditionTrue,
			"Skipped running test due to deployment failed")
		prQueue.SetState(s2hv1beta1.PullRequestQueueCollecting)

		return nil
	}

	return s2herrors.ErrEnsureComponentDeployed
}

func (c *controller) ensurePullRequestComponentsTesting(ctx context.Context, prQueue *s2hv1beta1.PullRequestQueue) error {
	prComps := prQueue.Spec.Components
	prNamespace := prQueue.Status.PullRequestNamespace
	deployedQueue, err := queue.EnsurePullRequestComponents(c.client, c.teamName, prNamespace, prQueue.Name, prComps)
	if err != nil {
		return errors.Wrapf(err, "cannot ensure pull request components, namespace %s", prNamespace)
	}

	if deployedQueue.Status.State == s2hv1beta1.Finished || // in case of queue state was finished without deploying
		(deployedQueue.Status.StartDeployTime != nil && deployedQueue.Status.State != s2hv1beta1.Creating) {
		if deployedQueue.IsTestSuccess() {
			// in case successful test
			logger.Debug("components have been tested successfully",
				"team", c.teamName, "component", prQueue.Spec.ComponentName,
				"prNumber", prQueue.Spec.PullRequestNumber)
			prQueue.Status.SetResult(s2hv1beta1.PullRequestQueueSuccess)
			prQueue.Status.SetCondition(s2hv1beta1.PullRequestQueueCondTested, corev1.ConditionTrue,
				"Components have been tested successfully")
		}

		// in case failure test
		prQueue.Status.SetResult(s2hv1beta1.PullRequestQueueFailure)
		prQueue.Status.SetCondition(s2hv1beta1.PullRequestQueueCondTested, corev1.ConditionFalse,
			"Test failed")
		prQueue.SetState(s2hv1beta1.PullRequestQueueCollecting)

		return nil
	}

	return s2herrors.ErrEnsureComponentTested
}

func (c *controller) collectPullRequestQueueResult(ctx context.Context, prQueue *s2hv1beta1.PullRequestQueue) error {
	prComps := prQueue.Spec.Components
	prNamespace := prQueue.Status.PullRequestNamespace
	deployedQueue, err := queue.EnsurePullRequestComponents(c.client, c.teamName, prNamespace, prQueue.Name, prComps)
	if err != nil {
		return errors.Wrapf(err, "cannot ensure pull request components, namespace %s", prNamespace)
	}

	prQueue.SetState(s2hv1beta1.PullRequestQueueEnvDestroying)
	prQueue.Status.SetDeploymentQueue(deployedQueue)
	prQueue.Status.SetCondition(s2hv1beta1.PullRequestQueueCondResultCollected, corev1.ConditionTrue,
		"Pull request queue result has been collected")

	prQueueHistName := generateHistoryName(prQueue.Name, prQueue.CreationTimestamp, prQueue.Spec.NoOfRetry)
	if _, err := c.getPullRequestQueueHistory(ctx, prQueueHistName); err != nil {
		if k8serrors.IsNotFound(err) {
			if err := c.createPullRequestQueueHistory(ctx, prQueue); err != nil {
				return err
			}

			// TODO: pohfy, send notification
			prQueue.Status.SetPullRequestQueueHistoryName(prQueueHistName)
			return nil
		}

		return err
	}

	return nil
}

func (c *controller) getPullRequestQueueHistory(ctx context.Context, prQueueHistName string) (*s2hv1beta1.PullRequestQueueHistory, error) {
	prQueueHist := &s2hv1beta1.PullRequestQueueHistory{}
	err := c.client.Get(ctx, types.NamespacedName{
		Namespace: c.namespace,
		Name:      prQueueHistName,
	}, prQueueHist)
	if err != nil {
		return nil, err
	}

	return prQueueHist, nil
}

func (c *controller) createPullRequestQueueHistory(ctx context.Context, prQueue *s2hv1beta1.PullRequestQueue) error {
	prQueueLabels := getPullRequestQueueLabels(c.teamName, prQueue.Spec.ComponentName, prQueue.Spec.PullRequestNumber)

	if err := c.deletePullRequestQueueHistoryOutOfRange(ctx); err != nil {
		return err
	}

	history := &s2hv1beta1.PullRequestQueueHistory{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateHistoryName(prQueue.Name, prQueue.CreationTimestamp, prQueue.Spec.NoOfRetry),
			Namespace: c.namespace,
			Labels:    prQueueLabels,
		},
		Spec: s2hv1beta1.PullRequestQueueHistorySpec{
			PullRequestQueue: &s2hv1beta1.PullRequestQueue{
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
	prQueueHists := s2hv1beta1.PullRequestQueueHistoryList{}
	if err := c.client.List(ctx, &prQueueHists, &client.ListOptions{Namespace: c.namespace}); err != nil {
		if k8serrors.IsNotFound(err) {
			return nil
		}

		return errors.Wrapf(err, "cannot list pull request queue histories of namespace: %s", c.namespace)
	}

	prConfig, err := c.s2hClient.GetPullRequestConfig(ctx, &samsahairpc.TeamName{Name: c.teamName})
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

	if skipReconcile, err := c.manageQueue(ctx, prQueue); err != nil || skipReconcile {
		if err != nil {
			return reconcile.Result{}, err
		}
		// current pull request has been updated, it will re-trigger the reconcile
		return reconcile.Result{}, nil
	}

	switch prQueue.Status.State {
	case "":
		c.setup(prQueue)

	case s2hv1beta1.PullRequestQueueWaiting:
		return reconcile.Result{
			Requeue:      true,
			RequeueAfter: 2 * time.Second,
		}, nil

	case s2hv1beta1.PullRequestQueueEnvCreating:
		if err := c.createPullRequestEnvironment(ctx, prQueue); err != nil && !k8serrors.IsAlreadyExists(err) {
			if s2herrors.IsNamespaceStillCreating(err) {
				return reconcile.Result{
					Requeue:      true,
					RequeueAfter: 2 * time.Second,
				}, nil
			}

			return reconcile.Result{}, err
		}

	case s2hv1beta1.PullRequestQueueDeploying:
		if err := c.ensurePullRequestComponentsDeploying(ctx, prQueue); err != nil {
			if s2herrors.IsEnsuringComponentDeployed(err) {
				return reconcile.Result{
					Requeue:      true,
					RequeueAfter: 2 * time.Second,
				}, nil
			}

			return reconcile.Result{}, err
		}

	case s2hv1beta1.PullRequestQueueTesting:
		if err := c.ensurePullRequestComponentsTesting(ctx, prQueue); err != nil {
			if s2herrors.IsEnsuringComponentTested(err) {
				return reconcile.Result{
					Requeue:      true,
					RequeueAfter: 2 * time.Second,
				}, nil
			}

			return reconcile.Result{}, err
		}

	case s2hv1beta1.PullRequestQueueCollecting:
		if err := c.collectPullRequestQueueResult(ctx, prQueue); err != nil {
			return reconcile.Result{}, err
		}

	case s2hv1beta1.PullRequestQueueEnvDestroying:
		if skipReconcile, err := c.destroyPullRequestEnvironment(ctx, prQueue); err != nil || skipReconcile {
			if err != nil {
				if s2herrors.IsNamespaceStillExists(err) {
					return reconcile.Result{
						Requeue:      true,
						RequeueAfter: 2 * time.Second,
					}, nil
				}

				return reconcile.Result{}, err
			}
			return reconcile.Result{}, nil
		}
	}

	if err := c.updatePullRequestQueue(ctx, prQueue); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}