package samsahai

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crctrl "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	s2hv1beta1 "github.com/agoda-com/samsahai/api/v1beta1"
	"github.com/agoda-com/samsahai/internal"
	s2hconfig "github.com/agoda-com/samsahai/internal/config"
	"github.com/agoda-com/samsahai/internal/errors"
	s2hlog "github.com/agoda-com/samsahai/internal/log"
	"github.com/agoda-com/samsahai/internal/reporter/reportermock"
	"github.com/agoda-com/samsahai/internal/reporter/rest"
	"github.com/agoda-com/samsahai/internal/reporter/shell"
	"github.com/agoda-com/samsahai/internal/reporter/slack"
	"github.com/agoda-com/samsahai/internal/samsahai/checker/harbor"
	"github.com/agoda-com/samsahai/internal/samsahai/checker/publicregistry"
	"github.com/agoda-com/samsahai/internal/samsahai/exporter"
	"github.com/agoda-com/samsahai/internal/samsahai/k8sobject"
	"github.com/agoda-com/samsahai/internal/samsahai/plugin"
	"github.com/agoda-com/samsahai/internal/util/cmd"
	"github.com/agoda-com/samsahai/internal/util/stringutils"
	"github.com/agoda-com/samsahai/internal/util/valuesutil"
	"github.com/agoda-com/samsahai/pkg/samsahai/rpc"
)

var logger = s2hlog.S2HLog.WithName("controller")

const (
	CtrlName          = "samsahai-ctrl"
	teamFinalizerName = "team.finalizers.samsahai.io"

	DefaultPluginsDir = "plugins"

	// MaxConcurrentProcess represents no. of concurrent process in internal process
	MaxConcurrentProcess = 1

	// MaxReconcileConcurrent represents no. of concurrent process in operator controller
	MaxReconcileConcurrent = 1

	DefaultWorkQueueBaseDelay = 5 * time.Millisecond
	DefaultWorkQueueMaxDelay  = 60 * time.Second
)

type controller struct {
	scheme        *runtime.Scheme
	muTeamConfigs *sync.Mutex
	client        client.Client
	clientset     *kubernetes.Clientset
	teamConfigs   map[string]internal.ConfigManager
	namespace     string

	rpcHandler rpc.TwirpServer

	internalStop    <-chan struct{}
	internalStopper chan<- struct{}

	queue workqueue.RateLimitingInterface

	// checkersDisabled represents should controller load checkers or not.
	checkersDisabled bool
	checkers         map[string]internal.DesiredComponentChecker
	// pluginsDisabled represents should controller load plugins or not.
	pluginsDisabled bool
	plugins         map[string]internal.Plugin

	// reportersDisabled represents should controller load reporter or not.
	reportersDisabled bool
	reporters         map[string]internal.Reporter

	configs internal.SamsahaiConfig
}

// New returns Samsahai controller and assign itself to Manager for
// doing the reconcile when `Team` CRD got changed.
func New(
	mgr manager.Manager,
	ns string,
	configs internal.SamsahaiConfig,
	options ...Option,
) internal.SamsahaiController {
	stop := make(chan struct{})
	queue := workqueue.NewRateLimitingQueue(
		workqueue.NewItemExponentialFailureRateLimiter(DefaultWorkQueueBaseDelay, DefaultWorkQueueMaxDelay))

	scheme := &runtime.Scheme{}
	if mgr != nil {
		scheme = mgr.GetScheme()
	}

	c := &controller{
		scheme:          scheme,
		muTeamConfigs:   &sync.Mutex{},
		teamConfigs:     map[string]internal.ConfigManager{},
		namespace:       ns,
		internalStop:    stop,
		internalStopper: stop,
		queue:           queue,
		checkers:        map[string]internal.DesiredComponentChecker{},
		plugins:         map[string]internal.Plugin{},
		reporters:       map[string]internal.Reporter{},
		configs:         configs,
	}

	if mgr != nil {
		// create runtime client
		c.client = mgr.GetClient()

		// creates clientset
		clientset, err := kubernetes.NewForConfig(mgr.GetConfig())
		if err != nil {
			logger.Error(errors.ErrInternalError, "cannot create clientset")
			panic(errors.ErrInternalError)
		}
		c.clientset = clientset

		if err := add(mgr, c); err != nil {
			logger.Error(err, "cannot add samsahai controller to manager")
			return nil
		}
	}

	for _, opt := range options {
		opt(c)
	}

	c.rpcHandler = rpc.NewRPCServer(c, nil)

	if !c.checkersDisabled {
		logger.Debug("loading checkers")
		c.loadCheckers()
	}

	if !c.reportersDisabled {
		logger.Debug("loading reporters")
		c.loadReporters()
	}

	if !c.pluginsDisabled {
		logger.Debug("loading plugins")
		pluginsDir := configs.PluginsDir
		if pluginsDir == "" {
			pluginsDir = DefaultPluginsDir
		}
		c.loadPlugins(pluginsDir)
	}

	return c
}

type Option func(*controller)

