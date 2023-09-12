package trigger

import (
	"context"
	"net/http"

	s2hv1 "github.com/agoda-com/samsahai/api/v1"
	"github.com/agoda-com/samsahai/internal"
	samsahairpc "github.com/agoda-com/samsahai/pkg/samsahai/rpc"
	"github.com/twitchtv/twirp"
	v1 "k8s.io/api/core/v1"
)

const sendTestPendingRetry = 3

func (c *controller) sendTestPendingResult(prTrigger *s2hv1.PullRequestTrigger) error {
	headers := make(http.Header)
	headers.Set(internal.SamsahaiAuthHeader, c.authToken)
	ctx, err := twirp.WithHTTPRequestHeaders(context.TODO(), headers)
	if err != nil {
		logger.Error(err, "cannot set request header")
		return err
	}

	for retry := 0; retry <= sendTestPendingRetry; retry++ {
		if _, err = c.s2hClient.RunPostPullRequestQueueTestRunnerTrigger(ctx, &samsahairpc.TeamWithPullRequest{
			TeamName:   c.teamName,
			Namespace:  internal.GenStagingNamespace(c.teamName),
			BundleName: internal.GenPullRequestBundleName(prTrigger.Spec.BundleName, prTrigger.Spec.PRNumber),
		}); err != nil {
			logger.Error(err,
				"cannot send pull request trigger pending status report,",
				"team", c.teamName, "component", prTrigger.Spec.BundleName, "pr number", prTrigger.Spec.PRNumber)
			// set state, cannot send test pending status
			prTrigger.Status.SetCondition(
				s2hv1.PullRequestTriggerCondPendingStatusSent,
				v1.ConditionFalse,
				"cannot send pull request trigger pending status")
			continue
		}
		logger.Info("sent pull request test runner pending status successfully",
			"team", c.teamName, "component", prTrigger.Spec.BundleName, "pr number", prTrigger.Spec.PRNumber)
		// set state, test pending status has been sent
		prTrigger.Status.SetCondition(
			s2hv1.PullRequestTriggerCondPendingStatusSent,
			v1.ConditionTrue,
			"pull request trigger pending status has been sent")
		break
	}
	return err
}
