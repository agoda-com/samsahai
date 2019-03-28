package active

import (
	"sort"
	"testing"
	"time"

	"github.com/agoda-com/samsahai/internal/samsahai/component"

	. "github.com/onsi/gomega"
)

func TestGetCurrentActiveComponents(t *testing.T) {
	g := NewGomegaWithT(t)
	now := time.Now()
	tests := map[string]struct {
		inCurrentActiveValuesFile map[string]component.ValuesFile
		inNewValuesFile           map[string]component.ValuesFile
		expectedComponents        []component.Component
	}{
		"single active components with same version": {
			inCurrentActiveValuesFile: map[string]component.ValuesFile{
				"component1": {
					Image: component.Image{Repository: "repo1", Tag: "1.1.0", Timestamp: 1551509631},
				},
			},
			inNewValuesFile: map[string]component.ValuesFile{
				"component1": {
					Image: component.Image{Repository: "repo1", Tag: "1.1.0", Timestamp: 1551509631},
				},
			},
			expectedComponents: []component.Component{
				{
					Name:           "component1",
					CurrentVersion: "1.1.0",
					NewVersion:     "1.1.0",
					OutdatedDays:   0,
				},
			},
		},
		"single active components with different version": {
			inCurrentActiveValuesFile: map[string]component.ValuesFile{
				"component1": {
					Image: component.Image{Repository: "repo1", Tag: "1.1.0", Timestamp: 1551596733},
				},
			},
			inNewValuesFile: map[string]component.ValuesFile{
				"component1": {
					Image: component.Image{Repository: "repo1", Tag: "1.1.1", Timestamp: now.AddDate(0, 0, -1).Unix()},
				},
			},
			expectedComponents: []component.Component{
				{
					Name:           "component1",
					CurrentVersion: "1.1.0",
					NewVersion:     "1.1.1",
					OutdatedDays:   2,
				},
			},
		},
		"current active values more than new values": {
			inCurrentActiveValuesFile: map[string]component.ValuesFile{
				"component1": {
					Image: component.Image{Repository: "repo1", Tag: "1.1.0", Timestamp: 1551509631},
				},
				"component2": {
					Image: component.Image{Repository: "repo2", Tag: "1.1.0", Timestamp: 1551509631},
				},
			},
			inNewValuesFile: map[string]component.ValuesFile{
				"component1": {
					Image: component.Image{Repository: "repo1", Tag: "1.1.0", Timestamp: 1551509631},
				},
			},
			expectedComponents: []component.Component{
				{
					Name:           "component1",
					CurrentVersion: "1.1.0",
					NewVersion:     "1.1.0",
					OutdatedDays:   0,
				},
				{
					Name:           "component2",
					CurrentVersion: "1.1.0",
					NewVersion:     "",
					OutdatedDays:   0,
				},
			},
		},
		"new values more than current active values": {
			inCurrentActiveValuesFile: map[string]component.ValuesFile{
				"component1": {
					Image: component.Image{Repository: "repo1", Tag: "1.1.0", Timestamp: 1551509631},
				},
			},
			inNewValuesFile: map[string]component.ValuesFile{
				"component1": {
					Image: component.Image{Repository: "repo1", Tag: "1.1.0", Timestamp: 1551509631},
				},
				"component2": {
					Image: component.Image{Repository: "repo2", Tag: "1.1.0", Timestamp: 1551509631},
				},
			},
			expectedComponents: []component.Component{
				{
					Name:           "component1",
					CurrentVersion: "1.1.0",
					NewVersion:     "1.1.0",
					OutdatedDays:   0,
				},
			},
		},
	}

	for desc, test := range tests {
		atvComps, err := GetCurrentActiveComponents(test.inCurrentActiveValuesFile, test.inNewValuesFile)
		g.Expect(err).Should(BeNil(), desc)

		// Ignore order
		sortComponents(atvComps)
		sortComponents(test.expectedComponents)
		g.Expect(atvComps).Should(Equal(test.expectedComponents), desc)
	}
}

// TODO: implements
func TestGetCurrentActiveNamespaceByOwner(t *testing.T) {
	g := NewGomegaWithT(t)
	currentActiveNamespace, _ := GetCurrentActiveNamespaceByOwner("")
	g.Expect(currentActiveNamespace, Equal(""))
}

func TestGetOutdatedDays(t *testing.T) {
	g := NewGomegaWithT(t)
	now := time.Now()
	tests := map[string]struct {
		in  int64
		out int
	}{
		"next day but outdated greater than fully day": {
			in:  now.AddDate(0, 0, -1).Unix(),
			out: 2,
		},
		"next day but outdated less than fully day": {
			in:  now.AddDate(0, 0, -1).Add(10 * time.Minute).Unix(),
			out: 1,
		},
		"same day": {
			in:  now.Add(-10 * time.Minute).Unix(),
			out: 1,
		},
	}

	for desc, test := range tests {
		outdatedDays := getOutdatedDays(test.in)
		g.Expect(outdatedDays).Should(Equal(test.out), desc)
	}
}

func sortComponents(components []component.Component) {
	sort.Slice(components, func(i, j int) bool { return components[i].Name > components[j].Name })
}
