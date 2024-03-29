package k8sobject

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	s2hv1 "github.com/agoda-com/samsahai/api/v1"
	"github.com/agoda-com/samsahai/internal"
	s2hlog "github.com/agoda-com/samsahai/internal/log"
)

var logger = s2hlog.S2HLog.WithName("k8sobject")

func getDefaultLabels(teamName string) map[string]string {
	defaultLabels := internal.GetDefaultLabels(teamName)
	defaultLabels["app.kubernetes.io/name"] = internal.StagingCtrlName
	defaultLabels["app.kubernetes.io/component"] = "custom-ctrl"
	defaultLabels["app.kubernetes.io/part-of"] = "samsahai"
	defaultLabels["app.kubernetes.io/managed-by"] = "samsahai"

	return defaultLabels
}

func getDefaultLabelsWithVersion(teamName string) map[string]string {
	defaultLabelsWithVersion := getDefaultLabels(teamName)
	defaultLabelsWithVersion["app.kubernetes.io/version"] = internal.Version

	return defaultLabelsWithVersion
}

func GetEmptyResourceQuota(namespaceName string) client.Object {
	resourceQuota := corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      namespaceName + internal.ResourcesQuotaSuffix,
			Namespace: namespaceName,
		},
	}

	return &resourceQuota
}

func GetResourceQuota(teamComp *s2hv1.Team, namespaceName string, resources corev1.ResourceList) client.Object {
	cpuResource := teamComp.Status.Used.Resources.Cpu()
	memoryResource := teamComp.Status.Used.Resources.Memory()

	if resources != nil {
		if resources.Cpu() != nil {
			cpuResource = resources.Cpu()
		}
		if resources.Memory() != nil {
			memoryResource = resources.Memory()
		}
	}

	resourceQuota := corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      namespaceName + internal.ResourcesQuotaSuffix,
			Namespace: namespaceName,
		},
		Spec: corev1.ResourceQuotaSpec{
			Hard: corev1.ResourceList{
				corev1.ResourceRequestsCPU:    *cpuResource,
				corev1.ResourceRequestsMemory: *memoryResource,
				corev1.ResourceLimitsCPU:      *cpuResource,
				corev1.ResourceLimitsMemory:   *memoryResource,
			},
		},
	}

	return &resourceQuota
}