func WithClient(client client.Client) Option {
	return func(c *controller) {
		c.client = client
	}
}
func WithConfigManager(teamName string, configManager internal.ConfigManager) Option {
	return func(c *controller) {
		if c.teamConfigs == nil {
			c.teamConfigs = map[string]internal.ConfigManager{}
		}
		c.teamConfigs[teamName] = configManager
	}
}
func WithDisableLoaders(checkers, plugins, reporters bool) Option {
	return func(c *controller) {
		c.checkersDisabled = checkers
		c.pluginsDisabled = plugins
		c.reportersDisabled = reporters
	}
}

func WithScheme(scheme *runtime.Scheme) Option {
	return func(c *controller) {
		c.scheme = scheme
	}
}

func (c *controller) loadReporters() {
	// init reporters
	reporters := []internal.Reporter{
		reportermock.New(),
		rest.New(),
		shell.New(),
	}

	if c.configs.SamsahaiCredential.SlackToken != "" {
		reporters = append(reporters, slack.New(c.configs.SamsahaiCredential.SlackToken))
	}

	for _, reporter := range reporters {
		if reporter == nil {
			continue
		}
		c.reporters[reporter.GetName()] = reporter
	}
}

func (c *controller) loadCheckers() {
	// init checkers
	checkers := []internal.DesiredComponentChecker{
		publicregistry.New(),
		harbor.New(),
	}
	for _, checker := range checkers {
		if checker == nil {
			continue
		}
		c.checkers[checker.GetName()] = checker
	}
}

func (c *controller) loadPlugins(dir string) {
	cwd, _ := os.Getwd()
	var files []string
	err := filepath.Walk(path.Join(cwd, dir), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		logger.Error(err, "loading plugins error", "path", path.Join(cwd, dir))
		return
	}

	for _, file := range files {
		p, err := plugin.New(file)
		if err != nil {
			logger.Warnf("cannot load plugin: %v", err)
			continue
		}
		if _, ok := c.plugins[p.GetName()]; ok {
			logger.Warn("duplicate plugin", "name", p.GetName(), "file", file)
			continue
		}
		c.plugins[p.GetName()] = p

		if _, ok := c.checkers[p.GetName()]; ok {
			logger.Warn("duplicate checker", "name", p.GetName(), "file", file)
		}
		c.checkers[p.GetName()] = p
	}
}

func (c *controller) GetTeamConfigManager(teamName string) (internal.ConfigManager, bool) {
	c.muTeamConfigs.Lock()
	defer c.muTeamConfigs.Unlock()
	teamConfigs, ok := c.teamConfigs[teamName]
	return teamConfigs, ok
}

func (c *controller) GetTeamConfigManagers() map[string]internal.ConfigManager {
	return c.teamConfigs
}

func (c *controller) GetPlugins() map[string]internal.Plugin {
	return c.plugins
}

type TeamNamespaceStatusOption func(teamComp *s2hv1beta1.Team) (string, s2hv1beta1.TeamConditionType)

func withTeamStagingNamespaceStatus(namespace string, isDelete ...bool) TeamNamespaceStatusOption {
	return func(teamComp *s2hv1beta1.Team) (string, s2hv1beta1.TeamConditionType) {
		teamComp.Status.Namespace.Staging = namespace
		if len(isDelete) > 0 && isDelete[0] {
			teamComp.Status.Namespace.Staging = ""
		}

		return namespace, s2hv1beta1.TeamNamespaceStagingCreated
	}
}

func withTeamPreActiveNamespaceStatus(namespace string, isDelete ...bool) TeamNamespaceStatusOption {
	return func(teamComp *s2hv1beta1.Team) (string, s2hv1beta1.TeamConditionType) {
		teamComp.Status.Namespace.PreActive = namespace
		if len(isDelete) > 0 && isDelete[0] {
			teamComp.Status.Namespace.PreActive = ""
		}

		return namespace, s2hv1beta1.TeamNamespacePreActiveCreated
	}
}

func withTeamPreviousActiveNamespaceStatus(namespace string, isDelete ...bool) TeamNamespaceStatusOption {
	return func(teamComp *s2hv1beta1.Team) (string, s2hv1beta1.TeamConditionType) {
		teamComp.Status.Namespace.PreviousActive = namespace
		if len(isDelete) > 0 && isDelete[0] {
			teamComp.Status.Namespace.PreviousActive = ""
		}

		return namespace, s2hv1beta1.TeamNamespacePreviousActiveCreated
	}
}

func withTeamActiveNamespaceStatus(namespace string, isDelete ...bool) TeamNamespaceStatusOption {
	return func(teamComp *s2hv1beta1.Team) (string, s2hv1beta1.TeamConditionType) {
		teamComp.Status.Namespace.Active = namespace
		if len(isDelete) > 0 && isDelete[0] {
			teamComp.Status.Namespace.Active = ""
		}

		return namespace, s2hv1beta1.TeamNamespaceActiveCreated
	}
}

func (c *controller) CreateStagingEnvironment(teamName, namespace string) error {
	return c.createNamespace(teamName, withTeamStagingNamespaceStatus(namespace))
}

func (c *controller) CreatePreActiveEnvironment(teamName, namespace string) error {
	return c.createNamespace(teamName, withTeamPreActiveNamespaceStatus(namespace))
}

