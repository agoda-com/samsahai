package samsahai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/twitchtv/twirp"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/agoda-com/samsahai/internal"
	"github.com/agoda-com/samsahai/internal/errors"
	"github.com/agoda-com/samsahai/internal/samsahai/exporter"
	"github.com/agoda-com/samsahai/internal/util/stringutils"
	s2hv1beta1 "github.com/agoda-com/samsahai/pkg/apis/env/v1beta1"
	"github.com/agoda-com/samsahai/pkg/samsahai/rpc"
	stagingrpc "github.com/agoda-com/samsahai/pkg/staging/rpc"
)

const maxDesiredMappingPerComp = 10

type changedComponent struct {
	Name       string
	Repository string
}

// updateTeamGit defines which team needs to update git
type updateTeamGit string

type updateHealth struct {
}

type exportMetric struct {
}

// updateTeamDesiredComponent defines which component of which team to be checked and updated
type updateTeamDesiredComponent struct {
	TeamName        string
	ComponentName   string
	ComponentSource string
	ComponentImage  internal.ComponentImage
}

func (c *controller) Start(stop <-chan struct{}) {
	defer close(c.internalStopper)
	defer c.queue.ShutDown()

	jitterPeriod := time.Millisecond * 500
	for i := 0; i < MaxConcurrentProcess; i++ {
		go wait.Until(func() {
			// Process work items
			for c.process() {
			}
		}, jitterPeriod, c.internalStop)
	}

	c.queue.Add(updateHealth{})
	c.queue.AddAfter(exportMetric{}, (30 * time.Second))

	<-stop

	logger.Info("stopping internal process")
}

func (c *controller) NotifyGitChanged(updated internal.GitInfo) {
	c.queue.Add(updated)
}

func (c *controller) NotifyComponentChanged(compName, repository string) {
	c.queue.Add(changedComponent{
		Repository: repository,
		Name:       compName,
	})
}

func (c *controller) process() bool {
	var err error
	obj, shutdown := c.queue.Get()
	if obj == nil {
		// Sometimes the Queue gives us nil items when it starts up
		c.queue.Forget(obj)
	}

	if shutdown {
		// Stop working
		return false
	}

	defer c.queue.Done(obj)

	switch v := obj.(type) {
	case internal.GitInfo:
		err = c.checkGitChanged(v)
	case updateTeamGit:
		err = c.updateTeamGit(string(v))
	case changedComponent:
		err = c.checkComponentChanged(v)
	case updateTeamDesiredComponent:
		err = c.updateTeamDesiredComponent(v)
	case updateHealth:
		err = c.updateHealthMetric()
	case exportMetric:
		err = c.exportAllMetric()
	default:
		c.queue.Forget(obj)
		return true
	}
	if err != nil {
		c.queue.AddRateLimited(obj)
		return false
	}

	c.queue.Forget(obj)

	return true
}

// checkComponentChanged checks matched components from every teams and add to queue for checking new version.
//
// Component should match both name and repository
func (c *controller) checkComponentChanged(component changedComponent) error {
	teamNames := c.getTeamNamesFromConfig()
	for i := range teamNames {
		teamName := teamNames[i]
		config, _ := c.GetTeamConfigManager(teamName)
		team := &s2hv1beta1.Team{}
		if err := c.getTeam(teamName, team); err != nil {
			logger.Error(err, "cannot get team", "team", teamName)
			return err
		}

		comps := config.GetComponents()
		for _, comp := range comps {
			if component.Name != comp.Name || comp.Source == nil {
				// ignored mismatch or missing source
				continue
			}

			if _, ok := c.checkers[string(*comp.Source)]; !ok {
				// ignore non-existing source
				continue
			}

			if component.Repository != "" && component.Repository != comp.Image.Repository {
				// ignore mismatch repository
				continue
			}

			// add to queue for processing
			c.queue.Add(updateTeamDesiredComponent{
				TeamName:        teamName,
				ComponentName:   comp.Name,
				ComponentSource: string(*comp.Source),
				ComponentImage:  comp.Image,
			})
		}
	}
	return nil
}

