package activepromotion

import (
	"context"
	"time"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	s2hv1beta1 "github.com/agoda-com/samsahai/api/v1beta1"
	"github.com/agoda-com/samsahai/internal"
	s2herrors "github.com/agoda-com/samsahai/internal/errors"
	"github.com/agoda-com/samsahai/internal/staging"
	"github.com/agoda-com/samsahai/internal/staging/deploy/helm3"
	"github.com/agoda-com/samsahai/internal/staging/deploy/mock"
)

type envType string

const (
	activeEnvirontment         envType = "Active"
	preActiveEnvirontment      envType = "preActive"
	previousActiveEnvirontment envType = "previousActive"
)

func (c *controller) destroyPreviousActiveEnvironment(ctx context.Context, atpComp *s2hv1beta1.ActivePromotion) error {
	teamName := atpComp.Name
	prevNs := atpComp.Status.PreviousActiveNamespace
	destroyedTime := atpComp.Status.DestroyedTime
	if err := c.destroyPreviousActiveEnvironmentAt(ctx, teamName, prevNs, destroyedTime); err != nil {
		return err
	}

	logger.Debug("previous active namespace has been destroyed",
		"team", teamName, "status", atpComp.Status.Result, "namespace", prevNs)
	atpComp.Status.SetCondition(s2hv1beta1.ActivePromotionCondPreviousActiveDestroyed, corev1.ConditionTrue,
		"Previous active namespace has been destroyed")
	atpComp.Status.SetCondition(s2hv1beta1.ActivePromotionCondFinished, corev1.ConditionTrue,
		"Active promotion process has been finished")
	atpComp.SetState(s2hv1beta1.ActivePromotionFinished, "Completed")

	return nil
}

func (c *controller) destroyPreviousActiveEnvironmentAt(ctx context.Context, teamName, prevNs string, destroyedTime *metav1.Time) error {
	if prevNs == "" {
		logger.Debug("previous active namespace is empty", "team", teamName)
		return nil
	}

	if destroyedTime.IsZero() {
		return s2herrors.ErrEnsureNamespaceDestroyed
	}

	if !metav1.Now().After(destroyedTime.Time) {
		return s2herrors.ErrEnsureNamespaceDestroyed
	}

	if err := c.ensureDestroyEnvironment(ctx, previousActiveEnvirontment, teamName, prevNs, destroyedTime); err != nil {
		return err
	}

	return nil
}

func (c *controller) destroyPreActiveEnvironment(ctx context.Context, atpComp *s2hv1beta1.ActivePromotion) error {
	targetNs := c.getTargetNamespace(atpComp)
	teamName := atpComp.Name

	startedCleaningTime := atpComp.Status.GetConditionLatestTime(s2hv1beta1.ActivePromotionCondActivePromoted)
	if err := c.ensureDestroyEnvironment(ctx, preActiveEnvirontment, teamName, targetNs, startedCleaningTime); err != nil {
		return err
	}

	logger.Debug("pre-active environment has been destroyed",
		"team", teamName, "status", atpComp.Status.Result, "namespace", targetNs)
	atpComp.Status.SetCondition(s2hv1beta1.ActivePromotionCondPreActiveDestroyed, corev1.ConditionTrue,
		"Pre-active environment has been destroyed")
	atpComp.Status.SetCondition(s2hv1beta1.ActivePromotionCondFinished, corev1.ConditionTrue,
		"Active promotion process has been finished")
	atpComp.SetState(s2hv1beta1.ActivePromotionFinished, "Completed")

	return nil
}

func (c *controller) destroyActiveEnvironment(ctx context.Context, atpComp *s2hv1beta1.ActivePromotion, startedCleanupTime *metav1.Time) error {
	teamName := atpComp.Name
	prevNs := atpComp.Status.PreviousActiveNamespace
	if err := c.ensureDestroyEnvironment(ctx, activeEnvirontment, teamName, prevNs, startedCleanupTime); err != nil {
		return err
	}

	return nil
}

