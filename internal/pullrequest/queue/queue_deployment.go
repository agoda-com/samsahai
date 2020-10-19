package queue

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"

	s2hv1beta1 "github.com/agoda-com/samsahai/api/v1beta1"
	"github.com/agoda-com/samsahai/internal"
	s2herrors "github.com/agoda-com/samsahai/internal/errors"
	"github.com/agoda-com/samsahai/internal/queue"
	samsahairpc "github.com/agoda-com/samsahai/pkg/samsahai/rpc"
)

func (c *controller) createPullRequestEnvironment(ctx context.Context, prQueue *s2hv1beta1.PullRequestQueue) error {
	prNamespace := fmt.Sprintf("%s%s-%s", internal.AppPrefix, c.teamName, prQueue.Name)
	_, err := c.s2hClient.CreatePullRequestEnvironment(ctx, &samsahairpc.TeamWithPullRequest{
		TeamName:      c.teamName,
		Namespace:     prNamespace,
		ComponentName: prQueue.Spec.ComponentName,
	})
	if err != nil {
		return err
	}

	prQueue.Status.SetPullRequestNamespace(prNamespace)
	prQueue.Status.SetCondition(s2hv1beta1.PullRequestQueueCondEnvCreated, corev1.ConditionTrue,
		"Pull request environment has been created")
	prQueue.SetState(s2hv1beta1.PullRequestQueueDeploying)

	return nil
}

func (c *controller) destroyPullRequestEnvironment(ctx context.Context, prQueue *s2hv1beta1.PullRequestQueue) (
	skipReconcile bool, err error) {

	prNamespace := prQueue.Status.PullRequestNamespace
	if err = queue.DeletePullRequestQueue(c.client, prNamespace, prQueue.Name); err != nil {
		return
	}

	_, err = c.s2hClient.DestroyPullRequestEnvironment(ctx, &samsahairpc.TeamWithNamespace{
		TeamName:  c.teamName,
		Namespace: prNamespace,
	})
	if err != nil {
		return
	}

	prQueue.Status.SetCondition(s2hv1beta1.PullRequestQueueCondEnvDestroyed, corev1.ConditionTrue,
		"Pull request environment has been destroyed")

	prConfig, err := c.s2hClient.GetPullRequestConfig(ctx, &samsahairpc.TeamWithComponentName{
		TeamName:      c.teamName,
		ComponentName: prQueue.Spec.ComponentName,
	})
	if err != nil {
		return
	}

	if prQueue.Spec.CommitSHA != prQueue.Spec.UpcomingCommitSHA {
		if err = c.SetRetryQueue(prQueue, 0, time.Now()); err != nil {
			return
		}

		c.resetQueueOrder(ctx)
		skipReconcile = true
		return
	}

	if prQueue.IsFailure() {
		maxRetryQueue := int(prConfig.MaxRetry)
		if prQueue.Spec.NoOfRetry < maxRetryQueue {
			prQueue.Spec.NoOfRetry++
			if err = c.SetRetryQueue(prQueue, prQueue.Spec.NoOfRetry, time.Now()); err != nil {
				return
			}

			c.resetQueueOrder(ctx)
			skipReconcile = true
			return
		}
	}

	prQueue.SetState(s2hv1beta1.PullRequestQueueFinished)

	return
}

