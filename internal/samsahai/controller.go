package samsahai

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/imdario/mergo"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/rand"
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
	configctrl "github.com/agoda-com/samsahai/internal/config"
	"github.com/agoda-com/samsahai/internal/errors"
	s2hlog "github.com/agoda-com/samsahai/internal/log"
	"github.com/agoda-com/samsahai/internal/reporter/github"
	"github.com/agoda-com/samsahai/internal/reporter/msteams"
	"github.com/agoda-com/samsahai/internal/reporter/reportermock"
	"github.com/agoda-com/samsahai/internal/reporter/rest"
	"github.com/agoda-com/samsahai/internal/reporter/shell"
	"github.com/agoda-com/samsahai/internal/reporter/slack"
	"github.com/agoda-com/samsahai/internal/samsahai/checker/harbor"
	"github.com/agoda-com/samsahai/internal/samsahai/checker/publicregistry"
	"github.com/agoda-com/samsahai/internal/samsahai/exporter"
	"github.com/agoda-com/samsahai/internal/samsahai/k8sobject"
	"github.com/agoda-com/samsahai/internal/samsahai/plugin"
	"github.com/agoda-com/samsahai/internal/staging/deploy/helm3"
	"github.com/agoda-com/samsahai/internal/staging/deploy/mock"
	"github.com/agoda-com/samsahai/internal/util/cmd"
	"github.com/agoda-com/samsahai/internal/util/stringutils"
	"github.com/agoda-com/samsahai/internal/util/valuesutil"
	"github.com/agoda-com/samsahai/pkg/samsahai/rpc"
)

var logger = s2hlog.S2HLog.WithName("controller")

const (
	CtrlName          = "samsahai-ctrl"
	teamFinalizerName = "team.finalizers.samsahai.io"

	DefaultPluginsDir  = "plugins"
	PromotedBySamsahai = "samsahai"

	// MaxConcurrentProcess represents no. of concurrent process in internal process
	MaxConcurrentProcess = 1

	// MaxReconcileConcurrent represents no. of concurrent process in operator controller
	MaxReconcileConcurrent = 1

	DefaultWorkQueueBaseDelay = 5 * time.Millisecond
	DefaultWorkQueueMaxDelay  = 60 * time.Second
)

