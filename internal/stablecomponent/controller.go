package stablecomponent

import (
	"context"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cr "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	k8scontroller "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	s2hv1beta1 "github.com/agoda-com/samsahai/api/v1beta1"
	"github.com/agoda-com/samsahai/internal"
	"github.com/agoda-com/samsahai/internal/errors"
	s2hlog "github.com/agoda-com/samsahai/internal/log"
	"github.com/agoda-com/samsahai/internal/util/stringutils"
)

var logger = s2hlog.Log.WithName(ctrlName)

const (
	ctrlName                = "stable-component-ctrl"
	stableFinalizerName     = "stable.finalizers.samsahai.io"
	maxConcurrentReconciles = 1
)

type controller struct {
	client  client.Client
	s2hCtrl internal.SamsahaiController
}

func New(
	mgr cr.Manager,
	s2hCtrl internal.SamsahaiController,
) internal.StableComponentController {
	c := &controller{
		s2hCtrl: s2hCtrl,
	}

	if mgr != nil {
		c.client = mgr.GetClient()
		if err := c.setupWithManager(mgr); err != nil {
			logger.Error(err, "cannot add new controller to manager")
			return nil
		}
	}

	return c
}

func (c *controller) setupWithManager(mgr cr.Manager) error {
	return cr.NewControllerManagedBy(mgr).
		WithOptions(k8scontroller.Options{MaxConcurrentReconciles: maxConcurrentReconciles}).
		For(&s2hv1beta1.StableComponent{}).
		Complete(c)
}

func (c *controller) updateStable(stableComp *s2hv1beta1.StableComponent) error {
	if err := c.client.Update(context.Background(), stableComp); err != nil {
		logger.Error(err, "cannot update stable component", "name", stableComp.Name, "namespace", stableComp.Namespace)
		return errors.Wrap(err, "cannot update stable component")
	}

	return nil
}

func (c *controller) updateTeam(team *s2hv1beta1.Team) error {
	if err := c.client.Update(context.Background(), team); err != nil {
		return errors.Wrap(err, "cannot update team")
	}

	return nil
}

func (c *controller) getTeamStaging(stableComp *s2hv1beta1.StableComponent) (*s2hv1beta1.Team, error) {
	var team *s2hv1beta1.Team
	labels := stableComp.GetLabels()
	if teamName, ok := labels[internal.GetTeamLabelKey()]; ok && teamName != "" {
		team = &s2hv1beta1.Team{}
		err := c.s2hCtrl.GetTeam(teamName, team)
		if err != nil {
			// ignore if team not found
			if k8serrors.IsNotFound(err) {
				return nil, nil
			}

			logger.Error(err, "cannot get team", "team", teamName)
			return nil, err
		}

		// ignore if it is not from staging namespace
		if team.Status.Namespace.Staging != stableComp.Namespace {
			logger.Debug("cannot modify stable component on non-staging namespace", "name", stableComp.Name, "namespace", stableComp.Namespace)
			return nil, nil
		}

		return team, nil
	}

	teams, err := c.s2hCtrl.GetTeams()
	if err != nil {
		logger.Error(err, "cannot list teams")
		return nil, err
	}

	for i := 0; i < len(teams.Items); i++ {
		if teams.Items[i].Status.Namespace.Staging == stableComp.Namespace {
			team = &teams.Items[i]
			return team, nil
		}
	}

	// not found any team match with staging namespace
	return nil, nil
}

func (c *controller) addFinalizer(stableComp *s2hv1beta1.StableComponent) error {
	// The object is not being deleted, so if it does not have our finalizer,
	// then lets add the finalizer and update the object.
	if !stringutils.ContainsString(stableComp.ObjectMeta.Finalizers, stableFinalizerName) {
		stableComp.ObjectMeta.Finalizers = append(stableComp.ObjectMeta.Finalizers, stableFinalizerName)
		if err := c.updateStable(stableComp); err != nil {
			return err
		}
	}

	return nil
}

func (c *controller) deleteFinalizer(stableComp *s2hv1beta1.StableComponent, team *s2hv1beta1.Team) error {
	if stringutils.ContainsString(stableComp.ObjectMeta.Finalizers, stableFinalizerName) {
		if team.Status.SetStableComponents(stableComp, true) {
			if err := c.updateTeam(team); err != nil && !k8serrors.IsNotFound(err) {
				return err
			}
		}

		// remove our finalizer from the list and update it.
		stableComp.ObjectMeta.Finalizers = stringutils.RemoveString(stableComp.ObjectMeta.Finalizers, stableFinalizerName)
		if err := c.updateStable(stableComp); err != nil {
			return err
		}

		logger.Info("remove stable component", "name", stableComp.Name, "team", team.GetName())
	}

	return nil
}

func (c *controller) Reconcile(req cr.Request) (cr.Result, error) {
	ctx := context.Background()
	stableComp := &s2hv1beta1.StableComponent{}
	if err := c.client.Get(ctx, req.NamespacedName, stableComp); err != nil {
		if k8serrors.IsNotFound(err) {
			// Object not found, return. Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}

		logger.Error(err, "cannot get StableComponent", "name", req.Name, "namespace", req.Namespace)
		return cr.Result{}, err
	}

	team, err := c.getTeamStaging(stableComp)
	if err != nil {
		return cr.Result{}, err
	}

	if team == nil {
		logger.Debug("cannot get team", "name", req.Name, "namespace", req.Namespace)
		return cr.Result{}, nil
	}

	// The object is being deleted
	if !stableComp.ObjectMeta.DeletionTimestamp.IsZero() {
		if err := c.deleteFinalizer(stableComp, team); err != nil {
			return reconcile.Result{}, err
		}

		return reconcile.Result{}, nil
	}

	if err := c.addFinalizer(stableComp); err != nil {
		return reconcile.Result{}, err
	}

	if ok := c.detectSpecChanged(stableComp, team); !ok {
		return reconcile.Result{}, nil
	}

	now := metav1.Now()
	stableComp.Status.UpdatedAt = &now
	if stableComp.Status.CreatedAt == nil {
		stableComp.Status.CreatedAt = &now
	}

	// Update team if stable component has changes
	if team.Status.SetStableComponents(stableComp, false) {
		if err := c.updateTeam(team); err != nil {
			return cr.Result{}, err
		}
	}

	// Update stable component status
	if err := c.updateStable(stableComp); err != nil {
		return cr.Result{}, err
	}

	return cr.Result{}, nil
}

func (c *controller) detectSpecChanged(stableComp *s2hv1beta1.StableComponent, teamComp *s2hv1beta1.Team) bool {
	if stableComp != nil {
		teamStableComp := teamComp.Status.GetStableComponent(stableComp.Name)
		if teamStableComp.Spec.Name != "" {
			if teamStableComp.Spec == stableComp.Spec {
				return false
			}
		}
	}

	return true
}
