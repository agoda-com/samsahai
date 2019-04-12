package util

import (
	"testing"

	"github.com/agoda-com/samsahai/internal/samsahai/component"
	. "github.com/onsi/gomega"
)

func TestParseValuesFileToStruct(t *testing.T) {
	g := NewGomegaWithT(t)
	valuesPath := "../../../../test/testdata/reporter/values.yaml"
	compMapping, err := ParseValuesFileToStruct(valuesPath)
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

func TestParseValuesFileToStructWithEmpty(t *testing.T) {
	g := NewGomegaWithT(t)
	compMapping, err := ParseValuesFileToStruct("")
	g.Expect(compMapping).Should(BeNil())
	g.Expect(err).Should(BeNil())
}
