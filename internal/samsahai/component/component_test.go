package component

import (
	"testing"
	"time"

	. "github.com/onsi/gomega"
)

func TestNewComponent(t *testing.T) {
	g := NewGomegaWithT(t)
	tests := map[string]struct {
		in                map[string]string
		expectedComponent *Component
		expectedErr       error
	}{
		"component should be created successfully": {
			in: map[string]string{
				"name":    "component-test",
				"version": "1.1.0",
			},
			expectedComponent: &Component{
				Name:    "component-test",
				Version: "1.1.0",
			},
			expectedErr: nil,
		},
		"component should not be created with missing arguments error": {
			in: map[string]string{
				"name":           "component-test",
				"currentVersion": "",
			},
			expectedComponent: nil,
			expectedErr:       ErrMissingComponentArgs,
		},
	}

	for desc, test := range tests {
		component, err := NewComponent(test.in["name"], test.in["version"])
		if test.expectedErr == nil {
			g.Expect(err).Should(BeNil(), desc)
		} else {
			g.Expect(err).Should(Equal(test.expectedErr), desc)
		}

		if test.expectedComponent == nil {
			g.Expect(component).Should(BeNil(), desc)
		} else {
			g.Expect(component).Should(Equal(test.expectedComponent), desc)
		}
	}
}

func TestNewOutdatedComponent(t *testing.T) {
	g := NewGomegaWithT(t)
	now := time.Now()
	tests := map[string]struct {
		in                map[string]string
		inOption          Option
		expectedComponent *OutdatedComponent
	}{
		"outdated component should be created successfully with full options": {
			in: map[string]string{
				"name":           "component-test",
				"currentVersion": "1.1.0",
			},
			inOption: Option{
				key:   optionNewVersion,
				value: versionTimestamp{"1.1.2", now.AddDate(0, 0, -1).Unix()},
			},
			expectedComponent: &OutdatedComponent{
				CurrentComponent: &Component{Name: "component-test", Version: "1.1.0"},
				NewComponent:     &Component{Name: "component-test", Version: "1.1.2"},
				OutdatedDays:     2,
			},
		},
		"outdated component should be created successfully without options": {
			in: map[string]string{
				"name":           "component-test",
				"currentVersion": "1.1.0",
			},
			inOption: Option{},
			expectedComponent: &OutdatedComponent{
				CurrentComponent: &Component{Name: "component-test", Version: "1.1.0"},
				NewComponent:     &Component{},
				OutdatedDays:     0,
			},
		},
	}

	for desc, test := range tests {
		component, err := NewOutdatedComponent(test.in["name"], test.in["currentVersion"], test.inOption)
		g.Expect(err).Should(BeNil(), desc)
		g.Expect(component).Should(Equal(test.expectedComponent), desc)
	}
}

func TestSortComponentsByOutdatedDays(t *testing.T) {
	g := NewGomegaWithT(t)
	components := []OutdatedComponent{
		{
			CurrentComponent: &Component{Name: "comp1", Version: "1.1.0"},
			NewComponent:     &Component{Name: "comp1", Version: "1.1.2"},
			OutdatedDays:     1,
		},
		{
			CurrentComponent: &Component{Name: "comp2", Version: "1.1.0"},
			NewComponent:     &Component{Name: "comp2", Version: "1.1.0"},
			OutdatedDays:     0,
		},
		{
			CurrentComponent: &Component{Name: "comp3", Version: "1.1.0"},
			NewComponent:     &Component{Name: "comp3", Version: "1.1.4"},
			OutdatedDays:     3,
		},
		{
			CurrentComponent: &Component{Name: "comp4", Version: "1.1.0"},
			NewComponent:     &Component{Name: "comp4", Version: "1.1.0"},
			OutdatedDays:     0,
		},
	}
	SortComponentsByOutdatedDays(components)
	g.Expect(components).Should(Equal(
		[]OutdatedComponent{
			{
				CurrentComponent: &Component{Name: "comp3", Version: "1.1.0"},
				NewComponent:     &Component{Name: "comp3", Version: "1.1.4"},
				OutdatedDays:     3,
			},
			{
				CurrentComponent: &Component{Name: "comp1", Version: "1.1.0"},
				NewComponent:     &Component{Name: "comp1", Version: "1.1.2"},
				OutdatedDays:     1,
			},
			{
				CurrentComponent: &Component{Name: "comp2", Version: "1.1.0"},
				NewComponent:     &Component{Name: "comp2", Version: "1.1.0"},
				OutdatedDays:     0,
			},
			{
				CurrentComponent: &Component{Name: "comp4", Version: "1.1.0"},
				NewComponent:     &Component{Name: "comp4", Version: "1.1.0"},
				OutdatedDays:     0,
			},
		},
	))
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