func (c *controller) PromoteActiveEnvironment(teamComp *s2hv1beta1.Team, namespace string) error {
	preActiveNamespace := teamComp.Status.Namespace.PreActive
	activeNamespace := teamComp.Status.Namespace.Active
	if namespace == preActiveNamespace {
		if err := c.storeCurrentActiveComponentsToTeam(teamComp, namespace); err != nil {
			return errors.Wrapf(err, "cannot store current active components of %s into team %s",
				namespace, teamComp.Name)
		}

		teamNsOpts := []TeamNamespaceStatusOption{
			withTeamActiveNamespaceStatus(preActiveNamespace),
			withTeamPreviousActiveNamespaceStatus(activeNamespace),
			withTeamPreActiveNamespaceStatus(""),
		}

		teamComp.Status.SetCondition(
			s2hv1beta1.TeamNamespaceActiveCreated,
			corev1.ConditionTrue,
			fmt.Sprintf("%s namespace is switched to active", preActiveNamespace))
		teamComp.Status.SetCondition(
			s2hv1beta1.TeamNamespacePreviousActiveCreated,
			corev1.ConditionTrue,
			fmt.Sprintf("%s namespace is switched to previous active", activeNamespace))
		teamComp.Status.SetCondition(
			s2hv1beta1.TeamNamespacePreActiveCreated,
			corev1.ConditionFalse,
			"pre-active namespace is reset")

		if err := c.updateTeamNamespacesStatus(teamComp, teamNsOpts...); err != nil {
			return errors.Wrap(err, "cannot update team conditions when promote active")
		}

		logger.Debug(fmt.Sprintf("switching %s to active namespace", preActiveNamespace))
		return nil
	}

	if namespace == activeNamespace {
		logger.Debug(fmt.Sprintf("%s namespace is switched to active namespace successfully", namespace))
		return nil
	}

	return fmt.Errorf("regarding " + namespace + " (pre-active namespace) is not consistent with " +
		preActiveNamespace + " (team pre-active namespace), so this pre-active namespace cannot be switched")
}

func (c *controller) storeCurrentActiveComponentsToTeam(teamComp *s2hv1beta1.Team, namespace string) error {
	stableList := &s2hv1beta1.StableComponentList{}
	if err := c.client.List(context.TODO(), stableList, &client.ListOptions{Namespace: namespace}); err != nil {
		return errors.Wrapf(err, "cannot get list of stable components, namespace %s", namespace)
	}

	teamComp.Status.SetCurrentActiveComponents(stableList.Items)
	return nil
}

func (c *controller) createNamespace(teamName string, teamNsOpt TeamNamespaceStatusOption) error {
	teamComp := &s2hv1beta1.Team{}
	if err := c.getTeam(teamName, teamComp); err != nil {
		return err
	}

	if err := c.createNamespaceByTeam(teamComp, teamNsOpt); err != nil {
		return err
	}

	return nil
}

func (c *controller) createNamespaceByTeam(teamComp *s2hv1beta1.Team, teamNsOpt TeamNamespaceStatusOption) error {
	namespace, nsConditionType := teamNsOpt(teamComp)
	namespaceObj := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}

	ctx := context.TODO()
	err := c.client.Get(ctx, types.NamespacedName{Name: namespace}, &namespaceObj)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			if nsConditionType == s2hv1beta1.TeamNamespaceStagingCreated {
				if err := controllerutil.SetControllerReference(teamComp, &namespaceObj, c.scheme); err != nil {
					return err
				}
			}

			if err := c.client.Create(ctx, &namespaceObj); err != nil && !k8serrors.IsAlreadyExists(err) {
				return err
			}

			return errors.ErrTeamNamespaceStillCreating
		}

		return err
	}

	err = c.createEnvironmentObjects(teamComp, namespace)
	if err != nil {
		return err
	}

	if !teamComp.Status.IsConditionTrue(nsConditionType) {
		if c.configs.PostNamespaceCreation != nil {
			logger.Debug("start executing command after creating namespace", "namespace", namespace)
			if err := c.runPostNamespaceCreation(namespace, teamComp); err != nil {
				logger.Error(err, "cannot execute command after creating namespace",
					"namespace", namespace)
			}
		}

		teamComp.Status.SetCondition(
			nsConditionType,
			corev1.ConditionTrue,
			fmt.Sprintf("%s namespace is created and staging ctrl is deployed", namespace))

		if err := c.updateTeamNamespacesStatus(teamComp, teamNsOpt); err != nil {
			return errors.Wrap(err, "cannot update team conditions when create namespace success")
		}
	}

	return nil
}

func (c *controller) runPostNamespaceCreation(ns string, team *s2hv1beta1.Team) error {
	cmdAndArgs := c.configs.PostNamespaceCreation.CommandAndArgs

	creationObj := internal.PostNamespaceCreation{
		Namespace: ns,
		Team: s2hv1beta1.Team{
			Spec:   team.Spec,
			Status: team.Status,
		},
		SamsahaiConfig: c.configs,
	}

	cmdObj := cmd.RenderTemplate(cmdAndArgs.Command, cmdAndArgs.Args, creationObj)
	out, err := cmd.ExecuteCommand(context.TODO(), c.configs.ConfigDirPath, cmdObj)
	if err != nil {
		return err
	}
	logger.Debug(fmt.Sprintf("output: %s", out), "namespace", ns)

	return nil
}

