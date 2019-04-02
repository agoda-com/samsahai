package send

import (
	"testing"

	"github.com/agoda-com/samsahai/internal/samsahai/component"

	. "github.com/onsi/gomega"
)

func TestSendImageMissing(t *testing.T) {
	g := NewGomegaWithT(t)
	mockReporter := mockReporter{}
	err := sendImageMissing(&mockReporter, []component.Image{})
	g.Expect(mockReporter.sendImageMissingCalls).Should(Equal(1), "should call send image missing func")
	g.Expect(err).Should(BeNil())
}

func TestGetImages(t *testing.T) {
	g := NewGomegaWithT(t)
	tests := map[string]struct {
		in             string
		expectedImages []component.Image
		expectedErr    error
	}{
		"should get images successfully": {
			in: "registry/comp-1:1.0.1-rc,registry/comp-1:1.0.2",
			expectedImages: []component.Image{
				{Repository: "registry/comp-1", Tag: "1.0.1-rc"},
				{Repository: "registry/comp-1", Tag: "1.0.2"},
			},
			expectedErr: nil,
		},
		"should not get images with wrong image format": {
			in:             "registry/comp-1:1.0.1-rc,registry/comp-1:1.0.2:1",
			expectedImages: nil,
			expectedErr:    ErrWrongImageFormat,
		},
	}

	for desc, test := range tests {
		images, err := getImages(test.in)
		if test.expectedErr == nil {
			g.Expect(err).Should(BeNil(), desc)
		} else {
			g.Expect(err).Should(Equal(test.expectedErr), desc)
		}

		if test.expectedImages == nil {
			g.Expect(images).Should(BeNil(), desc)
		} else {
			g.Expect(images).ShouldNot(BeNil(), desc)
		}
	}
}
