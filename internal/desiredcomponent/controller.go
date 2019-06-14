package desiredcomponent

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crctrl "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/agoda-com/samsahai/internal"
	"github.com/agoda-com/samsahai/internal/apis/env/v1beta1"
	"github.com/agoda-com/samsahai/internal/config"
	"github.com/agoda-com/samsahai/internal/desiredcomponent/checker/agodacspider"
	"github.com/agoda-com/samsahai/internal/desiredcomponent/checker/agodadaily"
	"github.com/agoda-com/samsahai/internal/desiredcomponent/checker/harbor"
	"github.com/agoda-com/samsahai/internal/desiredcomponent/checker/publicregistry"
	"github.com/agoda-com/samsahai/internal/queue"
)

const (
	CtrlName         = "desired component ctrl"
	EnvPodNamespace  = "pod-namespace"
	MaxRetryOnFailed = 3
)

type controller struct {
	checkers   map[string]internal.DesiredComponentChecker
	components map[string]*internal.Component
	checkJob   chan internal.Component
	stopCh     chan struct{}
	log        logr.Logger
	namespace  string
	queueCtrl  internal.QueueController
	restClient rest.Interface
	client     client.Client
	scheme     *runtime.Scheme
}

var _ internal.DesiredComponentController = &controller{}

func New(namespace string, cfg *rest.Config, mgr manager.Manager, queueCtrl internal.QueueController) internal.DesiredComponentController {
	log := log.Log.WithName(CtrlName)

	// register types at the scheme builder
	if err := v1beta1.AddToScheme(scheme.Scheme); err != nil {
		log.Error(err, "error addtoscheme")
		return nil
	}

	// create rest client
	restClient, err := rest.UnversionedRESTClientFor(config.GetRESTConfg(cfg, &v1beta1.SchemeGroupVersion))
	if err != nil {
		log.Error(err, "cannot create unversioned restclient")
		return nil
	}

	return NewWithClient(namespace, mgr, restClient, queueCtrl)
}

func NewWithClient(namespace string, mgr manager.Manager, restClient rest.Interface, queueCtrl internal.QueueController) internal.DesiredComponentController {
	log := log.Log.WithName(CtrlName)
	if queueCtrl == nil {
		log.Error(internal.ErrInternalError, "queue ctrl cannot be nil")
		panic(internal.ErrInternalError)
	}

	c := &controller{
		checkers:   map[string]internal.DesiredComponentChecker{},
		checkJob:   make(chan internal.Component),
		stopCh:     make(chan struct{}),
		log:        log,
		namespace:  namespace,
		queueCtrl:  queueCtrl,
		client:     mgr.GetClient(),
		restClient: restClient,
		scheme:     mgr.GetScheme(),
	}
	if err := add(mgr, c); err != nil {
		c.log.Error(err, "cannot add new controller to manager")
	}
	c.loadCheckers()
	return c
}

func (c *controller) LoadConfiguration() {
	// TODO: read configuration from `storage`
	updateSrc := internal.UpdatingSource("public-registry")
	config := internal.Configuration{
		Components: []*internal.Component{
			{
				Name: "alpine",
				Image: internal.ComponentImage{
					Repository: "alpine",
					Pattern:    "^3\\.9.+",
				},
				Source: &updateSrc,
			},
			{
				Name: "ubuntu",
				Image: internal.ComponentImage{
					Repository: "ubuntu",
					Pattern:    "^bionic.+",
				},
				Source: &updateSrc,
			},
		},
	}

	c.components = c.GetComponents(config)
}

func (c *controller) Start() {
	c.LoadConfiguration()

	// TODO: improve to be worker pattern
	// controllable concurrency
	go func() {
	exitLoop:
		for {
			select {
			case comp := <-c.checkJob:
				for i := 0; i < MaxRetryOnFailed; i++ {
					if err := c.check(comp); err == nil {
						break
					}
				}
			case <-c.stopCh:
				break exitLoop
			}
		}
	}()
}

func (c *controller) Stop() {
	c.stopCh <- struct{}{}
}

func (c *controller) AddChecker(name string, checker internal.DesiredComponentChecker) {
	c.checkers[name] = checker
}

func (c *controller) Clear() {
	err := c.DeleteCollection(nil, nil)
	if err != nil {
		c.log.Error(err, "cannot clear desired component")
	}
}

func (c *controller) TryCheck(names ...string) {
	var checkComps []string

	if len(names) == 0 {
		checkComps = make([]string, len(c.components))
		i := 0
		for name := range c.components {
			checkComps[i] = name
			i++
		}
	} else {
		checkComps = make([]string, 0)
		for _, name := range names {
			if _, exist := c.components[name]; !exist {
				continue
			}
			checkComps = append(checkComps, name)
		}
	}

	for _, name := range checkComps {
		c.checkJob <- *c.components[name]
	}
}

