package samsahai

import (
	"context"
	"crypto/subtle"
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	s2hv1 "github.com/agoda-com/samsahai/api/v1"
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

func (c *controller) GetTeamActiveNamespace(ctx context.Context, teamName *rpc.TeamName) (*rpc.TeamWithNamespace, error) {
	if err := c.authenticateRPC(ctx); err != nil {
		return nil, err
	}

	teamComp := &s2hv1.Team{}
	if err := c.getTeam(teamName.Name, teamComp); err != nil {
		return nil, errors.Wrapf(err, "cannot get of team %s", teamComp.Name)
	}

	return &rpc.TeamWithNamespace{
		TeamName:  teamName.Name,
		Namespace: teamComp.Status.Namespace.Active,
	}, nil
}

func (c *controller) GetMissingVersions(ctx context.Context, teamInfo *rpc.TeamWithCurrentComponent) (*rpc.ImageList, error) {
	if err := c.authenticateRPC(ctx); err != nil {
		return nil, err
	}

	teamComp := &s2hv1.Team{}
	if err := c.getTeam(teamInfo.TeamName, teamComp); err != nil {
		return nil, errors.Wrapf(err, "cannot get of team %s", teamComp.Name)
	}

	stableList := &s2hv1.StableComponentList{}

	if teamComp.Status.Namespace.Staging == "" {
		return nil, errors.Wrap(fmt.Errorf("staging namespace of %s is empty", teamInfo.TeamName),
			"staging namespace should not be empty")
	}

	err := c.client.List(ctx, stableList, &client.ListOptions{Namespace: teamComp.Status.Namespace.Staging})
	if err != nil && !k8serrors.IsNotFound(err) {
		return nil, errors.Wrapf(err,
			"cannot get list of stable components, namespace %s", teamComp.Status.Namespace.Staging)
	}

	comps, err := c.GetConfigController().GetComponents(teamComp.Name)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot get components of team %s", teamComp.Name)
	}

	return c.getMissingVersions(teamInfo, stableList, comps)
}

func (c *controller) getMissingVersions(teamInfo *rpc.TeamWithCurrentComponent, stableList *s2hv1.StableComponentList,
	comps map[string]*s2hv1.Component) (*rpc.ImageList, error) {

	// detect image missing of all stable components and current components concurrently
	timeout := 300 * time.Second
	missingImagesCh := make(chan []*rpc.Image, 2)

	ctx, cancelFunc := context.WithTimeout(context.Background(), timeout)
	defer cancelFunc()

	// get image missing of stable components
	go func() {
		missingImageCh := make(chan *rpc.Image)
		errCh := make(chan error)
		for _, stable := range stableList.Items {
			go func(stable s2hv1.StableComponent) {
				source, ok := c.getImageSource(comps, stable.Name)
				if !ok {
					errCh <- fmt.Errorf("source of image not found")
					return
				}

				// ignore current component
				for _, qComp := range teamInfo.Components {
					if qComp.Name == stable.Name {
						missingImageCh <- &rpc.Image{}
						return
					}
				}

				missingImage, err := c.detectMissingImage(*source, stable.Spec.Repository, stable.Name, stable.Spec.Version)
				if err != nil {
					errCh <- err
					return
				}
				missingImageCh <- missingImage
			}(stable)
		}

		missingImages := make([]*rpc.Image, 0)
		for i := 0; i < len(stableList.Items); i++ {
			select {

			case missingImage := <-missingImageCh:
				if missingImage.Repository != "" {
					missingImages = append(missingImages, missingImage)
				}
			case err := <-errCh:
				if err != nil {
					logger.Error(err, "cannot detect image missing")
				}
			}
		}
		missingImagesCh <- missingImages
	}()

	// get image missing of current components
	go func() {
		missingImageCh := make(chan *rpc.Image)
		errCh := make(chan error)
		for _, qComp := range teamInfo.Components {
			go func(qComp *rpc.Component) {
				source, ok := c.getImageSource(comps, qComp.Name)
				if !ok {
					errCh <- fmt.Errorf("source of image not found")
					return
				}

				missingImage, err := c.detectMissingImage(*source, qComp.Image.Repository, qComp.Name, qComp.Image.Tag)
				if err != nil {
					errCh <- err
					return
				}
				missingImageCh <- missingImage
			}(qComp)
		}

		missingImages := make([]*rpc.Image, 0)
		for i := 0; i < len(teamInfo.Components); i++ {
			select {
			case missingImage := <-missingImageCh:
				if missingImage.Repository != "" {
					missingImages = append(missingImages, missingImage)
				}
			case err := <-errCh:
				if err != nil {
					logger.Error(err, "cannot detect image missing")
				}
			}
		}
		missingImagesCh <- missingImages
	}()

	imgList := &rpc.ImageList{}
	for i := 0; i < 2; i++ {
		select {
		case <-ctx.Done():
			return nil, errors.Wrapf(s2herrors.ErrRequestTimeout, "detect missing images took longer than %v",
				timeout)
		case missingImages := <-missingImagesCh:
			if len(missingImages) > 0 {
				imgList.Images = append(imgList.Images, missingImages...)
			}
		}
	}

	return imgList, nil
}

