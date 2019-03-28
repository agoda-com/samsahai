package msg

import (
	"testing"

	"github.com/agoda-com/samsahai/internal/samsahai/component"
	. "github.com/onsi/gomega"
)

func TestNewActivePromotionMessage(t *testing.T) {
	g := NewGomegaWithT(t)
	ap := &ActivePromotion{
		Status:                 "success",
		ServiceOwner:           "owner",
		CurrentActiveNamespace: "namespace-1",
		Components: []component.Component{
			{Name: "comp1", CurrentVersion: "1.1.0", NewVersion: "1.1.2", OutdatedDays: 1},
		},
	}

	message := ap.NewActivePromotionMessage(true)
	g.Expect(message).Should(Equal("*Active Promotion:* SUCCESS \n*Owner:* owner \n*Current Active Namespace:* namespace-1\n*comp1* \n>Not update for 1 day(s) \n>Current version: 1.1.0 \n>New Version: 1.1.2\n"))
}

func TestGetStatusText(t *testing.T) {
	g := NewGomegaWithT(t)
	tests := map[string]struct {
		in  string
		out string
	}{
		"should get success status": {
			in:  "Success",
			out: "SUCCESS",
		},
		"should get fail status": {
			in:  "Failed",
			out: "FAIL",
		},
	}

	for desc, test := range tests {
		statusText := getStatusText(test.in)
		g.Expect(statusText).Should(Equal(test.out), desc)
	}
}

func TestGetOutdatedComponentsMessage(t *testing.T) {
	g := NewGomegaWithT(t)
	tests := map[string]struct {
		inComponents    []component.Component
		inShowedDetails bool
		out             string
	}{
		"should get outdated components message w/ details": {
			inComponents: []component.Component{
				{Name: "comp1", CurrentVersion: "1.1.0", NewVersion: "1.1.2", OutdatedDays: 1},
				{Name: "comp2", CurrentVersion: "1.1.0", NewVersion: "1.1.0", OutdatedDays: 0},
			},
			inShowedDetails: true,
			out:             "*comp1* \n>Not update for 1 day(s) \n>Current version: 1.1.0 \n>New Version: 1.1.2\n*comp2* \n>Current version: 1.1.0\n",
		},
		"should get outdated components message w/o detail": {
			inComponents: []component.Component{
				{Name: "comp1", CurrentVersion: "1.1.0", NewVersion: "1.1.2", OutdatedDays: 1},
				{Name: "comp2", CurrentVersion: "1.1.0", NewVersion: "1.1.0", OutdatedDays: 0},
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
	components := []component.Component{
		{Name: "comp1", CurrentVersion: "1.1.0", NewVersion: "1.1.2", OutdatedDays: 1},
		{Name: "comp2", CurrentVersion: "1.1.0", NewVersion: "1.1.0", OutdatedDays: 0},
		{Name: "comp3", CurrentVersion: "1.1.0", NewVersion: "1.1.4", OutdatedDays: 3},
		{Name: "comp4", CurrentVersion: "1.1.0", NewVersion: "1.1.0", OutdatedDays: 0},
	}
	sortComponentsByOutdatedDays(components)
	g.Expect(components).Should(Equal(
		[]component.Component{
			{Name: "comp3", CurrentVersion: "1.1.0", NewVersion: "1.1.4", OutdatedDays: 3},
			{Name: "comp1", CurrentVersion: "1.1.0", NewVersion: "1.1.2", OutdatedDays: 1},
			{Name: "comp2", CurrentVersion: "1.1.0", NewVersion: "1.1.0", OutdatedDays: 0},
			{Name: "comp4", CurrentVersion: "1.1.0", NewVersion: "1.1.0", OutdatedDays: 0},
		},
	))
}
