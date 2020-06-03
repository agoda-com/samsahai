package samsahai

import (
	"context"
	"crypto/subtle"
	"fmt"

	"github.com/pkg/errors"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	s2hv1beta1 "github.com/agoda-com/samsahai/api/v1beta1"
	s2h "github.com/agoda-com/samsahai/internal"
	s2herrors "github.com/agoda-com/samsahai/internal/errors"
	"github.com/agoda-com/samsahai/internal/samsahai/exporter"
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

func (c *controller) GetMissingVersion(ctx context.Context, teamInfo *rpc.TeamWithCurrentComponent) (*rpc.ImageList, error) {
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
		for _, qcomp := range teamInfo.Components {
			if qcomp.Name == stable.Name {
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
	for _, qcomp := range teamInfo.Components {
		source, ok := c.getImageSource(comps, qcomp.Name)
		if ok {
			c.detectAndAddImageMissing(*source, qcomp.Image.Repository, qcomp.Name, qcomp.Image.Tag, imgList)
		}
	}

	return imgList, nil
}

func (c *controller) detectAndAddImageMissing(source s2hv1beta1.UpdatingSource, repo, name, version string, imgList *rpc.ImageList) {
	checker := c.checkers[string(source)]
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
	if err := c.client.Get(context.TODO(), types.NamespacedName{
		Namespace: comp.GetNamespace(),
		Name:      comp.GetName()}, queue); err != nil {
		logger.Error(err, "cannot get the queue")
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

	compName := comp.GetName()
	if compName != "" {
		queue := &s2hv1beta1.Queue{}
		if err := c.client.Get(context.TODO(), types.NamespacedName{
			Namespace: comp.GetNamespace(),
			Name:      compName}, queue); err != nil {
			logger.Error(err, "cannot get the queue")
		}
		exporter.SetQueueMetric(queue)
	}

	return &rpc.Empty{}, nil
}

func (c *controller) GetBundleName(ctx context.Context, teamWithCompName *rpc.TeamWithComponentName) (*rpc.BundleName, error) {
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
