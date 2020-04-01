package samsahai

import (
	"context"
	"sort"
	"time"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"

	s2hv1beta1 "github.com/agoda-com/samsahai/api/v1beta1"
	"github.com/agoda-com/samsahai/internal"
	"github.com/agoda-com/samsahai/internal/errors"
	"github.com/agoda-com/samsahai/internal/samsahai/exporter"
	"github.com/agoda-com/samsahai/internal/util/stringutils"
	"github.com/agoda-com/samsahai/pkg/samsahai/rpc"
)

const maxDesiredMappingPerComp = 10

type changedComponent struct {
	Name       string
	Repository string
}

type updateHealth struct {
}

type exportMetric struct {
}

// updateTeamDesiredComponent defines which component of which team to be checked and updated
type updateTeamDesiredComponent struct {
	TeamName        string
	ComponentName   string
	ComponentSource string
	ComponentImage  s2hv1beta1.ComponentImage
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
	case changedComponent:
		err = c.checkComponentChanged(v)
	case updateTeamDesiredComponent:
		err = c.updateTeamDesiredComponent(v)
	case updateHealth:
		err = c.updateHealthMetric()
	case exportMetric:
		err = c.exportTeamMetric()
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
	configCtrl := c.GetConfigController()
	teamList, err := c.GetTeams()
	if err != nil {
		return err
	}

	for _, teamComp := range teamList.Items {
		teamName := teamComp.Name
		team := &s2hv1beta1.Team{}
		if err := c.getTeam(teamName, team); err != nil {
			logger.Error(err, "cannot get team", "team", teamName)
			return err
		}

		comps, _ := configCtrl.GetComponents(teamName)
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
					"name", desiredComp, "namespace", compNs)
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

	// Add metric updateQueueMetric
	queue := &s2hv1beta1.Queue{}
	if err = c.client.Get(ctx, types.NamespacedName{
		Name:      compName,
		Namespace: compNs}, queue); err != nil {
		logger.Error(err, "cannot get the queue")
	} else {
		exporter.SetQueueMetric(queue)
	}

	return nil
}

func (c *controller) sendImageMissingReport(teamName, repo, version string) {
	configCtrl := c.GetConfigController()
	for _, reporter := range c.reporters {
		img := &rpc.Image{Repository: repo, Tag: version}
		if err := reporter.SendImageMissing(teamName, configCtrl, img); err != nil {
			logger.Error(err, "cannot send image missing list report", "team", teamName)
		}
	}
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

func (c *controller) exportTeamMetric() error {
	teamList, err := c.GetTeams()
	if err != nil {
		return err
	}
	exporter.SetTeamNameMetric(teamList)
	return nil
}

func (c *controller) updateHealthMetric() error {
	exporter.SetHealthStatusMetric(internal.Version, internal.GitCommit, float64(time.Now().Unix()))
	c.queue.AddAfter(updateHealth{}, time.Minute)
	return nil
}
