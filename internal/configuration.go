package internal

import (
	s2hv1beta1 "github.com/agoda-com/samsahai/api/v1beta1"
)

type ConfigController interface {
	// Get returns configuration from memory
	Get(configName string) (*s2hv1beta1.Config, error)

	// GetComponents returns all components from `Configuration` that has valid `Source`
	GetComponents(configName string) (map[string]*s2hv1beta1.Component, error)

	//GetParentComponents returns components that doesn't have parent (nil Parent)
	GetParentComponents(configName string) (map[string]*s2hv1beta1.Component, error)

	// Update updates Config CRD
	Update(config *s2hv1beta1.Config) error

	// Delete deletes Config CRD
	Delete(configName string) error
}