func (c *controller) createEnvironmentObjects(teamComp *s2hv1beta1.Team, namespace string) error {
	secretKVs := []k8sobject.KeyValue{
		{
			Key:   internal.VKS2HAuthToken,
			Value: intstr.FromString(c.configs.SamsahaiCredential.InternalAuthToken),
		},
		{
			Key:   internal.VKTeamcityUsername,
			Value: intstr.FromString(c.configs.SamsahaiCredential.TeamcityUsername),
		},
		{
			Key:   internal.VKTeamcityPassword,
			Value: intstr.FromString(c.configs.SamsahaiCredential.TeamcityPassword),
		},
		{
			Key:   internal.VKTeamcityURL,
			Value: intstr.FromString(c.configs.TeamcityURL),
		},
	}
	k8sObjects := []runtime.Object{
		k8sobject.GetService(c.scheme, teamComp, namespace),
		k8sobject.GetServiceAccount(teamComp, namespace),
		k8sobject.GetRole(teamComp, namespace),
		k8sobject.GetRoleBinding(teamComp, namespace),
		k8sobject.GetSecret(c.scheme, teamComp, namespace, secretKVs...),
	}

	if teamComp.Spec.StagingCtrl != nil && !(*teamComp.Spec.StagingCtrl).IsDeploy {
		logger.Warn("skip deploying the staging controller deployment")
	} else {
		deploymentObj := k8sobject.GetDeployment(c.scheme, teamComp, namespace, c.configs.SamsahaiURL, c.configs.SamsahaiImage)
		k8sObjects = append(k8sObjects, deploymentObj)
	}

	if len(teamComp.Spec.Resources) > 0 {
		quotaObj := k8sobject.GetResourceQuota(teamComp, namespace)
		k8sObjects = append(k8sObjects, quotaObj)
	}

	for _, k8sObject := range k8sObjects {
		if err := deployStagingCtrl(c.client, k8sObject); err != nil {
			return errors.Wrap(err, "cannot deploy staging controller")
		}
	}

	return nil
}

func deployStagingCtrl(c client.Client, obj runtime.Object) error {
	ctx := context.TODO()
	target := obj.DeepCopyObject()
	objKey, err := client.ObjectKeyFromObject(obj)
	if err != nil {
		return err
	}

	if err := c.Get(ctx, objKey, obj); err != nil {
		if k8serrors.IsNotFound(err) {
			return c.Create(ctx, obj)
		}

		return err
	}

	if k8sobject.IsK8sObjectChanged(obj, target) {
		logger.Debug(fmt.Sprintf("%s of %s namespace has some changes", obj.GetObjectKind().GroupVersionKind(), objKey.Namespace))
		if err := c.Update(ctx, obj); err != nil {
			return err
		}
	}

	return nil
}

func getAllTeamNamespaces(teamComp *s2hv1beta1.Team, isDelete bool) []TeamNamespaceStatusOption {
	var teamNsOpts []TeamNamespaceStatusOption
	stagingNs := teamComp.Status.Namespace.Staging
	if !strings.EqualFold("", stagingNs) {
		teamNsOpts = append(teamNsOpts, withTeamStagingNamespaceStatus(stagingNs, isDelete))
	}

	previousActiveNs := teamComp.Status.Namespace.PreviousActive
	if !strings.EqualFold("", previousActiveNs) {
		teamNsOpts = append(teamNsOpts, withTeamPreviousActiveNamespaceStatus(previousActiveNs, isDelete))
	}

	preActiveNs := teamComp.Status.Namespace.PreActive
	if !strings.EqualFold("", preActiveNs) {
		teamNsOpts = append(teamNsOpts, withTeamPreActiveNamespaceStatus(preActiveNs, isDelete))
	}

	activeNs := teamComp.Status.Namespace.Active
	if !strings.EqualFold("", activeNs) {
		teamNsOpts = append(teamNsOpts, withTeamActiveNamespaceStatus(activeNs, isDelete))
	}

	return teamNsOpts
}

func (c *controller) DestroyActiveEnvironment(teamName, namespace string) error {
	return c.destroyNamespace(teamName, withTeamActiveNamespaceStatus(namespace, true))
}

func (c *controller) DestroyPreActiveEnvironment(teamName, namespace string) error {
	return c.destroyNamespace(teamName, withTeamPreActiveNamespaceStatus(namespace, true))
}

func (c *controller) DestroyPreviousActiveEnvironment(teamName, namespace string) error {
	return c.destroyNamespace(teamName, withTeamPreviousActiveNamespaceStatus(namespace, true))
}

func (c *controller) destroyNamespace(teamName string, teamNsOpt TeamNamespaceStatusOption) error {
	teamComp := &s2hv1beta1.Team{}
	if err := c.getTeam(teamName, teamComp); err != nil {
		if k8serrors.IsNotFound(err) {
			return nil
		}

		return err
	}

	return c.destroyNamespaces(teamComp, teamNsOpt)
}

