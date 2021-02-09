package samsahai

import (
	"context"
	"sort"
	"time"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"

	s2hv1 "github.com/agoda-com/samsahai/api/v1"
	"github.com/agoda-com/samsahai/internal"
	"github.com/agoda-com/samsahai/internal/errors"
	"github.com/agoda-com/samsahai/internal/samsahai/exporter"
	"github.com/agoda-com/samsahai/internal/util/stringutils"
)

const maxDesiredMappingPerComp = 10

type changedComponent struct {
	Name       string
	Repository string
	TeamName   string
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
	ComponentImage  s2hv1.ComponentImage
	ComponentBundle string
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
	c.queue.AddAfter(exportMetric{}, 30*time.Second)

	<-stop

	logger.Info("stopping internal process")
}

func (c *controller) NotifyComponentChanged(compName, repository, teamName string) {
	c.queue.Add(changedComponent{
		Repository: repository,
		Name:       compName,
		TeamName:   teamName,
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
	if component.TeamName != "" {
		if err := c.checkTeamComponentChanged(component.Name, component.Repository, component.TeamName); err != nil {
			logger.Error(err, "cannot check component changed", "team", component.TeamName)
			return err
		}
	} else {
		teamList, err := c.GetTeams()
		if err != nil {
			return err
		}

		for _, teamComp := range teamList.Items {
			teamName := teamComp.Name
			if err := c.checkTeamComponentChanged(component.Name, component.Repository, teamName); err != nil {
				logger.Error(err, "cannot check component changed", "team", teamName)
				continue
			}
		}
	}

	return nil
}

func (c *controller) checkTeamComponentChanged(compName, repository, teamName string) error {
	configCtrl := c.GetConfigController()
	team := &s2hv1.Team{}
	if err := c.getTeam(teamName, team); err != nil {
		logger.Error(err, "cannot get team", "team", teamName)
		return err
	}

	comps, _ := configCtrl.GetComponents(teamName)
	for _, comp := range comps {
		if compName != comp.Name || comp.Source == nil {
			// ignored mismatch or missing source
			continue
		}

		if _, err := c.getComponentChecker(string(*comp.Source)); err != nil {
			// ignore non-existing source
			continue
		}

		if repository != "" && repository != comp.Image.Repository {
			// ignore mismatch repository
			continue
		}

		logger.Debug("component has been notified", "team", teamName, "component", comp.Name)

		// add to queue for processing
		bundleName := c.getBundleName(comp.Name, teamName)
		c.queue.Add(updateTeamDesiredComponent{
			TeamName:        teamName,
			ComponentName:   comp.Name,
			ComponentSource: string(*comp.Source),
			ComponentImage:  comp.Image,
			ComponentBundle: bundleName,
		})
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
	checker, err := c.getComponentChecker(updateInfo.ComponentSource)
	if err != nil {
		logger.Error(err, "cannot get component checker",
			"team", updateInfo.TeamName, "source", updateInfo.ComponentSource)
		return err
	}
	checkPattern := updateInfo.ComponentImage.Pattern

	team := &s2hv1.Team{}
	if err := c.getTeam(updateInfo.TeamName, team); err != nil {
		logger.Error(err, "cannot get team", "team", updateInfo.TeamName)
		return err
	}

	compNs := team.Status.Namespace.Staging
	compName := updateInfo.ComponentName
	compRepository := updateInfo.ComponentImage.Repository
	compBundle := updateInfo.ComponentBundle

	// TODO: do caching for better performance
	version, vErr := checker.GetVersion(compRepository, compName, checkPattern)
	switch {
	case vErr == nil:
	case errors.IsImageNotFound(vErr) || errors.IsErrRequestTimeout(vErr):
	case errors.IsInternalCheckerError(vErr):
		c.sendImageMissingReport(updateInfo.TeamName, updateInfo.ComponentName, compRepository, version, vErr.Error())
		return nil
	default:
		logger.Error(vErr, "error while run checker.getversion",
			"team", updateInfo.TeamName, "name", compName, "repository", compRepository,
			"version pattern", checkPattern)
		return vErr
	}

	ctx := context.Background()
	now := metav1.Now()
	desiredImage := stringutils.ConcatImageString(compRepository, version)
	if vErr == nil {
		desiredImageTime := s2hv1.DesiredImageTime{
			Image: &s2hv1.Image{
				Repository: compRepository,
				Tag:        version,
			},
			CreatedTime:    now,
			CheckedTime:    now,
			IsImageMissing: false,
		}
		// update desired component version created time mapping
		team.Status.UpdateDesiredComponentImageCreatedTime(updateInfo.ComponentName, desiredImage, desiredImageTime)
		deleteDesiredMappingOutOfRange(team, maxDesiredMappingPerComp)
		if err := c.updateTeam(team); err != nil {
			return err
		}
	} else if errors.IsImageNotFound(vErr) || errors.IsErrRequestTimeout(vErr) {
		c.sendImageMissingReport(updateInfo.TeamName, updateInfo.ComponentName, compRepository, version, "")
		ImageMissingDesiredImageTime := s2hv1.DesiredImageTime{
			Image: &s2hv1.Image{
				Repository: compRepository,
				Tag:        version,
			},
			CreatedTime:    now,
			CheckedTime:    now,
			IsImageMissing: true,
		}

		// update missing version of desired component created time mapping
		team.Status.UpdateDesiredComponentImageCreatedTime(updateInfo.ComponentName, desiredImage, ImageMissingDesiredImageTime)
		deleteDesiredMappingOutOfRange(team, maxDesiredMappingPerComp)
		if err := c.updateTeam(team); err != nil {
			return err
		}
		return nil
	}

	desiredComp := &s2hv1.DesiredComponent{}
	err = c.client.Get(ctx, types.NamespacedName{Name: compName, Namespace: compNs}, desiredComp)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			// Create new DesiredComponent
			desiredLabels := internal.GetDefaultLabels(team.Name)
			desiredLabels["app"] = compName
			desiredComp = &s2hv1.DesiredComponent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      compName,
					Namespace: compNs,
					Labels:    desiredLabels,
				},
				Spec: s2hv1.DesiredComponentSpec{
					Version:    version,
					Name:       compName,
					Repository: compRepository,
					Bundle:     compBundle,
				},
				Status: s2hv1.DesiredComponentStatus{
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
	sameComp := desiredComp.IsSame(&s2hv1.DesiredComponent{
		Spec: s2hv1.DesiredComponentSpec{
			Name:       compName,
			Version:    version,
			Repository: compRepository,
			Bundle:     compBundle,
		},
	})
	if sameComp {
		return nil
	}

	// Update when version or repository changed
	desiredComp.Spec.Version = version
	desiredComp.Spec.Repository = compRepository
	desiredComp.Spec.Bundle = compBundle
	desiredComp.Status.UpdatedAt = &now

	if err = c.client.Update(ctx, desiredComp); err != nil {
		logger.Error(err, "cannot update DesiredComponent", "name", compName, "namespace", compNs)
		return err
	}

	return nil
}

func (c *controller) sendImageMissingReport(teamName, compName, repo, version, reason string) {
	configCtrl := c.GetConfigController()
	for _, reporter := range c.reporters {
		img := s2hv1.Image{Repository: repo, Tag: version}
		imageMissingRpt := internal.NewImageMissingReporter(img, c.configs, teamName, compName, reason)
		if err := reporter.SendImageMissing(configCtrl, imageMissingRpt); err != nil {
			logger.Error(err, "cannot send image missing list report", "team", teamName)
		}
	}
}

func (c *controller) QueueLen() int {
	return c.queue.Len()
}

type desiredTime struct {
	image            string
	desiredImageTime s2hv1.DesiredImageTime
}

func deleteDesiredMappingOutOfRange(team *s2hv1.Team, maxDesiredMapping int) {
	desiredMap := team.Status.DesiredComponentImageCreatedTime
	for compName, m := range desiredMap {
		desiredList := convertDesiredMapToDesiredTimeList(m)
		if len(desiredList) > maxDesiredMapping {
			sortDesiredList(desiredList)
			for i := len(desiredList) - 1; i > maxDesiredMapping-1; i-- {
				desiredImage := desiredList[i].image
				if isImageInActive(team, compName, desiredImage) {
					break
				}

				delete(desiredMap[compName], desiredImage)
			}
		}
	}
}

func isImageInActive(team *s2hv1.Team, compName, desiredImage string) bool {
	activeComponents := team.Status.ActiveComponents
	if activeComp, ok := activeComponents[compName]; ok {
		if activeComp.Spec.Name != "" {
			activeImage := stringutils.ConcatImageString(activeComp.Spec.Repository, activeComp.Spec.Version)
			if activeImage == desiredImage {
				return true
			}
		}
	}

	return false
}

// sortDesiredList by timestamp DESC
func sortDesiredList(desiredList []desiredTime) {
	sort.SliceStable(desiredList, func(i, j int) bool {
		return desiredList[i].desiredImageTime.CreatedTime.After(desiredList[j].desiredImageTime.CreatedTime.Time)
	})
}

func convertDesiredMapToDesiredTimeList(desiredMap map[string]s2hv1.DesiredImageTime) []desiredTime {
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
