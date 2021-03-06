package queue

import (
	"context"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"

	s2hv1 "github.com/agoda-com/samsahai/api/v1"
	"github.com/agoda-com/samsahai/internal/queue"
	samsahairpc "github.com/agoda-com/samsahai/pkg/samsahai/rpc"
)

func (c *controller) collectPullRequestQueueResult(ctx context.Context, prQueue *s2hv1.PullRequestQueue) error {
	prComps := prQueue.Spec.Components
	prNamespace := prQueue.Status.PullRequestNamespace

	if prQueue.Spec.IsPRTriggerFailed != nil && !*prQueue.Spec.IsPRTriggerFailed {
		deployedQueue, err := c.ensurePullRequestComponents(prQueue, prComps)
		if err != nil {
			return errors.Wrapf(err, "cannot ensure pull request components, namespace %s", prNamespace)
		}
		prQueue.Status.SetDeploymentQueue(deployedQueue)
	}

	prQueue.SetState(s2hv1.PullRequestQueueEnvDestroying)
	prQueue.Status.SetCondition(s2hv1.PullRequestQueueCondResultCollected, corev1.ConditionTrue,
		"Pull request queue result has been collected")

	prQueueHistName := generateHistoryName(prQueue.Name, prQueue.CreationTimestamp, prQueue.Spec.NoOfRetry)
	if prQueue.Status.PullRequestQueueHistoryName == "" {
		if err := c.createPullRequestQueueHistory(ctx, prQueue); err != nil && !k8serrors.IsAlreadyExists(err) {
			return err
		}

		prQueue.Status.SetPullRequestQueueHistoryName(prQueueHistName)

		// sent report only when pull request trigger success
		if prQueue.Spec.IsPRTriggerFailed != nil && !*prQueue.Spec.IsPRTriggerFailed {
			if err := c.sendPullRequestQueueReport(ctx, prQueue); err != nil {
				return err
			}
		}
		return nil
	}

	return nil
}

func (c *controller) sendPullRequestQueueReport(ctx context.Context, prQueue *s2hv1.PullRequestQueue) error {
	deploymentQueue := prQueue.Status.DeploymentQueue
	if deploymentQueue != nil {
		isDeploySuccess, isTestSuccess := deploymentQueue.IsDeploySuccess(), deploymentQueue.IsTestSuccess()

		compUpgradeStatus := samsahairpc.ComponentUpgrade_UpgradeStatus_FAILURE
		if prQueue.IsCanceled() {
			compUpgradeStatus = samsahairpc.ComponentUpgrade_UpgradeStatus_CANCELED
		} else {
			if isDeploySuccess && isTestSuccess {
				compUpgradeStatus = samsahairpc.ComponentUpgrade_UpgradeStatus_SUCCESS
			}
		}

		prConfig, err := c.s2hClient.GetPullRequestConfig(ctx, &samsahairpc.TeamWithBundleName{
			TeamName:   c.teamName,
			BundleName: prQueue.Spec.BundleName,
		})
		if err != nil {
			return err
		}

		prQueueRPC := &samsahairpc.TeamWithPullRequest{
			TeamName:      c.teamName,
			BundleName:    prQueue.Spec.BundleName,
			PRNumber:      prQueue.Spec.PRNumber,
			CommitSHA:     prQueue.Spec.CommitSHA,
			Namespace:     prQueue.Status.PullRequestNamespace,
			MaxRetryQueue: prConfig.MaxRetry,
		}

		prQueueHistName, prQueueHistNamespace := prQueue.Status.PullRequestQueueHistoryName, c.namespace
		comp := queue.GetComponentUpgradeRPCFromQueue(compUpgradeStatus, prQueueHistName,
			prQueueHistNamespace, deploymentQueue, prQueueRPC)
		if _, err := c.s2hClient.RunPostPullRequestQueue(ctx, comp); err != nil {
			return err
		}
	}

	return nil
}
