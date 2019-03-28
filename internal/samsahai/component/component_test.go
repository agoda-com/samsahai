package component

import (
	"strconv"
	"testing"

	. "github.com/onsi/gomega"
)

func TestNewComponent(t *testing.T) {
	g := NewGomegaWithT(t)
	tests := map[string]struct {
		in                map[string]string
		expectedComponent *Component
		expectedErr       error
	}{
		"component should be create successfully": {
			in: map[string]string{
				"name":           "component-test",
				"currentVersion": "1.1.0",
				"newVersion":     "1.1.2",
				"outdatedDays":   "2",
			},
			expectedComponent: &Component{Name: "component-test", CurrentVersion: "1.1.0", NewVersion: "1.1.2", OutdatedDays: 2},
			expectedErr:       nil,
		},
		"component should not be created with missing arguments error": {
			in: map[string]string{
				"name":           "",
				"currentVersion": "",
				"newVersion":     "1.1.2",
				"outdatedDays":   "2",
			},
			expectedComponent: nil,
			expectedErr:       ErrMissingComponentArgs,
		},
		"component should not be created with wrong format of arguments error": {
			in: map[string]string{
				"name":           "component-test",
				"currentVersion": "1.1.0",
				"newVersion":     "1.1.2",
				"outdatedDays":   "-1",
			},
			expectedComponent: nil,
			expectedErr:       ErrWrongFormatComponentArgs,
		},
	}

	for desc, test := range tests {
		outdatedDays, _ := strconv.Atoi(test.in["outdatedDays"])
		component, err := NewComponent(test.in["name"], test.in["currentVersion"], test.in["newVersion"], outdatedDays)
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
