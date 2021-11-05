package queue

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	s2hv1 "github.com/agoda-com/samsahai/api/v1"
	"github.com/agoda-com/samsahai/internal/queue"
	samsahairpc "github.com/agoda-com/samsahai/pkg/samsahai/rpc"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
)

func (c *controller) collectPullRequestQueueResult(ctx context.Context, prQueue *s2hv1.PullRequestQueue) error {
	prComps := prQueue.Spec.Components
	prNamespace := prQueue.Status.PullRequestNamespace

	deployedQueue, err := c.ensurePullRequestComponents(prQueue, prComps)
	if err != nil {
		return errors.Wrapf(err, "cannot ensure pull request components, namespace %s", prNamespace)
	}

	prQueue.Status.SetDeploymentQueue(deployedQueue)
	prQueue.SetState(s2hv1.PullRequestQueueEnvDestroying)
	prQueue.Status.SetCondition(s2hv1.PullRequestQueueCondResultCollected, corev1.ConditionTrue,
		"Pull request queue result has been collected")

	tearDownDuration := prQueue.Spec.TearDownDuration
	willUseTearDownDuration := c.isTearDownDurationCriteriaMet(tearDownDuration.Criteria, prQueue)

	// set destroyed time only when tearDownDuration is applicable
	if prQueue.Status.DestroyedTime == nil && willUseTearDownDuration {
		logger.Debug("pull request destroyed time has been set",
			"namespace", prNamespace)
		destroyedTime := metav1.Now().Add(tearDownDuration.Duration.Duration)
		prQueue.Status.SetDestroyedTime(metav1.Time{Time: destroyedTime})
	}

	prQueueHistName := generateHistoryName(prQueue.Name, prQueue.CreationTimestamp, prQueue.Spec.NoOfRetry)
	if prQueue.Status.PullRequestQueueHistoryName == "" {
		if err := c.createPullRequestQueueHistory(ctx, prQueue); err != nil && !k8serrors.IsAlreadyExists(err) {
			return err
		}

		prQueue.Status.SetPullRequestQueueHistoryName(prQueueHistName)

		// sent report even if pull request trigger fails
		if err := c.sendPullRequestQueueReport(ctx, prQueue); err != nil {
			return err
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

		missingImgListRPC := make([]*samsahairpc.Image, 0)
		for _, img := range prQueue.Spec.ImageMissingList {
			missingImgListRPC = append(missingImgListRPC, &samsahairpc.Image{Repository: img.Repository, Tag: img.Tag})
		}

		prQueueRPC := &samsahairpc.TeamWithPullRequest{
			TeamName:         c.teamName,
			BundleName:       prQueue.Spec.BundleName,
			PRNumber:         prQueue.Spec.PRNumber,
			CommitSHA:        prQueue.Spec.CommitSHA,
			Namespace:        prQueue.Status.PullRequestNamespace,
			MaxRetryQueue:    prConfig.MaxRetry,
			ImageMissingList: missingImgListRPC,
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

func (c *controller) isTearDownDurationCriteriaMet(criteria s2hv1.PullRequestTearDownDurationCriteria,
	prq *s2hv1.PullRequestQueue) bool {
	if (prq.Spec.IsPRTriggerFailed != nil && *prq.Spec.IsPRTriggerFailed) ||
		!prq.Status.IsConditionTrue(s2hv1.PullRequestQueueCondDeployed) {
		return false
	}

	switch criteria {
	case s2hv1.PullRequestTearDownDurationCriteriaBoth:
		return true
	case s2hv1.PullRequestTearDownDurationCriteriaFailure:
		return prq.Status.Result == s2hv1.PullRequestQueueFailure
	case s2hv1.PullRequestTearDownDurationCriteriaSuccess:
		return prq.Status.Result == s2hv1.PullRequestQueueSuccess
	}
	return false
}
