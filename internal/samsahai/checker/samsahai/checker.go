package samsahai

import (
	"regexp"
	"sort"

	"github.com/pkg/errors"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/agoda-com/samsahai/api/v1beta1"
	"github.com/agoda-com/samsahai/internal"
	s2herrors "github.com/agoda-com/samsahai/internal/errors"
	s2hlog "github.com/agoda-com/samsahai/internal/log"
)

var logger = s2hlog.Log.WithName(CheckerName)

const (
	CheckerName = "samsahai-stable"
)

type checker struct {
	samsahai internal.SamsahaiController
}

func New(samsahai internal.SamsahaiController) internal.DesiredComponentChecker {
	return &checker{samsahai: samsahai}
}

func (c *checker) GetName() string {
	return CheckerName
}

func (c *checker) GetVersion(repository, name, pattern string) (string, error) {
	if pattern == "" {
		pattern = ".*"
	}

	matcher, err := regexp.Compile(pattern)
	if err != nil {
		logger.Error(err, "invalid pattern", "pattern", pattern)
		return "", err
	}

	teamConfigs := c.samsahai.GetTeamConfigManagers()

	var matchedTags []string

	for teamName, configMgr := range teamConfigs {
		// Check if team name matched with pattern
		if !matcher.MatchString(teamName) {
			continue
		}

		comps := configMgr.GetComponents()
		comp, ok := comps[name]

		// Check if component exist in configuration
		if !ok {
			continue
		}

		// Check if component's source isn't `samsahai-stable`
		if comp.Source == nil || (*comp.Source == CheckerName) {
			// ignore this checker
			continue
		}

		// Get team
		team := &v1beta1.Team{}
		err := c.samsahai.GetTeam(teamName, team)
		if err != nil {
			if k8serrors.IsNotFound(err) {
				// ignore team not found
				continue
			}
			return "", errors.Wrap(err, "cannot get team")
		}

		var stableComp *v1beta1.StableComponent
		// create the stable components map
		for i := range team.Status.StableComponents {
			c := &team.Status.StableComponents[i]
			if c.Spec.Name == name {
				stableComp = c
				break
			}
		}

		if stableComp == nil {
			// ignore component not found
			continue
		}

		if repository != "" && stableComp.Spec.Repository != repository {
			// ignore repository mismatched
			continue
		}

		matchedTags = append(matchedTags, stableComp.Spec.Version)
	}

	if len(matchedTags) == 0 {
		kvs := []interface{}{
			"name", name,
			"pattern", pattern,
		}
		if repository != "" {
			kvs = append(kvs, "repository", repository)
		}
		logger.Warn("cannot get any version, no component matched", kvs...)
		return "", s2herrors.ErrImageVersionNotFound
	}

	sort.Sort(internal.SortableVersion(matchedTags))
	return matchedTags[len(matchedTags)-1], nil
}

func (c *checker) EnsureVersion(repository, name, version string) error {
	_, err := c.GetVersion(repository, name, version)
	return err
}
