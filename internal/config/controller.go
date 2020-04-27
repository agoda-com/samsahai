package config

import (
	"context"
	"time"

	"github.com/ghodss/yaml"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	cr "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	k8scontroller "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	s2hv1beta1 "github.com/agoda-com/samsahai/api/v1beta1"
	"github.com/agoda-com/samsahai/internal"
	"github.com/agoda-com/samsahai/internal/errors"
	s2hlog "github.com/agoda-com/samsahai/internal/log"
	"github.com/agoda-com/samsahai/internal/util/http"
	"github.com/agoda-com/samsahai/internal/util/valuesutil"
)

var logger = s2hlog.Log.WithName(ctrlName)

const (
	ctrlName                = "config-ctrl"
	maxConcurrentReconciles = 1
)

type controller struct {
	client  client.Client
	s2hCtrl internal.SamsahaiController
}

type Option func(*controller)

func WithClient(client client.Client) Option {
	return func(c *controller) {
		c.client = client
	}
}

func WithS2hCtrl(s2hCtrl internal.SamsahaiController) Option {
	return func(c *controller) {
		c.s2hCtrl = s2hCtrl
	}
}

func New(mgr cr.Manager, options ...Option) internal.ConfigController {
	c := &controller{}

	if mgr != nil {
		c.client = mgr.GetClient()
		if err := c.SetupWithManager(mgr); err != nil {
			logger.Error(err, "cannot add new controller to manager")
			return nil
		}
	}

	for _, opt := range options {
		opt(c)
	}

	return c
}

func (c *controller) SetupWithManager(mgr cr.Manager) error {
	return cr.NewControllerManagedBy(mgr).
		WithOptions(k8scontroller.Options{MaxConcurrentReconciles: maxConcurrentReconciles}).
		For(&s2hv1beta1.Config{}).
		Complete(c)
}

// Get returns configuration from Config CRD
func (c *controller) Get(configName string) (*s2hv1beta1.Config, error) {
	return c.getConfig(configName)
}

// GetComponents returns all components from `Configuration` that has valid `Source`
func (c *controller) GetComponents(configName string) (map[string]*s2hv1beta1.Component, error) {
	config, err := c.Get(configName)
	if err != nil {
		logger.Error(err, "cannot get Config", "name", configName)
		return map[string]*s2hv1beta1.Component{}, err
	}

	c.assignParent(&config.Spec)

	filteredComps := map[string]*s2hv1beta1.Component{}

	var comps []*s2hv1beta1.Component
	var comp *s2hv1beta1.Component

	comps = append(comps, config.Spec.Components...)

	for len(comps) > 0 {
		comp, comps = comps[0], comps[1:]
		if len(comp.Dependencies) > 0 {
			// add to comps
			for _, dep := range comp.Dependencies {
				comps = append(comps, &s2hv1beta1.Component{
					Parent: comp.Name,
					Name:   dep.Name,
					Image:  dep.Image,
					Source: dep.Source,
				})
			}
		}

		if _, exist := filteredComps[comp.Name]; exist {
			// duplication component name
			logger.Warnf("duplicate component: %s detected", comp.Name)
			continue
		}

		filteredComps[comp.Name] = comp
	}

	return filteredComps, nil
}

// GetParentComponents returns components that doesn't have parent (nil Parent)
func (c *controller) GetParentComponents(configName string) (map[string]*s2hv1beta1.Component, error) {
	filteredComps, err := c.GetComponents(configName)
	if err != nil {
		return map[string]*s2hv1beta1.Component{}, err
	}

	for name, v := range filteredComps {
		if v.Parent != "" {
			delete(filteredComps, name)
		}
	}

	return filteredComps, nil
}

// Update updates Config CRD
func (c *controller) Update(config *s2hv1beta1.Config) error {
	if err := c.client.Update(context.TODO(), config); err != nil {
		return err
	}

	return nil
}

// Delete delete Config CRD
func (c *controller) Delete(configName string) error {
	config, err := c.getConfig(configName)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil
		}

		return err

	}

	if err := c.client.Delete(context.TODO(), config); err != nil {
		return err
	}

	return nil
}

