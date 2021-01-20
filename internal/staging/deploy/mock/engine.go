package mock

import (
	"fmt"
	"time"

	"helm.sh/helm/v3/pkg/release"

	s2hv1 "github.com/agoda-com/samsahai/api/v1"
	"github.com/agoda-com/samsahai/internal"
	s2hlog "github.com/agoda-com/samsahai/internal/log"
)

var logger = s2hlog.Log.WithName(EngineName)

const (
	EngineName = "mock"
)

type CreateCallbackFn func(refName string, comp *s2hv1.Component, parentComp *s2hv1.Component, values map[string]interface{}, deployTimeout *time.Duration)
type DeleteCallbackFn func(refName string)

type engine struct {
	createFn CreateCallbackFn
	deleteFn DeleteCallbackFn
}

// New creates a new teamcity test runner
func New() internal.DeployEngine {
	return &engine{}
}

// New creates a new teamcity test runner
func NewWithCallback(creFn CreateCallbackFn, delFn DeleteCallbackFn) internal.DeployEngine {
	return &engine{
		createFn: creFn,
		deleteFn: delFn,
	}
}

func (e *engine) Create(
	refName string,
	comp *s2hv1.Component,
	parentComp *s2hv1.Component,
	values map[string]interface{},
	deployTimeout *time.Duration,
) error {
	if e.createFn != nil {
		e.createFn(refName, comp, parentComp, values, deployTimeout)
	}
	logger.Debug(fmt.Sprintf("create env with resource key: %s", refName))
	return nil
}

func (e *engine) Rollback(refName string, revision int) error {
	logger.Debug(fmt.Sprintf("rollback env with resource key: %s", refName))
	return nil
}

func (e *engine) GetHistories(refName string) ([]*release.Release, error) {
	logger.Debug(fmt.Sprintf("get helm histories of resource key: %s", refName))
	return []*release.Release{}, nil
}

func (e *engine) Delete(refName string) error {
	if e.deleteFn != nil {
		e.deleteFn(refName)
	}
	logger.Debug(fmt.Sprintf("delete env with resource key: %s", refName))
	return nil
}

func (e *engine) ForceDelete(refName string) error {
	logger.Debug(fmt.Sprintf("force delete env with resource key: %s", refName))
	return nil
}

func (e *engine) GetValues() (map[string][]byte, error) {
	logger.Debug("get yaml values of release")
	return nil, nil
}

func (e *engine) GetReleases() ([]*release.Release, error) {
	logger.Debug("get all releases")
	return []*release.Release{}, nil
}

func (e *engine) GetName() string {
	return EngineName
}

func (e *engine) GetLabelSelectors(refName string) map[string]string {
	return nil
}

func (e *engine) IsMocked() bool {
	return true
}
