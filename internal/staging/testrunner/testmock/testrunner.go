package testmock

import (
	"github.com/pkg/errors"

	"github.com/agoda-com/samsahai/api/v1beta1"
	"github.com/agoda-com/samsahai/internal"
	s2herrors "github.com/agoda-com/samsahai/internal/errors"
	s2hlog "github.com/agoda-com/samsahai/internal/log"
)

var logger = s2hlog.Log.WithName(TestRunnerName)

const (
	TestRunnerName = "testmock"
)

type testRunner struct{}

// New creates a new teamcity test runner
func New() internal.StagingTestRunner {
	return &testRunner{}
}

// GetName implements the staging testRunner GetName function
func (t *testRunner) GetName() string {
	return TestRunnerName
}

// Trigger implements the staging testRunner Trigger function
func (t *testRunner) Trigger(testConfig *internal.ConfigTestRunner, currentQueue *v1beta1.Queue) error {
	logger.Info("triggered")
	return nil
}

// GetResult implements the staging testRunner GetResult function
func (t *testRunner) GetResult(testConfig *internal.ConfigTestRunner, currentQueue *v1beta1.Queue) (isResultSuccess bool, isBuildFinished bool, err error) {
	if testConfig == nil {
		return false, false, errors.Wrapf(s2herrors.ErrTestConfiigurationNotFound,
			"test configuration should not be nil. queue: %s", currentQueue.Name)
	}

	if testConfig.TestMock == nil {
		return false, true, nil
	}

	result := testConfig.TestMock.Result
	if result {
		return true, true, nil
	}

	return false, true, nil
}