func (c *controller) RunPostComponentUpgrade(ctx context.Context, comp *rpc.ComponentUpgrade) (*rpc.Empty, error) {
	if err := c.authenticateRPC(ctx); err != nil {
		return nil, err
	}

	queueHist := &s2hv1.QueueHistory{}
	queueHistName := comp.QueueHistoryName
	namespace := comp.Namespace
	err := c.client.Get(context.TODO(), types.NamespacedName{Name: queueHistName, Namespace: namespace}, queueHist)
	if err != nil {
		return nil, errors.Wrapf(err,
			"cannot get queue history, name: %s, namespace: %s", queueHistName, namespace)
	}

	qHist := queueHist
	// in case of reverify, history will be the latest failure queue
	if comp.IsReverify && comp.IssueType == rpc.ComponentUpgrade_IssueType_DESIRED_VERSION_FAILED {
		var err error
		qHist, err = c.getLatestFailureQueueHistory(comp)
		if err != nil {
			return nil, err
		}
	}

	if err := c.sendDeploymentQueueReport(qHist.Name, qHist.Spec.Queue, comp); err != nil {
		return nil, err
	}

	// Add metric updateQueueMetric & histories
	queue := &s2hv1.Queue{}
	err = c.client.Get(context.TODO(), types.NamespacedName{Name: queueHistName, Namespace: namespace}, queue)
	if err != nil {
		if !k8serrors.IsNotFound(err) {
			logger.Error(err, "cannot get the queue")
		}
		return &rpc.Empty{}, nil
	}
	exporter.SetQueueMetric(queue)

	return &rpc.Empty{}, nil
}

func (c *controller) RunPostPullRequestQueue(ctx context.Context, comp *rpc.ComponentUpgrade) (*rpc.Empty, error) {
	if err := c.authenticateRPC(ctx); err != nil {
		return nil, err
	}

	prQueueHistName := comp.QueueHistoryName
	prQueueHistNamespace := comp.Namespace
	prQueueHist := &s2hv1.PullRequestQueueHistory{}
	err := c.client.Get(context.TODO(), types.NamespacedName{
		Name:      prQueueHistName,
		Namespace: prQueueHistNamespace,
	}, prQueueHist)
	if err != nil {
		return nil, err
	}

	if prQueueHist.Spec.PullRequestQueue != nil {
		deploymentQueue := prQueueHist.Spec.PullRequestQueue.Status.DeploymentQueue
		if err := c.sendDeploymentQueueReport(prQueueHistName, deploymentQueue, comp); err != nil {
			return nil, err
		}
	}

	return &rpc.Empty{}, nil
}

func (c *controller) RunPostPullRequestTrigger(ctx context.Context, prTriggerRPC *rpc.PullRequestTrigger) (
	*rpc.Empty, error) {

	if err := c.authenticateRPC(ctx); err != nil {
		return nil, err
	}

	prTriggerName := prTriggerRPC.Name
	prTriggerNamespace := prTriggerRPC.Namespace
	prTrigger := &s2hv1.PullRequestTrigger{}
	err := c.client.Get(context.TODO(), types.NamespacedName{
		Name:      prTriggerName,
		Namespace: prTriggerNamespace,
	}, prTrigger)
	if err != nil {
		return nil, err
	}

	c.sendPullRequestTriggerReport(prTrigger, prTriggerRPC)

	return &rpc.Empty{}, nil
}

