package staging

import (
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	s2hv1beta1 "github.com/agoda-com/samsahai/api/v1beta1"
	"github.com/agoda-com/samsahai/internal"
	s2herrors "github.com/agoda-com/samsahai/internal/errors"
	"github.com/agoda-com/samsahai/internal/staging/testrunner/teamcity"
	"github.com/agoda-com/samsahai/internal/staging/testrunner/testmock"
)

const (
	testTimeout = 1800 * time.Second
	testPolling = 5 * time.Second
)

func (c *controller) startTesting(queue *s2hv1beta1.Queue) error {
	testingTimeout := metav1.Duration{Duration: testTimeout}
	if testConfig := c.getTestConfiguration(queue); testConfig != nil && testConfig.Timeout.Duration != 0 {
		testingTimeout = testConfig.Timeout
	}

	// check testing timeout
	if err := c.checkTestTimeout(queue, testingTimeout); err != nil {
		return err
	}

	skipTest, testRunner, err := c.checkTestConfig(queue)
	if err != nil {
		return err
	} else if skipTest {
		return nil
	}

	// trigger the test
	if err := c.triggerTest(queue, testRunner); err != nil {
		return err
	}

	// get result from test (polling check)
	if err := c.getTestResult(queue, testRunner); err != nil {
		return err
	}

	return nil
}

func (c *controller) checkTestTimeout(queue *s2hv1beta1.Queue, testingTimeout metav1.Duration) error {
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

// checkTestConfig checks test configuration and return testRunner
func (c *controller) checkTestConfig(queue *s2hv1beta1.Queue) (skipTest bool, testRunner internal.StagingTestRunner, err error) {
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
		testRunner = c.testRunners[teamcity.TestRunnerName]
	} else if testConfig.TestMock != nil {
		testRunner = c.testRunners[testmock.TestRunnerName]
	}

	if testRunner == nil {
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

func (c *controller) triggerTest(queue *s2hv1beta1.Queue, testRunner internal.StagingTestRunner) error {
	testConfig := c.getTestConfiguration(queue)
	if !queue.Status.IsConditionTrue(s2hv1beta1.QueueTestTriggered) {
		if err := testRunner.Trigger(testConfig, c.getCurrentQueue()); err != nil {
			logger.Error(err, "testing triggered error")
			return err
		}

		queue.Status.SetCondition(
			s2hv1beta1.QueueTestTriggered,
			v1.ConditionTrue,
			"queue testing triggered")

		// update queue back to k8s
		if err := c.updateQueue(queue); err != nil {
			return err
		}
	}

	return nil
}

func (c *controller) getTestResult(queue *s2hv1beta1.Queue, testRunner internal.StagingTestRunner) error {
	testConfig := c.getTestConfiguration(queue)
	isResultSuccess, isBuildFinished, err := testRunner.GetResult(testConfig, c.getCurrentQueue())
	if err != nil {
		logger.Error(err, "testing get result error")
		return err
	}

	if !isBuildFinished {
		pollingTime := metav1.Duration{Duration: testPolling}
		if c.getTestConfiguration(queue).PollingTime.Duration != 0 {
			pollingTime = c.getTestConfiguration(queue).PollingTime
		}
		time.Sleep(pollingTime.Duration)
		return nil
	}

	testCondition := v1.ConditionTrue
	message := "queue testing succeeded"
	if !isResultSuccess {
		testCondition = v1.ConditionFalse
		message = "queue testing failed"
	}

	if err := c.updateTestQueueCondition(queue, testCondition, message); err != nil {
		return err
	}

	return nil
}

// updateTestQueueCondition updates queue status, condition and save to k8s for Testing state
func (c *controller) updateTestQueueCondition(queue *s2hv1beta1.Queue, status v1.ConditionStatus, message string) error {
	// testing timeout
	queue.Status.SetCondition(
		s2hv1beta1.QueueTested,
		status,
		message)

	// update queue back to k8s
	return c.updateQueueWithState(queue, s2hv1beta1.Collecting)
}
