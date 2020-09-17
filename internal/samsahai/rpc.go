package samsahai

import (
	"context"
	"crypto/subtle"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	s2hv1beta1 "github.com/agoda-com/samsahai/api/v1beta1"
	s2h "github.com/agoda-com/samsahai/internal"
	s2herrors "github.com/agoda-com/samsahai/internal/errors"
	"github.com/agoda-com/samsahai/internal/samsahai/exporter"
	"github.com/agoda-com/samsahai/internal/util/template"
	"github.com/agoda-com/samsahai/pkg/samsahai/rpc"
)

func (c *controller) authenticateRPC(ctx context.Context) error {
	authToken, ok := ctx.Value(s2h.HTTPHeader(s2h.SamsahaiAuthHeader)).(string)
	if !ok {
		return s2herrors.ErrAuthTokenNotFound
	}
	isMatch := subtle.ConstantTimeCompare([]byte(authToken), []byte(c.configs.SamsahaiCredential.InternalAuthToken))
	if isMatch != 1 {
		return s2herrors.ErrUnauthorized
	}
	return nil
}

func (c *controller) GetMissingVersions(ctx context.Context, teamInfo *rpc.TeamWithCurrentComponent) (*rpc.ImageList, error) {
	if err := c.authenticateRPC(ctx); err != nil {
		return nil, err
	}

	teamComp := &s2hv1beta1.Team{}
	if err := c.getTeam(teamInfo.TeamName, teamComp); err != nil {
		return nil, errors.Wrapf(err, "cannot get of team %s", teamComp.Name)
	}

	stableList := &s2hv1beta1.StableComponentList{}

	if teamComp.Status.Namespace.Staging == "" {
		return nil, errors.Wrap(fmt.Errorf("staging namespace of %s is empty", teamInfo.TeamName),
			"staging namespace should not be empty")
	}

	err := c.client.List(ctx, stableList, &client.ListOptions{Namespace: teamComp.Status.Namespace.Staging})
	if err != nil && !k8serrors.IsNotFound(err) {
		return nil, errors.Wrapf(err,
			"cannot get list of stable components, namespace %s", teamComp.Status.Namespace.Staging)
	}

	imgList := &rpc.ImageList{}
	comps, err := c.GetConfigController().GetComponents(teamComp.Name)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot get components of team %s", teamComp.Name)
	}

	// get image missing of stable components
	for _, stable := range stableList.Items {
		source, ok := c.getImageSource(comps, stable.Name)
		if !ok {
			continue
		}

		// ignore current component
		isFound := false
		for _, qComp := range teamInfo.Components {
			if qComp.Name == stable.Name {
				isFound = true
				break
			}
		}
		if isFound {
			continue
		}

		c.detectAndAddImageMissing(*source, stable.Spec.Repository, stable.Name, stable.Spec.Version, imgList)
	}

	// get image missing of current components
	for _, qComp := range teamInfo.Components {
		source, ok := c.getImageSource(comps, qComp.Name)
		if ok {
			c.detectAndAddImageMissing(*source, qComp.Image.Repository, qComp.Name, qComp.Image.Tag, imgList)
		}
	}

	return imgList, nil
}

func (c *controller) detectAndAddImageMissing(source s2hv1beta1.UpdatingSource, repo, name, version string, imgList *rpc.ImageList) {
	checker, err := c.getComponentChecker(string(source))
	if err != nil {
		logger.Error(err, "cannot get component checker", "source", string(source))
		return
	}

	if err := checker.EnsureVersion(repo, name, version); err != nil {
		if s2herrors.IsImageNotFound(err) || s2herrors.IsErrRequestTimeout(err) {
			imgList.Images = append(imgList.Images, &rpc.Image{
				Repository: repo,
				Tag:        version,
			})
			return
		}
		logger.Error(err, "cannot ensure version",
			"name", name, "source", source, "repository", repo, "version", version)
	}
}

func (c *controller) getImageSource(comps map[string]*s2hv1beta1.Component, name string) (*s2hv1beta1.UpdatingSource, bool) {
	if _, ok := comps[name]; !ok {
		return nil, false
	}

	source := comps[name].Source
	if source == nil {
		return nil, false
	}
	if _, ok := c.checkers[string(*source)]; !ok {
		// ignore non-existing source
		return nil, false
	}

	return source, true
}

