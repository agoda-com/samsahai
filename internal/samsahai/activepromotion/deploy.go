package activepromotion

import (
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"

	s2hv1beta1 "github.com/agoda-com/samsahai/api/v1beta1"
	s2herrors "github.com/agoda-com/samsahai/internal/errors"
	"github.com/agoda-com/samsahai/internal/queue"
)

func (c *controller) deployComponentsToTargetNamespace(atpComp *s2hv1beta1.ActivePromotion) error {
	teamName := atpComp.Name
	targetNs := c.getTargetNamespace(atpComp)
	q, err := c.ensurePreActiveComponentsDeployed(teamName, targetNs, atpComp.Spec.SkipTestRunner)
	if err != nil {
		return err
	}

	if q.IsDeploySuccess() {
		// in case of successful deployment
		logger.Debug("components has been deployed successfully",
			"team", atpComp.Name, "namespace", targetNs)
		atpComp.SetState(s2hv1beta1.ActivePromotionTestingPreActive, "Testing pre-active environment")

		return nil
	}

	// in case of failed deployment
	atpComp.Status.SetResult(s2hv1beta1.ActivePromotionFailure)
	atpComp.Status.SetCondition(s2hv1beta1.ActivePromotionCondVerified, corev1.ConditionTrue,
		"Deployment failed")
	atpComp.SetState(s2hv1beta1.ActivePromotionCollectingPreActiveResult, "Collecting result")

	return nil
}

func (c *controller) ensurePreActiveComponentsDeployed(teamName, targetNs string, skipTest bool) (*s2hv1beta1.Queue, error) {
	q, err := queue.EnsurePreActiveComponents(c.client, teamName, targetNs, skipTest)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot ensure pre-active components, namespace %s", targetNs)
	}

	// in case of queue state was finished without deploying
	if q.Status.State == s2hv1beta1.Finished {
		return q, nil
	}

	if q.Status.StartDeployTime != nil && q.Status.State != s2hv1beta1.Creating {
		return q, nil
	}

	return nil, s2herrors.ErrEnsureComponentDeployed
}

func (c *controller) testPreActiveEnvironment(atpComp *s2hv1beta1.ActivePromotion) error {
	teamName := atpComp.Name
	targetNs := c.getTargetNamespace(atpComp)
	q, err := c.ensurePreActiveComponentsTested(teamName, targetNs, atpComp.Spec.SkipTestRunner)
	if err != nil {
		return err
	}

	if q.IsTestSuccess() {
		// in case successful test
		logger.Debug("components have been tested successfully",
			"team", atpComp.Name, "namespace", targetNs)
		atpComp.Status.SetCondition(s2hv1beta1.ActivePromotionCondVerified, corev1.ConditionTrue,
			"Pre-active environment has been verified successfully")
	} else {
		// in case failure test
		atpComp.Status.SetResult(s2hv1beta1.ActivePromotionFailure)
		atpComp.Status.SetCondition(s2hv1beta1.ActivePromotionCondVerified, corev1.ConditionTrue,
			"Test failed")
	}

	atpComp.SetState(s2hv1beta1.ActivePromotionCollectingPreActiveResult, "Collecting result")

	return nil
}

func (c *controller) ensurePreActiveComponentsTested(teamName, targetNs string, skipTest bool) (*s2hv1beta1.Queue, error) {
	q, err := queue.EnsurePreActiveComponents(c.client, teamName, targetNs, skipTest)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot ensure pre-active components, namespace %s", targetNs)
	}

	if q.Status.State == s2hv1beta1.Finished {
		return q, nil
	}

	return nil, s2herrors.ErrEnsureComponentTested
}
