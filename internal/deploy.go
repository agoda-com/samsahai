package internal

import (
	"time"

	"helm.sh/helm/v3/pkg/release"
	"sigs.k8s.io/controller-runtime/pkg/client"

	s2hv1 "github.com/agoda-com/samsahai/api/v1"
)

type DeployEngine interface {
	// GetName returns name of deploy engine
	GetName() string

	// GetValues returns yaml values of release deployment
	GetValues() (map[string][]byte, error)

	// Create creates environment
	Create(refName string, comp *s2hv1.Component, parentComp *s2hv1.Component, values map[string]interface{}, deployTimeout *time.Duration) error

	// Rollback rollback helm release
	Rollback(refName string, revision int) error

	// GetHistories returns histories of release
	GetHistories(refName string) ([]*release.Release, error)

	// Delete deletes environment
	Delete(refName string) error

	// ForceDelete deletes environment when timeout
	ForceDelete(refName string) error

	// GetLabelSelectors returns map of label for select the components that created by the engine
	GetLabelSelectors(refName string) map[string]string

	// GetReleases returns all deployed releases
	GetReleases() ([]*release.Release, error)

	// WaitForPreHookReady waits until all pre-hook pods are completed
	WaitForPreHookReady(k8sClient client.Client, refName string) (bool, error)

	// IsMocked uses for skip some functions due to mock deploy
	//
	// Skipped function: WaitForComponentsCleaned
	IsMocked() bool
}

const (
	MaxReleaseNameLength = 45
)

// GenReleaseName returns the release name for deploying components
func GenReleaseName(compName string) string {
	if len(compName) > MaxReleaseNameLength {
		// component name is more important than team name
		return compName[len(compName)-MaxReleaseNameLength:]
	}
	return compName
}
