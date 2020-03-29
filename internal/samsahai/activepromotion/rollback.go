package activepromotion

import (
	"context"

	corev1 "k8s.io/api/core/v1"

	s2hv1beta1 "github.com/agoda-com/samsahai/api/v1beta1"
	s2herrors "github.com/agoda-com/samsahai/internal/errors"
	"github.com/agoda-com/samsahai/internal/queue"
)

func (c *controller) rollbackActiveEnvironment(ctx context.Context, atpComp *s2hv1beta1.ActivePromotion) error {
	if err := c.checkRollbackTimeout(ctx, atpComp); err != nil {
		return err
	}

	if err := c.demoteAndDestroyPreActive(ctx, atpComp); err != nil {
		if s2herrors.IsEnsuringActiveDemoted(err) ||
			s2herrors.IsEnsuringNamespaceDestroyed(err) {
			return s2herrors.ErrRollingBackActivePromotion
		}
		return err
	}

	if err := c.rePromoteCurrentActive(ctx, atpComp); err != nil {
		if s2herrors.IsEnsuringActivePromoted(err) {
			return s2herrors.ErrRollingBackActivePromotion
		}
		return err
	}

	return nil
}

func (c *controller) demoteAndDestroyPreActive(ctx context.Context, atpComp *s2hv1beta1.ActivePromotion) error {
	isTimeout, err := c.isTimeoutFromConfig(atpComp, timeoutActiveDemotionForRollback)
	if err != nil {
		return err
	}

	if !isTimeout {
		teamName := atpComp.Name
		targetNs := atpComp.Status.TargetNamespace

		errCh := make(chan error, 2)
		go func() {
			errCh <- c.demotePreActiveForRollback(ctx, teamName, targetNs)
		}()

		go func() {
			errCh <- c.destroyPreActiveEnvironmentForRollback(ctx, atpComp)
		}()

		for i := 0; i < 2; i++ {
			if err := <-errCh; err != nil {
				return err
			}
		}
	}

	return nil
}

func (c *controller) demotePreActiveForRollback(ctx context.Context, teamName, targetNs string) error {
	if err := queue.DeletePreActiveQueue(c.client, targetNs); err != nil {
		return err
	}

	if err := queue.DeletePromoteToActiveQueue(c.client, targetNs); err != nil {
		return err
	}

	teamComp, err := c.getTeam(ctx, teamName)
	if err != nil {
		return err
	}
	preActiveNs := teamComp.Status.Namespace.PreActive
	if preActiveNs != "" {
		if err := c.ensureQueueActiveDemoted(teamName, preActiveNs); err != nil {
			return err
		}
	}

	return nil
}

func (c *controller) destroyPreActiveEnvironmentForRollback(ctx context.Context, atpComp *s2hv1beta1.ActivePromotion) error {
	teamName := atpComp.Name
	targetNs := atpComp.Status.TargetNamespace
	startedCleaningTime := atpComp.Status.GetConditionLatestTime(s2hv1beta1.ActivePromotionCondRollbackStarted)
	if err := c.ensureDestroyEnvironment(ctx, preActiveEnvirontment, teamName, targetNs, startedCleaningTime); err != nil {
		return err
	}

	logger.Debug("pre-active environment has been destroyed due to rollback",
		"team", teamName, "status", atpComp.Status.Result, "namespace", targetNs)
	atpComp.Status.SetCondition(s2hv1beta1.ActivePromotionCondPreActiveDestroyed, corev1.ConditionTrue,
		"Pre-active environment has been destroyed due to rollback")

	return nil
}

func (c *controller) rePromoteCurrentActive(ctx context.Context, atpComp *s2hv1beta1.ActivePromotion) error {
	teamName := atpComp.Name
	targetNs := atpComp.Status.TargetNamespace
	// get current namespace from team, current namespace might be destroyed due to demoting timeout
	teamComp, err := c.getTeam(ctx, teamName)
	if err != nil {
		return err
	}
	currentNs := teamComp.Status.Namespace.Active

	if currentNs != "" {
		if err := queue.DeleteDemoteFromActiveQueue(c.client, currentNs); err != nil {
			return err
		}

		if err := c.ensureQueuePromotedToActive(teamName, currentNs); err != nil {
			return err
		}
	}

	if err := c.resetTeamNamespace(ctx, teamName, targetNs, currentNs); err != nil {
		return err
	}

	logger.Debug("activepromotion has been rolled back",
		"team", teamName, "status", atpComp.Status.Result, "namespace", currentNs)
	atpComp.Status.SetRollbackStatus(s2hv1beta1.ActivePromotionRollbackSuccess)
	atpComp.Status.SetCondition(s2hv1beta1.ActivePromotionCondFinished, corev1.ConditionTrue,
		"Active promotion process has been finished, rolled back successfully")
	atpComp.SetState(s2hv1beta1.ActivePromotionFinished, "Completed")

	return nil
}

func (c *controller) resetTeamNamespace(ctx context.Context, teamName, targetNs, prevNs string) error {
	teamComp, err := c.getTeam(ctx, teamName)
	if err != nil {
		return err
	}

	if err := c.s2hCtrl.SetActiveNamespace(teamComp, prevNs); err != nil {
		return err
	}

	if err := c.s2hCtrl.SetPreviousActiveNamespace(teamComp, ""); err != nil {
		return err
	}

	return nil
}

func (c *controller) checkRollbackTimeout(ctx context.Context, atpComp *s2hv1beta1.ActivePromotion) error {
	isTimeout, err := c.isTimeoutFromConfig(atpComp, timeoutActivePromotionRollback)
	if err != nil {
		return err
	}

	if isTimeout {
		atpComp.Status.SetRollbackStatus(s2hv1beta1.ActivePromotionRollbackFailure)
		atpComp.Status.SetCondition(s2hv1beta1.ActivePromotionCondFinished, corev1.ConditionFalse,
			"Active promotion process has not been finished, rolled back timeout")
		atpComp.SetState(s2hv1beta1.ActivePromotionFinished, "Rollback timeout")
		if err := c.updateActivePromotion(ctx, atpComp); err != nil {
			return err
		}

		return s2herrors.ErrRollbackActivePromotionTimeout
	}

	return nil
}
