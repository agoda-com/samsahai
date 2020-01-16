package activepromotion

import (
	"context"
	"sync"
	"time"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crctrl "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/agoda-com/samsahai/internal"
	s2herrors "github.com/agoda-com/samsahai/internal/errors"
	s2hlog "github.com/agoda-com/samsahai/internal/log"
	"github.com/agoda-com/samsahai/internal/queue"
	"github.com/agoda-com/samsahai/internal/samsahai/exporter"
	"github.com/agoda-com/samsahai/internal/util/stringutils"
	s2hv1beta1 "github.com/agoda-com/samsahai/pkg/apis/env/v1beta1"
)

var logger = s2hlog.Log.WithName(CtrlName)

const (
	CtrlName = "active-promotion-ctrl"

	activePromotionFinalizer = "activepromotion.finalizers.samsahai.io"
	tokenLength              = 6
)

type controller struct {
	s2hCtrl   internal.SamsahaiController
	client    client.Client
	clientset *kubernetes.Clientset
	restCfg   *rest.Config

	deployEngines map[string]internal.DeployEngine

	configs internal.SamsahaiConfig

	wg       sync.WaitGroup
	shutdown chan struct{}
}

func New(
	mgr manager.Manager,
	s2hCtrl internal.SamsahaiController,
	configs internal.SamsahaiConfig,
) internal.ActivePromotionController {

	// creates clientset
	clientset, err := kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		logger.Error(s2herrors.ErrInternalError, "cannot create clientset")
		panic(s2herrors.ErrInternalError)
	}

	c := &controller{
		s2hCtrl:       s2hCtrl,
		client:        mgr.GetClient(),
		configs:       configs,
		clientset:     clientset,
		restCfg:       mgr.GetConfig(),
		deployEngines: map[string]internal.DeployEngine{},
		shutdown:      make(chan struct{}),
		wg:            sync.WaitGroup{},
	}

	if err := add(mgr, c); err != nil {
		logger.Error(err, "cannot add new controller to manager")
		return nil
	}

	return c
}

func (c *controller) setup(ctx context.Context, atpComp *s2hv1beta1.ActivePromotion) error {
	// set teardown duration from configuration if it's unset
	if atpComp.Spec.TearDownDuration == nil {
		duration := c.configs.ActivePromotion.TearDownDuration

		configMgr, err := c.getTeamConfiguration(atpComp.Name)
		if err != nil {
			return err
		}
		if cfg := configMgr.Get(); cfg.ActivePromotion != nil && cfg.ActivePromotion.TearDownDuration.Duration != 0 {
			duration = cfg.ActivePromotion.TearDownDuration
		}

		atpComp.Spec.SetTearDownDuration(duration)
	}

	atpComp.SetState(s2hv1beta1.ActivePromotionWaiting, "Waiting in queue")
	return nil
}

func (c *controller) teardown(ctx context.Context, atpComp *s2hv1beta1.ActivePromotion) error {
	targetNs := atpComp.Status.TargetNamespace
	_ = queue.DeletePreActiveQueue(c.client, targetNs)
	_ = queue.DeletePromoteToActiveQueue(c.client, targetNs)
	_ = queue.DeleteDemoteFromActiveQueue(c.client, targetNs)

	prevNs := atpComp.Status.PreviousActiveNamespace
	if prevNs != "" {
		_ = queue.DeletePromoteToActiveQueue(c.client, prevNs)
		_ = queue.DeleteDemoteFromActiveQueue(c.client, prevNs)
	}

	if err := c.runPostActive(ctx, atpComp); err != nil {
		return err
	}

	return nil
}

// checkActivePromotionTimeout checks if promote active environment duration was longer than timeout.
func (c *controller) checkActivePromotionTimeout(ctx context.Context, atpComp *s2hv1beta1.ActivePromotion) error {
	if atpComp.Status.IsTimeout {
		return nil
	}

	// these states cannot be timeout
	if c.stateCannotBeTimeoutOrCancel(atpComp.Status.State) {
		return nil
	}

	isTimeout, err := c.isTimeoutFromConfig(atpComp, timeoutActivePromotion)
	if err != nil {
		return err
	}

	if isTimeout {
		logger.Debug("active promotion has been timeout", "team", atpComp.Name)
		atpComp.Status.SetIsTimeout()
		atpComp.Status.SetResult(s2hv1beta1.ActivePromotionFailure)

		if c.isToRollbackState(atpComp) {
			atpComp.Status.SetCondition(s2hv1beta1.ActivePromotionCondRollbackStarted, corev1.ConditionTrue,
				"Rollback process has been started due to promoting timeout")
			atpComp.SetState(s2hv1beta1.ActivePromotionRollback, "Active promotion has been timeout")
		} else {
			atpComp.Status.SetCondition(s2hv1beta1.ActivePromotionCondVerified, corev1.ConditionFalse,
				"Active promotion has been timeout")
			atpComp.SetState(s2hv1beta1.ActivePromotionCollectingPreActiveResult,
				"Active promotion has been timeout")
		}

		if err := c.updateActivePromotion(ctx, atpComp); err != nil {
			return err
		}
		return s2herrors.ErrActivePromotionTimeout
	}

	return nil
}

