package staging

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/agoda-com/samsahai/internal/util/gitlab"

	"github.com/pkg/errors"
	"github.com/twitchtv/twirp"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	s2hv1 "github.com/agoda-com/samsahai/api/v1"
	"github.com/agoda-com/samsahai/internal"
	"github.com/agoda-com/samsahai/internal/staging/deploy/mock"
	"github.com/agoda-com/samsahai/pkg/samsahai/rpc"
)

func (c *controller) getDeployConfiguration(queue *s2hv1.Queue) *s2hv1.ConfigDeploy {
	cfg, err := c.getConfiguration()
	if err != nil {
		logger.Error(err, "cannot get configuration", "team", c.teamName)
		return &s2hv1.ConfigDeploy{}
	}

	configDeploy := &s2hv1.ConfigDeploy{}

	switch {
	case queue.IsActivePromotionQueue():
		if cfg.ActivePromotion != nil && cfg.ActivePromotion.Deployment != nil {
			configDeploy = cfg.ActivePromotion.Deployment
		}
	case queue.IsPullRequestQueue():
		bundleName := queue.Spec.Name
		if cfg.PullRequest != nil && len(cfg.PullRequest.Bundles) > 0 {
			for _, bundle := range cfg.PullRequest.Bundles {
				if bundle.Name == bundleName {
					configDeploy = bundle.Deployment
				}
			}
		}
	default:
		if cfg.Staging != nil {
			configDeploy = cfg.Staging.Deployment
		}
	}

	return configDeploy
}

func (c *controller) getTestConfiguration(queue *s2hv1.Queue) *s2hv1.ConfigTestRunner {
	deployConfig := c.getDeployConfiguration(queue)
	if deployConfig == nil {
		return nil
	}

	testRunner := deployConfig.TestRunner

	// override testRunner
	if testRunnerOverrider := queue.GetTestRunnerExtraParameter(); testRunnerOverrider != nil {
		testRunner = testRunnerOverrider.Override(testRunner)
	}

	// try to infer gitlab MR branch in PR flow
	if queue.IsPullRequestQueue() && testRunner != nil {
		gitlabClientGetter := func() gitlab.Gitlab {
			return gitlab.NewClient(c.gitlabBaseURL, c.gitlabToken)
		}
		tryInferPullRequestGitlabBranch(testRunner.Gitlab, queue.Spec.PRNumber, gitlabClientGetter)
	}
	return testRunner
}

// tryInferPullRequestGitlabBranch will check whether the Gitlab MR could be fetched from project ID and pipeline token
// and override branch in testRunner with the associated MR branch.
func tryInferPullRequestGitlabBranch(confGitlab *s2hv1.ConfigGitlab, MRiid string,
	gitlabClientGetter func() gitlab.Gitlab) {

	confGitlabExists := confGitlab != nil

	// infer branch only if inferBranch flag == True or default branch is not set
	canInferBranch := confGitlabExists &&
		(confGitlab.GetInferBranch() ||
			confGitlab.Branch == "")
	canQueryGitlab := confGitlabExists &&
		confGitlab.ProjectID != "" &&
		confGitlab.PipelineTriggerToken != ""

	if canInferBranch && canQueryGitlab && gitlabClientGetter != nil {
		gl := gitlabClientGetter()
		if gl != nil {
			branch, err := gl.GetMRSourceBranch(confGitlab.ProjectID, MRiid)
			// silently ignore error (in case of error, don't override the branch)
			if err == nil && branch != "" {
				confGitlab.Branch = branch
			}
		}
	}
}

func (c *controller) getDeployEngine(queue *s2hv1.Queue) internal.DeployEngine {
	// Try to get DeployEngine from Queue
	if _, ok := c.deployEngines[queue.Status.DeployEngine]; queue.Status.DeployEngine != "" && ok {
		return c.deployEngines[queue.Status.DeployEngine]
	}

	// Get DeployEngine from configuration
	deployConfig := c.getDeployConfiguration(queue)

	var e string
	if deployConfig == nil || deployConfig.Engine == nil || *deployConfig.Engine == "" {
		e = mock.EngineName
	} else {
		e = *deployConfig.Engine
	}
	engine, ok := c.deployEngines[e]
	if !ok {
		logger.Warn("fallback to mock engine")
		return c.deployEngines[mock.EngineName]
	}
	return engine
}

func (c *controller) clearCurrentQueue() {
	c.mtQueue.Lock()
	defer c.mtQueue.Unlock()
	c.currentQueue = nil
}