func GetDeployment(scheme *runtime.Scheme, teamComp *s2hv1.Team, namespaceName string,
	configs *internal.SamsahaiConfig) client.Object {

	teamName := teamComp.GetName()

	samsahaiImage := configs.SamsahaiImage
	if teamComp.Status.Used.StagingCtrl != nil && !strings.EqualFold((*teamComp.Status.Used.StagingCtrl).Image, "") {
		samsahaiImage = (*teamComp.Status.Used.StagingCtrl).Image
	}

	defaultLabels := getDefaultLabels(teamName)
	defaultLabelsWithVersion := getDefaultLabelsWithVersion(teamName)

	envVars := []corev1.EnvVar{
		{
			Name:  "S2H_SERVER_URL",
			Value: configs.SamsahaiURL,
		},
		{
			Name:  "S2H_TEAM_NAME",
			Value: teamName,
		},
		{
			Name:  "POD_NAMESPACE",
			Value: namespaceName,
		},
	}

	for key, value := range configs.StagingEnvs {
		envVars = append(envVars, corev1.EnvVar{
			Name:  key,
			Value: value,
		})
	}

	deployment := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      internal.StagingCtrlName,
			Namespace: namespaceName,
			Labels:    defaultLabelsWithVersion,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: defaultLabels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: defaultLabelsWithVersion,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:                     internal.StagingCtrlName,
							Image:                    samsahaiImage,
							ImagePullPolicy:          "Always",
							Command:                  []string{"staging"},
							Args:                     []string{"start"},
							TerminationMessagePath:   "/dev/termination-log",
							TerminationMessagePolicy: "File",
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: internal.StagingDefaultPort,
									Protocol:      "TCP",
								},
							},
							Env: envVars,
							LivenessProbe: &corev1.Probe{
								InitialDelaySeconds: int32(20),
								TimeoutSeconds:      int32(1),
								PeriodSeconds:       int32(10),
								SuccessThreshold:    int32(1),
								FailureThreshold:    int32(3),
								Handler: corev1.Handler{
									HTTPGet: &corev1.HTTPGetAction{
										Path:   "/healthz",
										Port:   intstr.FromInt(internal.StagingDefaultPort),
										Scheme: corev1.URISchemeHTTP,
									},
								},
							},
							ReadinessProbe: &corev1.Probe{
								InitialDelaySeconds: int32(10),
								TimeoutSeconds:      int32(1),
								PeriodSeconds:       int32(10),
								SuccessThreshold:    int32(1),
								FailureThreshold:    int32(3),
								Handler: corev1.Handler{
									HTTPGet: &corev1.HTTPGetAction{
										Path:   "/healthz",
										Port:   intstr.FromInt(internal.StagingDefaultPort),
										Scheme: corev1.URISchemeHTTP,
									},
								},
							},
							EnvFrom: []corev1.EnvFromSource{
								{
									SecretRef: &corev1.SecretEnvSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: internal.StagingCtrlName,
										},
									},
								},
							},
						},
					},
					ServiceAccountName: internal.StagingCtrlName,
				},
			},
		},
	}

	// apply resource limit
	if len(teamComp.Status.Used.Resources) != 0 {
		deployment.Spec.Template.Spec.Containers[0].Resources = corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("1"),
				corev1.ResourceMemory: resource.MustParse("1Gi"),
			},
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("1"),
				corev1.ResourceMemory: resource.MustParse("1Gi"),
			},
		}
	}

	if teamComp.Status.Used.StagingCtrl != nil && (*teamComp.Status.Used.StagingCtrl).Resources.Size() > 0 {
		deployment.Spec.Template.Spec.Containers[0].Resources = (*teamComp.Status.Used.StagingCtrl).Resources
	}

	if err := controllerutil.SetControllerReference(teamComp, &deployment, scheme); err != nil {
		logger.Warn(fmt.Sprintf("cannot set controller reference for %s %s deployment", teamName, internal.StagingCtrlName))
	}

	return &deployment
}

func GetService(scheme *runtime.Scheme, teamComp *s2hv1.Team, namespaceName string) client.Object {
	teamName := teamComp.GetName()
	defaultLabelsWithVersion := getDefaultLabelsWithVersion(teamName)
	service := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      internal.StagingCtrlName,
			Namespace: namespaceName,
			Labels:    defaultLabelsWithVersion,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       internal.StagingDefaultPort,
					Protocol:   "TCP",
					TargetPort: intstr.FromInt(internal.StagingDefaultPort),
				},
			},
			Selector: defaultLabelsWithVersion,
			Type:     corev1.ServiceTypeClusterIP,
		},
	}

	if err := controllerutil.SetControllerReference(teamComp, &service, scheme); err != nil {
		logger.Warn(fmt.Sprintf("cannot set controller reference for %s %s service", teamName, internal.StagingCtrlName))
	}

	return &service
}

