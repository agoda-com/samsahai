package msg

import (
	"fmt"

	"github.com/agoda-com/samsahai/internal/samsahai/component"
)

// ImageMissing manages image missing report
type ImageMissing struct {
	Images []component.Image
}

// NewImageMissing creates a new image missing
func NewImageMissing(images []component.Image) *ImageMissing {
	return &ImageMissing{Images: images}
}

// NewImageMissingListMessage creates an image missing message
func (im *ImageMissing) NewImageMissingListMessage() string {
	var message string
	for _, image := range im.Images {
		message += fmt.Sprintln(getImageMissingMessage(image))
	}

	return message
}

func getImageMissingMessage(image component.Image) string {
	return fmt.Sprintf("%s:%s (image missing)", image.Repository, image.Tag)
}
