package send

import (
	"testing"

	"github.com/agoda-com/samsahai/internal/samsahai/component"

	. "github.com/onsi/gomega"
)

func TestSendComponentUpgradeFail(t *testing.T) {
	g := NewGomegaWithT(t)
	mockReporter := mockReporter{}
	err := sendComponentUpgradeFail(&mockReporter, &component.Component{})
	g.Expect(mockReporter.sendComponentUpgradeFailCalls).Should(Equal(1), "should call send image missing func")
	g.Expect(err).Should(BeNil())
}

func TestCreateComponent(t *testing.T) {
	g := NewGomegaWithT(t)
	tests := map[string]struct {
		in           map[string]string
		expectedComp *component.Component
		expectedErr  error
	}{
		"should create component successfully": {
			in: map[string]string{
				"name":  "comp1",
				"image": "image-1:1.1.0",
			},
			expectedComp: &component.Component{
				Name:    "comp1",
				Version: "1.1.0",
				Image:   &component.Image{Repository: "image-1", Tag: "1.1.0"},
			},
			expectedErr: nil,
		},
		"should not create component with wrong image format": {
			in: map[string]string{
				"name":  "comp1",
				"image": "image-1:1.1.0:1",
			},
			expectedComp: nil,
			expectedErr:  ErrWrongImageFormat,
		},
	}

	for desc, test := range tests {
		images, err := createComponent(test.in["name"], test.in["image"])
		if test.expectedErr == nil {
			g.Expect(err).Should(BeNil(), desc)
		} else {
			g.Expect(err).Should(Equal(test.expectedErr), desc)
		}

		if test.expectedComp == nil {
			g.Expect(images).Should(BeNil(), desc)
		} else {
			g.Expect(images).ShouldNot(BeNil(), desc)
		}
	}
}
