package staging

import (
	"context"
	"fmt"
	"strings"
	"time"

	"helm.sh/helm/v3/pkg/release"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	s2hv1beta1 "github.com/agoda-com/samsahai/api/v1beta1"
	"github.com/agoda-com/samsahai/internal"
	configctrl "github.com/agoda-com/samsahai/internal/config"
	s2herrors "github.com/agoda-com/samsahai/internal/errors"
	"github.com/agoda-com/samsahai/internal/third_party/k8s.io/kubernetes/deployment/util"
	"github.com/agoda-com/samsahai/internal/util/valuesutil"
)

// deployEnvironment deploy components into namespace
func (c *controller) deployEnvironment(queue *s2hv1beta1.Queue) error {
	deployTimeout := metav1.Duration{Duration: 1800 * time.Second}

	if deployConfig := c.getDeployConfiguration(queue); deployConfig != nil && deployConfig.Timeout.Duration != 0 {
		deployTimeout = deployConfig.Timeout
	}

	deployEngine := c.getDeployEngine(queue)

	// check deploy timeout
	if err := c.checkDeployTimeout(queue, deployTimeout); err != nil {
		return err
	}

	queueComps := make(map[string]*s2hv1beta1.Component)       // map[component name]component
	queueParentComps := make(map[string]*s2hv1beta1.Component) // map[parent component name]parent component

	switch queue.Spec.Type {
	case s2hv1beta1.QueueTypePreActive, s2hv1beta1.QueueTypePromoteToActive, s2hv1beta1.QueueTypeDemoteFromActive:
		if err := c.updateQueue(queue); err != nil {
			return err
		}
	default: // Upgrade, Reverify
		if isValid, err := c.validateQueue(queue); err != nil || !isValid {
			if err != nil {
				return err
			}
			return nil
		}

		configCtrl := c.getConfigController()
		comps, err := configCtrl.GetComponents(c.teamName)
		if err != nil {
			return err
		}

		newComps := make([]*s2hv1beta1.QueueComponent, 0)
		for _, qComp := range queue.Spec.Components {
			comp, ok := comps[qComp.Name]
			if !ok {
				continue
			}

			newComps = append(newComps, qComp)
			queueComps[qComp.Name] = comp
			queueParentComps[qComp.Name] = comp

			if comp.Parent != "" {
				delete(queueParentComps, qComp.Name)
				queueParentComps[comp.Parent] = comps[comp.Parent]
			}
		}

		// update queue if there are skipped components
		if len(newComps) != len(queue.Spec.Components) {
			queue.Spec.Components = newComps
			if err := c.updateQueue(queue); err != nil {
				return err
			}
		}
	}

	// Deploy
	if !queue.Status.IsConditionTrue(s2hv1beta1.QueueDeployStarted) {
		isDeployed, err := c.deployComponents(deployEngine, queue, queueComps, queueParentComps, deployTimeout.Duration)
		if err != nil && !isDeployed {
			return err
		}

		queue.Status.SetCondition(
			s2hv1beta1.QueueDeployStarted,
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

	if len(releases) == 0 {
		if !deployEngine.IsMocked() {
			return nil
		}
	}

	isDeployed, isFailed, errMsg := c.checkAllReleasesDeployed(deployEngine, releases)
	if isFailed {
		queue.Status.SetCondition(
			s2hv1beta1.QueueDeployed,
			corev1.ConditionFalse,
			fmt.Sprintf("release deployment failed: %s", errMsg))

		logger.Error(s2herrors.ErrReleaseFailed, fmt.Sprintf("queue: %s release failed", queue.Name))

		return c.updateQueueWithState(queue, s2hv1beta1.Collecting)
	} else if !isDeployed {
		time.Sleep(2 * time.Second)
		return nil
	}

	// checking environment is ready
	// change state if ready
	isReady, err := c.waitForComponentsReady(deployEngine)
	if err != nil {
		return err
	} else if !isReady {
		time.Sleep(2 * time.Second)
		return nil
	}

	// environment is ready
	queue.Status.SetCondition(
		s2hv1beta1.QueueDeployed,
		corev1.ConditionTrue,
		"queue deployment succeeded")
	return c.updateQueueWithState(queue, s2hv1beta1.Testing)
}

// checkDeployTimeout checks if deploy duration was longer than timeout.
// change state to `Collecting` if timeout
func (c *controller) checkDeployTimeout(queue *s2hv1beta1.Queue, deployTimeout metav1.Duration) error {
	now := metav1.Now()

	if queue.Status.StartDeployTime == nil {
		queue.Status.StartDeployTime = &now
		return c.updateQueue(queue)
	} else if now.Sub(queue.Status.StartDeployTime.Time) > deployTimeout.Duration {
		// deploy timeout
		queue.Status.SetCondition(
			s2hv1beta1.QueueDeployed,
			corev1.ConditionFalse,
			"queue deployment timeout")

		// update queue back to k8s
		if err := c.updateQueueWithState(queue, s2hv1beta1.Collecting); err != nil {
			return err
		}

		logger.Error(s2herrors.ErrDeployTimeout, fmt.Sprintf("queue: %s deploy timeout", queue.Name))

		return s2herrors.ErrDeployTimeout
	}

	return nil
}

// validateQueue checks if Queue exist in Configuration.
func (c *controller) validateQueue(queue *s2hv1beta1.Queue) (bool, error) {
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
		if err := c.client.Delete(context.TODO(), queue); err != nil {
			logger.Error(err, "deleting queue error")
			return false, err
		}
		c.clearCurrentQueue()
	}

	return true, nil
}

func (c *controller) getStableComponentsMap() (stableMap map[string]s2hv1beta1.StableComponent, err error) {
	// create StableComponentMap
	stableMap, err = valuesutil.GetStableComponentsMap(c.client, c.namespace)
	if err != nil {
		logger.Error(err, "cannot list StableComponents")
		return
	}
	return
}

func genCompValueFromQueue(compName string, qComps []*s2hv1beta1.QueueComponent) map[string]interface{} {
	for _, qComp := range qComps {
		if qComp.Name == compName {
			return map[string]interface{}{
				"image": map[string]interface{}{
					"repository": qComp.Repository,
					"tag":        qComp.Version,
				},
			}
		}
	}

	return map[string]interface{}{}
}

// applyEnvBaseConfig applies input values with specific env. configuration based on Queue.Spec.Type
func applyEnvBaseConfig(
	cfg *s2hv1beta1.ConfigSpec,
	values map[string]interface{},
	qt s2hv1beta1.QueueType,
	comp *s2hv1beta1.Component,
	teamName string,
) map[string]interface{} {
	var target map[string]s2hv1beta1.ComponentValues
	var err error

	switch qt {
	case s2hv1beta1.QueueTypePreActive:
		target, err = configctrl.GetEnvValues(cfg, s2hv1beta1.EnvPreActive, teamName)
	case s2hv1beta1.QueueTypePromoteToActive:
		target, err = configctrl.GetEnvValues(cfg, s2hv1beta1.EnvActive, teamName)
	case s2hv1beta1.QueueTypeUpgrade, s2hv1beta1.QueueTypeReverify:
		target, err = configctrl.GetEnvValues(cfg, s2hv1beta1.EnvStaging,teamName)
	case s2hv1beta1.QueueTypeDemoteFromActive:
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
	queue *s2hv1beta1.Queue,
	queueComps map[string]*s2hv1beta1.Component,
	queueParentComps map[string]*s2hv1beta1.Component,
	deployTimeout time.Duration,
) (isDeployed bool, err error) {
	stableMap, err := c.getStableComponentsMap()
	if err != nil {
		return false, err
	}

	releaseRevision := make(map[string]int)
	releases, err := deployEngine.GetReleases()
	if err != nil {
		return false, err
	}
	for _, rel := range releases {
		releaseRevision[rel.Name] = rel.Version
	}

	ctx, cancelFunc := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancelFunc()

	isDeployedCh := make(chan bool, 2)
	errCh := make(chan error, 2)
	go func() {
		isDeployed, err := c.deployComponentsExceptQueue(deployEngine, queue, queueParentComps, stableMap, deployTimeout)
		if err != nil {
			logger.Error(err, "cannot deploy components except current queue", "queue", queue.Name)
		}
		isDeployedCh <- isDeployed
		errCh <- err
	}()

	go func() {
		if !c.isUpgradeRelatedQueue(queue) {
			isDeployedCh <- true
			errCh <- nil
			return
		}

		err := c.deployQueueComponent(deployEngine, queue, queueComps, queueParentComps, stableMap, deployTimeout)
		if err != nil {
			logger.Error(err, "cannot deploy current queue component", "queue", queue.Name)
		}
		isDeployedCh <- isDeployed
		errCh <- err
	}()

	isDeployed = true
	for i := 0; i < 2; i++ {
		select {
		case <-ctx.Done():
			releases, err = deployEngine.GetReleases()
			if err != nil {
				return
			}

			if len(releases) == 0 {
				err = fmt.Errorf("release is being deployed")
				return
			}
			for _, rel := range releases {
				if revision, ok := releaseRevision[rel.Name]; ok {
					if rel.Version <= revision {
						err = fmt.Errorf("release is being deployed")
						return
					}
				}
			}

		case isDeployed := <-isDeployedCh:
			err := <-errCh
			if err != nil {
				return isDeployed, err
			}
		}
	}

	return
}

func (c *controller) isUpgradeRelatedQueue(q *s2hv1beta1.Queue) bool {
	return q.Spec.Type == s2hv1beta1.QueueTypeUpgrade || q.Spec.Type == s2hv1beta1.QueueTypeReverify
}

// deployComponentsExceptQueue ensures other components deployed with StableComponents
func (c *controller) deployComponentsExceptQueue(
	deployEngine internal.DeployEngine,
	queue *s2hv1beta1.Queue,
	queueParentComps map[string]*s2hv1beta1.Component,
	stableMap map[string]s2hv1beta1.StableComponent,
	deployTimeout time.Duration,
) (isDeployed bool, err error) {
	parentComps, err := c.getConfigController().GetParentComponents(c.teamName)
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

		baseValues, err := configctrl.GetEnvComponentValues(cfg, name, queue.Spec.TeamName, s2hv1beta1.EnvBase)
		if err != nil {
			return false, err
		}

		values := valuesutil.GenStableComponentValues(
			comp,
			stableMap,
			baseValues)

		switch queue.Spec.Type {
		case s2hv1beta1.QueueTypeDemoteFromActive:
			// rollback current active instead of upgrading
			if err := deployEngine.Rollback(c.genReleaseName(comp), 1); err != nil {
				return true, err
			}
		default:
			values = applyEnvBaseConfig(cfg, values, queue.Spec.Type, comp, queue.Spec.TeamName)
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
	queue *s2hv1beta1.Queue,
	queueComps map[string]*s2hv1beta1.Component,
	queueParentComps map[string]*s2hv1beta1.Component,
	stableMap map[string]s2hv1beta1.StableComponent,
	deployTimeout time.Duration,
) error {
	cfg, err := c.getConfiguration()
	if err != nil {
		return err
	}

	// deploy current queue
	for name, parentComp := range queueParentComps {
		baseValues, err := configctrl.GetEnvComponentValues(cfg, name, queue.Spec.TeamName, s2hv1beta1.EnvBase)
		if err != nil {
			return err
		}

		values := valuesutil.GenStableComponentValues(
			parentComp,
			stableMap,
			baseValues,
		)

		if queue.Spec.Type == s2hv1beta1.QueueTypeUpgrade {
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

		values = applyEnvBaseConfig(cfg, values, queue.Spec.Type, parentComp, queue.Spec.TeamName)
		if err := deployEngine.Create(c.genReleaseName(parentComp), parentComp, parentComp, values, &deployTimeout); err != nil {
			return err
		}
	}

	return nil
}

func (c *controller) waitForComponentsReady(deployEngine internal.DeployEngine) (bool, error) {
	parentComps, err := c.getConfigController().GetParentComponents(c.teamName)
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

	if len(pods.Items) == 0 {
		return false, nil
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
