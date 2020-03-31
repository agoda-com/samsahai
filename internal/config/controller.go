package config

import (
	"context"
	"time"

	"github.com/ghodss/yaml"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	cr "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	s2hv1beta1 "github.com/agoda-com/samsahai/api/v1beta1"
	"github.com/agoda-com/samsahai/internal"
	"github.com/agoda-com/samsahai/internal/errors"
	s2hlog "github.com/agoda-com/samsahai/internal/log"
	"github.com/agoda-com/samsahai/internal/util/http"
	"github.com/agoda-com/samsahai/internal/util/valuesutil"
)

var logger = s2hlog.Log.WithName(CtrlName)

const (
	CtrlName = "config-ctrl"
)

type controller struct {
	client client.Client
}

type Option func(*controller)

func WithClient(client client.Client) Option {
	return func(c *controller) {
		c.client = client
	}
}

func New(mgr cr.Manager, options ...Option) internal.ConfigController {
	c := &controller{}

	if mgr != nil {
		c.client = mgr.GetClient()
	}

	for _, opt := range options {
		opt(c)
	}

	return c
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
		valuesBytes, err := http.Get(url, opts...)
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
