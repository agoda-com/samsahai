package internal

import (
	s2hv1 "github.com/agoda-com/samsahai/api/v1"
)

type ConfigController interface {
	// Get returns configuration from memory
	Get(configName string) (*s2hv1.Config, error)

	// GetComponents returns all components from `Configuration` that has valid `Source`
	GetComponents(configName string) (map[string]*s2hv1.Component, error)

	// GetParentComponents returns components that doesn't have parent (nil Parent)
	GetParentComponents(configName string) (map[string]*s2hv1.Component, error)

	// GetPullRequestComponents returns all pull request components of a given bundle name
	// with or without dependencies from `Configuration` that has valid `Source`
	GetPullRequestComponents(configName, prBundleName string, depIncluded bool) (map[string]*s2hv1.Component, error)

	// GetBundles returns a group of components for each bundle
	GetBundles(configName string) (s2hv1.ConfigBundles, error)

	// GetPriorityQueues returns a list of priority queues which defined in Config
	GetPriorityQueues(configName string) ([]string, error)

	// GetPullRequestConfig returns a configuration of pull request
	GetPullRequestConfig(configName string) (*s2hv1.ConfigPullRequest, error)

	// GetPullRequestBundleDependencies returns dependencies list of a pull request bundle from configuration
	GetPullRequestBundleDependencies(configName, prBundleName string) ([]string, error)

	// Update updates Config CRD
	Update(config *s2hv1.Config) error

	// Delete deletes Config CRD
	Delete(configName string) error

	//EnsureConfigTemplateChanged updates config if template changed
	EnsureConfigTemplateChanged(config *s2hv1.Config) error
}