func (c *controller) RunPostComponentUpgrade(ctx context.Context, comp *rpc.ComponentUpgrade) (*rpc.Empty, error) {
	if err := c.authenticateRPC(ctx); err != nil {
		return nil, err
	}

	queueHist := &s2hv1beta1.QueueHistory{}
	if err := c.getQueueHistory(comp.QueueHistoryName, comp.Namespace, queueHist); err != nil {
		return nil, errors.Wrapf(err,
			"cannot get queue history, name: %s, namespace: %s", comp.QueueHistoryName, comp.Namespace)
	}

	if err := c.sendComponentUpgradeReport(queueHist, comp); err != nil {
		return nil, err
	}

	// Add metric updateQueueMetric & histories
	queue := &s2hv1beta1.Queue{}
	err := c.client.Get(context.TODO(), types.NamespacedName{
		Namespace: comp.GetNamespace(),
		Name:      comp.GetName()}, queue)
	if err != nil {
		if !k8serrors.IsNotFound(err) {
			logger.Error(err, "cannot get the queue")
		}
		return &rpc.Empty{}, nil
	}
	exporter.SetQueueMetric(queue)

	return &rpc.Empty{}, nil
}

func (c *controller) sendComponentUpgradeReport(queueHist *s2hv1beta1.QueueHistory, comp *rpc.ComponentUpgrade) error {
	configCtrl := c.GetConfigController()

	for _, reporter := range c.reporters {
		qHist := queueHist
		// in case of reverify, history will be the latest failure queue
		if comp.IsReverify && comp.IssueType == rpc.ComponentUpgrade_IssueType_DESIRED_VERSION_FAILED {
			var err error
			qHist, err = c.getLatestFailureQueueHistory(comp)
			if err != nil {
				return err
			}
		}

		testRunner := s2hv1beta1.TestRunner{}
		if qHist.Spec.Queue != nil {
			testRunner = qHist.Spec.Queue.Status.TestRunner
		}

		upgradeComp := s2h.NewComponentUpgradeReporter(
			comp,
			c.configs,
			s2h.WithTestRunner(testRunner),
			s2h.WithQueueHistoryName(qHist.Name),
		)

		if err := reporter.SendComponentUpgrade(configCtrl, upgradeComp); err != nil {
			logger.Error(err, "cannot send component upgrade failure report",
				"team", comp.TeamName)
		}
	}

	return nil
}

func (c *controller) getQueueHistory(queueHistName, ns string, queueHist *s2hv1beta1.QueueHistory) error {
	return c.client.Get(context.TODO(), types.NamespacedName{Name: queueHistName, Namespace: ns}, queueHist)
}

func (c *controller) getLatestFailureQueueHistory(comp *rpc.ComponentUpgrade) (*s2hv1beta1.QueueHistory, error) {
	qLabels := s2h.GetDefaultLabels(comp.TeamName)
	qLabels["app"] = comp.Name
	qHists, err := c.listQueueHistory(qLabels)
	if err != nil {
		return &s2hv1beta1.QueueHistory{}, errors.Wrapf(err,
			"cannot list queue history, labels: %+v, namespace: %s", qLabels, comp.Namespace)
	}

	qHist := &s2hv1beta1.QueueHistory{}
	if len(qHists.Items) > 1 {
		qHist = &qHists.Items[1]
	}
	return qHist, nil
}

func (c *controller) listQueueHistory(selectors map[string]string) (*s2hv1beta1.QueueHistoryList, error) {
	queueHists := &s2hv1beta1.QueueHistoryList{}
	listOpt := &client.ListOptions{LabelSelector: labels.SelectorFromSet(selectors)}
	err := c.client.List(context.TODO(), queueHists, listOpt)
	queueHists.SortDESC()
	return queueHists, err
}

