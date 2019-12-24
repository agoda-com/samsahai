package activepromotion

import (
	"context"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/agoda-com/samsahai/internal"
	s2herrors "github.com/agoda-com/samsahai/internal/errors"
	"github.com/agoda-com/samsahai/internal/k8s/helmrelease"
	"github.com/agoda-com/samsahai/internal/staging"
	"github.com/agoda-com/samsahai/internal/staging/deploy/fluxhelm"
	"github.com/agoda-com/samsahai/internal/staging/deploy/mock"
	s2hv1beta1 "github.com/agoda-com/samsahai/pkg/apis/env/v1beta1"
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
	destroyTime := atpComp.Status.DestroyTime
	if err := c.destroyPreviousActiveEnvironmentAt(ctx, teamName, prevNs, destroyTime); err != nil {
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

func (c *controller) destroyPreviousActiveEnvironmentAt(ctx context.Context, teamName, prevNs string, destroyTime *metav1.Time) error {
	if prevNs == "" {
		logger.Debug("previous active namespace is empty", "team", teamName)
		return nil
	}

	if destroyTime.IsZero() {
		return s2herrors.ErrEnsureNamespaceDestroyed
	}

	if !metav1.Now().After(destroyTime.Time) {
		return s2herrors.ErrEnsureNamespaceDestroyed
	}

	if err := c.ensureDestroyEnvironment(ctx, previousActiveEnvirontment, teamName, prevNs, destroyTime); err != nil {
		return err
	}

	return nil
}

func (c *controller) destroyPreActiveEnvironment(ctx context.Context, atpComp *s2hv1beta1.ActivePromotion) error {
	targetNs := atpComp.Status.TargetNamespace
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
	if err := c.deleteAllHelmReleasesInNamespace(teamName, ns, startedCleanupTime); err != nil {
		if s2herrors.IsDeletingReleases(err) || s2herrors.IsLoadingConfiguration(err) {
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

func (c *controller) deleteAllHelmReleasesInNamespace(teamName, ns string, startedCleanupTime *metav1.Time) error {
	hrClient := helmrelease.New(ns, c.restCfg)
	_, err := hrClient.List(metav1.ListOptions{})
	if err != nil {
		return errors.Wrapf(err, "cannot list helmreleases in namespace %s", ns)
	}

	if err := hrClient.DeleteCollection(nil, metav1.ListOptions{}); err != nil {
		return errors.Wrapf(err, "cannot delete helmreleases in namespace %s", ns)
	}

	ok, err := c.waitForComponentsCleaned(teamName, ns, startedCleanupTime)
	if err != nil {
		return err
	}
	if !ok {
		logger.Debug("Releases are being deleted", "team", teamName, "namespace", ns)
		return s2herrors.ErrDeletingReleases
	}

	return nil
}

func (c *controller) waitForComponentsCleaned(teamName, ns string, startedCleanupTime *metav1.Time) (bool, error) {
	configMgr, err := c.getTeamConfiguration(teamName)
	if err != nil {
		return false, err
	}

	deployEngine := c.getDeployEngine(configMgr, ns)

	return staging.WaitForComponentsCleaned(c.clientset, deployEngine, configMgr.GetParentComponents(),
		teamName, ns, startedCleanupTime, c.getComponentCleanupTimeout(configMgr))
}

func (c *controller) getComponentCleanupTimeout(configMgr internal.ConfigManager) *metav1.Duration {
	cfg := configMgr.Get()
	atpConfig := cfg.ActivePromotion

	if atpConfig == nil || atpConfig.Deployment == nil {
		return &metav1.Duration{Duration: 0}
	}

	return &atpConfig.Deployment.ComponentCleanupTimeout
}

func (c *controller) getDeployEngine(configMgr internal.ConfigManager, ns string) internal.DeployEngine {
	var e string
	cfg := configMgr.Get()
	atpConfig := cfg.ActivePromotion

	if atpConfig == nil || atpConfig.Deployment == nil || atpConfig.Deployment.Engine == nil || *atpConfig.Deployment.Engine == "" {
		e = internal.MockDeployEngine
	} else {
		e = *cfg.ActivePromotion.Deployment.Engine
	}

	var engine internal.DeployEngine

	switch e {
	case fluxhelm.EngineName:
		engine = fluxhelm.New(configMgr, helmrelease.New(ns, c.restCfg))
	default:
		engine = mock.New()
	}
	return engine
}
