package msg

import (
	"testing"

	"github.com/agoda-com/samsahai/internal/samsahai/component"
	. "github.com/onsi/gomega"
)

func TestNewOutdatedComponents(t *testing.T) {
	g := NewGomegaWithT(t)
	oc := NewOutdatedComponents([]component.OutdatedComponent{}, true)
	g.Expect(oc).Should(Equal(&OutdatedComponents{Components: []component.OutdatedComponent{}, ShowedDetails: true}))
}

func TestNewOutdatedComponentsMessage(t *testing.T) {
	g := NewGomegaWithT(t)
	oc := &OutdatedComponents{
		Components: []component.OutdatedComponent{
			{
				CurrentComponent: &component.Component{Name: "comp1", Version: "1.1.0"},
				NewComponent:     &component.Component{Name: "comp1", Version: "1.1.2"},
				OutdatedDays:     1,
			},
		},
		ShowedDetails: true,
	}

	message := oc.NewOutdatedComponentsMessage()
	g.Expect(message).Should(Equal("*Outdated Components* \n*comp1* \n>Not update for 1 day(s) \n>Current version: 1.1.0 \n>New Version: 1.1.2\n"))
}

func TestGetOutdatedComponentsMessage(t *testing.T) {
	g := NewGomegaWithT(t)
	tests := map[string]struct {
		inComponents    []component.OutdatedComponent
		inShowedDetails bool
		out             string
	}{
		"should get outdated components message w/ details": {
			inComponents: []component.OutdatedComponent{
				{
					CurrentComponent: &component.Component{Name: "comp1", Version: "1.1.0"},
					NewComponent:     &component.Component{Name: "comp1", Version: "1.1.2"},
					OutdatedDays:     1,
				},
				{
					CurrentComponent: &component.Component{Name: "comp2", Version: "1.1.0"},
					NewComponent:     &component.Component{Name: "comp2", Version: "1.1.0"},
					OutdatedDays:     0,
				},
			},
			inShowedDetails: true,
			out:             "*comp1* \n>Not update for 1 day(s) \n>Current version: 1.1.0 \n>New Version: 1.1.2\n*comp2* \n>Current version: 1.1.0\n",
		},
		"should get outdated components message w/o detail": {
			inComponents: []component.OutdatedComponent{
				{
					CurrentComponent: &component.Component{Name: "comp1", Version: "1.1.0"},
					NewComponent:     &component.Component{Name: "comp1", Version: "1.1.2"},
					OutdatedDays:     1,
				},
				{
					CurrentComponent: &component.Component{Name: "comp2", Version: "1.1.0"},
					NewComponent:     &component.Component{Name: "comp2", Version: "1.1.0"},
					OutdatedDays:     0,
				},
			},
			inShowedDetails: false,
			out:             "*comp1* \n>Not update for 1 day(s) \n>Current version: 1.1.0 \n>New Version: 1.1.2\n",
		},
		"should get empty string in case nil component": {
			inComponents:    nil,
			inShowedDetails: true,
			out:             "",
		},
	}

	for desc, test := range tests {
		message := getOutdatedComponentsMessage(test.inComponents, test.inShowedDetails)
		g.Expect(message).Should(Equal(test.out), desc)
	}
}

func TestSortComponentsByOutdatedDays(t *testing.T) {
	g := NewGomegaWithT(t)
	components := []component.OutdatedComponent{
		{
			CurrentComponent: &component.Component{Name: "comp1", Version: "1.1.0"},
			NewComponent:     &component.Component{Name: "comp1", Version: "1.1.2"},
			OutdatedDays:     1,
		},
		{
			CurrentComponent: &component.Component{Name: "comp2", Version: "1.1.0"},
			NewComponent:     &component.Component{Name: "comp2", Version: "1.1.0"},
			OutdatedDays:     0,
		},
		{
			CurrentComponent: &component.Component{Name: "comp3", Version: "1.1.0"},
			NewComponent:     &component.Component{Name: "comp3", Version: "1.1.4"},
			OutdatedDays:     3,
		},
		{
			CurrentComponent: &component.Component{Name: "comp4", Version: "1.1.0"},
			NewComponent:     &component.Component{Name: "comp4", Version: "1.1.0"},
			OutdatedDays:     0,
		},
	}
	sortComponentsByOutdatedDays(components)
	g.Expect(components).Should(Equal(
		[]component.OutdatedComponent{
			{
				CurrentComponent: &component.Component{Name: "comp3", Version: "1.1.0"},
				NewComponent:     &component.Component{Name: "comp3", Version: "1.1.4"},
				OutdatedDays:     3,
			},
			{
				CurrentComponent: &component.Component{Name: "comp1", Version: "1.1.0"},
				NewComponent:     &component.Component{Name: "comp1", Version: "1.1.2"},
				OutdatedDays:     1,
			},
			{
				CurrentComponent: &component.Component{Name: "comp2", Version: "1.1.0"},
				NewComponent:     &component.Component{Name: "comp2", Version: "1.1.0"},
				OutdatedDays:     0,
			},
			{
				CurrentComponent: &component.Component{Name: "comp4", Version: "1.1.0"},
				NewComponent:     &component.Component{Name: "comp4", Version: "1.1.0"},
				OutdatedDays:     0,
			},
		},
	))
}