func (c *controller) SendUpdateStateQueueMetric(ctx context.Context, comp *rpc.ComponentUpgrade) (*rpc.Empty, error) {
	if err := c.authenticateRPC(ctx); err != nil {
		return nil, err
	}

	queueName := comp.GetName()
	if queueName != "" {
		queue := &s2hv1beta1.Queue{}
		err := c.client.Get(context.TODO(), types.NamespacedName{
			Namespace: comp.GetNamespace(),
			Name:      queueName}, queue)
		if err != nil {
			if !k8serrors.IsNotFound(err) {
				logger.Error(err, "cannot get the queue")
			}
			return &rpc.Empty{}, nil
		}
		exporter.SetQueueMetric(queue)
	}

	return &rpc.Empty{}, nil
}

func (c *controller) GetBundleName(ctx context.Context, teamWithCompName *rpc.TeamWithComponentName) (
	*rpc.BundleName, error) {

	if err := c.authenticateRPC(ctx); err != nil {
		return nil, err
	}

	bundleName := c.getBundleName(teamWithCompName.ComponentName, teamWithCompName.TeamName)

	return &rpc.BundleName{Name: bundleName}, nil
}

func (c *controller) getBundleName(compName, teamName string) string {
	bundles, _ := c.GetConfigController().GetBundles(teamName)
	for bundleName, comps := range bundles {
		for _, comp := range comps {
			if comp == compName {
				return bundleName
			}
		}
	}

	return ""
}

// GetPullRequestComponentDependencies returns pull request dependencies from configuration
// repository and version are retrieved from active components
func (c *controller) GetPullRequestComponentDependencies(
	ctx context.Context,
	teamWithCompName *rpc.TeamWithComponentName,
) (*rpc.PullRequestDependencies, error) {

	if err := c.authenticateRPC(ctx); err != nil {
		return nil, err
	}

	teamName := teamWithCompName.TeamName
	compName := teamWithCompName.ComponentName
	deps, _ := c.GetConfigController().GetPullRequestComponentDependencies(teamName, compName)

	teamComp := &s2hv1beta1.Team{}
	if err := c.getTeam(teamName, teamComp); err != nil {
		return nil, err
	}

	compDeps := make([]*rpc.Component, 0)
	if len(teamComp.Status.ActiveComponents) > 0 {
		for _, dep := range deps {
			if activeComp, ok := teamComp.Status.ActiveComponents[dep]; ok {
				compDeps = append(compDeps, &rpc.Component{
					Name: dep,
					Image: &rpc.Image{
						Repository: activeComp.Spec.Repository,
						Tag:        activeComp.Spec.Version,
					},
				})
			}

		}
	}

	return &rpc.PullRequestDependencies{Dependencies: compDeps}, nil
}

func (c *controller) GetPriorityQueues(ctx context.Context, teamName *rpc.TeamName) (*rpc.PriorityQueues, error) {
	if err := c.authenticateRPC(ctx); err != nil {
		return nil, err
	}

	queues, _ := c.GetConfigController().GetPriorityQueues(teamName.Name)

	return &rpc.PriorityQueues{Queues: queues}, nil
}

func (c *controller) GetComponentVersion(ctx context.Context, compSource *rpc.ComponentSource) (*rpc.ComponentVersion, error) {
	if err := c.authenticateRPC(ctx); err != nil {
		return nil, err
	}

	source := compSource.Source
	checker, err := c.getComponentChecker(source)
	if err != nil {
		logger.Error(err, "cannot get component checker", "source", compSource.Source)
		return nil, err
	}

	var (
		imgRepository string
		imgTag        string
	)
	if compSource.Image != nil {
		imgRepository = compSource.Image.Repository
		imgTag = compSource.Image.Tag
	}

	// use pattern if tag is not defined
	if imgTag == "" {
		imgTag = compSource.Pattern
	}

	version, err := checker.GetVersion(imgRepository, compSource.ComponentName, imgTag)
	if err != nil {
		switch err.Error() {
		case s2herrors.ErrNoDesiredComponentVersion.Error(), s2herrors.ErrRequestTimeout.Error():
			return nil, fmt.Errorf("%s:%s not found, %v", imgRepository, imgTag, err)
		default:
			return nil, err
		}
	}

	return &rpc.ComponentVersion{Version: version}, nil
}