func (c *controller) destroyNamespaces(teamComp *s2hv1beta1.Team, teamNsOpts ...TeamNamespaceStatusOption) error {
	ctx := context.TODO()
	for _, teamNsOpt := range teamNsOpts {
		namespace, nsConditionType := teamNsOpt(teamComp)
		if namespace == "" {
			teamComp.Status.SetCondition(
				nsConditionType,
				corev1.ConditionFalse,
				"there is no namespace to destroy")

			if err := c.updateTeamNamespacesStatus(teamComp, teamNsOpt); err != nil {
				return errors.Wrap(err, "cannot update team conditions when no namespace")
			}

			continue
		}

		namespaceObj := corev1.Namespace{}
		err := c.client.Get(ctx, types.NamespacedName{Name: namespace}, &namespaceObj)
		if err != nil && k8serrors.IsNotFound(err) {
			logger.Debug(fmt.Sprintf("%s namespace does not exist", namespace))
			teamComp.Status.SetCondition(
				nsConditionType,
				corev1.ConditionFalse,
				fmt.Sprintf("%s namespace is destroyed", namespace))

			if err := c.updateTeamNamespacesStatus(teamComp, teamNsOpt); err != nil {
				return errors.Wrap(err, "cannot update team conditions when destroy namespace success")
			}
			continue
		}

		if err != nil {
			return errors.Wrap(err, "cannot get namespaceObj")
		}

		err = c.client.Delete(ctx, &namespaceObj)
		if err != nil && k8serrors.IsConflict(err) {
			logger.Debug(fmt.Sprintf("%s namespace is destroying", namespace))
			continue
		}

		if err != nil {
			return errors.Wrap(err, "cannot destroy namespace")
		}

		logger.Debug(fmt.Sprintf("%s namespace is started to destroy", namespace))

		return errors.ErrTeamNamespaceStillExists
	}

	return nil
}

func (c *controller) SetPreviousActiveNamespace(teamComp *s2hv1beta1.Team, namespace string) error {
	msg := fmt.Sprintf("%s namespace is switched to previous active", namespace)
	cond := corev1.ConditionTrue
	if namespace == "" {
		msg = "previous active namespace is reset"
		cond = corev1.ConditionFalse
	}

	teamComp.Status.SetCondition(
		s2hv1beta1.TeamNamespacePreviousActiveCreated,
		cond,
		msg)

	return c.updateTeamNamespacesStatus(teamComp, withTeamPreviousActiveNamespaceStatus(namespace))
}

func (c *controller) SetPreActiveNamespace(teamComp *s2hv1beta1.Team, namespace string) error {
	msg := fmt.Sprintf("%s namespace is switched to pre-active", namespace)
	cond := corev1.ConditionTrue
	if namespace == "" {
		msg = "pre-active namespace is reset"
		cond = corev1.ConditionFalse
	}

	teamComp.Status.SetCondition(
		s2hv1beta1.TeamNamespacePreActiveCreated,
		cond,
		msg)

	return c.updateTeamNamespacesStatus(teamComp, withTeamPreActiveNamespaceStatus(namespace))
}

func (c *controller) SetActiveNamespace(teamComp *s2hv1beta1.Team, namespace string) error {
	msg := fmt.Sprintf("%s namespace is switched to active", namespace)
	cond := corev1.ConditionTrue
	if namespace == "" {
		msg = "active namespace is reset"
		cond = corev1.ConditionFalse
	}

	teamComp.Status.SetCondition(
		s2hv1beta1.TeamNamespaceActiveCreated,
		cond,
		msg)

	return c.updateTeamNamespacesStatus(teamComp, withTeamActiveNamespaceStatus(namespace))
}

func (c *controller) updateTeamNamespacesStatus(teamComp *s2hv1beta1.Team, teamNsOpts ...TeamNamespaceStatusOption) error {
	for _, teamNsOpt := range teamNsOpts {
		teamNsOpt(teamComp)
	}

	return c.updateTeam(teamComp)
}

func (c *controller) LoadTeamSecret(teamComp *s2hv1beta1.Team) error {
	s2hSecret := corev1.Secret{}
	secretName := teamComp.Spec.Credential.SecretName
	if secretName == "" {
		return nil
	}

	err := c.client.Get(context.TODO(), types.NamespacedName{Name: secretName, Namespace: c.namespace}, &s2hSecret)
	if err != nil && k8serrors.IsNotFound(err) {
		return errors.Wrapf(err, "cannot find %s secret in %s namespace", secretName, c.namespace)
	}

	gitCred := teamComp.Spec.Credential.Git
	if gitCred != nil {
		gitUsername := gitCred.UsernameRef
		teamComp.Spec.Credential.Git.Username = string(s2hSecret.Data[gitUsername.Key])

		gitPassword := gitCred.PasswordRef
		teamComp.Spec.Credential.Git.Password = string(s2hSecret.Data[gitPassword.Key])
	}

	tcCred := teamComp.Spec.Credential.Teamcity
	if tcCred != nil {
		tcUsername := tcCred.UsernameRef
		teamComp.Spec.Credential.Teamcity.Username = string(s2hSecret.Data[tcUsername.Key])

		tcPassword := tcCred.PasswordRef
		teamComp.Spec.Credential.Teamcity.Password = string(s2hSecret.Data[tcPassword.Key])
	}

	return nil
}