// GetEnvValues returns component values per component name by the given env type
func GetEnvValues(config *s2hv1beta1.ConfigSpec, envType s2hv1beta1.EnvType) (
	map[string]s2hv1beta1.ComponentValues, error) {

	chartValuesURLs, ok := config.Envs[envType]
	if !ok {
		return map[string]s2hv1beta1.ComponentValues{}, nil
	}

	var err error
	out := make(map[string]s2hv1beta1.ComponentValues)

	for chart := range chartValuesURLs {
		out[chart], err = GetEnvComponentValues(config, chart, envType)
		if err != nil {
			return map[string]s2hv1beta1.ComponentValues{}, err
		}
	}

	return out, nil
}

// GetEnvComponentValues returns component values by the given env type and component name
func GetEnvComponentValues(config *s2hv1beta1.ConfigSpec, compName string, envType s2hv1beta1.EnvType) (
	s2hv1beta1.ComponentValues, error) {

	opts := []http.Option{
		http.WithTimeout(10 * time.Second),
	}

	chartValuesURLs, ok := config.Envs[envType]
	if !ok {
		return s2hv1beta1.ComponentValues{}, nil
	}

	urls, ok := chartValuesURLs[compName]
	if !ok {
		return s2hv1beta1.ComponentValues{}, nil
	}

	baseValues := map[string]interface{}{}
	for _, url := range urls {
		_, valuesBytes, err := http.Get(url, opts...)
		if err != nil {
			return nil, errors.Wrapf(err,
				"cannot get values file of %s env from url %s", envType, url)
		}

		var v map[string]interface{}
		if err := yaml.Unmarshal(valuesBytes, &v); err != nil {
			logger.Error(err, "cannot parse component values",
				"env", envType, "component", compName)
			return nil, err
		}

		baseValues = valuesutil.MergeValues(baseValues, v)
	}

	return baseValues, nil
}

// assignParent assigns Parent to SubComponent
// only support 1 level of dependencies
func (c *controller) assignParent(config *s2hv1beta1.ConfigSpec) {
	comps := config.Components
	for i := range config.Components {
		for j := range comps[i].Dependencies {
			comps[i].Dependencies[j].Parent = comps[i].Name
		}
	}
}

func (c *controller) getConfig(configName string) (*s2hv1beta1.Config, error) {
	config := &s2hv1beta1.Config{}
	if err := c.client.Get(context.TODO(), types.NamespacedName{Name: configName}, config); err != nil {
		return config, err
	}

	return config, nil
}

// ensureComponentChanged detects added or removed component
func (c *controller) ensureComponentChanged(teamName, namespace string) error {
	comps, err := c.GetComponents(teamName)
	if err != nil {
		logger.Error(err, "cannot get components from configuration",
			"team", teamName, "namespace", namespace)
		return err
	}

	if err := c.detectRemovedDesiredComponents(comps, namespace); err != nil {
		return err
	}

	if err := c.detectRemovedTeamDesiredComponents(comps, teamName); err != nil {
		return err
	}

	if err := c.detectRemovedQueues(comps, namespace); err != nil {
		return err
	}

	if err := c.detectRemovedStableComponents(comps, namespace); err != nil {
		return err
	}

	c.notifyComponentChanged(teamName, namespace, comps)

	return nil
}

func (c *controller) notifyComponentChanged(teamName, namespace string, comps map[string]*s2hv1beta1.Component) {
	if c.s2hCtrl == nil {
		logger.Debug("no s2h ctrl, skip notify changed")
	}

	logger.Debug("start notifying components", "team", teamName, "namespace", namespace)
	for _, comp := range comps {
		c.s2hCtrl.NotifyComponentChanged(comp.Name, comp.Image.Repository)
	}
}

func (c *controller) detectRemovedDesiredComponents(comps map[string]*s2hv1beta1.Component, namespace string) error {
	ctx := context.Background()
	desiredComps := &s2hv1beta1.DesiredComponentList{}
	if err := c.client.List(ctx, desiredComps, &client.ListOptions{Namespace: namespace}); err != nil {
		logger.Error(err, "cannot list desired components", "namespace", namespace)
		return err
	}

	for i := len(desiredComps.Items) - 1; i >= 0; i-- {
		d := desiredComps.Items[i]
		if _, ok := comps[d.Name]; !ok {
			if err := c.client.Delete(ctx, &d); err != nil {
				logger.Error(err, "cannot remove desired component",
					"namespace", namespace, "component", d.Name)
				return err
			}

			logger.Debug("desired component has been removed",
				"namespace", namespace, "component", d.Name)
		}
	}

	return nil
}

