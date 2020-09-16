package samsahai

import (
	"context"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"

	s2hv1beta1 "github.com/agoda-com/samsahai/api/v1beta1"
	"github.com/agoda-com/samsahai/internal"
)

// TriggerPullRequestDeployment creates/updates PullRequestTrigger crd object
func (c *controller) TriggerPullRequestDeployment(teamName, component, tag, prNumber string) error {
	ctx := context.TODO()

	teamComp := s2hv1beta1.Team{}
	if err := c.GetTeam(teamName, &teamComp); err != nil {
		return err
	}

	namespace := teamComp.Status.Namespace.Staging
	prTriggerName := internal.GenPullRequestComponentName(component, prNumber)
	prTrigger := s2hv1beta1.PullRequestTrigger{}
	err := c.client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: prTriggerName}, &prTrigger)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			prTrigger := s2hv1beta1.PullRequestTrigger{
				ObjectMeta: v1.ObjectMeta{
					Name:      prTriggerName,
					Namespace: namespace,
					Labels:    getPullRequestTriggerLabels(teamName, component, prNumber),
				},
				Spec: s2hv1beta1.PullRequestTriggerSpec{
					Component:         component,
					PullRequestNumber: intstr.FromString(prNumber),
					Image:             &s2hv1beta1.Image{Tag: tag},
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

	if prTrigger.Spec.Image == nil {
		prTrigger.Spec.Image = &s2hv1beta1.Image{}
	}

	prTrigger.Spec.Image.Tag = tag

	if err := c.client.Update(ctx, &prTrigger); err != nil {
		return err
	}

	return nil
}

func getPullRequestTriggerLabels(teamName, component, prNumber string) map[string]string {
	prLabels := internal.GetDefaultLabels(teamName)
	prLabels["component"] = component
	prLabels["pr-number"] = prNumber

	return prLabels
}