func (c *controller) getCurrentQueue() *s2hv1.Queue {
	c.mtQueue.Lock()
	defer c.mtQueue.Unlock()
	return c.currentQueue
}

func (c *controller) updateQueue(queue *s2hv1.Queue) error {
	if err := c.client.Update(context.TODO(), queue); err != nil {
		return errors.Wrap(err, "updating queue error")
	}
	return nil
}

func (c *controller) deleteQueue(q *s2hv1.Queue) error {
	// update queue history before processing next queue
	if err := c.updateQueueHistory(q); err != nil {
		return errors.Wrap(err, "updating queuehistory error")
	}

	isDeploySuccess, isTestSuccess, isReverify := q.IsDeploySuccess(), q.IsTestSuccess(), q.IsReverify()

	if isDeploySuccess && isTestSuccess && !isReverify {
		// success deploy and test without reverify state
		// delete queue
		if err := c.client.Delete(context.TODO(), q); err != nil && !k8serrors.IsNotFound(err) {
			logger.Error(err, "deleting queue error")
			return err
		}
	} else if isReverify {
		// reverify
		// TODO: fix me, 24 hours hard-code
		if err := c.queueCtrl.SetRetryQueue(q, 0, time.Now().Add(24*time.Hour),
			nil, nil, nil); err != nil {
			logger.Error(err, "cannot set retry queue")
			return err
		}
	} else {
		// Testing or deploying failed
		// Retry this component
		q.Spec.NoOfRetry++

		maxNoOfRetry := 0

		cfg, err := c.getConfiguration()
		if err != nil {
			return err
		}

		if cfg != nil && cfg.Staging != nil {
			maxNoOfRetry = cfg.Staging.MaxRetry
		}

		if q.Spec.NoOfRetry > maxNoOfRetry {
			// Retry reached maximum retry limit, we need to verify that is our system still ok?
			if err := c.queueCtrl.SetReverifyQueueAtFirst(q); err != nil {
				logger.Error(err, "cannot set reverify queue")
				return err
			}
		} else {
			if err := c.queueCtrl.SetRetryQueue(q, q.Spec.NoOfRetry, time.Now(),
				nil, nil, nil); err != nil {
				logger.Error(err, "cannot set retry queue")
				return err
			}
		}
	}

	c.clearCurrentQueue()

	return nil
}

func (c *controller) updateQueueWithState(q *s2hv1.Queue, state s2hv1.QueueState) error {
	headers := make(http.Header)
	headers.Set(internal.SamsahaiAuthHeader, c.authToken)
	ctx := context.TODO()
	ctx, err := twirp.WithHTTPRequestHeaders(ctx, headers)
	if err != nil {
		return errors.Wrap(err, "cannot set request header")
	}

	q.SetState(state)
	logger.Debug(fmt.Sprintf("queue %s/%s update to state: %s", q.GetNamespace(), q.GetName(), q.Status.State))
	comp := &rpc.ComponentUpgrade{
		Name:      q.Spec.Name,
		Namespace: q.Namespace,
	}

	if c.s2hClient != nil {
		if _, err := c.s2hClient.SendUpdateStateQueueMetric(ctx, comp); err != nil {
			logger.Error(err, "cannot send updateQueueWithState queue metric")
		}
	}

	return c.updateQueue(q)
}

func (c *controller) genReleaseName(comp *s2hv1.Component) string {
	return internal.GenReleaseName(c.namespace, comp.Name)
}

func (c *controller) updateQueueHistory(q *s2hv1.Queue) error {
	ctx := context.TODO()

	qHistName := q.Status.QueueHistoryName
	fetched := &s2hv1.QueueHistory{}
	err := c.client.Get(ctx, types.NamespacedName{Name: qHistName, Namespace: c.namespace}, fetched)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			logger.Warnf("queuehistory %s not found, creating", qHistName)
			if err := c.createQueueHistory(q); err != nil {
				return err
			}
			return nil
		}

		logger.Error(err, fmt.Sprintf("cannot get queuehistory: %s", qHistName))
		return err
	}

	fetched.Spec.Queue = &s2hv1.Queue{
		Spec:   q.Spec,
		Status: q.Status,
	}

	if err := c.client.Update(ctx, fetched); err != nil {
		logger.Error(err, fmt.Sprintf("cannot update queuehistory: %s", qHistName))
		return err
	}

	return nil
}
