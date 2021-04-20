package staging

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/twitchtv/twirp"
	"helm.sh/helm/v3/pkg/release"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	s2hv1 "github.com/agoda-com/samsahai/api/v1"
	"github.com/agoda-com/samsahai/internal"
	configctrl "github.com/agoda-com/samsahai/internal/config"
	s2herrors "github.com/agoda-com/samsahai/internal/errors"
	"github.com/agoda-com/samsahai/internal/third_party/k8s.io/kubernetes/deployment/util"
	"github.com/agoda-com/samsahai/internal/util/valuesutil"
	"github.com/agoda-com/samsahai/pkg/samsahai/rpc"
)

// deployEnvironment deploy components into namespace
func (c *controller) deployEnvironment(queue *s2hv1.Queue) error {
	deployTimeout := metav1.Duration{Duration: 1800 * time.Second}

	if deployConfig := c.getDeployConfiguration(queue); deployConfig != nil && deployConfig.Timeout.Duration != 0 {
		deployTimeout = deployConfig.Timeout
	}

	deployEngine := c.getDeployEngine(queue)

	// check deploy timeout
	if err := c.checkDeployTimeout(queue, deployTimeout); err != nil {
		return err
	}

	queueComps := make(map[string]*s2hv1.Component)       // map[component name]component
	queueParentComps := make(map[string]*s2hv1.Component) // map[parent component name]parent component

	switch {
	case queue.IsActivePromotionQueue():
		if err := c.updateQueue(queue); err != nil {
			return err
		}
	default: // upgrade, reverify of staging or pull request queue
		if !queue.IsPullRequestQueue() {
			if isValid, err := c.validateStagingQueue(queue); err != nil || !isValid {
				if err != nil {
					return err
				}
				return nil
			}
		}

		var err error
		queueParentComps, queueComps, err = c.getParentAndQueueCompsFromQueueType(queue)
		if err != nil {
			return err
		}

		if err := c.updateQueueComponentsSpec(queue); err != nil {
			return err
		}
	}

	// Deploy
	if !queue.Status.IsConditionTrue(s2hv1.QueueDeployStarted) {
		isDeployed, err := c.deployComponents(deployEngine, queue, queueComps, queueParentComps, deployTimeout.Duration)
		if err != nil {
			if !isDeployed {
				return err
			}

			queue.Status.SetCondition(
				s2hv1.QueueDeployStarted,
				corev1.ConditionTrue,
				"queue started to deploy")

			queue.Status.SetCondition(
				s2hv1.QueueDeployed,
				corev1.ConditionFalse,
				fmt.Sprintf("release deployment failed: %s", err.Error()))

			logger.Error(err, fmt.Sprintf("queue: %s release failed", queue.Name))

			return c.updateQueueWithState(queue, s2hv1.Collecting)
		}

		queue.Status.SetCondition(
			s2hv1.QueueDeployStarted,
			corev1.ConditionTrue,
			"queue started to deploy")
		if err := c.updateQueue(queue); err != nil {
			return err
		}
	}

	//check helm deployment result
	releases, err := deployEngine.GetReleases()
	if err != nil {
		return err
	}

	if len(releases) == 0 && !deployEngine.IsMocked() {
		logger.Debug("there is no release found, release has been being installed",
			"queue", queue.Name)
		return nil
	}

	if len(releases) != 0 && queue.IsPullRequestQueue() {
		if err := c.deployActiveServicesIntoPullRequestEnvironment(); err != nil {
			logger.Error(err, "cannot deploy active services into pull request environment",
				"queue", queue.Name)
			return err
		}
	}

	isDeployed, isFailed, errMsg := c.checkAllReleasesDeployed(deployEngine, releases)
	if isFailed {
		queue.Status.SetCondition(
			s2hv1.QueueDeployed,
			corev1.ConditionFalse,
			fmt.Sprintf("release deployment failed: %s", errMsg))

		logger.Error(s2herrors.ErrReleaseFailed, fmt.Sprintf("queue: %s release failed", queue.Name))

		return c.updateQueueWithState(queue, s2hv1.Collecting)
	} else if !isDeployed {
		time.Sleep(2 * time.Second)
		return nil
	}

	// checking environment is ready
	// change state if ready
	isReady, err := c.waitForComponentsReady(deployEngine, queue)
	if err != nil {
		return err
	} else if !isReady {
		time.Sleep(2 * time.Second)
		return nil
	}

	// environment is ready
	queue.Status.SetCondition(
		s2hv1.QueueDeployed,
		corev1.ConditionTrue,
		"queue deployment succeeded")
	return c.updateQueueWithState(queue, s2hv1.Testing)
}