func (c *controller) deleteFinalizerWhenFinished(ctx context.Context, atpComp *s2hv1beta1.ActivePromotion) (
	skipUpdate bool, err error) {

	if atpComp.Status.State == s2hv1beta1.ActivePromotionFinished {
		if err = c.teardown(ctx, atpComp); err != nil {
			return
		}

		if atpComp.ObjectMeta.DeletionTimestamp.IsZero() {
			logger.Debug("process has been finished and activepromotion has been deleted",
				"team", atpComp.Name, "status", atpComp.Status.Result)

			if err = c.deleteActivePromotion(ctx, atpComp); err != nil {
				return
			}
			// skip updating active promotion due to active promotion is being deleted
			return true, err
		}
	}

	if !atpComp.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is being deleted
		if stringutils.ContainsString(atpComp.ObjectMeta.Finalizers, activePromotionFinalizer) {
			if atpComp.Status.State == s2hv1beta1.ActivePromotionFinished {
				if err = c.removeFinalizerObject(ctx, atpComp); err != nil {
					return
				}

				// Add metric activePromotion
				atpList := &s2hv1beta1.ActivePromotionList{}
				if err := c.client.List(context.TODO(), nil, atpList); err != nil {
					logger.Error(err, "cannot list all active promotion")
				}
				exporter.SetActivePromotionMetric(atpList)

				// active promotion process has been finished
				return true, nil
			}

			if atpComp.IsActivePromotionCanceled() {
				return
			}

			if atpComp.Status.State == s2hv1beta1.ActivePromotionDestroyingPreviousActive {
				destroyedTime := metav1.Now()
				atpComp.Status.SetDestroyedTime(destroyedTime)
				return
			}

			if c.stateCannotBeTimeoutOrCancel(atpComp.Status.State) {
				return
			}

			atpComp.Status.SetResult(s2hv1beta1.ActivePromotionCanceled)

			if c.isToRollbackState(atpComp) {
				atpComp.Status.SetCondition(s2hv1beta1.ActivePromotionCondRollbackStarted, corev1.ConditionTrue,
					"Rollback process has been started due to canceling")
				atpComp.SetState(s2hv1beta1.ActivePromotionRollback, "Active promotion has been canceled")
			} else {
				atpComp.Status.SetCondition(s2hv1beta1.ActivePromotionCondVerified, corev1.ConditionFalse,
					"Active promotion has been canceled")
				atpComp.SetState(s2hv1beta1.ActivePromotionCollectingPreActiveResult,
					"Active promotion has been canceled")
			}

			if err = c.updateActivePromotion(ctx, atpComp); err != nil {
				return
			}

			return true, nil
		}
	}

	return
}

func (c *controller) addFinalizer(atpComp *s2hv1beta1.ActivePromotion) {
	if atpComp.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object.
		if !stringutils.ContainsString(atpComp.ObjectMeta.Finalizers, activePromotionFinalizer) {
			atpComp.ObjectMeta.Finalizers = append(atpComp.ObjectMeta.Finalizers, activePromotionFinalizer)
		}
	}
}

func (c *controller) stateCannotBeTimeoutOrCancel(state s2hv1beta1.ActivePromotionState) bool {
	// waiting state doesn't have finalizer
	return state == s2hv1beta1.ActivePromotionWaiting ||
		state == s2hv1beta1.ActivePromotionDestroyingPreviousActive ||
		state == s2hv1beta1.ActivePromotionDestroyingPreActive ||
		state == s2hv1beta1.ActivePromotionFinished ||
		state == s2hv1beta1.ActivePromotionRollback
}