func (c *controller) ensureDestroyEnvironment(ctx context.Context, envType envType, teamName, ns string, startedCleanupTime *metav1.Time) error {
	if err := c.deleteAllComponentsInNamespace(teamName, ns, startedCleanupTime); err != nil {
		if s2herrors.IsDeletingReleases(err) {
			return s2herrors.ErrEnsureNamespaceDestroyed
		}
		return err
	}

	switch envType {
	case activeEnvirontment:
		if err := c.s2hCtrl.DestroyActiveEnvironment(teamName, ns); err != nil {
			if !s2herrors.IsNamespaceStillExists(err) {
				return errors.Wrapf(err, "cannot destroy active environment, namespace %s", ns)
			}
			return s2herrors.ErrEnsureNamespaceDestroyed
		}

	case preActiveEnvirontment:
		if err := c.s2hCtrl.DestroyPreActiveEnvironment(teamName, ns); err != nil {
			if !s2herrors.IsNamespaceStillExists(err) {
				return errors.Wrapf(err, "cannot destroy pre-active environment, namespace %s", ns)
			}
			return s2herrors.ErrEnsureNamespaceDestroyed
		}

	case previousActiveEnvirontment:
		if err := c.s2hCtrl.DestroyPreviousActiveEnvironment(teamName, ns); err != nil {
			if !s2herrors.IsNamespaceStillExists(err) {
				return errors.Wrapf(err, "cannot destroy previous active environment, namespace %s", ns)
			}

			return s2herrors.ErrEnsureNamespaceDestroyed
		}
	}

	if err := c.ensureNamespaceDestroyed(ctx, teamName, ns); err != nil {
		return err
	}

	return nil
}

func (c *controller) ensureNamespaceDestroyed(ctx context.Context, teamName, ns string) error {
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: ns,
		},
	}

	if err := c.client.Get(ctx, types.NamespacedName{Name: ns}, namespace); err != nil {
		if k8serrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	return s2herrors.ErrEnsureNamespaceDestroyed
}

func (c *controller) deleteAllComponentsInNamespace(teamName, ns string, startedCleanupTime *metav1.Time) error {
	configCtrl := c.s2hCtrl.GetConfigController()

	deployEngine := c.getDeployEngine(teamName, ns, configCtrl)

	parentComps, err := configCtrl.GetParentComponents(teamName)
	if err != nil {
		return err
	}

	for compName := range parentComps {
		refName := internal.GenReleaseName(ns, compName)
		if err := deployEngine.Delete(refName); err != nil {
			logger.Error(err, "cannot delete release",
				"refName", refName,
				"namespace", ns,
				"component", compName)
		}
	}

	cleanupTimeout := c.getComponentCleanupTimeout(teamName, configCtrl)

	ok, err := staging.WaitForComponentsCleaned(
		c.client,
		deployEngine,
		parentComps,
		ns,
		startedCleanupTime,
		cleanupTimeout.Duration)
	if err != nil {
		return err
	}
	if !ok {
		logger.Debug("Releases are being deleted", "team", teamName, "namespace", ns)
		return s2herrors.ErrDeletingReleases
	}

	return nil
}

func (c *controller) getComponentCleanupTimeout(teamName string, configCtrl internal.ConfigController) *metav1.Duration {
	cleanupTimeout := &metav1.Duration{Duration: 15 * time.Minute}

	config, err := configCtrl.Get(teamName)
	if err != nil {
		return cleanupTimeout
	}

	atpConfig := config.Spec.ActivePromotion

	if atpConfig == nil || atpConfig.Deployment == nil {
		return cleanupTimeout
	}

	return &atpConfig.Deployment.ComponentCleanupTimeout
}

func (c *controller) getDeployEngine(teamName, ns string, configCtrl internal.ConfigController) internal.DeployEngine {
	var e string
	config, err := configCtrl.Get(teamName)
	if err != nil {
		return mock.New()
	}

	atpConfig := config.Spec.ActivePromotion

	if atpConfig == nil || atpConfig.Deployment == nil || atpConfig.Deployment.Engine == nil || *atpConfig.Deployment.Engine == "" {
		e = mock.EngineName
	} else {
		e = *config.Spec.ActivePromotion.Deployment.Engine
	}

	var engine internal.DeployEngine

	switch e {
	case helm3.EngineName:
		engine = helm3.New(ns, false)
	default:
		engine = mock.New()
	}
	return engine
}