// GetComponents returns map of `Component` from `Configuration` that has valid check `Source`
func (c *controller) GetComponents(config internal.Configuration) (filteredComps map[string]*internal.Component) {
	filteredComps = map[string]*internal.Component{}

	var comps []*internal.Component
	var comp *internal.Component

	comps = append(comps, config.Components...)

	for len(comps) > 0 {
		comp, comps = comps[0], comps[1:]
		if len(comp.Dependencies) > 0 {
			// add to comps
			for _, dep := range comp.Dependencies {
				comps = append(comps, &internal.Component{
					Name:   dep.Name,
					Image:  dep.Image,
					Source: dep.Source,
				})
			}
		}

		if comp.Source == nil {
			// ignore if no source provided
			continue
		}
		if _, exist := c.checkers[string(*comp.Source)]; !exist {
			// no checker match for this component
			c.log.V(-1).Info(fmt.Sprintf("no checker: %s for component: %s", string(*comp.Source), comp.Name))
			continue
		}

		if _, exist := filteredComps[comp.Name]; exist {
			// duplication component name
			c.log.V(-1).Info(fmt.Sprintf("duplicate component: %s detected", comp.Name))
			continue
		}

		filteredComps[comp.Name] = comp
	}

	return filteredComps
}

func (c *controller) check(comp internal.Component) error {
	checker := c.checkers[string(*comp.Source)]
	var desiredComp *v1beta1.DesiredComponent
	var err error
	var version string
	ctx := context.TODO()

	if comp.Image.Pattern == "" {
		version, err = checker.GetVersion(comp.Image.Repository, comp.Name, ".*")
	} else {
		version, err = checker.GetVersion(comp.Image.Repository, comp.Name, comp.Image.Pattern)
	}
	if err != nil {
		c.log.Error(err, "cannot when checker.getversion")
		return err
	}

	now := metav1.Now()
	desiredComp = &v1beta1.DesiredComponent{}
	err = c.client.Get(ctx, types.NamespacedName{Name: comp.Name, Namespace: c.namespace}, desiredComp)
	if err != nil && errors.IsNotFound(err) {
		desiredComp = &v1beta1.DesiredComponent{
			ObjectMeta: metav1.ObjectMeta{
				Name:      comp.Name,
				Namespace: c.namespace,
			},
			Spec: v1beta1.DesiredComponentSpec{
				Version:    version,
				Name:       comp.Name,
				Repository: comp.Image.Repository,
			},
			Status: v1beta1.DesiredComponentStatus{
				CreatedAt: &now,
				UpdatedAt: &now,
			},
		}

		if err = c.client.Create(ctx, desiredComp); err != nil {
			c.log.Error(err, "cannot create 'desiredcomponent'")
			return err
		}
	} else if err != nil {
		c.log.Error(err, fmt.Sprintf("cannot get 'desiredcomponent' name: %s", comp.Name))
		return err
	} else {
		if desiredComp.Spec.Version == version {
			// if version isn't change, noop
			return nil
		}

		// update
		desiredComp.Spec.Version = version
		desiredComp.Spec.Repository = comp.Image.Repository
		desiredComp.Status.UpdatedAt = &now

		if err = c.client.Update(ctx, desiredComp); err != nil {
			c.log.Error(err, "cannot update 'desiredcomponent'")
			return err
		}
	}

	return nil
}

func (c *controller) loadCheckers() {
	// init checkers
	checkers := []internal.DesiredComponentChecker{
		publicregistry.New(),
		harbor.New(),
		agodadaily.New(),
		agodacspider.New(),
	}
	for _, checker := range checkers {
		if checker == nil {
			continue
		}
		c.checkers[checker.GetName()] = checker
	}
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
	err = c.Watch(&source.Kind{Type: &v1beta1.DesiredComponent{}}, &handler.EnqueueRequestForObject{})
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
func (c *controller) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	ctx := context.TODO()
	now := metav1.Now()
	comp := &v1beta1.DesiredComponent{}
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

	c.log.V(1).Info("add to queue")
	err = c.queueCtrl.Add(
		queue.NewUpgradeQueue(req.Namespace, comp.Spec.Name, comp.Spec.Repository, comp.Spec.Version),
	)
	if err != nil {
		return reconcile.Result{}, err
	}

	comp.Status.UpdatedAt = &now
	if err := c.client.Update(context.TODO(), comp); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}
