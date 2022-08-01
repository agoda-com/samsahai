package staging

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	s2hv1 "github.com/agoda-com/samsahai/api/v1"
	"github.com/agoda-com/samsahai/internal"
	s2herrors "github.com/agoda-com/samsahai/internal/errors"
	"github.com/agoda-com/samsahai/internal/staging/testrunner/gitlab"
	"github.com/agoda-com/samsahai/internal/staging/testrunner/teamcity"
	"github.com/agoda-com/samsahai/internal/staging/testrunner/testmock"
	samsahairpc "github.com/agoda-com/samsahai/pkg/samsahai/rpc"
)

type testResult string

const (
	testTimeout        = 30 * time.Minute // 30 minutes
	testPolling        = 5 * time.Second  // 5 secs
	testTriggerTimeout = 1 * time.Minute  // 1 minutes

	testResultRetry = 3 // 3 times

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
	notReachTriggerTimeout := queue.Status.StartTestingTime != nil &&
		metav1.Now().Sub(queue.Status.StartTestingTime.Time) <= testTriggerTimeout
	// check testing timeout
	if notTriggeredTest && notReachTriggerTimeout {
		err = c.triggerTest(queue, testRunners)
		if err != nil {
			// retry util time passed testTriggerTimeout
			return err
		}
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

	// send commit status while test is running
	if queue.Spec.Type == s2hv1.QueueTypePullRequest {
		if _, err := c.s2hClient.RunPostPullRequestQueueTestRunnerTrigger(context.TODO(), &samsahairpc.TeamWithPullRequest{
			TeamName:   c.teamName,
			Namespace:  queue.Namespace,
			BundleName: queue.Spec.Bundle,
		}); err != nil {
			return errors.Wrapf(err,
				"cannot send pull request test runner pending status report, team: %s, component: %s, prNumber: %s",
				c.teamName, queue.Spec.Bundle, queue.Spec.PRNumber)
		}
	}

	// get result from tests (polling check)
	testCondition := v1.ConditionTrue
	message := "queue testing succeeded"
	allTestFinished := true
	triggerTestMsg := ""
	for _, testRunner := range testRunners {
		testRunnerName := testRunner.GetName()

		if !testRunner.IsTriggered(queue) {
			// add message in k8s object
			triggerTestMsg += fmt.Sprintf("cannot trigger test on %s, ", testRunnerName)
			continue
		}

		testResult, err := c.getTestResult(queue, testRunner)

		// unfinished test
		if testResult == testResultUnknown && err == nil {
			allTestFinished = false
			continue
		}

		// if finished, then update test result
		if err := c.setTestResultCondition(queue, testRunnerName, testResult); err != nil {
			return err
		}

		// if some runners test were failed
		if testResult == testResultUnknown || testResult == testResultFailure {
			testCondition = v1.ConditionFalse
			message = "queue testing failed"
			if triggerTestMsg != "" {
				message = strings.TrimSuffix(triggerTestMsg, ", ")
			}
		}
	}
	// test finished, change state to `s2hv1.Collecting`
	if allTestFinished {
		return c.updateTestQueueCondition(queue, testCondition, message)
	}
	return nil
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

func (c *controller) triggerTest(queue *s2hv1.Queue, testRunners []internal.StagingTestRunner) error {
	var wg sync.WaitGroup
	errs := make([]error, len(testRunners))
	testConfig := c.getTestConfiguration(queue)

	for i, testRunner := range testRunners {
		// if test is not triggered yet, do...
		if !testRunner.IsTriggered(queue) {
			wg.Add(1)
			go func(runner internal.StagingTestRunner) {
				defer wg.Done()
				// trigger test and update k8s object
				if err := runner.Trigger(testConfig, c.getCurrentQueue()); err != nil {
					logger.Error(err, "testing triggered error", "name", runner.GetName())
					errs[i] = err
					return
				}

				// set teamcity build number to message
				if tr := runner.GetName(); tr == teamcity.TestRunnerName {
					queue.Status.TestRunner.Teamcity.BuildNumber = "Build cannot be triggered in time"
				}
			}(testRunner)
		}
	}

	// wait all trigger complete
	wg.Wait()

	// if some trigger error, retry util exceed testTriggerTimeout
	for _, err := range errs {
		if err != nil {
			time.Sleep(testPolling)
			return err
		}
	}

	return nil
}

func (c *controller) getTestResult(queue *s2hv1.Queue, testRunner internal.StagingTestRunner) (testResult, error) {
	pollingTime := metav1.Duration{Duration: testPolling}
	if c.getTestConfiguration(queue).PollingTime.Duration != 0 {
		pollingTime = c.getTestConfiguration(queue).PollingTime
	}

	testRunnerName := testRunner.GetName()
	testConfig := c.getTestConfiguration(queue)

	// Getting result with retry MAXRETRY times
	var isResultSuccess, isBuildFinished bool
	var err error
	for retry := 0; retry <= testResultRetry; retry++ {
		isResultSuccess, isBuildFinished, err = testRunner.GetResult(testConfig, c.getCurrentQueue())
		if err != nil {
			// if configuration is invalid, or test is not triggered in first place
			if isBuildFinished || retry == testResultRetry {
				logger.Error(err, "testing get result error", "name", testRunnerName)
				return testResultUnknown, err
			}
			// if getting result failed
			// sleep and try fetch result again...
			logger.Debug("waiting for test result", "name", testRunnerName)
			time.Sleep(pollingTime.Duration)
		}
	}

	// if test is still running
	if !isBuildFinished {
		time.Sleep(pollingTime.Duration)
		return testResultUnknown, nil
	}

	// if test finished and result failed
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
		message = "queue testing failed"
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