func (c *controller) SendUpdateStateQueueMetric(ctx context.Context, comp *rpc.ComponentUpgrade) (*rpc.Empty, error) {
	if err := c.authenticateRPC(ctx); err != nil {
		return nil, err
	}

	queueName := comp.GetName()
	if queueName != "" {
		queue := &s2hv1.Queue{}
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

func (c *controller) GetBundleName(ctx context.Context,
	teamWithCompName *rpc.TeamWithBundleName) (
	*rpc.BundleName, error) {

	if err := c.authenticateRPC(ctx); err != nil {
		return nil, err
	}

	bundleName := c.getBundleName(teamWithCompName.BundleName, teamWithCompName.TeamName)

	return &rpc.BundleName{Name: bundleName}, nil
}

// GetPullRequestBundleDependencies returns pull request dependencies from configuration
// repository and version are retrieved from active components
func (c *controller) GetPullRequestBundleDependencies(
	ctx context.Context,
	teamWithBundleName *rpc.TeamWithBundleName,
) (*rpc.PullRequestDependencies, error) {

	if err := c.authenticateRPC(ctx); err != nil {
		return nil, err
	}

	teamName := teamWithBundleName.TeamName
	bundleName := teamWithBundleName.BundleName
	deps, _ := c.GetConfigController().GetPullRequestBundleDependencies(teamName, bundleName)

	teamComp := &s2hv1.Team{}
	if err := c.getTeam(teamName, teamComp); err != nil {
		return nil, err
	}

	compDeps := make([]*rpc.Component, 0)
	for _, dep := range deps {
		compDep := &rpc.Component{
			Name:  dep,
			Image: &rpc.Image{},
		}
		if len(teamComp.Status.ActiveComponents) > 0 {
			if activeComp, ok := teamComp.Status.ActiveComponents[dep]; ok {
				if activeComp.Spec.Repository != "" {
					compDep.Image.Repository = activeComp.Spec.Repository
				}
				if activeComp.Spec.Version != "" {
					compDep.Image.Tag = activeComp.Spec.Version
				}
			}
		}

		compDeps = append(compDeps, compDep)
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

	if imgRepository == "" || imgTag == "" {
		return nil, fmt.Errorf("image repository and tag should not be empty, repository: %s, tag: %s",
			imgRepository, imgTag)
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

func (c *controller) GetPullRequestConfig(ctx context.Context, teamWithComp *rpc.TeamWithBundleName) (
	*rpc.PullRequestConfig, error) {

	if err := c.authenticateRPC(ctx); err != nil {
		return nil, err
	}

	configCtrl := c.GetConfigController()
	prConfig, err := configCtrl.GetPullRequestConfig(teamWithComp.TeamName)
	if err != nil {
		return &rpc.PullRequestConfig{}, err
	}

	maxRetryTrigger := &c.configs.PullRequest.MaxTriggerRetryCounts
	if prConfig.Trigger.MaxRetry != nil {
		maxRetryTrigger = prConfig.Trigger.MaxRetry
	}

	pollingTimeTrigger := c.configs.PullRequest.TriggerPollingTime
	if prConfig.Trigger.PollingTime.Duration != 0 {
		pollingTimeTrigger = prConfig.Trigger.PollingTime
	}

	queueConcurrences := c.configs.PullRequest.QueueConcurrences
	if prConfig.Concurrences != 0 {
		queueConcurrences = prConfig.Concurrences
	}

	maxRetryVerification := &c.configs.PullRequest.MaxVerificationRetryCounts
	if prConfig.MaxRetry != nil {
		maxRetryVerification = prConfig.MaxRetry
	}

	var gitRepository string
	for _, bundle := range prConfig.Bundles {
		if bundle.Name == teamWithComp.BundleName {
			gitRepository = bundle.GitRepository
			if bundle.MaxRetry != nil {
				maxRetryVerification = bundle.MaxRetry
			}
			break
		}
	}

	maxHistoryDays := c.configs.PullRequest.MaxHistoryDays
	if prConfig.MaxHistoryDays != 0 {
		maxHistoryDays = prConfig.MaxHistoryDays
	}

	rpcPRConfig := &rpc.PullRequestConfig{
		Concurrences:   int32(queueConcurrences),
		MaxRetry:       int32(*maxRetryVerification),
		MaxHistoryDays: int32(maxHistoryDays),
		Trigger: &rpc.PullRequestTriggerConfig{
			MaxRetry:    int32(*maxRetryTrigger),
			PollingTime: pollingTimeTrigger.Duration.String(),
		},
		GitRepository: gitRepository,
	}

	return rpcPRConfig, nil
}

func (c *controller) GetPullRequestComponentSources(ctx context.Context, teamWithPR *rpc.TeamWithPullRequest) (
	*rpc.ComponentSourceList, error) {

	if err := c.authenticateRPC(ctx); err != nil {
		return nil, err
	}

	configCtrl := c.GetConfigController()

	teamName := teamWithPR.TeamName
	prComps, err := configCtrl.GetPullRequestComponents(teamName, teamWithPR.BundleName, false)
	if err != nil {
		return nil, err
	}

	compSources := make([]*rpc.ComponentSource, 0)
	for _, prComp := range prComps {
		compSource := &rpc.ComponentSource{Image: &rpc.Image{}}
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

		// render image tag
		prData := s2h.PullRequestData{PRNumber: teamWithPR.PRNumber}
		compSource.Pattern = template.TextRender("PullRequestTagPattern", compSource.Pattern, prData)
		if compSource.Image.Tag == "" {
			compSource.Image.Tag = compSource.Pattern
		}

		compSource.ComponentName = prComp.Name
		compSources = append(compSources, compSource)
	}

	comps, err := configCtrl.GetComponents(teamName)
	if err != nil {
		return nil, err
	}

	// fill empty data
	for compName, comp := range comps {
		for _, compSource := range compSources {
			if compName == compSource.ComponentName {
				if compSource.Source == "" && comp.Source != nil {
					compSource.Source = string(*comp.Source)
				}
				if compSource.Pattern == "" {
					compSource.Pattern = comp.Image.Pattern
				}
				if compSource.Image == nil {
					compSource.Image = &rpc.Image{}
				}
				if compSource.Image.Repository == "" {
					compSource.Image.Repository = comp.Image.Repository
				}
				if compSource.Image.Tag == "" {
					compSource.Image.Tag = comp.Image.Tag
				}
			}
		}
	}

	return &rpc.ComponentSourceList{ComponentSources: compSources}, nil
}

func (c *controller) DeployActiveServicesIntoPullRequestEnvironment(ctx context.Context, teamWithNS *rpc.TeamWithNamespace) (*rpc.Empty, error) {
	if err := c.authenticateRPC(ctx); err != nil {
		return nil, err
	}

	teamName := teamWithNS.TeamName
	prNamespace := teamWithNS.Namespace
	teamComp := &s2hv1.Team{}
	if err := c.getTeam(teamName, teamComp); err != nil {
		return nil, err
	}

	activeNs := teamComp.Status.Namespace.Active
	if activeNs == "" {
		return &rpc.Empty{}, nil
	}

	activeSvcList := &corev1.ServiceList{}
	listOpts := &client.ListOptions{Namespace: activeNs}
	if err := c.client.List(ctx, activeSvcList, listOpts); err != nil {
		return nil, err
	}

	prSvcList := &corev1.ServiceList{}
	listOpts = &client.ListOptions{Namespace: prNamespace}
	if err := c.client.List(ctx, prSvcList, listOpts); err != nil {
		return nil, err
	}

	diffSvcs := c.getDifferentServices(prSvcList, activeSvcList)
	for _, svc := range diffSvcs {
		srcSvcName := svc.Name
		srcNamespace := svc.Namespace
		svcName := c.replaceServiceFromReleaseName(srcSvcName, prNamespace, activeNs)
		newSvc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      svcName,
				Namespace: teamWithNS.Namespace,
			},
			Spec: corev1.ServiceSpec{
				Type:         corev1.ServiceTypeExternalName,
				ExternalName: fmt.Sprintf("%s.%s.svc.%s", srcSvcName, srcNamespace, c.configs.ClusterDomain),
			},
		}
		if err := c.client.Create(ctx, newSvc); err != nil && !k8serrors.IsAlreadyExists(err) {
			return nil, err
		}
	}

	return &rpc.Empty{}, nil
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
		return nil, err
	}

	resources := prConfig.Resources
	for _, bundle := range prConfig.Bundles {
		if bundle.Name == teamWithPR.BundleName {
			if bundle.Resources != nil {
				resources = bundle.Resources
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

func (c *controller) detectMissingImage(source s2hv1.UpdatingSource, repo, name, version string) (*rpc.Image, error) {
	checker, err := c.getComponentChecker(string(source))
	if err != nil {
		return &rpc.Image{}, errors.Wrapf(err, "cannot get component checker, source: %s", string(source))
	}

	if repo != "" {
		if err := checker.EnsureVersion(repo, name, version); err != nil {
			if !s2herrors.IsImageNotFound(err) && !s2herrors.IsErrRequestTimeout(err) {
				return &rpc.Image{},
					errors.Wrapf(err, "cannot ensure version, name: %s, source: %s, repository: %s, version: %s",
						name, source, repo, version)

			}

			return &rpc.Image{Repository: repo, Tag: version}, nil
		}
	}

	return &rpc.Image{}, nil
}

func (c *controller) getImageSource(comps map[string]*s2hv1.Component, name string) (*s2hv1.UpdatingSource, bool) {
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

func (c *controller) sendDeploymentQueueReport(queueHistName string, queue *s2hv1.Queue, comp *rpc.ComponentUpgrade) error {
	configCtrl := c.GetConfigController()

	teamComp := &s2hv1.Team{}
	err := c.getTeam(comp.TeamName, teamComp)
	if err != nil {
		return err
	}

	if err := c.LoadTeamSecret(teamComp); err != nil {
		logger.Error(err, "cannot load team secret", "team", teamComp.Name)
		return err
	}

	for _, reporter := range c.reporters {
		testRunner := s2hv1.TestRunner{}
		if queue != nil {
			testRunner = queue.Status.TestRunner
		}

		upgradeComp := s2h.NewComponentUpgradeReporter(
			comp,
			c.configs,
			s2h.WithTestRunner(testRunner),
			s2h.WithQueueHistoryName(queueHistName),
			s2h.WithNamespace(comp.PullRequestNamespace),
			s2h.WithComponentUpgradeOptCredential(teamComp.Status.Used.Credential),
		)

		if comp.PullRequestComponent != nil && comp.PullRequestComponent.PRNumber != "" {
			if err := reporter.SendPullRequestQueue(configCtrl, upgradeComp); err != nil {
				logger.Error(err, "cannot send component upgrade failure report",
					"team", comp.TeamName, "bundle", comp.Name)
			}
		} else {
			if err := reporter.SendComponentUpgrade(configCtrl, upgradeComp); err != nil {
				logger.Error(err, "cannot send component upgrade failure report",
					"team", comp.TeamName, "component", comp.Name)
			}
		}
	}

	return nil
}

func (c *controller) listQueueHistory(selectors map[string]string) (*s2hv1.QueueHistoryList, error) {
	queueHists := &s2hv1.QueueHistoryList{}
	listOpt := &client.ListOptions{LabelSelector: labels.SelectorFromSet(selectors)}
	err := c.client.List(context.TODO(), queueHists, listOpt)
	queueHists.SortDESC()
	return queueHists, err
}

func (c *controller) getLatestFailureQueueHistory(comp *rpc.ComponentUpgrade) (*s2hv1.QueueHistory, error) {
	qLabels := s2h.GetDefaultLabels(comp.TeamName)
	qLabels["app"] = comp.Name
	qHists, err := c.listQueueHistory(qLabels)
	if err != nil {
		return &s2hv1.QueueHistory{}, errors.Wrapf(err,
			"cannot list queue history, labels: %+v, namespace: %s", qLabels, comp.Namespace)
	}

	qHist := &s2hv1.QueueHistory{}
	if len(qHists.Items) > 1 {
		qHist = &qHists.Items[1]
	}
	return qHist, nil
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

func (c *controller) ensureTeamPullRequestNamespaceUpdated(teamName, targetNs string) error {
	teamComp := &s2hv1.Team{}
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

func (c *controller) sendPullRequestTriggerReport(prTrigger *s2hv1.PullRequestTrigger, prTriggerRPC *rpc.PullRequestTrigger) {
	configCtrl := c.GetConfigController()

	bundleName := prTrigger.Spec.BundleName
	prNumber := prTrigger.Spec.PRNumber
	comps := prTrigger.Spec.Components
	for _, reporter := range c.reporters {
		noOfRetry := 0
		if prTrigger.Spec.NoOfRetry != nil {
			noOfRetry = *prTrigger.Spec.NoOfRetry
		}

		prTriggerRpt := s2h.NewPullRequestTriggerResultReporter(prTrigger.Status, c.configs, prTriggerRPC.TeamName,
			bundleName, prTrigger.Spec.PRNumber, prTriggerRPC.Result, noOfRetry, comps)

		if err := reporter.SendPullRequestTriggerResult(configCtrl, prTriggerRpt); err != nil {
			logger.Error(err, "cannot send pull request trigger result report",
				"team", prTriggerRPC.TeamName, "bundle", bundleName, "prNumber", prNumber)
		}
	}
}