func (c *controller) getAllComponentsFromQueueType(q *s2hv1.Queue) (
	comps map[string]*s2hv1.Component, err error) {

	configCtrl := c.getConfigController()
	if q.IsPullRequestQueue() {
		prBundleName := q.Spec.Name
		comps, err = configCtrl.GetPullRequestComponents(c.teamName, prBundleName, true)
		if err != nil {
			return
		}
	} else {
		comps, err = configCtrl.GetComponents(c.teamName)
		if err != nil {
			return
		}
	}

	return
}

func (c *controller) getParentAndQueueCompsFromQueueType(q *s2hv1.Queue) (
	queueParentComps, queueComps map[string]*s2hv1.Component, err error) {

	queueComps = make(map[string]*s2hv1.Component)       // map[component name]component
	queueParentComps = make(map[string]*s2hv1.Component) // map[parent component name]parent component

	comps, err := c.getAllComponentsFromQueueType(q)
	if err != nil {
		return
	}

	for _, qComp := range q.Spec.Components {
		comp, ok := comps[qComp.Name]
		if !ok {
			continue
		}

		queueComps[qComp.Name] = comp
		queueParentComps[qComp.Name] = comp

		if comp.Parent != "" {
			delete(queueParentComps, qComp.Name)
			if _, ok := comps[comp.Parent]; !ok {
				var parentComps map[string]*s2hv1.Component
				configCtrl := c.getConfigController()
				parentComps, err = configCtrl.GetParentComponents(c.teamName)
				if err != nil {
					return
				}
				queueParentComps[comp.Parent] = parentComps[comp.Parent]
			} else {
				queueParentComps[comp.Parent] = comps[comp.Parent]
			}
		}
	}

	return
}

func (c *controller) updateQueueComponentsSpec(q *s2hv1.Queue) error {
	comps, err := c.getAllComponentsFromQueueType(q)
	if err != nil {
		return err
	}

	newComps := make([]*s2hv1.QueueComponent, 0)
	for _, qComp := range q.Spec.Components {
		if _, ok := comps[qComp.Name]; !ok {
			continue
		}

		newComps = append(newComps, qComp)
	}

	// update queue if there are skipped components
	if len(newComps) != len(q.Spec.Components) {
		q.Spec.Components = newComps
		if err := c.updateQueue(q); err != nil {
			return err
		}
	}

	return nil
}

// checkDeployTimeout checks if deploy duration was longer than timeout.
// change state to `Collecting` if timeout
func (c *controller) checkDeployTimeout(queue *s2hv1.Queue, deployTimeout metav1.Duration) error {
	now := metav1.Now()

	if queue.Status.StartDeployTime == nil {
		queue.Status.StartDeployTime = &now
		return c.updateQueue(queue)
	} else if now.Sub(queue.Status.StartDeployTime.Time) > deployTimeout.Duration {
		// deploy timeout
		queue.Status.SetCondition(
			s2hv1.QueueDeployed,
			corev1.ConditionFalse,
			"queue deployment timeout")

		// update queue back to k8s
		if err := c.updateQueueWithState(queue, s2hv1.Collecting); err != nil {
			return err
		}

		logger.Error(s2herrors.ErrDeployTimeout, fmt.Sprintf("queue: %s deploy timeout", queue.Name))

		return s2herrors.ErrDeployTimeout
	}

	return nil
}

// validateStagingQueue checks if Queue exist in Configuration.
func (c *controller) validateStagingQueue(queue *s2hv1.Queue) (bool, error) {
	configCtrl := c.getConfigController()
	comps, err := configCtrl.GetComponents(c.teamName)
	if err != nil {
		return false, err
	}

	isBundleQueue := queue.Spec.Bundle != "" && queue.Spec.Name == queue.Spec.Bundle
	bundles, err := configCtrl.GetBundles(c.teamName)
	if err != nil {
		return false, err
	}

	isNotExist := false
	if isBundleQueue {
		// delete queue if no bundle exist in config
		if _, ok := bundles[queue.Spec.Name]; !ok {
			isNotExist = true
		}
	} else {
		if len(queue.Spec.Components) == 0 {
			isNotExist = true
		} else {
			// delete queue if component does not exist in config
			if _, ok := comps[queue.Spec.Components[0].Name]; !ok {
				isNotExist = true
			}
		}
	}

	if isNotExist {
		if err := c.client.Delete(context.TODO(), queue); err != nil && !k8serrors.IsNotFound(err) {
			logger.Error(err, "deleting queue error")
			return false, err
		}
		c.clearCurrentQueue()
	}

	return true, nil
}

