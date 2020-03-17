package internal

import (
	"github.com/agoda-com/samsahai/api/v1beta1"
)

type DeployEngine interface {
	// GetName returns name of deploy engine
	GetName() string

	// GetValues returns yaml values of release deployment
	GetValues() (map[string][]byte, error)

	// Create creates environment
	Create(refName string, comp *Component, parentComp *Component, values map[string]interface{}) error

	// Delete deletes environment
	Delete(refName string) error

	// ForceDelete deletes environment when timeout
	ForceDelete(refName string) error

	// IsReady checks the environment is ready to use or not
	IsReady(queue *v1beta1.Queue) (bool, error)

	// GetLabelSelector returns map of label for select the components that created by the engine
	GetLabelSelectors(refName string) map[string]string

	// IsMocked uses for skip some functions due to mock deploy
	//
	// Skipped function: WaitForComponentsCleaned
	IsMocked() bool
}

const (
	MaxReleaseNameLength = 200
)

// GenReleaseName returns the release name for deploying components
func GenReleaseName(teamName, namespace, compName string) string {
	refName := teamName + "-" + namespace + "-" + compName
	if len(refName) > MaxReleaseNameLength {
		// component name is more important than team name
		return refName[len(refName)-MaxReleaseNameLength:]
	}
	return refName
}
