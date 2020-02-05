package mock

import (
	"fmt"

	"github.com/agoda-com/samsahai/api/v1beta1"
	"github.com/agoda-com/samsahai/internal"
	s2hlog "github.com/agoda-com/samsahai/internal/log"
)

var logger = s2hlog.Log.WithName(EngineName)

const (
	EngineName = "mock"
)

type CreateCallbackFn func(refName string, comp *internal.Component, parentComp *internal.Component, values map[string]interface{})
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
	comp *internal.Component,
	parentComp *internal.Component,
	values map[string]interface{},
) error {
	if e.createFn != nil {
		e.createFn(refName, comp, parentComp, values)
	}
	logger.Debug(fmt.Sprintf("create env with resource key: %s", refName))
	return nil
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

func (e *engine) IsReady(queue *v1beta1.Queue) (bool, error) {
	logger.Debug(fmt.Sprintf("env with resource key '%s' is ready", queue.Status.ReleaseName))
	return true, nil
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
