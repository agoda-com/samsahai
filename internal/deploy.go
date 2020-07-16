package internal

import (
	"time"

	"github.com/agoda-com/samsahai/api/v1beta1"
)

type DeployEngine interface {
	// GetName returns name of deploy engine
	GetName() string

	// GetValues returns yaml values of release deployment
	GetValues() (map[string][]byte, error)

	// Create creates environment
	Create(refName string, comp *v1beta1.Component, parentComp *v1beta1.Component, values map[string]interface{}, deployTimeout time.Duration) error

	// Delete deletes environment
	Delete(refName string) error

	// ForceDelete deletes environment when timeout
	ForceDelete(refName string) error

	// GetLabelSelector returns map of label for select the components that created by the engine
	GetLabelSelectors(refName string) map[string]string

	// IsMocked uses for skip some functions due to mock deploy
	//
	// Skipped function: WaitForComponentsCleaned
	IsMocked() bool
}

const (
	MaxReleaseNameLength = 53
)

// GenReleaseName returns the release name for deploying components
func GenReleaseName(namespace, compName string) string {
	refName := namespace + "-" + compName
	if len(refName) > MaxReleaseNameLength {
		// component name is more important than team name
		return refName[len(refName)-MaxReleaseNameLength:]
	}
	return refName
}