// updateTeamDesiredComponent gets new version from checker and checks with DesiredComponent of team.
//
// updateInfo will always has valid checker (from checkComponentChanged)
//
// Update to the desired version if mismatch.
func (c *controller) updateTeamDesiredComponent(updateInfo updateTeamDesiredComponent) error {
	var err error

	// run checker to get desired version
	checker := c.checkers[updateInfo.ComponentSource]
	checkPattern := updateInfo.ComponentImage.Pattern

	team := &s2hv1beta1.Team{}
	if err := c.getTeam(updateInfo.TeamName, team); err != nil {
		logger.Error(err, "cannot get team", "team", updateInfo.TeamName)
		return err
	}

	compNs := team.Status.Namespace.Staging
	compName := updateInfo.ComponentName
	compRepository := updateInfo.ComponentImage.Repository

	// TODO: do caching for better performance
	version, vErr := checker.GetVersion(compRepository, compName, checkPattern)
	if vErr != nil && !errors.IsImageNotFound(vErr) && !errors.IsErrRequestTimeout(vErr) {
		logger.Error(vErr, "error while run checker.getversion",
			"team", updateInfo.TeamName, "name", compName, "repository", compRepository,
			"version pattern", checkPattern)
		return vErr
	}

	ctx := context.Background()
	now := metav1.Now()
	desiredImage := stringutils.ConcatImageString(compRepository, version)
	desiredImageTime := s2hv1beta1.DesiredImageTime{
		Image: &s2hv1beta1.Image{
			Repository: compRepository,
			Tag:        version,
		},
		CreatedTime: now,
	}

	// update desired component version created time mapping
	team.Status.UpdateDesiredComponentImageCreatedTime(updateInfo.ComponentName, desiredImage, desiredImageTime)
	deleteDesiredMappingOutOfRange(team, maxDesiredMappingPerComp)
	if err := c.updateTeam(team); err != nil {
		return err
	}

	if vErr != nil && (errors.IsImageNotFound(vErr) || errors.IsErrRequestTimeout(vErr)) {
		c.sendImageMissingReport(updateInfo.TeamName, compRepository, version)
		return nil
	}

	desiredComp := &s2hv1beta1.DesiredComponent{}
	err = c.client.Get(ctx, types.NamespacedName{Name: compName, Namespace: compNs}, desiredComp)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			// Create new DesiredComponent
			desiredLabels := internal.GetDefaultLabels(team.Name)
			desiredLabels["app"] = compName
			desiredComp = &s2hv1beta1.DesiredComponent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      compName,
					Namespace: compNs,
					Labels:    desiredLabels,
				},
				Spec: s2hv1beta1.DesiredComponentSpec{
					Version:    version,
					Name:       compName,
					Repository: compRepository,
				},
				Status: s2hv1beta1.DesiredComponentStatus{
					CreatedAt: &now,
					UpdatedAt: &now,
				},
			}

			if err = c.client.Create(ctx, desiredComp); err != nil {
				logger.Error(err, "cannot create DesiredComponent",
					"name", compName, "namespace", compNs)
			}

			return nil
		}

		logger.Error(err, "cannot get DesiredComponent", "name", compName, "namespace", compNs)
		return err
	}

	// DesiredComponent found, check the version
	if desiredComp.Spec.Version == version && desiredComp.Spec.Repository == compRepository {
		return nil
	}

	// Update when version or repository changed
	desiredComp.Spec.Version = version
	desiredComp.Spec.Repository = compRepository
	desiredComp.Status.UpdatedAt = &now

	if err = c.client.Update(ctx, desiredComp); err != nil {
		logger.Error(err, "cannot update DesiredComponent", "name", compName, "namespace", compNs)
		return err
	}

	// Add matric updateQueueMetric
	queueList := &s2hv1beta1.QueueList{}
	if err = c.client.List(context.TODO(), nil, queueList); err != nil {
		logger.Error(err, "cannot list all queue")
	}
	exporter.SetQueueMetric(queueList, c.teamConfigs)

	return nil
}

func (c *controller) sendImageMissingReport(teamName, repo, version string) {
	configMgr, _ := c.GetTeamConfigManager(teamName)
	for _, reporter := range c.reporters {
		img := &rpc.Image{Repository: repo, Tag: version}
		if err := reporter.SendImageMissing(configMgr, img); err != nil {
			logger.Error(err, "cannot send image missing list report", "team", teamName)
		}
	}
}