func (c *controller) getStableComponentsMapFromQueueType(q *s2hv1.Queue) (
	stableMap map[string]s2hv1.StableComponent, err error) {

	namespace := c.namespace
	if q.IsPullRequestQueue() {
		namespace, err = c.getTeamActiveNamespace()
		if err != nil {
			return
		}
	}

	// create StableComponentMap
	runtimeClient, err := c.getRuntimeClient()
	if err != nil {
		logger.Error(err, "cannot get runtime client")
		return
	}

	stableMap, err = valuesutil.GetStableComponentsMap(runtimeClient, namespace)
	if err != nil {
		logger.Error(err, "cannot list StableComponents")
		return
	}
	return
}

func (c *controller) getTeamActiveNamespace() (string, error) {
	headers := make(http.Header)
	headers.Set(internal.SamsahaiAuthHeader, c.authToken)
	ctx := context.TODO()
	ctx, err := twirp.WithHTTPRequestHeaders(ctx, headers)
	if err != nil {
		return "", errors.Wrap(err, "cannot set request header")
	}

	teamWithNs, err := c.s2hClient.GetTeamActiveNamespace(ctx, &rpc.TeamName{Name: c.teamName})
	if err != nil {
		return "", err
	}

	return teamWithNs.Namespace, nil
}

func genCompValueFromQueue(compName string, qComps []*s2hv1.QueueComponent) map[string]interface{} {
	for _, qComp := range qComps {
		if qComp.Name == compName {
			image := make(map[string]interface{})
			if qComp.Repository != "" {
				image["repository"] = qComp.Repository
			}
			if qComp.Version != "" {
				image["tag"] = qComp.Version
			}

			return map[string]interface{}{
				"image": image,
			}
		}
	}

	return map[string]interface{}{}
}

// applyEnvBaseConfig applies input values with specific env. configuration based on Queue.Spec.Type
func applyEnvBaseConfig(
	cfg *s2hv1.ConfigSpec,
	values map[string]interface{},
	qt s2hv1.QueueType,
	comp *s2hv1.Component,
	teamName string,
) map[string]interface{} {
	var target map[string]s2hv1.ComponentValues
	var err error

	switch qt {
	case s2hv1.QueueTypePreActive:
		target, err = configctrl.GetEnvValues(cfg, s2hv1.EnvPreActive, teamName)
	case s2hv1.QueueTypePromoteToActive:
		target, err = configctrl.GetEnvValues(cfg, s2hv1.EnvActive, teamName)
	case s2hv1.QueueTypeUpgrade, s2hv1.QueueTypeReverify:
		target, err = configctrl.GetEnvValues(cfg, s2hv1.EnvStaging, teamName)
	case s2hv1.QueueTypeDemoteFromActive:
		return values
	default:
		return values
	}
	if err != nil {
		logger.Error(err, "cannot get env values")
		return values
	} else if len(target) == 0 {
		// env not found in config
		return values
	} else if _, compOK := target[comp.Name]; !compOK {
		// component not found in config
		return values
	}

	return valuesutil.MergeValues(values, target[comp.Name])
}

