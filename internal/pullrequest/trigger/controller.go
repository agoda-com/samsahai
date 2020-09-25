package trigger

import (
	"context"
	"net/http"
	"time"

	"github.com/twitchtv/twirp"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crctrl "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	s2hv1beta1 "github.com/agoda-com/samsahai/api/v1beta1"
	"github.com/agoda-com/samsahai/internal"
	"github.com/agoda-com/samsahai/internal/errors"
	s2hlog "github.com/agoda-com/samsahai/internal/log"
	prqueuectrl "github.com/agoda-com/samsahai/internal/pullrequest/queue"
	samsahairpc "github.com/agoda-com/samsahai/pkg/samsahai/rpc"
)

const (
	CtrlName = "pull-request-trigger-ctrl"
)

var logger = s2hlog.Log.WithName(CtrlName)

type controller struct {
	teamName    string
	client      client.Client
	prQueueCtrl internal.QueueController
	authToken   string
	s2hClient   samsahairpc.RPC
}

func New(
	teamName string,
	mgr manager.Manager,
	prQueueCtrl internal.QueueController,
	authToken string,
	s2hClient samsahairpc.RPC,
) internal.PullRequestTriggerController {
	c := &controller{
		teamName:    teamName,
		client:      mgr.GetClient(),
		prQueueCtrl: prQueueCtrl,
		authToken:   authToken,
		s2hClient:   s2hClient,
	}

	if err := add(mgr, c); err != nil {
		logger.Error(err, "cannot add new controller to manager")
	}

	return c
}

var _ reconcile.Reconciler = &controller{}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := crctrl.New(CtrlName, mgr, crctrl.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to PullRequestTrigger
	err = c.Watch(&source.Kind{Type: &s2hv1beta1.PullRequestTrigger{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

// Reconcile reads that state of the cluster for a PullRequestTrigger object and makes changes based on the state read
// and what is in the PullRequestTrigger.Spec
// +kubebuilder:rbac:groups=env.samsahai.io,resources=pullrequesttriggers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=env.samsahai.io,resources=pullrequesttriggers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=env.samsahai.io,resources=pullrequestqueues,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=env.samsahai.io,resources=pullrequestqueues/status,verbs=get;update;patch
func (c *controller) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	ctx := context.TODO()
	now := metav1.Now()
	prTrigger := &s2hv1beta1.PullRequestTrigger{}
	err := c.client.Get(ctx, req.NamespacedName, prTrigger)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			// Object not found, return. Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// wait until next run
	nextProcessAt := prTrigger.Status.NextProcessAt
	if nextProcessAt != nil && now.Before(nextProcessAt) {
		return reconcile.Result{
			Requeue:      true,
			RequeueAfter: 2 * time.Second,
		}, nil
	}

	// make request with samsahai controller
	headers := make(http.Header)
	headers.Set(internal.SamsahaiAuthHeader, c.authToken)
	ctx, err = twirp.WithHTTPRequestHeaders(ctx, headers)
	if err != nil {
		logger.Error(err, "cannot set request header")
	}

	prConfig, err := c.s2hClient.GetPullRequestConfig(ctx, &samsahairpc.TeamName{Name: c.teamName})
	if err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "cannot get pull request config of team: %s", c.teamName)
	}

	pollingTime, err := time.ParseDuration(prConfig.Trigger.PollingTime)
	if err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "cannot parse trigger polling time string: %s",
			prConfig.Trigger.PollingTime)
	}

	zeroRetry := 0
	noOfRetry := prTrigger.Status.NoOfRetry
	maxRetry := prConfig.Trigger.MaxRetry
	if maxRetry >= 0 && noOfRetry != nil && *noOfRetry >= int(maxRetry) {
		if err := c.deleteAndSendPullRequestTriggerResult(ctx, prTrigger); err != nil {
			return reconcile.Result{}, err
		}

		return reconcile.Result{}, nil
	}

	// fill empty data
	prCompSource, err := c.getOverridingComponentSource(ctx, prTrigger)
	if err != nil {
		return reconcile.Result{}, err
	}

	changed := c.fillEmptyData(prTrigger, prCompSource)
	if changed {
		if err := c.client.Update(context.TODO(), prTrigger); err != nil {
			return reconcile.Result{}, err
		}

		return reconcile.Result{}, nil
	}

	nextProcessAt = &metav1.Time{Time: now.Add(pollingTime)}
	version, err := c.s2hClient.GetComponentVersion(ctx, prCompSource)
	if err != nil {
		// cannot get component version from image registry
		prTrigger.Status.UpdatedAt = &now
		prTrigger.Status.NextProcessAt = nextProcessAt

		if prTrigger.Status.NoOfRetry == nil {
			prTrigger.Status.NoOfRetry = &zeroRetry
		} else {
			*prTrigger.Status.NoOfRetry++
		}

		prTrigger.Status.SetCondition(s2hv1beta1.PullRequestTriggerCondFailed, corev1.ConditionTrue, err.Error())
		prTrigger.Status.SetResult(s2hv1beta1.PullRequestTriggerFailure)

		if err := c.client.Update(context.TODO(), prTrigger); err != nil {
			return reconcile.Result{}, err
		}

		return reconcile.Result{}, nil
	}

	// successfully get component version from image registry
	prTrigger.Status.SetResult(s2hv1beta1.PullRequestTriggerSuccess)

	imgRepo := prTrigger.Spec.Image.Repository
	prNumber := prTrigger.Spec.PRNumber
	err = c.createPullRequestQueue(req.Namespace, prTrigger.Spec.Component, imgRepo, version.Version, prNumber.String())
	if err != nil {
		return reconcile.Result{}, err
	}

	if err := c.deleteAndSendPullRequestTriggerResult(ctx, prTrigger); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (c *controller) fillEmptyData(prTrigger *s2hv1beta1.PullRequestTrigger, prCompSource *samsahairpc.ComponentSource) (changed bool) {
	now := metav1.Now()

	if prTrigger.Status.CreatedAt == nil {
		prTrigger.Status.CreatedAt = &now
		changed = true
	}
	if prTrigger.Status.UpdatedAt == nil {
		prTrigger.Status.UpdatedAt = &now
		changed = true
	}

	if prTrigger.Spec.Image == nil {
		prTrigger.Spec.Image = &s2hv1beta1.Image{}
		changed = true
	}
	if prTrigger.Spec.Image.Repository == "" {
		prTrigger.Spec.Image.Repository = prCompSource.Image.Repository
		changed = true
	}
	if prTrigger.Spec.Pattern == "" {
		prTrigger.Spec.Pattern = prCompSource.Pattern
		changed = true
	}
	if prTrigger.Spec.Source == "" {
		prTrigger.Spec.Source = s2hv1beta1.UpdatingSource(prCompSource.Source)
		changed = true
	}

	return
}

