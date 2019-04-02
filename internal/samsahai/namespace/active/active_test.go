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
		expectedComponents        []component.OutdatedComponent
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
			expectedComponents: []component.OutdatedComponent{
				{
					CurrentComponent: &component.Component{Name: "component1", Version: "1.1.0"},
					NewComponent:     &component.Component{Name: "component1", Version: "1.1.0"},
					OutdatedDays:     0,
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
			expectedComponents: []component.OutdatedComponent{
				{
					CurrentComponent: &component.Component{Name: "component1", Version: "1.1.0"},
					NewComponent:     &component.Component{Name: "component1", Version: "1.1.1"},
					OutdatedDays:     2,
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
			expectedComponents: []component.OutdatedComponent{
				{
					CurrentComponent: &component.Component{Name: "component1", Version: "1.1.0"},
					NewComponent:     &component.Component{Name: "component1", Version: "1.1.0"},
					OutdatedDays:     0,
				},
				{
					CurrentComponent: &component.Component{Name: "component2", Version: "1.1.0"},
					NewComponent:     &component.Component{},
					OutdatedDays:     0,
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
			expectedComponents: []component.OutdatedComponent{
				{
					CurrentComponent: &component.Component{Name: "component1", Version: "1.1.0"},
					NewComponent:     &component.Component{Name: "component1", Version: "1.1.0"},
					OutdatedDays:     0,
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

func sortComponents(components []component.OutdatedComponent) {
	sort.Slice(components, func(i, j int) bool { return components[i].CurrentComponent.Name > components[j].CurrentComponent.Name })
}