func (c *controller) GetPullRequestConfig(ctx context.Context, teamName *rpc.TeamName) (*rpc.PullRequestConfig, error) {
	if err := c.authenticateRPC(ctx); err != nil {
		return nil, err
	}

	configCtrl := c.GetConfigController()
	prConfig, err := configCtrl.GetPullRequestConfig(teamName.Name)
	if err != nil {
		return &rpc.PullRequestConfig{},
			errors.Wrapf(err, "cannot get pull request configuration of team: %s", teamName)
	}

	maxRetryTrigger := &c.configs.PullRequest.MaxTriggerRetryCounts
	if prConfig.Trigger.MaxRetry != nil {
		maxRetryTrigger = prConfig.Trigger.MaxRetry
	}

	pollingTimeTrigger := c.configs.PullRequest.TriggerPollingTime
	if prConfig.Trigger.PollingTime.Duration != 0 {
		pollingTimeTrigger = prConfig.Trigger.PollingTime
	}

	maxRetryVerification := &c.configs.PullRequest.MaxVerificationRetryCounts
	if prConfig.MaxRetry != nil {
		maxRetryVerification = prConfig.MaxRetry
	}

	maxHistoryDays := c.configs.PullRequest.MaxHistoryDays
	if prConfig.MaxHistoryDays != 0 {
		maxHistoryDays = prConfig.MaxHistoryDays
	}

	rpcPRConfig := &rpc.PullRequestConfig{
		Parallel:       int32(prConfig.Parallel),
		MaxRetry:       int32(*maxRetryVerification),
		MaxHistoryDays: int32(maxHistoryDays),
		Trigger: &rpc.PullRequestTriggerConfig{
			MaxRetry:    int32(*maxRetryTrigger),
			PollingTime: pollingTimeTrigger.Duration.String(),
		},
	}

	return rpcPRConfig, nil
}

// PullRequestData defines a pull request data for template rendering
type PullRequestData struct {
	PRNumber string
}

func (c *controller) GetPullRequestComponentSource(ctx context.Context, teamWithPR *rpc.TeamWithPullRequest) (*rpc.ComponentSource, error) {
	if err := c.authenticateRPC(ctx); err != nil {
		return nil, err
	}

	configCtrl := c.GetConfigController()

	teamName := teamWithPR.TeamName
	prConfig, err := configCtrl.GetPullRequestConfig(teamWithPR.TeamName)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot get pull request configuration of team: %s", teamName)
	}

	compSource := &rpc.ComponentSource{Image: &rpc.Image{}}

	comps, err := configCtrl.GetComponents(teamName)
	if err != nil {
		return nil, err
	}

	for compName, comp := range comps {
		if compName == teamWithPR.ComponentName {
			if comp.Source != nil {
				compSource.Source = string(*comp.Source)
			}
			compSource.Pattern = comp.Image.Pattern
			compSource.Image.Repository = comp.Image.Repository
			compSource.Image.Tag = comp.Image.Tag
		}
	}

	for _, prComp := range prConfig.Components {
		if prComp.Name == teamWithPR.ComponentName {
			if prComp.Source != nil && *prComp.Source != "" {
				compSource.Source = string(*prComp.Source)
			}

			// resolve pull request number for image tag pattern
			if prComp.Image.Pattern != "" {
				compSource.Pattern = prComp.Image.Pattern
			}

			if prComp.Image.Repository != "" {
				compSource.Image.Repository = prComp.Image.Repository
			}

			if prComp.Image.Tag != "" {
				compSource.Image.Tag = prComp.Image.Tag
			}
		}
	}

	prData := PullRequestData{PRNumber: teamWithPR.PullRequestNumber}
	compSource.Pattern = template.TextRender("PullRequestTagPattern", compSource.Pattern, prData)

	return compSource, nil
}

