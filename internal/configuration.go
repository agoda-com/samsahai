package internal

import (
	s2hv1beta1 "github.com/agoda-com/samsahai/api/v1beta1"
)

type ConfigController interface {
	// Get returns configuration from memory
	Get(configName string) (*s2hv1beta1.ConfigSpec, error)

	// GetComponents returns all components from `Configuration` that has valid `Source`
	GetComponents(configName string) (map[string]*s2hv1beta1.Component, error)

	//GetParentComponents returns components that doesn't have parent (nil Parent)
	GetParentComponents(configName string) (map[string]*s2hv1beta1.Component, error)

	// Delete delete Config CRD
	Delete(configName string) error

	//// GetEnvValues returns component values by env type
	//GetEnvValues(teamName, compName string, envType s2hv1beta1.EnvType) (s2hv1beta1.ComponentValues, error)
}