func (c *controller) detectRemovedTeamDesiredComponents(comps map[string]*s2hv1beta1.Component, teamName string) error {
	if c.s2hCtrl == nil {
		logger.Debug("no s2h ctrl, skip detect team desired", "team", teamName)
		return nil
	}

	ctx := context.Background()
	teamComp := &s2hv1beta1.Team{}
	if err := c.s2hCtrl.GetTeam(teamName, teamComp); err != nil {
		logger.Error(err, "cannot get team", "team", teamName)
		return err
	}

	teamDesiredComps := teamComp.Status.DesiredComponentImageCreatedTime
	for td := range teamDesiredComps {
		if _, ok := comps[td]; !ok {
			logger.Debug("desired component has been removed from team",
				"team", teamName, "component", td)
			teamComp.Status.RemoveDesiredComponentImageCreatedTime(td)
		}
	}

	if err := c.client.Update(ctx, teamComp); err != nil {
		logger.Error(err, "cannot update team", "team", teamName)
		return err
	}

	return nil
}

func (c *controller) detectRemovedQueues(comps map[string]*s2hv1beta1.Component, namespace string) error {
	ctx := context.Background()
	qComps := &s2hv1beta1.QueueList{}
	if err := c.client.List(ctx, qComps, &client.ListOptions{Namespace: namespace}); err != nil {
		logger.Error(err, "cannot list queues", "namespace", namespace)
		return err
	}

	for i := len(qComps.Items) - 1; i >= 0; i-- {
		q := qComps.Items[i]
		if _, ok := comps[q.Name]; !ok {
			if err := c.client.Delete(ctx, &q); err != nil {
				logger.Error(err, "cannot remove queue",
					"namespace", namespace, "component", q.Name)
				return err
			}

			logger.Debug("queue has been removed",
				"namespace", namespace, "component", q.Name)
		}
	}

	return nil
}

func (c *controller) detectRemovedStableComponents(comps map[string]*s2hv1beta1.Component, namespace string) error {
	ctx := context.Background()
	stableComps := &s2hv1beta1.StableComponentList{}
	if err := c.client.List(ctx, stableComps, &client.ListOptions{Namespace: namespace}); err != nil {
		logger.Error(err, "cannot list stable components", "namespace", namespace)
		return err
	}

	for i := len(stableComps.Items) - 1; i >= 0; i-- {
		s := stableComps.Items[i]
		if _, ok := comps[s.Name]; !ok {
			if err := c.client.Delete(ctx, &s); err != nil {
				logger.Error(err, "cannot remove stable component",
					"namespace", namespace, "component", s.Name)
				return err
			}

			logger.Debug("stable component has been removed",
				"namespace", namespace, "component", s.Name)
		}
	}

	return nil
}

func (c *controller) Reconcile(req cr.Request) (cr.Result, error) {
	ctx := context.Background()
	configComp := &s2hv1beta1.Config{}
	if err := c.client.Get(ctx, req.NamespacedName, configComp); err != nil {
		if k8serrors.IsNotFound(err) {
			// Object not found, return. Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}

		logger.Error(err, "cannot get config", "team", req.Name, "namespace", req.Namespace)
		return cr.Result{}, err
	}

	if c.s2hCtrl == nil {
		logger.Debug("no s2h ctrl, skip detect changed component", "team", req.Name)
		return cr.Result{}, nil
	}

	teamComp := s2hv1beta1.Team{}
	if err := c.s2hCtrl.GetTeam(req.Name, &teamComp); err != nil {
		logger.Error(err, "cannot get team", "team", req.Name)
		return cr.Result{}, err
	}

	stagingNs := teamComp.Status.Namespace.Staging
	if stagingNs == "" {
		logger.Debug("no staging namespace to process", "team", req.Name)
		return cr.Result{}, nil
	}

	if err := c.ensureComponentChanged(req.Name, stagingNs); err != nil {
		return cr.Result{}, err
	}

	return cr.Result{}, nil
}