func GetRole(teamComp *s2hv1.Team, namespaceName string) client.Object {
	teamName := teamComp.GetName()
	defaultLabelsWithVersion := getDefaultLabelsWithVersion(teamName)
	role := rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      internal.StagingCtrlName,
			Namespace: namespaceName,
			Labels:    defaultLabelsWithVersion,
		},
		Rules: []rbacv1.PolicyRule{
			// samsahai
			{
				APIGroups: []string{
					"env.samsahai.io",
				},
				Resources: []string{
					"desiredcomponents",
					"queues",
					"queuehistories",
					"stablecomponents",
					"pullrequesttriggers",
					"pullrequestqueues",
					"pullrequestqueuehistories",
				},
				Verbs: []string{"*"},
			},
			// deploy
			{
				APIGroups: []string{
					"",
				},
				Resources: []string{
					"pods",
					"pods/log",
					"services",
					"endpoints",
					"serviceaccounts",
					"configmaps",
					"secrets",
					"persistentvolumeclaims",
					"replicationcontrollers",
				},
				Verbs: []string{"*"},
			},
			{
				APIGroups: []string{
					"rbac.authorization.k8s.io",
				},
				Resources: []string{
					"roles",
					"rolebindings",
				},
				Verbs: []string{"*"},
			},
			{
				APIGroups: []string{
					"apps",
				},
				Resources: []string{
					"deployments",
					"statefulsets",
					"replicasets",
				},
				Verbs: []string{"*"},
			},
			{
				APIGroups: []string{
					"autoscaling",
				},
				Resources: []string{
					"horizontalpodautoscalers",
				},
				Verbs: []string{"*"},
			},
			{
				APIGroups: []string{
					"batch",
				},
				Resources: []string{
					"jobs",
					"cronjobs",
				},
				Verbs: []string{"*"},
			},
			{
				APIGroups: []string{
					"extensions",
				},
				Resources: []string{
					"deployments",
					"statefulsets",
					"replicasets",
					"ingresses",
					"networkpolicies",
				},
				Verbs: []string{"*"},
			},
			{
				APIGroups: []string{
					"policy",
				},
				Resources: []string{
					"poddisruptionbudgets",
				},
				Verbs: []string{"*"},
			},
			{
				APIGroups: []string{
					"networking.k8s.io",
				},
				Resources: []string{
					"networkpolicies",
					"ingresses",
				},
				Verbs: []string{"*"},
			},
			{
				APIGroups: []string{
					"networking.istio.io",
				},
				Resources: []string{
					"virtualservices",
					"destinationrules",
				},
				Verbs: []string{"*"},
			},
			{
				APIGroups: []string{
					"",
				},
				Resources: []string{
					"bindings",
					"events",
					"namespaces",
					"resourcequotas",
				},
				Verbs: []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{
					"env.samsahai.io",
				},
				Resources: []string{
					"configs",
				},
				Verbs: []string{"get", "list", "watch"},
			},
		},
	}

	return &role
}

func GetRoleBinding(teamComp *s2hv1.Team, namespaceName string) client.Object {
	teamName := teamComp.GetName()
	defaultLabelsWithVersion := getDefaultLabelsWithVersion(teamName)
	roleBinding := rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      internal.StagingCtrlName,
			Namespace: namespaceName,
			Labels:    defaultLabelsWithVersion,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     internal.StagingCtrlName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      internal.StagingCtrlName,
				Namespace: namespaceName,
			},
		},
	}

	return &roleBinding
}

func GetClusterRole(teamComp *s2hv1.Team, namespace string) client.Object {
	teamName := teamComp.GetName()
	defaultLabelsWithVersion := getDefaultLabelsWithVersion(teamName)
	role := rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:   GenClusterRoleName(namespace),
			Labels: defaultLabelsWithVersion,
		},
		Rules: []rbacv1.PolicyRule{
			// samsahai
			{
				APIGroups: []string{
					"env.samsahai.io",
				},
				Resources: []string{
					"configs",
					"stablecomponents",
				},
				Verbs: []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{
					"env.samsahai.io",
				},
				Resources: []string{
					"queues",
				},
				Verbs: []string{"*"},
			},
			{
				APIGroups: []string{
					"policy",
				},
				Resources: []string{
					"podsecuritypolicies",
				},
				Verbs: []string{"get", "list", "watch", "create", "update", "patch", "delete"},
			},
		},
	}

	return &role
}

func GetClusterRoleBinding(teamComp *s2hv1.Team, namespace string) client.Object {
	teamName := teamComp.GetName()
	defaultLabelsWithVersion := getDefaultLabelsWithVersion(teamName)
	roleBinding := rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   GenClusterRoleName(namespace),
			Labels: defaultLabelsWithVersion,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     GenClusterRoleName(namespace),
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      internal.StagingCtrlName,
				Namespace: namespace,
			},
		},
	}

	return &roleBinding
}

func GetServiceAccount(teamComp *s2hv1.Team, namespaceName string) client.Object {
	teamName := teamComp.GetName()
	defaultLabelsWithVersion := getDefaultLabelsWithVersion(teamName)
	serviceAccount := corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      internal.StagingCtrlName,
			Namespace: namespaceName,
			Labels:    defaultLabelsWithVersion,
		},
	}

	return &serviceAccount
}