func (c *controller) ensurePullRequestComponentsDeploying(ctx context.Context, prQueue *s2hv1beta1.PullRequestQueue) error {
	prComps := prQueue.Spec.Components
	prNamespace := prQueue.Status.PullRequestNamespace

	err := c.updatePullRequestComponentDependenciesVersion(ctx, c.teamName, prQueue.Spec.ComponentName, &prComps)
	if err != nil {
		return err
	}

	if !prQueue.Status.IsConditionTrue(s2hv1beta1.PullRequestQueueCondDependenciesUpdated) {
		prQueue.Status.SetCondition(s2hv1beta1.PullRequestQueueCondDependenciesUpdated, corev1.ConditionTrue,
			"Pull request dependencies have been updated into queue successfully")
		return nil
	}

	deployedQueue, err := c.ensurePullRequestComponents(prQueue, prComps)
	if err != nil {
		return errors.Wrapf(err, "cannot ensure pull request components deployed, namespace %s", prNamespace)
	}

	if deployedQueue.Status.State == s2hv1beta1.Finished || // in case of queue state was finished without deploying
		(deployedQueue.Status.StartDeployTime != nil && deployedQueue.Status.State != s2hv1beta1.Creating) {
		if deployedQueue.IsDeploySuccess() {
			// in case successful deployment
			logger.Debug("components has been deployed successfully",
				"team", c.teamName, "component", prQueue.Spec.ComponentName,
				"prNumber", prQueue.Spec.PRNumber)
			prQueue.Status.SetCondition(s2hv1beta1.PullRequestQueueCondDeployed, corev1.ConditionTrue,
				"Components have been deployed successfully")
			prQueue.SetState(s2hv1beta1.PullRequestQueueTesting)
			return nil
		}

		// in case failure deployment
		prQueue.Status.SetResult(s2hv1beta1.PullRequestQueueFailure)
		prQueue.Status.SetCondition(s2hv1beta1.PullRequestQueueCondDeployed, corev1.ConditionFalse,
			"Deployment failed")
		prQueue.Status.SetCondition(s2hv1beta1.PullRequestQueueCondTested, corev1.ConditionTrue,
			"Skipped running test due to deployment failed")
		prQueue.SetState(s2hv1beta1.PullRequestQueueCollecting)

		return nil
	}

	return s2herrors.ErrEnsureComponentDeployed
}

func (c *controller) ensurePullRequestComponentsTesting(ctx context.Context, prQueue *s2hv1beta1.PullRequestQueue) error {
	prComps := prQueue.Spec.Components
	prNamespace := prQueue.Status.PullRequestNamespace
	deployedQueue, err := c.ensurePullRequestComponents(prQueue, prComps)
	if err != nil {
		return errors.Wrapf(err, "cannot ensure pull request components tested, namespace %s", prNamespace)
	}

	if deployedQueue.Status.State == s2hv1beta1.Finished {
		if deployedQueue.IsTestSuccess() {
			// in case successful test
			logger.Debug("components have been tested successfully",
				"team", c.teamName, "component", prQueue.Spec.ComponentName,
				"prNumber", prQueue.Spec.PRNumber)
			prQueue.Status.SetResult(s2hv1beta1.PullRequestQueueSuccess)
			prQueue.Status.SetCondition(s2hv1beta1.PullRequestQueueCondTested, corev1.ConditionTrue,
				"Components have been tested successfully")
		} else {
			// in case failure test
			prQueue.Status.SetResult(s2hv1beta1.PullRequestQueueFailure)
			prQueue.Status.SetCondition(s2hv1beta1.PullRequestQueueCondTested, corev1.ConditionFalse,
				"Test failed")
		}

		prQueue.SetState(s2hv1beta1.PullRequestQueueCollecting)

		return nil
	}

	return s2herrors.ErrEnsureComponentTested
}

func (c *controller) updatePullRequestComponentDependenciesVersion(ctx context.Context, teamName, prCompName string,
	prComps *s2hv1beta1.QueueComponents) error {

	if prComps == nil {
		return nil
	}

	prDependencies, err := c.s2hClient.GetPullRequestComponentDependencies(ctx, &samsahairpc.TeamWithComponentName{
		TeamName:      teamName,
		ComponentName: prCompName,
	})
	if err != nil {
		return err
	}

	for _, prDep := range prDependencies.Dependencies {
		imgRepo := prDep.Image.Repository
		imgTag := prDep.Image.Tag
		depFound := false
		for i := range *prComps {
			if (*prComps)[i].Name == prDep.Name {
				depFound = true
				(*prComps)[i].Repository = imgRepo
				(*prComps)[i].Version = imgTag
			}
		}

		if !depFound {
			*prComps = append(*prComps, &s2hv1beta1.QueueComponent{
				Name:       prDep.Name,
				Repository: imgRepo,
				Version:    imgTag,
			})
		}
	}

	return nil
}

func (c *controller) ensurePullRequestComponents(
	prQueue *s2hv1beta1.PullRequestQueue,
	prComps s2hv1beta1.QueueComponents,
) (*s2hv1beta1.Queue, error) {
	runtimeClient, err := c.getRuntimeClient()
	if err != nil {
		return nil, err
	}

	prNamespace := prQueue.Status.PullRequestNamespace
	deployedQueue, err := queue.EnsurePullRequestComponents(runtimeClient, c.teamName, prNamespace, prQueue.Name,
		prQueue.Spec.ComponentName, prQueue.Spec.PRNumber, prComps, prQueue.Spec.NoOfRetry)
	if err != nil {
		return nil, err
	}

	return deployedQueue, nil
}