func (c *controller) getOverridingComponentSource(ctx context.Context, prTrigger *s2hv1beta1.PullRequestTrigger) (*samsahairpc.ComponentSource, error) {
	compName := prTrigger.Spec.Component
	prCompSource, err := c.s2hClient.GetPullRequestComponentSource(ctx, &samsahairpc.TeamWithPullRequest{
		TeamName:      c.teamName,
		ComponentName: compName,
		PRNumber:      prTrigger.Spec.PRNumber.String(),
	})
	if err != nil {
		return &samsahairpc.ComponentSource{}, err
	}

	overridingPRImage := prTrigger.Spec.Image
	if overridingPRImage != nil {
		if overridingPRImage.Repository != "" {
			prCompSource.Image.Repository = overridingPRImage.Repository
		}

		if overridingPRImage.Tag != "" {
			prCompSource.Image.Tag = overridingPRImage.Tag
		}
	}

	overridingPattern := prTrigger.Spec.Pattern
	if overridingPattern != "" {
		prCompSource.Pattern = overridingPattern
	}

	overridingSource := prTrigger.Spec.Source
	if overridingSource != "" {
		prCompSource.Source = string(overridingSource)
	}

	return prCompSource, nil
}

func (c *controller) createPullRequestQueue(namespace, compName, compRepo, compVersion, prNumber string) error {
	comps := s2hv1beta1.QueueComponents{
		{
			Name:       compName,
			Repository: compRepo,
			Version:    compVersion,
		},
	}

	prQueue := prqueuectrl.NewPullRequestQueue(c.teamName, namespace, compName, prNumber, comps)
	if err := c.prQueueCtrl.Add(prQueue, nil); err != nil {
		return err
	}

	return nil
}

func (c *controller) deleteAndSendPullRequestTriggerResult(ctx context.Context, prTrigger *s2hv1beta1.PullRequestTrigger) error {
	prTriggerRPC := &samsahairpc.PullRequestTrigger{
		Name:      prTrigger.Name,
		Namespace: prTrigger.Namespace,
		TeamName:  c.teamName,
		Result:    string(prTrigger.Status.Result),
	}
	if _, err := c.s2hClient.RunPostPullRequestTrigger(ctx, prTriggerRPC); err != nil {
		return errors.Wrapf(err,
			"cannot send pull request trigger result report, team: %s, component: %s, prNumber: %s",
			c.teamName, prTrigger.Spec.Component, prTrigger.Spec.PRNumber)
	}

	if err := c.client.Delete(context.TODO(), prTrigger); err != nil && !k8serrors.IsNotFound(err) {
		return err
	}

	return nil
}
