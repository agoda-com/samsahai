package send

import (
	"log"
	"strings"

	"github.com/agoda-com/samsahai/internal/samsahai/component"
	"github.com/agoda-com/samsahai/internal/samsahai/reporter"
	"github.com/spf13/cobra"
)

// imageMissingArgs defines arguments of image-missing command
type imageMissingArgs struct {
	images string
}

// imageMissing is global var for binding value
var imageMissing imageMissingArgs

func imageMissingCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "image-missing",
		Short: "send image missing list",
		RunE:  sendImageMissingListCmd,
	}

	cmd.Flags().StringVar(&imageMissing.images, "images", "", "List of image missing (Required)")

	if err := cmd.MarkFlagRequired("images"); err != nil {
		log.Fatalf("images argument not found [%v]", err)
	}

	return cmd
}

// sendImageMissingListCmd runs when command is executed
func sendImageMissingListCmd(cmd *cobra.Command, args []string) error {
	images, err := getImages(imageMissing.images)
	if err != nil {
		return err
	}

	if slack.enabled {
		slackCli, err := newSlackReporter()
		if err != nil {
			return err
		}

		if err := sendImageMissing(slackCli, images); err != nil {
			return err
		}
	}

	return nil
}

func sendImageMissing(r reporter.Reporter, images []component.Image) error {
	if err := r.SendImageMissingList(images); err != nil {
		return err
	}

	return nil
}

func getImages(value string) ([]component.Image, error) {
	imageList := getMultipleValues(value)
	images := make([]component.Image, 0)

	for _, image := range imageList {
		imageTag := strings.Split(image, ":")
		if len(imageTag) != 2 {
			return nil, ErrWrongImageFormat
		}
		images = append(images, component.Image{Repository: imageTag[0], Tag: imageTag[1]})
	}

	return images, nil
}