func (c *controller) loadGitStorage(teamComp *s2hv1beta1.Team) error {
	teamName := teamComp.GetName()
	teamGitStorage := teamComp.Spec.GitStorage
	teamGitCredential := teamComp.Spec.Credential.Git
	configMgr, ok := c.GetTeamConfigManager(teamName)
	if !ok || configMgr.HasGitChanges(teamGitStorage) {
		var err error
		configMgr, err = s2hconfig.NewWithGit(teamName, teamGitStorage, teamGitCredential)
		if err != nil {
			return err
		}
	}

	c.muTeamConfigs.Lock()
	defer c.muTeamConfigs.Unlock()
	c.teamConfigs[teamName] = configMgr

	return nil
}

func (c *controller) getTeamNamesFromConfig() []string {
	c.muTeamConfigs.Lock()
	defer c.muTeamConfigs.Unlock()
	var names []string
	for teamName := range c.teamConfigs {
		names = append(names, teamName)
	}
	return names
}

func (c *controller) deleteTeamConfig(teamName string) {
	c.muTeamConfigs.Lock()
	defer c.muTeamConfigs.Unlock()
	delete(c.teamConfigs, teamName)
}

func (c *controller) GetTeam(teamName string, teamComp *s2hv1beta1.Team) error {
	return c.getTeam(teamName, teamComp)
}

func (c *controller) GetConnections(namespace string) (map[string][]internal.Connection, error) {
	var err error
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()
	nodes := corev1.NodeList{}
	err = c.client.List(ctx, &nodes, &client.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "cannot list nodes")
	}
	services := corev1.ServiceList{}
	err = c.client.List(ctx, &services, &client.ListOptions{Namespace: namespace})
	if err != nil {
		return nil, errors.Wrap(err, "cannot list services")
	}
	ingresses := v1beta1.IngressList{}
	err = c.client.List(ctx, &ingresses, &client.ListOptions{Namespace: namespace})
	if err != nil {
		return nil, errors.Wrap(err, "cannot list ingresses")
	}
	data := map[string][]internal.Connection{}
	servicesAndPorts := map[string]*corev1.ServicePort{}

	for _, svc := range services.Items {
		if svc.Spec.Type != corev1.ServiceTypeNodePort {
			continue
		}

		nodeIP := getNodeIP(&nodes)

		for i, port := range svc.Spec.Ports {
			servicesAndPorts[fmt.Sprintf("%s,%d", svc.Name, port.Port)] = &svc.Spec.Ports[i]
			servicesAndPorts[fmt.Sprintf("%s,%s", svc.Name, port.Name)] = &svc.Spec.Ports[i]
			data[svc.Name] = append(data[svc.Name], internal.Connection{
				Name:          port.Name,
				IP:            nodeIP,
				Port:          strconv.Itoa(int(port.NodePort)),
				Type:          "NodePort",
				ServicePort:   strconv.Itoa(int(port.Port)),
				ContainerPort: port.TargetPort.String(),
			})
		}
	}

	httpsHosts := map[string]struct{}{}

	for _, ing := range ingresses.Items {
		for _, tlsHosts := range ing.Spec.TLS {
			for _, host := range tlsHosts.Hosts {
				httpsHosts[host] = struct{}{}
			}
		}

		for _, rule := range ing.Spec.Rules {
			proto := "http://"
			if _, ok := httpsHosts[rule.Host]; ok {
				proto = "https://"
			}

			var port *corev1.ServicePort
			// find match service
			for _, path := range rule.HTTP.Paths {
				key := fmt.Sprintf("%s,%s", path.Backend.ServiceName, path.Backend.ServicePort.String())
				if _, ok := servicesAndPorts[key]; ok {
					port = servicesAndPorts[key]
					break
				}
			}
			conn := internal.Connection{
				Name: ing.Name,
				URL:  proto + rule.Host,
				Type: "Ingress",
			}
			if port != nil {
				conn.Name = port.Name
				conn.ServicePort = strconv.Itoa(int(port.Port))
				conn.ContainerPort = port.TargetPort.String()
			}
			data[ing.Name] = append(data[ing.Name], conn)
		}
	}
	return data, nil
}

func (c *controller) GetTeams() (v *s2hv1beta1.TeamList, err error) {
	v = &s2hv1beta1.TeamList{}
	err = c.client.List(context.TODO(), v, &client.ListOptions{})
	return v, errors.Wrap(err, "cannot list teams")
}

func (c *controller) GetQueueHistories(namespace string) (v *s2hv1beta1.QueueHistoryList, err error) {
	v = &s2hv1beta1.QueueHistoryList{}
	err = c.client.List(context.TODO(), v, &client.ListOptions{Namespace: namespace})
	return v, errors.Wrap(err, "cannot list queue histories")
}