type controller struct {
	scheme    *runtime.Scheme
	client    client.Client
	namespace string

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

	configs    internal.SamsahaiConfig
	configCtrl internal.ConfigController
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
		namespace:       ns,
		internalStop:    stop,
		internalStopper: stop,
		queue:           queue,
		checkers:        map[string]internal.DesiredComponentChecker{},
		plugins:         map[string]internal.Plugin{},
		reporters:       map[string]internal.Reporter{},
		configs:         configs,
	}

	c.configCtrl = configctrl.New(mgr, configctrl.WithS2hCtrl(c), configctrl.WithS2hConfig(c.configs))

	if mgr != nil {
		// create runtime client
		c.client = mgr.GetClient()

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

func WithConfigCtrl(configCtrl internal.ConfigController) Option {
	return func(c *controller) {
		c.configCtrl = configCtrl
	}
}

// TODO: be able to override per team from secret
func (c *controller) loadReporters() {
	// init reporters
	cred := c.configs.SamsahaiCredential
	reporters := []internal.Reporter{
		reportermock.New(),
		rest.New(),
		shell.New(),
		github.New(github.WithGithubURL(c.configs.GithubURL), github.WithGithubToken(cred.GithubToken)),
	}

	if cred.SlackToken != "" {
		reporters = append(reporters, slack.New(cred.SlackToken))
	}

	if cred.MSTeams.TenantID != "" && cred.MSTeams.ClientID != "" && cred.MSTeams.ClientSecret != "" &&
		cred.MSTeams.Username != "" && cred.MSTeams.Password != "" {
		reporters = append(reporters, msteams.New(cred.MSTeams.TenantID, cred.MSTeams.ClientID,
			cred.MSTeams.ClientSecret, cred.MSTeams.Username, cred.MSTeams.Password))
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

		if _, err := c.getComponentChecker(p.GetName()); err != nil {
			logger.Warn("duplicate checker", "name", p.GetName(), "file", file)
		}
		c.checkers[p.GetName()] = p
	}
}

func (c *controller) GetConfigController() internal.ConfigController {
	return c.configCtrl
}

func (c *controller) GetPlugins() map[string]internal.Plugin {
	return c.plugins
}

type TeamNamespaceStatusOption func(teamComp *s2hv1beta1.Team) (string, corev1.ResourceList, s2hv1beta1.TeamConditionType)

func withTeamStagingNamespaceStatus(namespace string, isDelete ...bool) TeamNamespaceStatusOption {
	return func(teamComp *s2hv1beta1.Team) (string, corev1.ResourceList, s2hv1beta1.TeamConditionType) {
		teamComp.Status.Namespace.Staging = namespace
		if len(isDelete) > 0 && isDelete[0] {
			teamComp.Status.Namespace.Staging = ""
		}

		return namespace, nil, s2hv1beta1.TeamNamespaceStagingCreated
	}
}

func withTeamPreActiveNamespaceStatus(namespace string, isDelete ...bool) TeamNamespaceStatusOption {
	return func(teamComp *s2hv1beta1.Team) (string, corev1.ResourceList, s2hv1beta1.TeamConditionType) {
		teamComp.Status.Namespace.PreActive = namespace
		if len(isDelete) > 0 && isDelete[0] {
			teamComp.Status.Namespace.PreActive = ""
		}

		return namespace, nil, s2hv1beta1.TeamNamespacePreActiveCreated
	}
}

func withTeamPreviousActiveNamespaceStatus(namespace string, isDelete ...bool) TeamNamespaceStatusOption {
	return func(teamComp *s2hv1beta1.Team) (string, corev1.ResourceList, s2hv1beta1.TeamConditionType) {
		teamComp.Status.Namespace.PreviousActive = namespace
		if len(isDelete) > 0 && isDelete[0] {
			teamComp.Status.Namespace.PreviousActive = ""
		}

		return namespace, nil, s2hv1beta1.TeamNamespacePreviousActiveCreated
	}
}

func withTeamActiveNamespaceStatus(namespace, promotedBy string, isDelete ...bool) TeamNamespaceStatusOption {
	return func(teamComp *s2hv1beta1.Team) (string, corev1.ResourceList, s2hv1beta1.TeamConditionType) {
		teamComp.Status.Namespace.Active = namespace
		if promotedBy != "" {
			teamComp.Status.ActivePromotedBy = promotedBy
		}
		if len(isDelete) > 0 && isDelete[0] {
			teamComp.Status.Namespace.Active = ""
			teamComp.Status.ActivePromotedBy = ""
		}

		return namespace, nil, s2hv1beta1.TeamNamespaceActiveCreated
	}
}

func withTeamPullRequestNamespaceStatus(namespace string, resources corev1.ResourceList, isDelete ...bool) TeamNamespaceStatusOption {
	return func(teamComp *s2hv1beta1.Team) (string, corev1.ResourceList, s2hv1beta1.TeamConditionType) {
		if len(teamComp.Status.Namespace.PullRequests) == 0 {
			teamComp.Status.Namespace.PullRequests = make([]string, 0)
		}

		found := false
		for _, prNamespace := range teamComp.Status.Namespace.PullRequests {
			if prNamespace == namespace {
				found = true
				break
			}
		}

		if !found {
			teamComp.Status.Namespace.PullRequests = append(teamComp.Status.Namespace.PullRequests, namespace)
		}

		if len(isDelete) > 0 && isDelete[0] {
			newPRNamespaces := make([]string, 0)
			for _, prNamespace := range teamComp.Status.Namespace.PullRequests {
				if prNamespace != namespace {
					newPRNamespaces = append(newPRNamespaces, prNamespace)
				}
			}

			teamComp.Status.Namespace.PullRequests = newPRNamespaces
		}

		return namespace, resources, s2hv1beta1.TeamNamespacePullRequestCreated
	}
}

func (c *controller) CreateStagingEnvironment(teamName, namespace string) error {
	return c.createNamespace(teamName, withTeamStagingNamespaceStatus(namespace))
}

func (c *controller) CreatePreActiveEnvironment(teamName, namespace string) error {
	return c.createNamespace(teamName, withTeamPreActiveNamespaceStatus(namespace))
}

func (c *controller) PromoteActiveEnvironment(
	teamComp *s2hv1beta1.Team,
	namespace string,
	promotedBy string,
	comps map[string]s2hv1beta1.StableComponent,
) error {
	preActiveNamespace := teamComp.Status.Namespace.PreActive
	activeNamespace := teamComp.Status.Namespace.Active
	if namespace == preActiveNamespace {
		if err := c.storeActiveComponentsToTeam(teamComp, comps); err != nil {
			return errors.Wrapf(err, "cannot store active components of %s into team %s",
				namespace, teamComp.Name)
		}

		teamNsOpts := []TeamNamespaceStatusOption{
			withTeamActiveNamespaceStatus(preActiveNamespace, promotedBy),
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
		teamComp.Status.SetCondition(
			s2hv1beta1.TeamActiveEnvironmentDeleted,
			corev1.ConditionTrue,
			fmt.Sprintf("%s namespace is exist", activeNamespace))

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

func (c *controller) storeActiveComponentsToTeam(teamComp *s2hv1beta1.Team, comps map[string]s2hv1beta1.StableComponent) error {
	teamComp.Status.SetActiveComponents(comps)
	return nil
}

func (c *controller) createNamespace(teamName string, teamNsOpt TeamNamespaceStatusOption) error {
	teamComp := &s2hv1beta1.Team{}
	if err := c.getTeam(teamName, teamComp); err != nil {
		return err
	}

	namespace, _, nsConditionType := teamNsOpt(teamComp)
	if err := c.createNamespaceByTeam(teamComp, teamNsOpt); err != nil {
		if errors.IsNamespaceStillCreating(err) ||
			errors.IsNewNamespaceEnvObjsCreated(err) ||
			errors.IsNewNamespaceComponentNotified(err) ||
			errors.IsNewNamespacePromotionCreated(err) {
			teamComp.Status.SetCondition(
				nsConditionType,
				corev1.ConditionFalse,
				fmt.Sprintf("%s %s", namespace, err.Error()))
			if err := c.updateTeamNamespacesStatus(teamComp, teamNsOpt); err != nil {
				return errors.Wrap(err, "cannot update team conditions while creating namespace")
			}
		}

		return err
	}

	if !teamComp.Status.IsConditionTrue(nsConditionType) {
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

func (c *controller) createNamespaceByTeam(teamComp *s2hv1beta1.Team, teamNsOpt TeamNamespaceStatusOption) error {
	namespace, resources, nsConditionType := teamNsOpt(teamComp)
	namespaceObj := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}

	ctx := context.TODO()
	if err := c.client.Get(ctx, types.NamespacedName{Name: namespace}, &namespaceObj); err != nil {
		if k8serrors.IsNotFound(err) {
			logger.Debug("start creating namespace", "team", teamComp.Name, "namespace", namespace)
			if nsConditionType == s2hv1beta1.TeamNamespaceStagingCreated {
				if err := controllerutil.SetControllerReference(teamComp, &namespaceObj, c.scheme); err != nil {
					return err
				}

				teamComp.Status.SetCondition(
					s2hv1beta1.TeamFirstNotifyComponentChanged,
					corev1.ConditionFalse,
					"namespace has been being created")

				if c.configs.ActivePromotion.PromoteOnTeamCreation {
					teamComp.Status.SetCondition(
						s2hv1beta1.TeamFirstActivePromotionRun,
						corev1.ConditionFalse,
						"namespace has been being created")
				}
			}

			if err := c.client.Create(ctx, &namespaceObj); err != nil && !k8serrors.IsAlreadyExists(err) {
				return err
			}

			if c.configs.PostNamespaceCreation != nil {
				setPostNamespaceCreationCondition(teamComp, nsConditionType, corev1.ConditionFalse,
					"namespace has been being created")
			}

			return errors.ErrTeamNamespaceStillCreating
		}

		return err
	}

	logger.Debug("start creating s2h environment objects",
		"team", teamComp.Name, "namespace", namespace)
	if err := c.createEnvironmentObjects(teamComp, namespace, resources); err != nil {
		logger.Error(err, "cannot create environment objects",
			"team", teamComp.Name, "namespace", namespace)
		return errors.ErrTeamNamespaceEnvObjsCreated
	}

	if !teamComp.Status.IsConditionTrue(nsConditionType) {
		if c.configs.PostNamespaceCreation != nil {
			postStagingNsNotRun := nsConditionType == s2hv1beta1.TeamNamespaceStagingCreated &&
				!teamComp.Status.IsConditionTrue(s2hv1beta1.TeamPostStagingNamespaceCreationRun)
			postPreActiveNsNotRun := nsConditionType == s2hv1beta1.TeamNamespacePreActiveCreated &&
				!teamComp.Status.IsConditionTrue(s2hv1beta1.TeamPostPreActiveNamespaceCreationRun)

			if postStagingNsNotRun || postPreActiveNsNotRun {
				logger.Debug("start executing command after creating namespace",
					"team", teamComp.Name, "namespace", namespace)
				if err := c.runPostNamespaceCreation(namespace, teamComp); err != nil {
					logger.Error(err, "cannot execute command after creating namespace",
						"namespace", namespace)
				}

				setPostNamespaceCreationCondition(teamComp, nsConditionType, corev1.ConditionTrue,
					"post namespace creation has been executed successfully")
			}
		}
	}

	if nsConditionType == s2hv1beta1.TeamNamespaceStagingCreated {
		if !teamComp.Status.IsConditionTrue(s2hv1beta1.TeamFirstNotifyComponentChanged) {
			logger.Debug("start notifying component", "team", teamComp.Name, "namespace", namespace)
			if err := c.notifyComponentChanged(teamComp.Name); err != nil {
				logger.Error(err, "cannot notify component changed while creating staging namespace",
					"team", teamComp.Name, "namespace", namespace)
				return errors.ErrTeamNamespaceComponentNotified
			}

			teamComp.Status.SetCondition(
				s2hv1beta1.TeamFirstNotifyComponentChanged,
				corev1.ConditionTrue,
				fmt.Sprintf("notified component changed successfully"))
		}

		if c.configs.ActivePromotion.PromoteOnTeamCreation &&
			!teamComp.Status.IsConditionTrue(s2hv1beta1.TeamFirstActivePromotionRun) {

			logger.Debug("start creating active promotion",
				"team", teamComp.Name, "namespace", namespace)
			if err := c.createActivePromotion(teamComp.Name); err != nil {
				logger.Error(err, "cannot create active promotion while creating staging namespace",
					"namespace", namespace)
				return errors.ErrTeamNamespacePromotionCreated
			}

			teamComp.Status.SetCondition(
				s2hv1beta1.TeamFirstActivePromotionRun,
				corev1.ConditionTrue,
				fmt.Sprintf("triggered active promotion successfully"))
		}
	}

	return nil
}

func (c *controller) runPostNamespaceCreation(ns string, team *s2hv1beta1.Team) error {
	cmdAndArgs := c.configs.PostNamespaceCreation.CommandAndArgs

	creationObj := internal.PostNamespaceCreation{
		Namespace: ns,
		Team: s2hv1beta1.Team{
			Spec:   team.Status.Used,
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

func (c *controller) createEnvironmentObjects(teamComp *s2hv1beta1.Team, namespace string, resources corev1.ResourceList) error {
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
		k8sobject.GetClusterRole(teamComp, namespace),
		k8sobject.GetClusterRoleBinding(teamComp, namespace),
		k8sobject.GetSecret(c.scheme, teamComp, namespace, secretKVs...),
	}

	if teamComp.Status.Used.StagingCtrl != nil && !(*teamComp.Status.Used.StagingCtrl).IsDeploy {
		logger.Warn("skip deploying the staging controller deployment")
	} else {
		deploymentObj := k8sobject.GetDeployment(c.scheme, teamComp, namespace, &c.configs)
		k8sObjects = append(k8sObjects, deploymentObj)
	}

	if len(teamComp.Status.Used.Resources) > 0 {
		quotaObj := k8sobject.GetResourceQuota(teamComp, namespace, nil)
		k8sObjects = append(k8sObjects, quotaObj)
	}

	if len(resources) > 0 {
		quotaObj := k8sobject.GetResourceQuota(teamComp, namespace, resources)
		k8sObjects = append(k8sObjects, quotaObj)
	}

	for _, k8sObject := range k8sObjects {
		if err := deployStagingCtrl(c.client, k8sObject); err != nil {
			return errors.Wrap(err, "cannot deploy staging controller")
		}
	}

	return nil
}

func setPostNamespaceCreationCondition(teamComp *s2hv1beta1.Team, nsConditionType s2hv1beta1.TeamConditionType,
	cond corev1.ConditionStatus, message string) {

	switch nsConditionType {
	case s2hv1beta1.TeamNamespaceStagingCreated:
		teamComp.Status.SetCondition(
			s2hv1beta1.TeamPostStagingNamespaceCreationRun,
			cond,
			message)
	case s2hv1beta1.TeamNamespacePreActiveCreated:
		teamComp.Status.SetCondition(
			s2hv1beta1.TeamPostPreActiveNamespaceCreationRun,
			cond,
			message)
	}
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
		teamNsOpts = append(teamNsOpts, withTeamActiveNamespaceStatus(activeNs, "", isDelete))
	}

	pullRequestNs := teamComp.Status.Namespace.PullRequests
	if len(pullRequestNs) > 0 {
		for _, ns := range pullRequestNs {
			teamNsOpts = append(teamNsOpts, withTeamPullRequestNamespaceStatus(ns, nil, isDelete))
		}
	}

	return teamNsOpts
}

func (c *controller) DestroyActiveEnvironment(teamName, namespace string) error {
	return c.destroyNamespace(teamName, withTeamActiveNamespaceStatus(namespace, "", true))
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
		namespace, _, nsConditionType := teamNsOpt(teamComp)
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

		if err := c.destroyAllStableComponents(namespace); err != nil {
			return errors.Wrap(err, "cannot delete all stable components")
		}

		if err := c.destroyClusterRole(namespace); err != nil {
			return errors.Wrap(err, "cannot delete clusterrole")
		}

		if err := c.destroyClusterRoleBinding(namespace); err != nil {
			return errors.Wrap(err, "cannot delete clusterrolebinding")
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

	return c.updateTeamNamespacesStatus(teamComp, withTeamActiveNamespaceStatus(namespace, ""))
}

func (c *controller) updateTeamNamespacesStatus(teamComp *s2hv1beta1.Team, teamNsOpts ...TeamNamespaceStatusOption) error {
	for _, teamNsOpt := range teamNsOpts {
		teamNsOpt(teamComp)
	}

	return c.updateTeam(teamComp)
}

func (c *controller) LoadTeamSecret(teamComp *s2hv1beta1.Team) error {
	s2hSecret := corev1.Secret{}
	secretName := teamComp.Status.Used.Credential.SecretName
	if secretName == "" {
		return nil
	}

	err := c.client.Get(context.TODO(), types.NamespacedName{Name: secretName, Namespace: c.namespace}, &s2hSecret)
	if err != nil && k8serrors.IsNotFound(err) {
		return errors.Wrapf(err, "cannot find %s secret in %s namespace", secretName, c.namespace)
	}

	// currently, has been used only sending reporter not test runner
	tcCred := teamComp.Status.Used.Credential.Teamcity
	if tcCred != nil {
		tcUsername := tcCred.UsernameRef
		teamComp.Status.Used.Credential.Teamcity.Username = string(s2hSecret.Data[tcUsername.Key])

		tcPassword := tcCred.PasswordRef
		teamComp.Status.Used.Credential.Teamcity.Password = string(s2hSecret.Data[tcPassword.Key])
	}

	gitCred := teamComp.Status.Used.Credential.Github
	if gitCred != nil {
		gitToken := gitCred.TokenRef
		teamComp.Status.Used.Credential.Github.Token = string(s2hSecret.Data[gitToken.Key])
	}

	return nil
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

func (c *controller) GetPullRequestQueueHistories(namespace string) (v *s2hv1beta1.PullRequestQueueHistoryList, err error) {
	v = &s2hv1beta1.PullRequestQueueHistoryList{}
	err = c.client.List(context.TODO(), v, &client.ListOptions{Namespace: namespace})
	return v, errors.Wrap(err, "cannot list pull request queue histories")
}

func (c *controller) GetPullRequestQueueHistory(name, namespace string) (v *s2hv1beta1.PullRequestQueueHistory, err error) {
	v = &s2hv1beta1.PullRequestQueueHistory{}
	err = c.client.Get(context.TODO(), client.ObjectKey{Namespace: namespace, Name: name}, v)
	return
}

func (c *controller) GetPullRequestQueues(namespace string) (v *s2hv1beta1.PullRequestQueueList, err error) {
	v = &s2hv1beta1.PullRequestQueueList{}
	err = c.client.List(context.TODO(), v, &client.ListOptions{Namespace: namespace})
	return v, errors.Wrap(err, "cannot list pull request queues")
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

func (c *controller) GetStableValues(team *s2hv1beta1.Team, comp *s2hv1beta1.Component) (s2hv1beta1.ComponentValues, error) {
	// TODO: can get stable components map from team.status
	stableComps, err := valuesutil.GetStableComponentsMap(c.client, team.Status.Namespace.Staging)
	if err != nil {
		logger.Error(err, "get stable components map")
		return nil, err
	}

	configCtrl := c.GetConfigController()
	config, err := configCtrl.Get(team.Name)
	if err != nil {
		return nil, err
	}

	values, err := configctrl.GetEnvComponentValues(&config.Status.Used, comp.Name, team.Name, s2hv1beta1.EnvBase)
	if err != nil {
		logger.Error(err, "cannot get values file",
			"env", s2hv1beta1.EnvBase, "component", comp.Name, "team", team.Name)
		return nil, err
	}

	return valuesutil.GenStableComponentValues(comp, stableComps, values), nil
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

func (c *controller) GetDeployEngine(teamName, ns string) internal.DeployEngine {
	var e string
	configCtrl := c.GetConfigController()
	config, err := configCtrl.Get(teamName)
	if err != nil {
		return mock.New()
	}

	atpConfig := config.Status.Used.ActivePromotion

	if atpConfig == nil || atpConfig.Deployment == nil || atpConfig.Deployment.Engine == nil || *atpConfig.Deployment.Engine == "" {
		e = mock.EngineName
	} else {
		e = *config.Status.Used.ActivePromotion.Deployment.Engine
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

func (c *controller) getComponentChecker(source string) (internal.DesiredComponentChecker, error) {
	checker, ok := c.checkers[source]
	if !ok {
		return nil, fmt.Errorf("component checker source %s not found", source)
	}

	return checker, nil
}

func (c *controller) notifyComponentChanged(teamName string) error {
	configCtrl := c.GetConfigController()
	comps, err := configCtrl.GetComponents(teamName)
	if err != nil {
		logger.Error(err, "cannot get values file")
		return err
	}

	for comp := range comps {
		c.NotifyComponentChanged(comp, "", teamName)
	}

	return nil
}

func (c *controller) createActivePromotion(teamName string) error {
	atp := &s2hv1beta1.ActivePromotion{
		ObjectMeta: metav1.ObjectMeta{
			Name: teamName,
		},
		Spec: s2hv1beta1.ActivePromotionSpec{
			PromotedBy: PromotedBySamsahai,
		},
	}

	if err := c.client.Create(context.TODO(), atp); err != nil && !k8serrors.IsAlreadyExists(err) {
		return err
	}

	return nil
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

func (c *controller) destroyAllStableComponents(namespace string) error {
	ctx := context.TODO()
	if err := c.client.DeleteAllOf(ctx, &s2hv1beta1.StableComponent{}, client.InNamespace(namespace)); err != nil {
		return err
	}

	stableList := &s2hv1beta1.StableComponentList{}
	if err := c.client.List(ctx, stableList, &client.ListOptions{Namespace: namespace}); err != nil {
		logger.Error(err, "cannot list stable components", "namespace", namespace)
		return err
	}

	if len(stableList.Items) > 0 {
		return errors.ErrEnsureStableComponentsDestroyed
	}

	return nil
}

func (c *controller) destroyClusterRole(namespace string) error {
	ctx := context.TODO()

	clusterRoleName := k8sobject.GenClusterRoleName(namespace)
	clusterRole := &rbacv1.ClusterRole{}
	err := c.client.Get(ctx, types.NamespacedName{Name: clusterRoleName}, clusterRole)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrapf(err, "cannot get clusterrole name %s", clusterRoleName)
	}

	if err = c.client.Delete(ctx, clusterRole); err != nil &&
		k8serrors.IsNotFound(err) {
		return nil
	}
	return err
}

func (c *controller) destroyClusterRoleBinding(namespace string) error {
	ctx := context.TODO()

	clusterRoleBindingName := k8sobject.GenClusterRoleName(namespace)
	clusterRoleBinding := &rbacv1.ClusterRoleBinding{}
	err := c.client.Get(ctx, types.NamespacedName{Name: clusterRoleBindingName}, clusterRoleBinding)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrapf(err, "cannot get clusterrole name %s", clusterRoleBindingName)
	}

	if err = c.client.Delete(ctx, clusterRoleBinding); err != nil &&
		k8serrors.IsNotFound(err) {
		return nil
	}
	return err
}

func (c *controller) DeleteTeamActiveEnvironment(teamName, namespace string) error {
	teamComp := &s2hv1beta1.Team{}
	if err := c.getTeam(teamName, teamComp); err != nil {
		if k8serrors.IsNotFound(err) {
			return nil
		}
	}

	teamComp.Status.SetCondition(
		s2hv1beta1.TeamActiveEnvironmentDeleted,
		corev1.ConditionFalse,
		fmt.Sprintf("%s namespace is deleting", namespace))

	if err := c.updateTeam(teamComp); err != nil {
		return errors.Wrap(err, "cannot update team conditions when active environment is deleting")
	}

	configCtrl := c.GetConfigController()

	deployEngine := c.GetDeployEngine(teamName, namespace)

	parentComps, err := configCtrl.GetParentComponents(teamName)
	if err != nil {
		return err
	}

	for compName := range parentComps {
		refName := internal.GenReleaseName(namespace, compName)
		if err := deployEngine.Delete(refName); err != nil &&
			!k8serrors.IsNotFound(err) {
			logger.Error(err, "cannot delete release",
				"refName", refName,
				"namespace", namespace,
				"component", compName)
		}
	}

	if err = c.DestroyActiveEnvironment(teamName, namespace); err != nil &&
		!errors.IsNamespaceStillExists(err) && err != errors.ErrEnsureStableComponentsDestroyed {
		// condition false
		teamComp.Status.SetCondition(
			s2hv1beta1.TeamActiveEnvironmentDeleted,
			corev1.ConditionFalse,
			fmt.Sprintf("%s namespace is deleting", namespace))

		if err := c.updateTeam(teamComp); err != nil {
			logger.Error(err, "cannot update team conditions when active environment is deleting")
			return nil
		}
		//return err
	}

	teamComp = &s2hv1beta1.Team{}
	if err := c.getTeam(teamName, teamComp); err != nil {
		if k8serrors.IsNotFound(err) {
			return nil
		}
	}

	teamComp.Status.Namespace.Active = ""
	teamComp.Status.ActivePromotedBy = ""
	teamComp.Status.SetCondition(
		s2hv1beta1.TeamActiveEnvironmentDeleted,
		corev1.ConditionTrue,
		fmt.Sprintf("%s namespace has been deleted successfully", namespace))
	teamComp.Status.SetCondition(
		s2hv1beta1.TeamNamespaceActiveCreated,
		corev1.ConditionFalse,
		fmt.Sprintf("%s namespace is destroyed", namespace))

	if err := c.updateTeam(teamComp); err != nil {
		logger.Error(err, "cannot update team conditions when delete active environment completed")
	}

	return nil
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

func (c *controller) addFinalizer(teamComp *s2hv1beta1.Team) {
	// The object is not being deleted, so if it does not have our finalizer,
	// then lets add the finalizer and update the object.
	if !stringutils.ContainsString(teamComp.ObjectMeta.Finalizers, teamFinalizerName) {
		teamComp.ObjectMeta.Finalizers = append(teamComp.ObjectMeta.Finalizers, teamFinalizerName)
	}
}

func (c *controller) deleteFinalizer(teamComp *s2hv1beta1.Team) error {
	if stringutils.ContainsString(teamComp.ObjectMeta.Finalizers, teamFinalizerName) {
		teamNs := getAllTeamNamespaces(teamComp, true)
		if err := c.destroyNamespaces(teamComp, teamNs...); err != nil {
			return err
		}

		if err := c.GetConfigController().Delete(teamComp.Name); err != nil {
			return err
		}

		if err := c.ensureConfigDestroyed(teamComp.Name); err != nil {
			return err
		}

		// remove our finalizer from the list and update it.
		teamComp.ObjectMeta.Finalizers = stringutils.RemoveString(teamComp.ObjectMeta.Finalizers, teamFinalizerName)
		if err := c.updateTeam(teamComp); err != nil {
			return err
		}

		// Add metric teamname
		teamList, err := c.GetTeams()
		if err != nil {
			return err
		}
		exporter.SetTeamNameMetric(teamList)
	}

	return nil
}

func (c *controller) ensureConfigDestroyed(configName string) error {
	config := &s2hv1beta1.Config{
		ObjectMeta: metav1.ObjectMeta{
			Name: configName,
		},
	}

	if err := c.client.Get(context.TODO(), types.NamespacedName{Name: configName}, config); err != nil {
		if k8serrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	return errors.ErrEnsureConfigDestroyed
}

func (c *controller) ensureAndUpdateConfig(teamComp *s2hv1beta1.Team) error {
	// ensure config of team is deployed
	config, err := c.configCtrl.Get(teamComp.Name)
	if err != nil {
		logger.Error(err, "cannot get config", "name", teamComp.Name)
		return err
	}

	// set owner references
	if len(config.ObjectMeta.OwnerReferences) == 0 {
		if err := controllerutil.SetControllerReference(teamComp, config, c.scheme); err != nil {
			return err
		}

		if err := c.configCtrl.Update(config); err != nil {
			logger.Error(err, "cannot set controller reference of config", "name", teamComp.Name)
			return err
		}
	}

	return nil
}

func (c *controller) EnsureTeamTemplateChanged(teamComp *s2hv1beta1.Team) error {
	configComp, err := c.configCtrl.Get(teamComp.Name)
	if err != nil {
		logger.Error(err, "cannot get config", "name", teamComp.Name)
		return err
	}

	template := configComp.Spec.Template
	if template != "" && template != teamComp.Name {
		templateObj := &s2hv1beta1.Team{}
		err := c.getTeam(template, templateObj)
		if err != nil {
			logger.Error(err, "team template not found", "template", template)
			return err
		}

		if err = applyTeamTemplate(teamComp, templateObj); err != nil {
			return err
		}

	} else {
		teamComp.Status.Used = teamComp.Spec
	}

	bytesTeamComp, _ := json.Marshal(&teamComp.Status.Used)
	bytesHashID := md5.Sum(bytesTeamComp)
	hashID := fmt.Sprintf("%x", bytesHashID)

	if !teamComp.Status.SyncTemplate {
		teamComp.Status.SyncTemplate = true
	}

	if teamComp.Status.TemplateUID != hashID {

		teamComp.Status.TemplateUID = hashID
		teamComp.Status.SetCondition(
			s2hv1beta1.TeamUsedUpdated,
			corev1.ConditionFalse,
			"need update team")
	}
	return nil
}

func (c *controller) ensureTriggerChildrenTeam(name string) error {
	ctx := context.TODO()
	configs := &s2hv1beta1.ConfigList{}
	if err := c.client.List(ctx, configs, &client.ListOptions{}); err != nil {
		logger.Error(err, "cannot list Configs ")
		return err
	}
	for _, conf := range configs.Items {
		if conf.Spec.Template == name {
			team := &s2hv1beta1.Team{}
			if err := c.getTeam(conf.Name, team); err != nil {
				return err
			}
			team.Status.SyncTemplate = false
			if err := c.updateTeam(team); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *controller) ValidateTeamRequiredField(teamComp *s2hv1beta1.Team) error {
	emptyStagingCtrl := s2hv1beta1.StagingCtrl{}
	if teamComp.Status.Used.StagingCtrl == &emptyStagingCtrl ||
		len(teamComp.Status.Used.Owners) == 0 {

		return errors.New("team used cannot be empty")
	}
	return nil
}

func applyTeamTemplate(teamComp, teamTemplate *s2hv1beta1.Team) error {
	teamComp.Status.Used = teamComp.Spec
	if err := mergo.Merge(&teamComp.Status.Used, teamTemplate.Spec); err != nil {
		return err
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
			if errors.IsNamespaceStillExists(err) || errors.IsEnsuringConfigDestroyed(err) {
				return reconcile.Result{
					Requeue:      true,
					RequeueAfter: 2 * time.Second,
				}, nil
			}

			return reconcile.Result{}, err
		}

		return reconcile.Result{}, nil
	}

	c.addFinalizer(teamComp)

	if err := c.ensureAndUpdateConfig(teamComp); err != nil {
		teamComp.Status.SetCondition(
			s2hv1beta1.TeamConfigExisted,
			corev1.ConditionFalse,
			err.Error())

		if err := c.updateTeam(teamComp); err != nil {
			return reconcile.Result{}, errors.Wrap(err,
				"cannot update team conditions when config does not exist")
		}

		return reconcile.Result{}, err
	}

	if !teamComp.Status.IsConditionTrue(s2hv1beta1.TeamConfigExisted) {
		teamComp.Status.SetCondition(
			s2hv1beta1.TeamConfigExisted,
			corev1.ConditionTrue,
			"Config exists")

		if err := c.updateTeam(teamComp); err != nil {
			return reconcile.Result{}, errors.Wrap(err, "cannot update team conditions when config exists")
		}
	}

	if err := c.EnsureTeamTemplateChanged(teamComp); err != nil {
		return reconcile.Result{}, err
	}

	if !teamComp.Status.IsConditionTrue(s2hv1beta1.TeamUsedUpdated) {
		teamComp.Status.SetCondition(
			s2hv1beta1.TeamUsedUpdated,
			corev1.ConditionTrue,
			"update team template successfully")

		if err := c.updateTeam(teamComp); err != nil {
			return reconcile.Result{}, err
		}
	}

	if err := c.ensureTriggerChildrenTeam(teamComp.Name); err != nil {
		return reconcile.Result{}, err
	}

	if err := c.ValidateTeamRequiredField(teamComp); err != nil {
		teamComp.Status.SetCondition(
			s2hv1beta1.TeamRequiredFieldsValidated,
			corev1.ConditionFalse,
			"invalid required fields")

		if err := c.updateTeam(teamComp); err != nil {
			return reconcile.Result{}, errors.Wrap(err, "cannot update team conditions when require fields is invalid")
		}

		return reconcile.Result{}, err
	}

	if !teamComp.Status.IsConditionTrue(s2hv1beta1.TeamRequiredFieldsValidated) {
		teamComp.Status.SetCondition(
			s2hv1beta1.TeamRequiredFieldsValidated,
			corev1.ConditionTrue,
			"validate required fields successfully")

		if err := c.updateTeam(teamComp); err != nil {
			return reconcile.Result{}, errors.Wrap(err, "cannot update team conditions when require fields is valid")
		}
	}

	if teamComp.Status.Namespace.Active != "" && !teamComp.Status.IsConditionTrue(s2hv1beta1.TeamActiveEnvironmentDeleted) {
		activeNamespace := teamComp.Status.Namespace.Active
		if err := c.DeleteTeamActiveEnvironment(teamComp.Name, activeNamespace); err != nil {
			return reconcile.Result{}, err
		}
	}

	teamName := teamComp.GetName()
	if err := c.CreateStagingEnvironment(teamName, internal.GenStagingNamespace(teamName)); err != nil {
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

	// add metric teamname
	teamList, err := c.GetTeams()
	if err != nil {
		return reconcile.Result{}, err
	}
	exporter.SetTeamNameMetric(teamList)

	// Our finalizer has finished, so the reconciler can do nothing.
	return reconcile.Result{}, nil
}