// checkGitChanged determines which team's configuration matched with GitInfo.
//
// If matched, added to queue.
func (c *controller) checkGitChanged(updated internal.GitInfo) error {
	teamNames := c.getTeamNamesFromConfig()
	for i := range teamNames {
		teamName := teamNames[i]
		cfg, ok := c.GetTeamConfigManager(teamName)
		if !ok || cfg == nil {
			continue
		}

		gitInfo := cfg.GetGitInfo()

		// TODO(phantomat) should check fullname instead of name?
		isRepoNameMatched := gitInfo.Name == "" || gitInfo.Name != updated.Name
		if isRepoNameMatched {
			continue
		}

		isBranchMatched := updated.BranchName != gitInfo.BranchName || (updated.BranchName == "master" && gitInfo.BranchName != "")
		if !isBranchMatched {
			continue
		}

		c.queue.Add(updateTeamGit(teamName))
	}
	return nil
}

// updateTeamGit pulls git to the latest of refs spec,
// and sent the latest configuration to Staging controller.
func (c *controller) updateTeamGit(teamName string) error {
	cfg, ok := c.GetTeamConfigManager(teamName)
	if !ok || cfg == nil {
		return nil
	}

	err := cfg.Sync()
	if err != nil {
		if errors.IsErrGitPulling(err) {
			logger.Debug("still pulling git", "team", teamName)
			return err
		} else if errors.IsGitNoErrAlreadyUpToDate(err) {
			return nil
		}
		logger.Error(err, "cannot sync git", "team", teamName)
		return err
	}

	// TODO(phantomnat) can check webhook revision after git pull

	team := &s2hv1beta1.Team{}
	if err = c.getTeam(teamName, team); err != nil {
		logger.Error(err, "cannot get team", "team", teamName)
		return err
	}

	gitRevision := cfg.GetGitLatestRevision()
	msg := fmt.Sprintf("with revision %s", gitRevision)
	team.Status.SetCondition(
		s2hv1beta1.TeamGitCheckoutUpToDate,
		corev1.ConditionTrue,
		fmt.Sprintf("git pulling is success %s", msg),
	)
	if err := c.updateTeam(team); err != nil {
		return errors.Wrapf(err, "cannot update team conditions when git pulling success, team %s", teamName)
	}

	// create gRPC connection with Staging controller
	stagingURI := fmt.Sprintf("http://%s.%s.svc.%s:%d",
		internal.StagingCtrlName,
		team.Status.Namespace.Staging,
		c.configs.ClusterDomain,
		internal.StagingDefaultPort)

	// override endpoint from configuration
	if team.Spec.StagingCtrl != nil && (*team.Spec.StagingCtrl).Endpoint != "" {
		stagingURI = (*team.Spec.StagingCtrl).Endpoint
	}
	stagingRPC := stagingrpc.NewRPCProtobufClient(stagingURI, &http.Client{})

	headers := make(http.Header)
	headers.Set(internal.SamsahaiAuthHeader, c.configs.SamsahaiCredential.InternalAuthToken)
	ctx := context.TODO()
	ctx, _ = twirp.WithHTTPRequestHeaders(ctx, headers)

	configBytes, err := json.Marshal(cfg.Get())
	if err != nil {
		logger.Error(err, "cannot marshal configuration", "team", teamName)
		return err
	}

	_, err = stagingRPC.UpdateConfiguration(ctx, &stagingrpc.Configuration{
		Config:      configBytes,
		GitRevision: gitRevision,
	})
	if err != nil {
		if twerr, ok := err.(twirp.Error); ok {
			logger.Error(twerr, "cannot call staging for update configuration",
				"team", teamName,
				"namespace", team.Status.Namespace.Staging,
				"stagingURI", stagingURI,
				"errMsg", twerr.Msg(),
				"errCode", twerr.Code(),
				"errMetaBody", twerr.Meta("body"),
				"errMetaStatusCode", twerr.Meta("status_code"),
			)
			return err
		}
		logger.Error(err, "cannot call staging for update configuration ",
			"team", teamName,
			"namespace", team.Status.Namespace.Staging,
			"stagingURI", stagingURI)
		return err
	}

	return nil
}