func (c *controller) GetQueueHistory(name, namespace string) (v *s2hv1beta1.QueueHistory, err error) {
	v = &s2hv1beta1.QueueHistory{}
	err = c.client.Get(context.TODO(), client.ObjectKey{Namespace: namespace, Name: name}, v)
	return
}

func (c *controller) GetQueues(namespace string) (v *s2hv1beta1.QueueList, err error) {
	v = &s2hv1beta1.QueueList{}
	err = c.client.List(context.TODO(), v, &client.ListOptions{Namespace: namespace})
	return v, errors.Wrap(err, "cannot list queues")
}

func (c *controller) GetStableValues(team *s2hv1beta1.Team, comp *internal.Component) (internal.ComponentValues, error) {
	// TODO: can get stable components map from team.status
	stableComps, err := valuesutil.GetStableComponentsMap(c.client, team.Status.Namespace.Staging)
	if err != nil {
		logger.Error(err, "get stable components map")
		return nil, err
	}
	cfgMgr := c.teamConfigs[team.Name]
	return valuesutil.GenStableComponentValues(comp, stableComps, cfgMgr.Get().Envs["base"][comp.Name]), nil
}

func (c *controller) GetActivePromotions() (v *s2hv1beta1.ActivePromotionList, err error) {
	v = &s2hv1beta1.ActivePromotionList{}
	err = c.client.List(context.TODO(), v, &client.ListOptions{})
	return
}

func (c *controller) GetActivePromotion(name string) (v *s2hv1beta1.ActivePromotion, err error) {
	v = &s2hv1beta1.ActivePromotion{}
	err = c.client.Get(context.TODO(), client.ObjectKey{Name: name}, v)
	return
}

func (c *controller) GetActivePromotionHistories(selectors map[string]string) (v *s2hv1beta1.ActivePromotionHistoryList, err error) {
	v = &s2hv1beta1.ActivePromotionHistoryList{}
	listOpt := &client.ListOptions{LabelSelector: labels.SelectorFromSet(selectors)}
	err = c.client.List(context.TODO(), v, listOpt)
	return
}

func (c *controller) GetActivePromotionHistory(name string) (v *s2hv1beta1.ActivePromotionHistory, err error) {
	v = &s2hv1beta1.ActivePromotionHistory{}
	err = c.client.Get(context.TODO(), client.ObjectKey{Name: name}, v)
	return
}

