package desiredcomponent

import (
	"context"
	"fmt"
	"net/http"

	"github.com/twitchtv/twirp"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	"github.com/agoda-com/samsahai/internal/queue"
	samsahairpc "github.com/agoda-com/samsahai/pkg/samsahai/rpc"
)

const (
	CtrlName = "desired-component-ctrl"
)

var logger = s2hlog.Log.WithName(CtrlName)

type controller struct {
	teamName  string
	queueCtrl internal.QueueController
	client    client.Client
	authToken string
	s2hClient samsahairpc.RPC
}

var _ internal.DesiredComponentController = &controller{}

func New(
	teamName string,
	mgr manager.Manager,
	queueCtrl internal.QueueController,
	authToken string,
	s2hClient samsahairpc.RPC,
) internal.DesiredComponentController {
	if queueCtrl == nil {
		logger.Error(s2herrors.ErrInternalError, "queue ctrl cannot be nil")
		panic(s2herrors.ErrInternalError)
	}

	c := &controller{
		teamName:  teamName,
		queueCtrl: queueCtrl,
		client:    mgr.GetClient(),
		authToken: authToken,
		s2hClient: s2hClient,
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

	// Watch for changes to DesiredComponent
	err = c.Watch(&source.Kind{Type: &s2hv1.DesiredComponent{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

// Reconcile reads that state of the cluster for a DesiredComponent object and makes changes based on the state read
// and what is in the DesiredComponent.Spec
// +kubebuilder:rbac:groups=env.samsahai.io,resources=desiredcomponents,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=env.samsahai.io,resources=desiredcomponents/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=env.samsahai.io,resources=queues,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=env.samsahai.io,resources=queues/status,verbs=get;update;patch
func (c *controller) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	now := metav1.Now()
	comp := &s2hv1.DesiredComponent{}
	err := c.client.Get(ctx, req.NamespacedName, comp)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return. Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	if comp.Status.CreatedAt == nil {
		comp.Status.CreatedAt = &now
	}
	if comp.Status.UpdatedAt == nil {
		comp.Status.UpdatedAt = &now
	}

	logger.Debug(fmt.Sprintf("add %s (%s:%s) to queue", comp.Spec.Name, comp.Spec.Repository, comp.Spec.Version))

	headers := make(http.Header)
	headers.Set(internal.SamsahaiAuthHeader, c.authToken)
	ctx, err = twirp.WithHTTPRequestHeaders(ctx, headers)
	if err != nil {
		logger.Error(err, "cannot set request header")
	}

	bundle, err := c.s2hClient.GetBundleName(ctx, &samsahairpc.TeamWithComponentName{
		TeamName:      c.teamName,
		ComponentName: comp.Spec.Name,
	})
	if err != nil {
		logger.Error(err, "cannot get bundle name", "team", c.teamName, "component", comp.Spec.Name)
	}

	priorityQueues, err := c.s2hClient.GetPriorityQueues(ctx, &samsahairpc.TeamName{Name: c.teamName})
	if err != nil {
		logger.Error(err, "cannot get priority queues", "team", c.teamName)
	}

	comps := []*s2hv1.QueueComponent{
		{
			Name:       comp.Spec.Name,
			Repository: comp.Spec.Repository,
			Version:    comp.Spec.Version,
		},
	}
	q := queue.NewQueue(c.teamName, req.Namespace, comp.Spec.Name, bundle.Name, comps, s2hv1.QueueTypeUpgrade)
	err = c.queueCtrl.Add(q, priorityQueues.GetQueues())
	if err != nil {
		return reconcile.Result{}, err
	}

	rpcComp := &samsahairpc.ComponentUpgrade{
		Name:      q.Spec.Name,
		Namespace: q.Namespace,
	}
	if c.s2hClient != nil {
		if _, err := c.s2hClient.SendUpdateStateQueueMetric(ctx, rpcComp); err != nil {
			logger.Error(err, "cannot send updateQueueWithState queue metric")
		}
	}

	if err := c.client.Update(context.TODO(), comp); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}
