package activepromotion

import (
	"context"
	"fmt"
	"math/rand"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	s2hv1beta1 "github.com/agoda-com/samsahai/api/v1beta1"
	"github.com/agoda-com/samsahai/internal"
	s2herrors "github.com/agoda-com/samsahai/internal/errors"
)

func (c *controller) createPreActiveEnvAndDeployStableCompObjects(ctx context.Context, atpComp *s2hv1beta1.ActivePromotion) error {
	teamName := atpComp.Name
	suffix := c.randomToken(tokenLength, atpComp.CreationTimestamp.UnixNano())
	targetNs := fmt.Sprintf("%s%s-%s", internal.AppPrefix, teamName, suffix)

	if err := c.createPreActiveEnvironment(ctx, teamName, targetNs); err != nil {
		return err
	}

	teamComp, err := c.getTeam(ctx, teamName)
	if err != nil {
		return err
	}

	stagingNs := teamComp.Status.Namespace.Staging
	if err = c.copyStableComponentObjectsToTargetNamespace(ctx, stagingNs, targetNs); err != nil {
		return err
	}

	logger.Debug("start deploying stable components into target namespace",
		"team", teamName, "namespace", targetNs)
	atpComp.Status.SetNamespace(targetNs, teamComp.Status.Namespace.Active)
	atpComp.Status.SetCondition(s2hv1beta1.ActivePromotionCondPreActiveCreated, corev1.ConditionTrue,
		"Pre-active environment has been created")
	atpComp.Status.SetCondition(s2hv1beta1.ActivePromotionCondVerificationStarted, corev1.ConditionTrue,
		"Verifying pre-active environment")
	atpComp.SetState(s2hv1beta1.ActivePromotionDeployingComponents,
		"Deploying stable components into target namespace")
	return nil
}

func (c *controller) ensureActiveEnvironmentPromoted(ctx context.Context, teamName, targetNs string) error {
	teamComp, err := c.getTeam(ctx, teamName)
	if err != nil {
		return err
	}

	if err := c.s2hCtrl.PromoteActiveEnvironment(teamComp, targetNs); err != nil && k8serrors.IsAlreadyExists(err) {
		return err
	}

	if err := c.ensureTeamNamespaceUpdated(teamComp.Status.Namespace.Active, targetNs); err != nil {
		return s2herrors.ErrEnsureActivePromoted
	}

	return nil
}

func (c *controller) createPreActiveEnvironment(ctx context.Context, teamName, preActiveNs string) error {
	if err := c.s2hCtrl.CreatePreActiveEnvironment(teamName, preActiveNs); err != nil && !k8serrors.IsAlreadyExists(err) {
		return err
	}

	teamComp, err := c.getTeam(ctx, teamName)
	if err != nil {
		return err
	}

	if err := c.ensureTeamNamespaceUpdated(teamComp.Status.Namespace.PreActive, preActiveNs); err != nil {
		return err
	}

	if err := c.ensureNamespaceReady(ctx, preActiveNs); err != nil {
		return err
	}

	return nil
}

func (c *controller) copyStableComponentObjectsToTargetNamespace(ctx context.Context, baseNs, targetNs string) error {
	stableComps, err := c.getStableComponentObjects(ctx, baseNs)
	if err != nil && !k8serrors.IsNotFound(err) {
		return errors.Wrapf(err, "cannot get stable components from staging namespace %s", baseNs)
	}

	if err = c.deployStableComponentObjects(ctx, stableComps, targetNs); err != nil {
		return err
	}

	return nil
}

func (c *controller) getStableComponentObjects(ctx context.Context, ns string) (*s2hv1beta1.StableComponentList, error) {
	stableComps := &s2hv1beta1.StableComponentList{}
	if err := c.client.List(ctx, stableComps, &client.ListOptions{Namespace: ns}); err != nil {
		return &s2hv1beta1.StableComponentList{}, err
	}

	return stableComps, nil
}

func (c *controller) deployStableComponentObjects(ctx context.Context, comps *s2hv1beta1.StableComponentList, targetNS string) error {
	if targetNS == "" {
		return errors.Wrap(fmt.Errorf("target namespace is empty"), "cannot deploy stable components")
	}

	for _, comp := range comps.Items {
		newComp := s2hv1beta1.StableComponent{
			ObjectMeta: metav1.ObjectMeta{
				Name:      comp.Name,
				Namespace: targetNS,
				Labels:    comp.Labels,
			},
			Spec: comp.Spec,
		}

		if err := c.client.Create(ctx, &newComp); err != nil {
			if k8serrors.IsAlreadyExists(err) {
				// update stable components
				ctemp := s2hv1beta1.StableComponent{}
				err := c.client.Get(ctx, types.NamespacedName{Name: comp.Name, Namespace: targetNS}, &ctemp)
				if err != nil {
					return err
				}

				ctemp.Spec = newComp.Spec
				ctemp.Status = newComp.Status
				if err := c.client.Update(ctx, &ctemp); err != nil {
					return err
				}
			} else {
				return errors.Wrapf(err, "cannot deploy stable components into target namespace %s", targetNS)
			}
		}
	}

	return nil
}

func (c *controller) ensureTeamNamespaceUpdated(teamNs, atpNs string) error {
	if teamNs != atpNs {
		return fmt.Errorf("team pre-active namespace was not updated, expected: %s, actual: %s", atpNs, teamNs)
	}

	return nil
}

func (c *controller) ensureNamespaceReady(ctx context.Context, ns string) error {
	preActiveNs := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: ns,
		},
	}

	if err := c.client.Get(ctx, types.NamespacedName{Name: ns}, preActiveNs); err != nil {
		return errors.Wrapf(err, "cannot get namespace %s", ns)
	}

	return nil
}

func (c *controller) randomToken(l int, seed int64) string {
	rand.Seed(seed)
	letters := []rune("0123456789abcdefghijklmnopqrstuvwxyz")
	b := make([]rune, l)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
