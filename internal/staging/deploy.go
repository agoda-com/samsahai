package staging

import (
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	deploymentutil "k8s.io/kubernetes/pkg/controller/deployment/util"

	"github.com/agoda-com/samsahai/internal"
	s2herrors "github.com/agoda-com/samsahai/internal/errors"
	"github.com/agoda-com/samsahai/internal/staging/deploy/mock"
	"github.com/agoda-com/samsahai/internal/util/valuesutil"
	s2hv1beta1 "github.com/agoda-com/samsahai/pkg/apis/env/v1beta1"
)

// deployEnvironment deploy components into namespace
func (c *controller) deployEnvironment(queue *s2hv1beta1.Queue) error {
	deployTimeout := metav1.Duration{Duration: 1800 * time.Second}

	if deployConfig := c.getDeployConfiguration(queue); deployConfig != nil {
		deployTimeout = deployConfig.Timeout
	}

	deployEngine := c.getDeployEngine(c.getDeployConfiguration(queue))

	// check deploy timeout
	if err := c.checkDeployTimeout(queue, deployTimeout); err != nil {
		return err
	}

	var comp *internal.Component
	var parentComp *internal.Component

	switch queue.Spec.Type {
	case s2hv1beta1.QueueTypePreActive, s2hv1beta1.QueueTypePromoteToActive, s2hv1beta1.QueueTypeDemoteFromActive:
		queue.Status.ReleaseName = string(queue.Spec.Type)

		if err := c.updateQueue(queue); err != nil {
			return err
		}
	default: // Upgrade, Reverify
		if err := c.validateQueue(queue); err != nil {
			return err
		}

		comps := c.getConfigManager().GetComponents()
		comp = comps[queue.Spec.Name]
		parentComp = comp

		if comp.Parent != "" {
			parentComp = comps[comp.Parent]
		}

		// checking if release name was generated
		if err := c.createReleaseName(queue, parentComp); err != nil {
			return err
		}
	}

	// Deploy
	if !queue.Status.IsConditionTrue(s2hv1beta1.QueueDeployStarted) {

		err := c.deployComponents(deployEngine, queue, comp, parentComp)
		if err != nil {
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

	if c.isUpgradeRelatedQueue(queue) {
		isReady, err := deployEngine.IsReady(queue)
		if err != nil {
			return err
		} else if !isReady {
			time.Sleep(2 * time.Second)
			return nil
		}
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

func (c *controller) getDeployEngine(deployConfig *internal.ConfigDeploy) internal.DeployEngine {
	var e string
	if deployConfig == nil || deployConfig.Engine == nil || *deployConfig.Engine == "" {
		e = internal.MockDeployEngine
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

		logger.Error(s2herrors.ErrDeployTimeout, fmt.Sprintf("release: %s deploy timeout", queue.Status.ReleaseName))

		return s2herrors.ErrDeployTimeout
	}

	return nil
}

//
func (c *controller) createReleaseName(queue *s2hv1beta1.Queue, parentCom *internal.Component) error {
	if queue.Status.ReleaseName == "" {
		queue.Status.ReleaseName = c.genReleaseName(parentCom)
		if err := c.updateQueue(queue); err != nil {
			return err
		}
	}
	return nil
}

// validateQueue checks if Queue exist in Configuration.
func (c *controller) validateQueue(queue *s2hv1beta1.Queue) error {
	comps := c.getConfigManager().GetComponents()
	var isCompExist bool

	if _, isCompExist = comps[queue.Spec.Name]; !isCompExist {

		// delete queue
		if err := c.deleteQueue(queue); err != nil {
			return err
		}
		return nil
	}

	return nil
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

func genCompValueFromQueue(queue *s2hv1beta1.Queue) map[string]interface{} {
	return map[string]interface{}{
		"image": map[string]interface{}{
			"repository": queue.Spec.Repository,
			"tag":        queue.Spec.Version,
		},
	}
}

// applyEnvBaseConfig applies input values with specific env. configuration based on Queue.Spec.Type
func applyEnvBaseConfig(
	cfg *internal.Configuration,
	values map[string]interface{},
	qt s2hv1beta1.QueueType,
	comp *internal.Component,
) map[string]interface{} {
	var target map[string]internal.ComponentValues
	var envOK bool

	switch qt {
	case s2hv1beta1.QueueTypePreActive:
		target, envOK = cfg.Envs[internal.EnvPreActive]
	case s2hv1beta1.QueueTypePromoteToActive:
		target, envOK = cfg.Envs[internal.EnvActive]
	case s2hv1beta1.QueueTypeUpgrade, s2hv1beta1.QueueTypeReverify:
		target, envOK = cfg.Envs[internal.EnvStaging]
	case s2hv1beta1.QueueTypeDemoteFromActive:
		return values
	default:
		return values
	}
	if !envOK {
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
	comp *internal.Component,
	parentComp *internal.Component,
) error {

	stableMap, err := c.getStableComponentsMap()
	if err != nil {
		return err
	}

	err = c.deployComponentsExceptQueue(deployEngine, queue, parentComp, stableMap)
	if err != nil {
		return err
	}

	if !c.isUpgradeRelatedQueue(queue) {
		// ignore queue component if Queue type is not Upgrade or Reverify
		return nil
	}

	err = c.deployQueueComponent(deployEngine, queue, comp, parentComp, stableMap)
	if err != nil {
		return err
	}

	return nil
}

func (c *controller) isUpgradeRelatedQueue(q *s2hv1beta1.Queue) bool {
	return q.Spec.Type == s2hv1beta1.QueueTypeUpgrade || q.Spec.Type == s2hv1beta1.QueueTypeReverify
}

// deployComponentsExceptQueue ensures other components deployed with StableComponents
func (c *controller) deployComponentsExceptQueue(
	deployEngine internal.DeployEngine,
	queue *s2hv1beta1.Queue,
	queueParentComp *internal.Component,
	stableMap map[string]s2hv1beta1.StableComponent,
) error {
	parentComps := c.getConfigManager().GetParentComponents()
	queueParentCompName := ""
	if queueParentComp != nil {
		queueParentCompName = queueParentComp.Name
	}

	for name, comp := range parentComps {
		// skip current Queue
		if queueParentCompName == name {
			continue
		}

		values := valuesutil.GenStableComponentValues(
			comp,
			stableMap,
			c.getConfiguration().Envs["base"][name])

		values = applyEnvBaseConfig(c.getConfiguration(), values, queue.Spec.Type, comp)

		err := deployEngine.Create(c.genReleaseName(comp), comp, comp, values)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *controller) deployQueueComponent(
	deployEngine internal.DeployEngine,
	queue *s2hv1beta1.Queue,
	comp *internal.Component,
	parentComp *internal.Component,
	stableMap map[string]s2hv1beta1.StableComponent,
) error {
	values := valuesutil.GenStableComponentValues(
		parentComp,
		stableMap,
		c.getConfiguration().Envs["base"][parentComp.Name])

	if queue.Spec.Type == s2hv1beta1.QueueTypeUpgrade {
		// merge stable only matched component or dependencies
		v := genCompValueFromQueue(queue)
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

	values = applyEnvBaseConfig(c.getConfiguration(), values, queue.Spec.Type, parentComp)
	err := deployEngine.Create(c.genReleaseName(parentComp), comp, parentComp, values)
	if err != nil {
		return err
	}

	return nil
}

func (c *controller) waitForComponentsReady(deployEngine internal.DeployEngine) (bool, error) {
	parentComps := c.getConfigManager().GetParentComponents()

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

	listOpt := metav1.ListOptions{LabelSelector: labels.SelectorFromSet(selectors).String()}

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

func (c *controller) isDeploymentsReady(listOpt metav1.ListOptions) (bool, error) {
	list, err := c.clientset.AppsV1().Deployments(c.namespace).List(listOpt)
	if err != nil {
		logger.Error(err, "list appsv1.deployments error: "+listOpt.String())
		return false, err
	}

	for i, deploy := range list.Items {
		rs, err := deploymentutil.GetNewReplicaSet(&list.Items[i], c.clientset.AppsV1())
		if err != nil {
			logger.Error(err, "deploymentutil.getnewreplicaset error")
			return false, err
		} else if rs == nil {
			return false, nil
		}
		if deploy.Spec.Replicas == nil {
			// success
		} else if !(rs.Status.ReadyReplicas >= *deploy.Spec.Replicas-deploymentutil.MaxUnavailable(deploy)) {
			return false, nil
		}
	}

	return true, nil
}

func (c *controller) isPodsReady(listOpt metav1.ListOptions) (bool, error) {
	podList, err := c.clientset.CoreV1().Pods(c.namespace).List(listOpt)
	if err != nil {
		logger.Error(err, "list pods error", "namespace", c.namespace)
		return false, err
	}

	if len(podList.Items) == 0 {
		return false, nil
	}

	for _, pod := range podList.Items {
		isReady := false
		for _, cond := range pod.Status.Conditions {
			if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
				isReady = true
				break
			}
		}
		if !isReady {
			return false, nil
		}
	}

	return true, nil
}

func (c *controller) isServicesReady(listOpt metav1.ListOptions) (bool, error) {
	list, err := c.clientset.CoreV1().Services(c.namespace).List(listOpt)
	if err != nil {
		logger.Error(err, "list services error: "+listOpt.String())
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

func (c *controller) isPVCsReady(listOpt metav1.ListOptions) (bool, error) {
	list, err := c.clientset.CoreV1().PersistentVolumeClaims(c.namespace).List(listOpt)
	if err != nil {
		logger.Error(err, "list pvcs error: "+listOpt.String())
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