func (c *controller) isToRollbackState(atpComp *s2hv1beta1.ActivePromotion) bool {
	return atpComp.Status.IsConditionTrue(s2hv1beta1.ActivePromotionCondVerified)
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := crctrl.New(CtrlName, mgr, crctrl.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to ActivePromotion
	err = c.Watch(&source.Kind{Type: &s2hv1beta1.ActivePromotion{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

// Reconcile reads that state of the cluster for a ActivePromotion object and makes changes based on the state read
// and what is in the ActivePromotion.Spec
// +kubebuilder:rbac:groups=env.samsahai.io,resources=activepromotions,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=env.samsahai.io,resources=activepromotions/status,verbs=get;update;patch
func (c *controller) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	ctx := context.TODO()

	isTriggered, err := c.manageQueue(ctx)
	if err != nil {
		return reconcile.Result{}, err
	}
	if isTriggered {
		return reconcile.Result{}, nil
	}

	atpComp := &s2hv1beta1.ActivePromotion{}
	if err := c.client.Get(ctx, req.NamespacedName, atpComp); err != nil {
		if k8serrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	if _, err := c.getTeam(ctx, atpComp.Name); err != nil && k8serrors.IsNotFound(err) {
		logger.Error(err, "team not found, start deleting activepromotion", "team", atpComp.Name)
		// if team not found, delete active promotion
		if err := c.forceDeleteActivePromotion(ctx, atpComp); err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	}

	isSkipped, err := c.deleteFinalizerWhenFinished(ctx, atpComp)
	if err != nil {
		if s2herrors.IsLoadingConfiguration(err) {
			return reconcile.Result{
				Requeue:      true,
				RequeueAfter: 1 * time.Second,
			}, nil
		}
		return reconcile.Result{}, err
	}
	if isSkipped {
		return reconcile.Result{}, nil
	}

	if err := c.checkActivePromotionTimeout(ctx, atpComp); err != nil {
		if s2herrors.IsLoadingConfiguration(err) || s2herrors.IsErrActivePromotionTimeout(err) {
			return reconcile.Result{
				Requeue:      true,
				RequeueAfter: 1 * time.Second,
			}, nil
		}
		return reconcile.Result{}, err
	}

	switch atpComp.Status.State {
	case "":
		logger.Info("activepromotion has been created", "team", atpComp.Name)
		if err := c.setup(ctx, atpComp); err != nil {
			if s2herrors.IsLoadingConfiguration(err) {
				return reconcile.Result{
					Requeue:      true,
					RequeueAfter: 1 * time.Second,
				}, nil
			}
			return reconcile.Result{}, err
		}

	case s2hv1beta1.ActivePromotionWaiting:
		logger.Debug("activepromotion is waiting in queue", "team", atpComp.Name)
		return reconcile.Result{}, nil

	case s2hv1beta1.ActivePromotionCreatingPreActive:
		if err := c.createPreActiveEnvAndDeployStableCompObjects(ctx, atpComp); err != nil {
			if s2herrors.IsNamespaceStillCreating(err) {
				return reconcile.Result{
					Requeue:      true,
					RequeueAfter: 2 * time.Second,
				}, nil
			}
			return reconcile.Result{}, err
		}

	case s2hv1beta1.ActivePromotionDeployingComponents:
		if err := c.deployComponentsToTargetNamespace(atpComp); err != nil {
			if s2herrors.IsEnsuringComponentDeployed(err) {
				return reconcile.Result{
					Requeue:      true,
					RequeueAfter: 2 * time.Second,
				}, nil
			}
			return reconcile.Result{}, err
		}

	case s2hv1beta1.ActivePromotionTestingPreActive:
		if err := c.testPreActiveEnvironment(atpComp); err != nil {
			if s2herrors.IsEnsuringActiveTested(err) {
				return reconcile.Result{
					Requeue:      true,
					RequeueAfter: 2 * time.Second,
				}, nil
			}
			return reconcile.Result{}, err
		}

	case s2hv1beta1.ActivePromotionCollectingPreActiveResult:
		if err := c.collectResult(ctx, atpComp); err != nil {
			if s2herrors.IsEnsuringActiveTested(err) {
				return reconcile.Result{
					Requeue:      true,
					RequeueAfter: 2 * time.Second,
				}, nil
			}
			return reconcile.Result{}, err
		}

	case s2hv1beta1.ActivePromotionDemoting:
		if err := c.demoteActiveEnvironment(ctx, atpComp); err != nil {
			if s2herrors.IsEnsuringActiveDemoted(err) || s2herrors.IsErrActiveDemotionTimeout(err) {
				return reconcile.Result{
					Requeue:      true,
					RequeueAfter: 1 * time.Second,
				}, nil
			}
			return reconcile.Result{}, err
		}

	case s2hv1beta1.ActivePromotionActiveEnvironment:
		if err := c.promoteActiveEnvironment(ctx, atpComp); err != nil {
			if s2herrors.IsEnsuringActivePromoted(err) {
				return reconcile.Result{
					Requeue:      true,
					RequeueAfter: 2 * time.Second,
				}, nil
			}
			return reconcile.Result{}, err
		}

	case s2hv1beta1.ActivePromotionDestroyingPreviousActive:
		if err := c.destroyPreviousActiveEnvironment(ctx, atpComp); err != nil {
			if s2herrors.IsEnsuringNamespaceDestroyed(err) {
				return reconcile.Result{
					Requeue:      true,
					RequeueAfter: 2 * time.Second,
				}, nil
			}
			return reconcile.Result{}, err
		}

	case s2hv1beta1.ActivePromotionDestroyingPreActive:
		if err := c.destroyPreActiveEnvironment(ctx, atpComp); err != nil {
			if s2herrors.IsEnsuringNamespaceDestroyed(err) {
				return reconcile.Result{
					Requeue:      true,
					RequeueAfter: 2 * time.Second,
				}, nil
			}
			return reconcile.Result{}, err
		}

	case s2hv1beta1.ActivePromotionRollback:
		if err := c.rollbackActiveEnvironment(ctx, atpComp); err != nil {
			if s2herrors.IsRollingBackActivePromotion(err) || s2herrors.IsErrRollbackActivePromotionTimeout(err) {
				return reconcile.Result{
					Requeue:      true,
					RequeueAfter: 1 * time.Second,
				}, nil
			}
			return reconcile.Result{}, errors.Wrapf(err, "cannot rollback activepromotion %s", atpComp.Name)
		}
	}

	if err := c.updateActivePromotion(ctx, atpComp); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}
