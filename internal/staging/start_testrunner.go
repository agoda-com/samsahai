package staging

import (
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	s2hv1 "github.com/agoda-com/samsahai/api/v1"
	"github.com/agoda-com/samsahai/internal"
	s2herrors "github.com/agoda-com/samsahai/internal/errors"
	"github.com/agoda-com/samsahai/internal/staging/testrunner/gitlab"
	"github.com/agoda-com/samsahai/internal/staging/testrunner/teamcity"
	"github.com/agoda-com/samsahai/internal/staging/testrunner/testmock"
)

type testResult string

const (
	testTimeout = 30 * time.Minute // 30 minutes
	testPolling = 5 * time.Second  // 5 secs

	testResultSuccess testResult = "PASSED"
	testResultFailure testResult = "FAILED"
	testResultUnknown testResult = "UNKNOWN"
)

func (c *controller) startTesting(queue *s2hv1.Queue) error {
	testingTimeout := metav1.Duration{Duration: testTimeout}
	if testConfig := c.getTestConfiguration(queue); testConfig != nil && testConfig.Timeout.Duration != 0 {
		testingTimeout = testConfig.Timeout
	}

	// check testing timeout
	// if timeout, change state to `s2hv1.Collecting`
	if err := c.checkTestTimeout(queue, testingTimeout); err != nil {
		return err
	}

	// check test config
	// if no test configuration, change state to `s2hv1.Collecting`
	skipTest, testRunners, err := c.checkTestConfig(queue)
	if err != nil || skipTest {
		return err
	}

	// trigger the tests
	notTriggeredTest := !queue.Status.IsConditionTrue(s2hv1.QueueTestTriggered)
	if notTriggeredTest {
		for _, testRunner := range testRunners {
			_ = c.triggerTest(queue, testRunner)
		}

		// set state, test has been triggered
		queue.Status.SetCondition(
			s2hv1.QueueTestTriggered,
			v1.ConditionTrue,
			"queue testing triggered")

		// update queue back to k8s
		if err := c.updateQueue(queue); err != nil {
			return err
		}
	}

	// get result from tests (polling check)
	testCondition := v1.ConditionTrue
	message := "queue testing succeeded"
	for _, testRunner := range testRunners {
		testRunnerName := testRunner.GetName()
		testResult, err := c.getTestResult(queue, testRunner)

		unfinishedTest := err == nil && testResult == testResultUnknown
		if unfinishedTest {
			// return function, wait to be call again...
			return nil
		}

		// if finished, then update test result
		if err := c.setTestResultCondition(queue, testRunnerName, testResult); err != nil {
			return err
		}
	}

	// test finished, change state to `s2hv1.Collecting`
	return c.updateTestQueueCondition(queue, testCondition, message)
}

func (c *controller) checkTestTimeout(queue *s2hv1.Queue, testingTimeout metav1.Duration) error {
	now := metav1.Now()

	// check testing timeout
	if queue.Status.StartTestingTime != nil &&
		now.Sub(queue.Status.StartTestingTime.Time) > testingTimeout.Duration {

		// testing timeout
		if err := c.updateTestQueueCondition(queue, v1.ConditionFalse, "queue testing timeout"); err != nil {
			return err
		}

		logger.Error(s2herrors.ErrTestTimeout, "test timeout")
		return s2herrors.ErrTestTimeout
	}

	return nil
}

