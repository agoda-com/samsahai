package staging

import (
	"context"
	"net/http"

	"github.com/pkg/errors"
	"github.com/twitchtv/twirp"
	corev1 "k8s.io/api/core/v1"

	s2hv1 "github.com/agoda-com/samsahai/api/v1"
	"github.com/agoda-com/samsahai/internal"
	"github.com/agoda-com/samsahai/pkg/samsahai/rpc"
)

func (c *controller) detectImageMissing(queue *s2hv1.Queue) error {
	var err error
	headers := make(http.Header)
	headers.Set(internal.SamsahaiAuthHeader, c.authToken)
	ctx := context.TODO()
	ctx, err = twirp.WithHTTPRequestHeaders(ctx, headers)
	if err != nil {
		return errors.Wrap(err, "cannot set request header")
	}

	var imgList *rpc.ImageList
	comp := &rpc.TeamWithCurrentComponent{
		TeamName: c.teamName,
		CompName: queue.Name,
		Image:    &rpc.Image{Repository: queue.Spec.Repository, Tag: queue.Spec.Version},
	}
	if c.s2hClient != nil {
		imgList, err = c.s2hClient.GetMissingVersion(ctx, comp)
		if err != nil {
			return errors.Wrap(err, "cannot get image missing list")
		}
	}

	if imgList != nil && imgList.Images != nil && len(imgList.Images) > 0 {
		if err := c.updateImageMissingWithQueueState(queue, imgList); err != nil {
			return err
		}

		return nil
	}

	return c.updateQueueWithState(queue, s2hv1.Creating)
}

func (c *controller) updateImageMissingWithQueueState(queue *s2hv1.Queue, imgList *rpc.ImageList) error {
	outImgList := make([]s2hv1.Image, 0)
	for _, img := range imgList.Images {
		outImgList = append(outImgList, s2hv1.Image{Repository: img.Repository, Tag: img.Tag})
	}

	queue.Status.SetImageMissingList(outImgList)
	queue.Status.SetCondition(
		s2hv1.QueueDeployed,
		corev1.ConditionFalse,
		"queue image missing")

	// update queue back to k8s
	return c.updateQueueWithState(queue, s2hv1.Collecting)
}