type KeyValue struct {
	Key   string
	Value intstr.IntOrString
}

func GetSecret(scheme *runtime.Scheme, teamComp *s2hv1.Team, namespaceName string, kvs ...KeyValue) client.Object {
	teamName := teamComp.GetName()
	data := map[string][]byte{}
	for i := range kvs {
		kv := kvs[i]
		if kv.Key == "" || kv.Value.String() == "" {
			continue
		}
		key := strings.Replace(strings.ToUpper(kv.Key), "-", "_", -1)
		data[key] = []byte(kv.Value.String())
	}
	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      internal.StagingCtrlName,
			Namespace: namespaceName,
			Labels:    getDefaultLabelsWithVersion(teamName),
		},
		Type: corev1.SecretTypeOpaque,
		Data: data,
	}

	if err := controllerutil.SetControllerReference(teamComp, &secret, scheme); err != nil {
		logger.Warn(fmt.Sprintf("cannot set controller reference for %s %s secret", teamName, internal.StagingCtrlName))
	}

	return &secret
}

func GetTeamSecretName(teamName string) string {
	return fmt.Sprintf("%s%s-secret", internal.AppPrefix, teamName)
}

func IsK8sObjectChanged(found, target runtime.Object) bool {
	switch target.(type) {
	case *appsv1.Deployment:
		return isDeploymentChanged(found, target)
	case *corev1.ResourceQuota:
		return isResourceQuotaChanged(found, target)
	case *rbacv1.Role:
		return isRoleChanged(found, target)
	case *rbacv1.RoleBinding:
		return isRoleBindingChanged(found, target)
	case *corev1.Secret:
		return isSecretChanged(found, target)
	case *corev1.Service:
		return isServiceChanged(found, target)
	case *corev1.ServiceAccount:
		return isServiceAccountChanged(found, target)
	}

	return false
}

func GenClusterRoleName(namespace string) string {
	return internal.StagingCtrlName + "-" + namespace
}

func isDeploymentChanged(found, target interface{}) bool {
	var isObjChanged bool
	foundLabels := found.(*appsv1.Deployment).Labels
	targetLabels := target.(*appsv1.Deployment).Labels
	if !deepEqual(foundLabels, targetLabels) {
		logger.Debug("found deployment labels changed",
			"foundLabels", foundLabels, "targetLabels", targetLabels)
		isObjChanged = true
		found.(*appsv1.Deployment).Labels = targetLabels
	}

	foundSpecTmplLabels := found.(*appsv1.Deployment).Spec.Template.Labels
	targetSpecTmplLabels := target.(*appsv1.Deployment).Spec.Template.Labels
	if !deepEqual(foundSpecTmplLabels, targetSpecTmplLabels) {
		logger.Debug("found pod template labels changed",
			"foundLabels", foundSpecTmplLabels, "targetLabels", targetSpecTmplLabels)
		isObjChanged = true
		found.(*appsv1.Deployment).Spec.Template.Labels = targetSpecTmplLabels
	}

	containersLen := len(found.(*appsv1.Deployment).Spec.Template.Spec.Containers)
	for i := 0; i < containersLen; i++ {
		foundTmplContainer := found.(*appsv1.Deployment).Spec.Template.Spec.Containers[i]
		targetTmplContainer := target.(*appsv1.Deployment).Spec.Template.Spec.Containers[i]

		if !areContainersEqual(foundTmplContainer, targetTmplContainer) {
			logger.Debug("found containers changed",
				"foundContainers", foundTmplContainer, "targetContainers", targetTmplContainer)
			isObjChanged = true
			found.(*appsv1.Deployment).Spec.Template.Spec.Containers[i] = targetTmplContainer
		}
	}

	return isObjChanged
}

func areContainersEqual(firstContainer, secondContainer corev1.Container) bool {
	if len(firstContainer.Env) > 0 {
		sort.SliceStable(firstContainer.Env, func(i, j int) bool {
			return firstContainer.Env[i].Name < firstContainer.Env[j].Name
		})
	}

	if len(secondContainer.Env) > 0 {
		sort.SliceStable(secondContainer.Env, func(i, j int) bool {
			return secondContainer.Env[i].Name < secondContainer.Env[j].Name
		})
	}

	return deepEqual(firstContainer, secondContainer)
}

