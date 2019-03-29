package send

import (
	"testing"

	"github.com/agoda-com/samsahai/internal/samsahai/component"

	. "github.com/onsi/gomega"
)

func TestSendActivePromotionStatus(t *testing.T) {
	g := NewGomegaWithT(t)
	mockReporter := mockReporter{}
	_ = sendActivePromotionStatus(&mockReporter, []component.Component{})
	g.Expect(mockReporter.sendActivePromotionStatusCalls).Should(Equal(1), "should call send active promotion status func")
}

func TestGetActiveComponentsFromValuesFile(t *testing.T) {
	g := NewGomegaWithT(t)
	tests := map[string]struct {
		in                 map[string]string
		expectedComponents []component.Component
		expectedErr        error
	}{
		"should get active components successfully": {
			in: map[string]string{
				"activeValuesFile": "../../../../test/testdata/reporter/active-values.yaml",
				"newValuesFile":    "",
			},
			expectedComponents: []component.Component{},
			expectedErr:        nil,
		},
		"should get empty active components": {
			in: map[string]string{
				"activeValuesFile": "",
				"newValuesFile":    "../../../../test/testdata/reporter/new-values.yaml",
			},
			expectedComponents: nil,
			expectedErr:        nil,
		},
	}

	for desc, test := range tests {
		comps, err := getActiveComponentsFromValuesFile(test.in["activeValuesFile"], test.in["newValuesFile"])
		if test.expectedErr == nil {
			g.Expect(err).Should(BeNil(), desc)
		} else {
			g.Expect(err).Should(Equal(test.expectedErr), desc)
		}

		if test.expectedComponents == nil {
			g.Expect(comps).Should(BeNil(), desc)
		} else {
			g.Expect(comps).ShouldNot(BeNil(), desc)
		}
	}
}

func TestParseValuesfileToStruct(t *testing.T) {
	g := NewGomegaWithT(t)
	valuesPath := "../../../../test/testdata/reporter/values.yaml"
	compMapping, err := parseValuesfileToStruct(valuesPath)
	g.Expect(err).Should(BeNil())
	g.Expect(compMapping).Should(Equal(map[string]component.ValuesFile{
		"comp1": {
			Image: component.Image{
				Repository: "registry/image-comp1", Tag: "1.1.3", Timestamp: 1553507991,
			}},
		"comp2": {
			Image: component.Image{
				Repository: "registry/image-comp2", Tag: "1.1.0", Timestamp: 1553157675,
			}},
	}))
}
