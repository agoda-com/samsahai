package internal

import (
	"net/http"

	s2hv1 "github.com/agoda-com/samsahai/api/v1"
	stagingrpc "github.com/agoda-com/samsahai/pkg/staging/rpc"
)

// StagingConfig represents configuration of Staging
type StagingConfig struct {
	// MaxHistoryDays defines maximum days of QueueHistory stored
	MaxHistoryDays int `json:"maxHistoryDays" yaml:"maxHistoryDays"`
}

type StagingTestRunner interface {
	// GetName returns type of test runner
	GetName() string

	// Trigger makes http request to run the test build
	Trigger(testConfig *s2hv1.ConfigTestRunner, currentQueue *s2hv1.Queue) error

	// GetResult makes http request to get result of test build [FAILURE/SUCCESS/UNKNOWN]
	// It returns bool results of is build success and is build finished
	GetResult(testConfig *s2hv1.ConfigTestRunner, currentQueue *s2hv1.Queue) (
		isResultSuccess bool, isBuildFinished bool, err error)

	// IsTriggered check if the test has been triggered or not
	IsTriggered(queue *s2hv1.Queue) bool
}

type StagingController interface {
	// should implement RPC
	stagingrpc.RPC

	// should be able to serve http
	http.Handler

	// Start runs internal worker
	Start(stop <-chan struct{})

	// IsBusy returns true if controller still processing queue
	IsBusy() bool

	// LoadTestRunner loads single test runner to controller
	LoadTestRunner(runner StagingTestRunner)

	// LoadDeployEngine loads single deploy engine to controller
	LoadDeployEngine(engine DeployEngine)
}

type PullRequestTriggerController interface {
}