// deployComponents
func (c *controller) deployComponents(
	deployEngine internal.DeployEngine,
	queue *s2hv1.Queue,
	queueComps map[string]*s2hv1.Component,
	queueParentComps map[string]*s2hv1.Component,
	deployTimeout time.Duration,
) (isDeployed bool, err error) {
	isDeployed = true
	stableMap, err := c.getStableComponentsMapFromQueueType(queue)
	if err != nil {
		return false, err
	}

	releaseRevision := make(map[string]int)
	preInstalledReleases, err := deployEngine.GetReleases()
	if err != nil {
		return false, err
	}
	for _, rel := range preInstalledReleases {
		releaseRevision[rel.Name] = rel.Version
	}

	timeout := 5 * time.Minute
	ctx, cancelFunc := context.WithTimeout(context.Background(), timeout)
	defer cancelFunc()

	isDeployedCh := make(chan bool, 2)
	errCh := make(chan error, 2)
	go func() {
		if queue.Spec.Type == s2hv1.QueueTypePullRequest {
			isDeployedCh <- true
			errCh <- nil
			return
		}

		isDeployed, err := c.deployComponentsExceptQueue(deployEngine, queue, queueParentComps, stableMap, deployTimeout)
		if err != nil {
			logger.Error(err, "cannot deploy components except current queue",
				"queue", queue.Name, "queueType", queue.Spec.Type)
		}
		isDeployedCh <- isDeployed
		errCh <- err
	}()

	go func() {
		if !c.isUpgradeRelatedQueue(queue) && !c.isPullRequestRelatedQueue(queue) {
			isDeployedCh <- true
			errCh <- nil
			return
		}

		err := c.deployQueueComponent(deployEngine, queue, queueComps, queueParentComps, stableMap, deployTimeout)
		if err != nil {
			logger.Error(err, "cannot deploy current queue component",
				"queue", queue.Name, "queueType", queue.Spec.Type)
		}
		isDeployedCh <- isDeployed
		errCh <- err
	}()

	for i := 0; i < 2; i++ {
		select {
		case <-ctx.Done():
			logger.Warnf("validating helm release took longer than %.0f seconds, queue: %s", timeout.Seconds(),
				queue.Name)

			var postInstalledReleases []*release.Release
			postInstalledReleases, err = deployEngine.GetReleases()
			if err != nil {
				return
			}

			if len(postInstalledReleases) == 0 {
				logger.Warn("there is no release found", "queue", queue.Name)
				return
			}
			for _, rel := range postInstalledReleases {
				if revision, ok := releaseRevision[rel.Name]; ok {
					if rel.Version <= revision {
						logger.Warn("there is no latest revision of release found",
							"queue", queue.Name)
						return
					}
				}
			}

			return

		case isDeployed := <-isDeployedCh:
			err := <-errCh
			if err != nil {
				return isDeployed, err
			}
		}
	}

	return
}

func (c *controller) isUpgradeRelatedQueue(q *s2hv1.Queue) bool {
	return q.Spec.Type == s2hv1.QueueTypeUpgrade || q.Spec.Type == s2hv1.QueueTypeReverify
}

func (c *controller) isPullRequestRelatedQueue(q *s2hv1.Queue) bool {
	return q.Spec.Type == s2hv1.QueueTypePullRequest
}

// deployComponentsExceptQueue ensures other components deployed with StableComponents
func (c *controller) deployComponentsExceptQueue(
	deployEngine internal.DeployEngine,
	queue *s2hv1.Queue,
	queueParentComps map[string]*s2hv1.Component,
	stableMap map[string]s2hv1.StableComponent,
	deployTimeout time.Duration,
) (isDeployed bool, err error) {

	configCtrl := c.getConfigController()
	parentComps, err := configCtrl.GetParentComponents(c.teamName)
	if err != nil {
		return false, err
	}

	cfg, err := c.getConfiguration()
	if err != nil {
		return false, err
	}

	for name, comp := range parentComps {
		// skip current queue
		if _, ok := queueParentComps[name]; ok {
			continue
		}

		baseValues, err := configctrl.GetEnvComponentValues(cfg, name, c.teamName, s2hv1.EnvBase)
		if err != nil {
			return false, err
		}

		values := valuesutil.GenStableComponentValues(
			comp,
			stableMap,
			baseValues,
		)

		switch queue.Spec.Type {
		case s2hv1.QueueTypeDemoteFromActive:
			// rollback current active instead of upgrading
			if err := deployEngine.Rollback(c.genReleaseName(comp), 1); err != nil {
				return true, err
			}
		default:
			values = applyEnvBaseConfig(cfg, values, queue.Spec.Type, comp, c.teamName)
			if err := deployEngine.Create(c.genReleaseName(comp), comp, comp, values, &deployTimeout); err != nil {
				return true, err
			}
		}
	}

	return true, nil
}