// checkTestConfig checks test configuration and return list of testRunners
func (c *controller) checkTestConfig(queue *s2hv1.Queue) (
	skipTest bool, testRunners []internal.StagingTestRunner, err error) {

	testRunners = make([]internal.StagingTestRunner, 0)

	if queue.Spec.SkipTestRunner {
		if err = c.updateTestQueueCondition(
			queue,
			v1.ConditionTrue,
			"skip running test"); err != nil {
			return
		}

		return true, nil, nil
	}

	testConfig := c.getTestConfiguration(queue)
	if testConfig == nil {
		if err = c.updateTestQueueCondition(
			queue,
			v1.ConditionTrue,
			"queue testing succeeded because no testing configuration"); err != nil {
			return
		}

		return true, nil, nil
	}

	skipTest = false

	if testConfig.Teamcity != nil {
		testRunners = append(testRunners, c.testRunners[teamcity.TestRunnerName])
	}
	if testConfig.Gitlab != nil {
		testRunners = append(testRunners, c.testRunners[gitlab.TestRunnerName])
	}
	if testConfig.TestMock != nil {
		testRunners = append(testRunners, c.testRunners[testmock.TestRunnerName])
	}

	if len(testRunners) == 0 {
		if err = c.updateTestQueueCondition(queue, v1.ConditionFalse, "test runner not found"); err != nil {
			return
		}
		logger.Error(s2herrors.ErrTestRunnerNotFound, "test runner not found (testRunner: nil)")
		err = s2herrors.ErrTestRunnerNotFound
		return
	}

	now := metav1.Now()
	if queue.Status.StartTestingTime == nil {
		queue.Status.StartTestingTime = &now
		err = c.updateQueue(queue)
	}
	return
}

func (c *controller) triggerTest(queue *s2hv1.Queue, testRunner internal.StagingTestRunner) error {
	testRunnerName := testRunner.GetName()
	testConfig := c.getTestConfiguration(queue)

	// trigger test and update k8s object
	if err := testRunner.Trigger(testConfig, c.getCurrentQueue()); err != nil {
		logger.Error(err, "testing triggered error", "name", testRunnerName)
		return err
	}

	// set teamcity build number to message
	if tr := testRunner.GetName(); tr == teamcity.TestRunnerName {
		queue.Status.TestRunner.Teamcity.BuildNumber = "Build cannot be triggered in time"
	}

	return nil
}

func (c *controller) getTestResult(queue *s2hv1.Queue, testRunner internal.StagingTestRunner) (testResult, error) {
	testRunnerName := testRunner.GetName()
	testConfig := c.getTestConfiguration(queue)
	isResultSuccess, isBuildFinished, err := testRunner.GetResult(testConfig, c.getCurrentQueue())
	if err != nil {
		logger.Error(err, "testing get result error", "name", testRunnerName)
		return testResultUnknown, err
	}

	if !isBuildFinished {
		pollingTime := metav1.Duration{Duration: testPolling}
		if c.getTestConfiguration(queue).PollingTime.Duration != 0 {
			pollingTime = c.getTestConfiguration(queue).PollingTime
		}
		time.Sleep(pollingTime.Duration)
		return testResultUnknown, nil
	}

	if !isResultSuccess {
		return testResultFailure, nil
	}
	return testResultSuccess, nil
}

// updateTestQueueCondition updates queue status, condition and save to k8s for Testing state
func (c *controller) updateTestQueueCondition(queue *s2hv1.Queue, status v1.ConditionStatus, message string) error {
	// testing timeout
	queue.Status.SetCondition(
		s2hv1.QueueTested,
		status,
		message)

	// update queue back to k8s
	return c.updateQueueWithState(queue, s2hv1.Collecting)
}

func (c *controller) setTestResultCondition(queue *s2hv1.Queue, testRunnerName string, testResult testResult) error {

	var condType s2hv1.QueueConditionType
	switch testRunnerName {
	case gitlab.TestRunnerName:
		condType = s2hv1.QueueGitlabTestResult
	case teamcity.TestRunnerName:
		condType = s2hv1.QueueTeamcityTestResult
	default:
		return nil
	}

	message := "queue testing succeeded"
	cond := v1.ConditionTrue
	switch testResult {
	case testResultUnknown:
		message = "unable to get result from runner"
		cond = v1.ConditionFalse
	case testResultFailure:
		message = "queue testing of failed"
		cond = v1.ConditionFalse
	case testResultSuccess:
	}

	// testing timeout
	queue.Status.SetCondition(
		condType,
		cond,
		message)

	// update queue back to k8s
	return c.updateQueue(queue)
}