func (c *controller) DeployActiveServicesIntoPullRequestEnvironment(ctx context.Context, teamWithNS *rpc.TeamWithNamespace) (*rpc.Empty, error) {
	if err := c.authenticateRPC(ctx); err != nil {
		return nil, err
	}

	teamName := teamWithNS.TeamName
	prNamespace := teamWithNS.Namespace
	teamComp := &s2hv1beta1.Team{}
	if err := c.getTeam(teamName, teamComp); err != nil {
		return nil, err
	}

	prSvcList := &corev1.ServiceList{}
	listOpts := &client.ListOptions{Namespace: prNamespace}
	if err := c.client.List(ctx, prSvcList, listOpts); err != nil {
		return nil, err
	}

	activeNs := teamComp.Status.Namespace.Active
	activeSvcList := &corev1.ServiceList{}
	listOpts = &client.ListOptions{Namespace: activeNs}
	if err := c.client.List(ctx, activeSvcList, listOpts); err != nil {
		return nil, err
	}

	diffSvcs := c.getDifferentServices(prSvcList, activeSvcList)
	for _, svc := range diffSvcs {
		svcName := c.replaceServiceFromReleaseName(svc.Name, prNamespace, activeNs)
		newSvc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      svcName,
				Namespace: teamWithNS.Namespace,
			},
			Spec: corev1.ServiceSpec{
				Type:         corev1.ServiceTypeExternalName,
				ExternalName: fmt.Sprintf("%s.%s.svc.%s", svcName, svc.Namespace, c.configs.ClusterDomain),
			},
		}
		if err := c.client.Create(ctx, newSvc); err != nil && !k8serrors.IsAlreadyExists(err) {
			return nil, err
		}
	}

	return &rpc.Empty{}, nil
}

func (c *controller) getDifferentServices(prSvcList, activeSvcList *corev1.ServiceList) []corev1.Service {
	diffSvcs := make([]corev1.Service, 0)
	if activeSvcList == nil {
		return diffSvcs
	}

	if prSvcList.Items == nil {
		return activeSvcList.Items
	}

	if prSvcList != nil {
		for _, activeSvc := range activeSvcList.Items {
			found := false
			for _, prSvc := range prSvcList.Items {
				if prSvc.Name == activeSvc.Name {
					found = true
					break
				}
			}

			if !found {
				diffSvcs = append(diffSvcs, activeSvc)
			}
		}
	}

	return diffSvcs
}

func (c *controller) replaceServiceFromReleaseName(svcName, prNamespace, activeNamespace string) string {
	prReleaseName := s2h.GenReleaseName(prNamespace, "")
	activeReleaseName := s2h.GenReleaseName(activeNamespace, "")

	return strings.ReplaceAll(svcName, activeReleaseName, prReleaseName)
}

func (c *controller) CreatePullRequestEnvironment(ctx context.Context, teamWithPR *rpc.TeamWithPullRequest) (*rpc.Empty, error) {
	if err := c.authenticateRPC(ctx); err != nil {
		return nil, err
	}

	teamName := teamWithPR.TeamName
	namespace := teamWithPR.Namespace
	configCtrl := c.GetConfigController()
	prConfig, err := configCtrl.GetPullRequestConfig(teamName)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot get pull request configuration of team: %s", teamName)
	}

	resources := prConfig.Resources
	for _, comp := range prConfig.Components {
		if comp.Name == teamWithPR.ComponentName {
			if comp.Resources != nil {
				resources = comp.Resources
			}
		}
	}

	return &rpc.Empty{},
		c.createNamespace(teamWithPR.TeamName, withTeamPullRequestNamespaceStatus(namespace, resources))
}

func (c *controller) DestroyPullRequestEnvironment(ctx context.Context, teamWithNS *rpc.TeamWithNamespace) (*rpc.Empty, error) {
	if err := c.authenticateRPC(ctx); err != nil {
		return nil, err
	}

	teamName := teamWithNS.TeamName
	namespace := teamWithNS.Namespace
	err := c.destroyNamespace(teamName, withTeamPullRequestNamespaceStatus(namespace, nil, true))
	if err != nil {
		return nil, err
	}

	if err := c.ensureTeamPullRequestNamespaceUpdated(teamName, namespace); err != nil {
		return nil, err
	}

	return &rpc.Empty{}, nil

}

func (c *controller) ensureTeamPullRequestNamespaceUpdated(teamName, targetNs string) error {
	teamComp := &s2hv1beta1.Team{}
	if err := c.getTeam(teamName, teamComp); err != nil {
		if k8serrors.IsNotFound(err) {
			return nil
		}

		return err
	}

	for _, prNamespace := range teamComp.Status.Namespace.PullRequests {
		if prNamespace == targetNs {
			return s2herrors.ErrTeamNamespaceStillExists

		}
	}

	return nil
}
