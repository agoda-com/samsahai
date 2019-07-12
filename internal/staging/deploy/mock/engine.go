package mock

import (
	"fmt"

	"github.com/agoda-com/samsahai/internal"
	s2hlog "github.com/agoda-com/samsahai/internal/log"
	"github.com/agoda-com/samsahai/pkg/apis/env/v1beta1"
)

var logger = s2hlog.Log.WithName(EngineName)

const (
	EngineName = "mock"
)

type CreateCallbackFn func(refName string, comp *internal.Component, parentComp *internal.Component, values map[string]interface{})
type DeleteCallbackFn func(queue *v1beta1.Queue)

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

func (e *engine) Delete(queue *v1beta1.Queue) error {
	if e.deleteFn != nil {
		e.deleteFn(queue)
	}
	logger.Debug(fmt.Sprintf("delete env with resource key: %s", queue.Status.ReleaseName))
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