func isResourceQuotaChanged(found, target interface{}) bool {
	foundSpec := found.(*corev1.ResourceQuota).Spec
	targetSpec := target.(*corev1.ResourceQuota).Spec
	if !deepEqual(foundSpec, targetSpec) {
		logger.Debug("found resource quota changed",
			"foundSpec", foundSpec, "targetSpec", targetSpec)
		found.(*corev1.ResourceQuota).Spec = targetSpec
		return true
	}

	return false
}

func isRoleChanged(found, target interface{}) bool {
	var isObjChanged bool
	foundLabels := found.(*rbacv1.Role).Labels
	targetLabels := target.(*rbacv1.Role).Labels
	if !deepEqual(foundLabels, targetLabels) {
		isObjChanged = true
		found.(*rbacv1.Role).Labels = targetLabels
	}

	foundRules := found.(*rbacv1.Role).Rules
	targetRules := target.(*rbacv1.Role).Rules
	if !deepEqual(foundRules, targetRules) {
		isObjChanged = true
		found.(*rbacv1.Role).Rules = targetRules
	}

	return isObjChanged
}

func isRoleBindingChanged(found, target interface{}) bool {
	var isObjChanged bool
	foundLabels := found.(*rbacv1.RoleBinding).Labels
	targetLabels := target.(*rbacv1.RoleBinding).Labels
	if !deepEqual(foundLabels, targetLabels) {
		isObjChanged = true
		found.(*rbacv1.RoleBinding).Labels = targetLabels
	}

	foundSubjects := found.(*rbacv1.RoleBinding).Subjects
	targetSubjects := target.(*rbacv1.RoleBinding).Subjects
	if !deepEqual(foundSubjects, targetSubjects) {
		isObjChanged = true
		found.(*rbacv1.RoleBinding).Subjects = targetSubjects
	}

	return isObjChanged
}

func isSecretChanged(found, target interface{}) bool {
	var isObjChanged bool
	foundLabels := found.(*corev1.Secret).Labels
	targetLabels := target.(*corev1.Secret).Labels
	if !deepEqual(foundLabels, targetLabels) {
		isObjChanged = true
		found.(*corev1.Secret).Labels = targetLabels
	}

	foundData := found.(*corev1.Secret).Data
	targetData := target.(*corev1.Secret).Data
	if !deepEqual(foundData, targetData) {
		isObjChanged = true
		found.(*corev1.Secret).Data = targetData
	}

	return isObjChanged
}

func isServiceChanged(found, target interface{}) bool {
	var isObjChanged bool
	foundLabels := found.(*corev1.Service).Labels
	targetLabels := target.(*corev1.Service).Labels
	if !deepEqual(foundLabels, targetLabels) {
		isObjChanged = true
		found.(*corev1.Service).Labels = targetLabels
	}

	foundSpecPorts := found.(*corev1.Service).Spec.Ports
	targetSpecPorts := target.(*corev1.Service).Spec.Ports
	if !deepEqual(foundSpecPorts, targetSpecPorts) {
		isObjChanged = true
		found.(*corev1.Service).Spec.Ports = targetSpecPorts
	}

	foundSpecSelector := found.(*corev1.Service).Spec.Selector
	targetSpecSelector := target.(*corev1.Service).Spec.Selector
	if !deepEqual(foundSpecSelector, targetSpecSelector) {
		isObjChanged = true
		found.(*corev1.Service).Spec.Selector = targetSpecSelector
	}

	return isObjChanged
}

func isServiceAccountChanged(found, target interface{}) bool {
	var isObjChanged bool
	foundLabels := found.(*corev1.ServiceAccount).Labels
	targetLabels := target.(*corev1.ServiceAccount).Labels
	if !deepEqual(foundLabels, targetLabels) {
		isObjChanged = true
		found.(*corev1.ServiceAccount).Labels = targetLabels
	}

	return isObjChanged
}

func deepEqual(found, target interface{}) bool {
	return reflect.DeepEqual(found, target)
}
