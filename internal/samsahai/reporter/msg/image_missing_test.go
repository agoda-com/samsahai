package msg

import (
	"testing"

	"github.com/agoda-com/samsahai/internal/samsahai/component"
	. "github.com/onsi/gomega"
)

func TestNewImageMissing(t *testing.T) {
	g := NewGomegaWithT(t)
	im := NewImageMissing([]component.Image{})
	g.Expect(im).Should(Equal(&ImageMissing{Images: []component.Image{}}))
}

func TestNewImageMissingListMessage(t *testing.T) {
	g := NewGomegaWithT(t)
	im := ImageMissing{
		Images: []component.Image{
			{Repository: "registry/comp-1", Tag: "1.0.0"},
			{Repository: "registry/comp-2", Tag: "1.0.1-rc"},
		},
	}
	message := im.NewImageMissingListMessage()
	g.Expect(message).Should(Equal("registry/comp-1:1.0.0 (image missing)\nregistry/comp-2:1.0.1-rc (image missing)\n"))
}