// deployQueueComponent ensures queue components deployed
// will be skipped if queue type is not upgrade or reverify
func (c *controller) deployQueueComponent(
	deployEngine internal.DeployEngine,
	queue *s2hv1.Queue,
	queueComps map[string]*s2hv1.Component,
	queueParentComps map[string]*s2hv1.Component,
	stableMap map[string]s2hv1.StableComponent,
	deployTimeout time.Duration,
) error {
	cfg, err := c.getConfiguration()
	if err != nil {
		return err
	}

	errCh := make(chan error, len(queueParentComps))

	ctx, cancelFunc := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancelFunc()

	// deploy current queue
	for parentName, parentComp := range queueParentComps {
		go func(parentName string, parentComp *s2hv1.Component) {
			if parentComp == nil {
				errCh <- fmt.Errorf("parent components should not be empty, component: %s", parentName)
				return
			}

			envType := s2hv1.EnvBase
			parentBaseValues := s2hv1.ComponentValues{}
			if queue.IsPullRequestQueue() {
				envType = s2hv1.EnvPullRequest
			} else {
				// get parent values except pull request queue type
				parentBaseValues, err = configctrl.GetEnvComponentValues(cfg, parentName, c.teamName, envType)
				if err != nil {
					errCh <- err
					return
				}
			}

			values := valuesutil.GenStableComponentValues(
				parentComp,
				stableMap,
				parentBaseValues,
			)

			if queue.IsComponentUpgradeQueue() || queue.IsPullRequestQueue() {
				if queue.IsPullRequestQueue() {
					bundleName := queue.Spec.Name
					envValues, err := configctrl.GetEnvComponentValues(cfg, bundleName, c.teamName, envType)
					if err != nil {
						errCh <- err
						return
					}
					values = valuesutil.MergeValues(values, envValues)
				}

				// merge stable only matched component or dependencies
				for _, comp := range queueComps {
					v := genCompValueFromQueue(comp.Name, queue.Spec.Components)
					if comp.Name == parentComp.Name {
						// queue is parent
						values = valuesutil.MergeValues(values, v)
					} else if comp.Parent != "" && comp.Parent == parentComp.Name {
						// queue is dependency of parent
						values = valuesutil.MergeValues(values, map[string]interface{}{
							comp.Name: v,
						})
					}
				}
			}

			values = applyEnvBaseConfig(cfg, values, queue.Spec.Type, parentComp, c.teamName)
			err = deployEngine.Create(c.genReleaseName(parentComp), parentComp, parentComp, values, &deployTimeout)
			if err != nil {
				errCh <- err
				return
			}

			errCh <- nil
		}(parentName, parentComp)
	}

	for i := 0; i < len(queueParentComps); i++ {
		select {
		case <-ctx.Done():
			return nil
		case err := <-errCh:
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (c *controller) waitForComponentsReady(deployEngine internal.DeployEngine, q *s2hv1.Queue) (bool, error) {
	parentComps, _, err := c.getParentAndQueueCompsFromQueueType(q)
	if err != nil {
		return false, err
	}

	for _, comp := range parentComps {
		selectors := deployEngine.GetLabelSelectors(c.genReleaseName(comp))
		isReady, err := c.waitForReady(selectors)
		if err != nil {
			return false, err
		} else if !isReady {
			return false, nil
		}
	}

	return true, nil
}

// waitForReady checks resources readiness based-on selectors, always ready if selectors is empty
func (c *controller) waitForReady(selectors map[string]string) (bool, error) {
	if len(selectors) == 0 {
		return true, nil
	}

	listOpt := &client.ListOptions{
		Namespace:     c.namespace,
		LabelSelector: labels.SelectorFromSet(selectors),
	}

	// check pods
	if isReady, err := c.isPodsReady(listOpt); err != nil || !isReady {
		return false, err
	}

	// check deployments
	if isReady, err := c.isDeploymentsReady(listOpt); err != nil || !isReady {
		return false, err
	}

	// check pvcs
	if isReady, err := c.isPVCsReady(listOpt); err != nil || !isReady {
		return false, err
	}

	// check services
	if isReady, err := c.isServicesReady(listOpt); err != nil || !isReady {
		return false, err
	}

	// TODO: check statefulset

	return true, nil
}

func (c *controller) isDeploymentsReady(listOpt *client.ListOptions) (bool, error) {
	deployments := &appsv1.DeploymentList{}
	err := c.client.List(context.TODO(), deployments, listOpt)
	if err != nil {
		logger.Error(err, "list appsv1.deployments error: "+listOpt.AsListOptions().String())
		return false, err
	}

	for i, deploy := range deployments.Items {
		rs, err := util.GetNewReplicaSet(&deployments.Items[i], c.client)
		if err != nil {
			logger.Error(err, "deploymentutil.getnewreplicaset error")
			return false, err
		} else if rs == nil {
			return false, nil
		}
		if deploy.Spec.Replicas == nil {
			// success
		} else if !(rs.Status.ReadyReplicas >= *deploy.Spec.Replicas-util.MaxUnavailable(deploy)) {
			return false, nil
		}
	}

	return true, nil
}

func (c *controller) isPodsReady(listOpt *client.ListOptions) (bool, error) {
	pods := &corev1.PodList{}
	err := c.client.List(context.TODO(), pods, listOpt)
	if err != nil {
		logger.Error(err, "list pods error", "namespace", c.namespace)
		return false, err
	}

	for _, pod := range pods.Items {
		isReady := false
		for _, podRef := range pod.OwnerReferences {
			if strings.ToLower(podRef.Kind) == "job" {
				job := &batchv1.Job{}
				err := c.client.Get(context.TODO(), types.NamespacedName{Name: podRef.Name, Namespace: pod.Namespace}, job)
				if err != nil {
					logger.Error(err, "cannot get job %s", podRef.Name)
				}

				if job.Status.CompletionTime == nil {
					return false, nil
				}

				isReady = true
				break
			}
		}

		if !isReady {
			for _, cond := range pod.Status.Conditions {
				if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
					isReady = true
					break
				}
			}
		}

		if !isReady {
			return false, nil
		}
	}

	return true, nil
}

func (c *controller) isServicesReady(listOpt *client.ListOptions) (bool, error) {
	list := &corev1.ServiceList{}
	err := c.client.List(context.TODO(), list, listOpt)
	if err != nil {
		logger.Error(err, "list services error: "+listOpt.AsListOptions().String())
		return false, err
	}

	for _, s := range list.Items {
		if s.Spec.Type == corev1.ServiceTypeExternalName {
			continue
		}
		// Make sure the service is not explicitly set to "None" before checking the IP
		if s.Spec.ClusterIP == "" {
			logger.Debug("service is not ready: %s/%s", s.GetNamespace(), s.GetName())
			return false, nil
		}
		// This checks if the service has a LoadBalancer and that balancer has an Ingress defined
		if s.Spec.Type == corev1.ServiceTypeLoadBalancer && s.Status.LoadBalancer.Ingress == nil {
			logger.Debug("service is not ready: %s/%s", s.GetNamespace(), s.GetName())
			return false, nil
		}
	}
	return true, nil
}

func (c *controller) isPVCsReady(listOpt *client.ListOptions) (bool, error) {
	list := &corev1.PersistentVolumeClaimList{}
	err := c.client.List(context.TODO(), list, listOpt)
	if err != nil {
		logger.Error(err, "list pvcs error: "+listOpt.AsListOptions().String())
		return false, err
	}

	for _, v := range list.Items {
		if v.Status.Phase != corev1.ClaimBound {
			logger.Debug("PersistentVolumeClaim is not ready: %s/%s", v.GetNamespace(), v.GetName())
			return false, nil
		}
	}

	return true, nil
}

func (c *controller) checkAllReleasesDeployed(deployEngine internal.DeployEngine, releases []*release.Release) (
	isDeployed, isFailed bool, errMsg string,
) {
	for _, r := range releases {
		histories, err := deployEngine.GetHistories(r.Name)
		if err != nil {
			return false, false, ""
		}

		foundHistoryDeployed := false
		for _, hist := range histories {
			switch hist.Info.Status {
			case release.StatusDeployed:
				foundHistoryDeployed = true
			case release.StatusFailed:
				return false, true, hist.Info.Description
			case release.StatusPendingInstall, release.StatusPendingUpgrade:
				return false, false, ""
			}
		}

		if !foundHistoryDeployed {
			return false, false, ""
		}
	}

	return true, false, ""
}

func (c *controller) deployActiveServicesIntoPullRequestEnvironment() error {
	headers := make(http.Header)
	headers.Set(internal.SamsahaiAuthHeader, c.authToken)
	ctx := context.TODO()
	ctx, err := twirp.WithHTTPRequestHeaders(ctx, headers)
	if err != nil {
		return errors.Wrap(err, "cannot set request header")
	}

	_, err = c.s2hClient.DeployActiveServicesIntoPullRequestEnvironment(ctx, &rpc.TeamWithNamespace{
		TeamName:  c.teamName,
		Namespace: c.namespace,
	})
	if err != nil {
		return err
	}

	return nil
}

func (c *controller) getRuntimeClient() (client.Client, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		logger.Error(err, "unable to set up client config")
		return nil, err
	}

	runtimeClient, err := client.New(cfg, client.Options{Scheme: c.scheme})
	if err != nil {
		logger.Error(err, "cannot create unversioned restclient")
		return nil, err
	}

	return runtimeClient, nil
}
