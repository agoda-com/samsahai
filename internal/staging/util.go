package staging

import (
	"context"
	"fmt"
	"net/http"
	"time"

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

	if queue.IsActivePromotionQueue() {
		if cfg.ActivePromotion != nil && cfg.ActivePromotion.Deployment != nil {
			return cfg.ActivePromotion.Deployment
		}
		return &s2hv1.ConfigDeploy{}
	}
	if cfg.Staging != nil {
		return cfg.Staging.Deployment
	}
	return &s2hv1.ConfigDeploy{}
}

func (c *controller) getTestConfiguration(queue *s2hv1.Queue) *s2hv1.ConfigTestRunner {
	deployConfig := c.getDeployConfiguration(queue)
	if deployConfig == nil {
		return nil
	}

	return deployConfig.TestRunner
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
		if err := c.client.Delete(context.TODO(), q); err != nil {
			logger.Error(err, "deleting queue error")
			return err
		}
	} else if isReverify {
		// reverify
		// TODO: fix me, 24 hours hard-code
		if err := c.queueCtrl.SetRetryQueue(q, 0, time.Now().Add(24*time.Hour)); err != nil {
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
			if err := c.queueCtrl.SetRetryQueue(q, q.Spec.NoOfRetry, time.Now()); err != nil {
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