func getNodeIP(nodes *corev1.NodeList) string {
	i := rand.IntnRange(0, len(nodes.Items))
	hostName := ""
	externalIP := ""
	internalIP := ""
	for _, addr := range nodes.Items[i].Status.Addresses {
		switch addr.Type {
		case corev1.NodeInternalIP:
			internalIP = addr.Address
		case corev1.NodeExternalIP:
			externalIP = addr.Address
		case corev1.NodeHostName:
			hostName = addr.Address
		}
	}
	if internalIP != "" {
		return internalIP
	} else if externalIP != "" {
		return externalIP
	}
	return hostName
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := crctrl.New(CtrlName, mgr, crctrl.Options{Reconciler: r, MaxConcurrentReconciles: MaxReconcileConcurrent})
	if err != nil {
		return err
	}

	// Watching changes of Team
	err = c.Watch(&source.Kind{Type: &s2hv1beta1.Team{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watching changes of namespace belongs to Team
	err = c.Watch(&source.Kind{Type: &corev1.Namespace{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &s2hv1beta1.Team{},
	})
	if err != nil {
		return err
	}

	// Watching changes of deployment belongs to Team
	err = c.Watch(&source.Kind{Type: &appsv1.Deployment{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &s2hv1beta1.Team{},
	})
	if err != nil {
		return err
	}

	// Watching changes of service belongs to Team
	err = c.Watch(&source.Kind{Type: &corev1.Service{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &s2hv1beta1.Team{},
	})
	if err != nil {
		return err
	}

	// Watching changes of secret belongs to Team
	err = c.Watch(&source.Kind{Type: &corev1.Secret{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &s2hv1beta1.Team{},
	})
	if err != nil {
		return err
	}

	return nil
}

func (c *controller) getTeam(teamName string, teamComp *s2hv1beta1.Team) (err error) {
	return c.client.Get(context.TODO(), types.NamespacedName{Name: teamName}, teamComp)
}

func (c *controller) updateTeam(teamComp *s2hv1beta1.Team) error {
	if err := c.client.Update(context.TODO(), teamComp); err != nil {
		return errors.Wrap(err, "cannot update team")
	}

	return nil
}

func (c *controller) addFinalizer(teamComp *s2hv1beta1.Team) error {
	// The object is not being deleted, so if it does not have our finalizer,
	// then lets add the finalizer and update the object.
	if !stringutils.ContainsString(teamComp.ObjectMeta.Finalizers, teamFinalizerName) {
		teamComp.ObjectMeta.Finalizers = append(teamComp.ObjectMeta.Finalizers, teamFinalizerName)

		if err := c.updateTeam(teamComp); err != nil {
			return err
		}
	}

	return nil
}

func (c *controller) deleteFinalizer(teamComp *s2hv1beta1.Team) error {
	if stringutils.ContainsString(teamComp.ObjectMeta.Finalizers, teamFinalizerName) {
		teamNs := getAllTeamNamespaces(teamComp, true)
		if err := c.destroyNamespaces(teamComp, teamNs...); err != nil {
			return err
		}

		configMgr, _ := c.GetTeamConfigManager(teamComp.GetName())
		if configMgr != nil {
			if err := configMgr.Clean(); err != nil {
				logger.Error(err, "cannot remove team git directory")
			}
		}

		c.deleteTeamConfig(teamComp.Name)

		// remove our finalizer from the list and update it.
		teamComp.ObjectMeta.Finalizers = stringutils.RemoveString(teamComp.ObjectMeta.Finalizers, teamFinalizerName)
		if err := c.updateTeam(teamComp); err != nil {
			return err
		}

		// Add metric teamname
		exporter.SetTeamNameMetric(c.teamConfigs)
	}

	return nil
}

// Reconcile reads that state of the cluster for a Team object and makes changes based on the state read
// and what is in the Team.Spec
// +kubebuilder:rbac:groups=,resources=nodes,verbs=get;list;watch
// +kubebuilder:rbac:groups=,resources=nodes/status,verbs=get
// +kubebuilder:rbac:groups=,resources=services,verbs=get;list;watch
// +kubebuilder:rbac:groups=,resources=services/status,verbs=get
// +kubebuilder:rbac:groups=extensions,resources=ingresses,verbs=get;list;watch
// +kubebuilder:rbac:groups=extensions,resources=ingresses/status,verbs=get
// +kubebuilder:rbac:groups=env.samsahai.io,resources=teams,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=env.samsahai.io,resources=teams/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=env.samsahai.io,resources=activepromotions,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=env.samsahai.io,resources=activepromotions/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=env.samsahai.io,resources=activepromotionhistories,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=env.samsahai.io,resources=activepromotionhistories/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=env.samsahai.io,resources=desiredcomponents,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=env.samsahai.io,resources=desiredcomponents/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=env.samsahai.io,resources=queues,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=env.samsahai.io,resources=queues/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=env.samsahai.io,resources=queuehistories,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=env.samsahai.io,resources=queuehistories/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=env.samsahai.io,resources=stablecomponents,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=env.samsahai.io,resources=stablecomponents/status,verbs=get;update;patch
func (c *controller) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	ctx := context.TODO()
	teamComp := &s2hv1beta1.Team{}
	err := c.client.Get(ctx, types.NamespacedName{Name: req.NamespacedName.Name}, teamComp)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			// Object not found, return. Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// The object is being deleted
	if !teamComp.ObjectMeta.DeletionTimestamp.IsZero() {
		if err := c.deleteFinalizer(teamComp); err != nil {
			if errors.IsNamespaceStillExists(err) {
				return reconcile.Result{
					Requeue:      true,
					RequeueAfter: 2 * time.Second,
				}, nil
			}

			return reconcile.Result{}, err
		}

		return reconcile.Result{}, nil
	}

	if err := c.addFinalizer(teamComp); err != nil {
		return reconcile.Result{}, err
	}

	teamName := teamComp.GetName()
	if err := c.CreateStagingEnvironment(teamName, internal.AppPrefix+teamName); err != nil {
		if errors.IsNamespaceStillCreating(err) {
			return reconcile.Result{
				Requeue:      true,
				RequeueAfter: 2 * time.Second,
			}, nil
		}

		return reconcile.Result{}, err
	}

	if err := c.LoadTeamSecret(teamComp); err != nil {
		return reconcile.Result{}, err
	}

	if err := c.loadGitStorage(teamComp); err != nil {
		if errors.IsErrGitCloning(err) {
			return reconcile.Result{
				Requeue:      true,
				RequeueAfter: 2 * time.Second,
			}, nil
		}

		teamComp.Status.SetCondition(
			s2hv1beta1.TeamGitCheckoutUpToDate,
			corev1.ConditionFalse,
			err.Error())

		if err := c.updateTeam(teamComp); err != nil {
			return reconcile.Result{}, errors.Wrap(err,
				"cannot update team conditions when git checkout failure")
		}

		// need to stop because we need correct configuration from git
		return reconcile.Result{}, nil
	}

	if !teamComp.Status.IsConditionTrue(s2hv1beta1.TeamGitCheckoutUpToDate) {
		teamConfigs, _ := c.GetTeamConfigManager(teamName)
		gitRevision := teamConfigs.GetGitLatestRevision()
		msg := fmt.Sprintf("with revision %s", gitRevision)

		teamComp.Status.SetCondition(
			s2hv1beta1.TeamGitCheckoutUpToDate,
			corev1.ConditionTrue,
			fmt.Sprintf("git cloning is success %s", msg))

		if err := c.updateTeam(teamComp); err != nil {
			return reconcile.Result{}, errors.Wrap(err, "cannot update team conditions when git checkout success")
		}
	}

	// add metric teamname
	exporter.SetTeamNameMetric(c.teamConfigs)

	// Our finalizer has finished, so the reconciler can do nothing.
	return reconcile.Result{}, nil
}
