package samsahai

import (
	"context"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	s2hv1 "github.com/agoda-com/samsahai/api/v1"
	"github.com/agoda-com/samsahai/internal"
)

// TriggerPullRequestDeployment creates/updates PullRequestTrigger crd object
func (c *controller) TriggerPullRequestDeployment(teamName, bundleName, prNumber, commitSHA string,
	bundleCompsTag map[string]string) error {

	ctx := context.TODO()

	teamComp := s2hv1.Team{}
	if err := c.GetTeam(teamName, &teamComp); err != nil {
		return err
	}

	components := make([]*s2hv1.PullRequestTriggerComponent, 0)
	for name, tag := range bundleCompsTag {
		components = append(components, &s2hv1.PullRequestTriggerComponent{
			ComponentName: name,
			Image:         &s2hv1.Image{Tag: tag},
		})
	}

	namespace := teamComp.Status.Namespace.Staging
	prTriggerName := internal.GenPullRequestBundleName(bundleName, prNumber)
	prTrigger := s2hv1.PullRequestTrigger{}
	err := c.client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: prTriggerName}, &prTrigger)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			prTrigger := s2hv1.PullRequestTrigger{
				ObjectMeta: v1.ObjectMeta{
					Name:      prTriggerName,
					Namespace: namespace,
					Labels:    getPullRequestTriggerLabels(teamName, bundleName, prNumber),
				},
				Spec: s2hv1.PullRequestTriggerSpec{
					BundleName: bundleName,
					PRNumber:   prNumber,
					CommitSHA:  commitSHA,
					Components: components,
				},
			}

			if err := c.client.Create(ctx, &prTrigger); err != nil {
				return err
			}

			return nil
		}

		logger.Error(err, "cannot list pull request trigger", "team", teamName)
		return err
	}

	prTrigger.Spec.Components = components
	prTrigger.Spec.CommitSHA = commitSHA

	// reset pr nextProcessAt and noOfRetry on every trigger
	initRetry := 0
	prTrigger.Spec.NextProcessAt = nil
	prTrigger.Spec.NoOfRetry = &initRetry

	if err := c.client.Update(ctx, &prTrigger); err != nil {
		return err
	}

	return nil
}

func getPullRequestTriggerLabels(teamName, bundle, prNumber string) map[string]string {
	prLabels := internal.GetDefaultLabels(teamName)
	prLabels["bundle"] = bundle
	prLabels["pr-number"] = prNumber

	return prLabels
}