func (c *controller) QueueLen() int {
	return c.queue.Len()
}

type desiredTime struct {
	image            string
	desiredImageTime s2hv1beta1.DesiredImageTime
}

func deleteDesiredMappingOutOfRange(team *s2hv1beta1.Team, maxDesiredMapping int) {
	desiredMap := team.Status.DesiredComponentImageCreatedTime
	for compName, m := range desiredMap {
		desiredList := convertDesiredMapToDesiredTimeList(m)
		if len(desiredList) > maxDesiredMapping {
			sortDesiredList(desiredList)
			for i := len(desiredList) - 1; i > maxDesiredMapping-1; i-- {
				delete(desiredMap[compName], desiredList[i].image)
			}
		}
	}
}

// sortDesiredList by timestamp DESC
func sortDesiredList(desiredList []desiredTime) {
	sort.SliceStable(desiredList, func(i, j int) bool {
		return desiredList[i].desiredImageTime.CreatedTime.After(desiredList[j].desiredImageTime.CreatedTime.Time)
	})
}

func convertDesiredMapToDesiredTimeList(desiredMap map[string]s2hv1beta1.DesiredImageTime) []desiredTime {
	out := make([]desiredTime, 0)
	for k, v := range desiredMap {
		out = append(out, desiredTime{image: k, desiredImageTime: v})
	}

	return out
}

type outdatedComponentTime struct {
	Component   *s2hv1beta1.ActivePromotion
	CreatedTime *metav1.Time
}

func (c *controller) exportAllMetric() error {

	//team name
	exporter.SetTeamNameMetric(c.teamConfigs)

	//queue
	queueList := &s2hv1beta1.QueueList{}
	if err := c.client.List(context.TODO(), nil, queueList); err != nil {
		logger.Error(err, "cannot list all queue")
	}
	exporter.SetQueueMetric(queueList, c.teamConfigs)

	//queue histories
	queueHistoriesList := &s2hv1beta1.QueueHistoryList{}
	if err := c.client.List(context.TODO(), nil, queueHistoriesList); err != nil {
		logger.Error(err, "cannot list all queue")
	}
	exporter.SetQueueHistoriesMetric(queueHistoriesList, c.configs.SamsahaiURL)

	//active Promotion
	atpList := &s2hv1beta1.ActivePromotionList{}
	if err := c.client.List(context.TODO(), nil, atpList); err != nil {
		logger.Error(err, "cannot list all active promotion")
	}
	exporter.SetActivePromotionMetric(atpList)

	//active Promotion histories
	atpHisList := &s2hv1beta1.ActivePromotionHistoryList{}
	if err := c.client.List(context.TODO(), nil, atpHisList); err != nil {
		logger.Error(err, "cannot list all active promotion histories")
	}
	exporter.SetActivePromotionHistoriesMetric(atpHisList)

	//outdated component
	oc := map[string]outdatedComponentTime{}
	for _, atpHistories := range atpHisList.Items {
		teamName := atpHistories.Spec.TeamName
		if teamName == "" {
			teamName = atpHistories.Labels["samsahai.io/teamname"]
		}
		if atpHistories.Spec.ActivePromotion == nil {
			continue
		}
		if atpHistories.Spec.ActivePromotion.Status.Result == s2hv1beta1.ActivePromotionCanceled {
			continue
		}
		itemCreateTime := atpHistories.CreationTimestamp
		if obj, ok := oc[teamName]; ok {
			if !obj.CreatedTime.Before(&itemCreateTime) {
				continue
			}
		}
		atpHistories.Spec.ActivePromotion.Name = teamName
		oc[teamName] = outdatedComponentTime{
			atpHistories.Spec.ActivePromotion,
			&itemCreateTime,
		}
	}
	for _, obj := range oc {
		exporter.SetOutdatedComponentMetric(obj.Component)
	}
	return nil
}

func (c *controller) updateHealthMetric() error {
	exporter.SetHealthStatusMetric(internal.Version, internal.GitCommit, float64(time.Now().Unix()))
	c.queue.AddAfter(updateHealth{}, time.Minute)
	return nil
}
