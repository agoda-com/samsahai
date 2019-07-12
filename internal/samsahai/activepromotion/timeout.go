package activepromotion

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/agoda-com/samsahai/internal"
	s2hv1beta1 "github.com/agoda-com/samsahai/pkg/apis/env/v1beta1"
)

type timeoutType string

const (
	timeoutActivePromotion           timeoutType = "ActivePromotionTimeout"
	timeoutActiveDemotion            timeoutType = "ActiveDemotionTimeout"
	timeoutActivePromotionRollback   timeoutType = "ActivePromotionRollbackTimeout"
	timeoutActiveDemotionForRollback timeoutType = "ActiveDemotionForRollbackTimeout"
)

func (c *controller) isTimeoutFromConfig(atpComp *s2hv1beta1.ActivePromotion, timeoutType timeoutType) (bool, error) {
	configMgr, err := c.getTeamConfiguration(atpComp.Name)
	if err != nil {
		return false, err
	}

	var timeout metav1.Duration
	var startedTime *metav1.Time
	now := metav1.Now()
	switch timeoutType {
	case timeoutActivePromotion:
		timeout = c.getActivePromotionTimeout(configMgr)
		startedTime = atpComp.Status.GetConditionLatestTime(s2hv1beta1.ActivePromotionCondStarted)
	case timeoutActiveDemotion:
		timeout = c.getActiveDemotionTimeout(configMgr)
		startedTime = atpComp.Status.GetConditionLatestTime(s2hv1beta1.ActivePromotionCondActiveDemotionStarted)
	case timeoutActivePromotionRollback:
		timeout = c.getActivePromotionRollbackTimeout(configMgr)
		startedTime = atpComp.Status.GetConditionLatestTime(s2hv1beta1.ActivePromotionCondRollbackStarted)
	case timeoutActiveDemotionForRollback:
		timeout = c.getActiveDemotionTimeout(configMgr)
		startedTime = atpComp.Status.GetConditionLatestTime(s2hv1beta1.ActivePromotionCondRollbackStarted)
	}

	if startedTime == nil {
		return false, nil
	}
	if now.Sub(startedTime.Time) > timeout.Duration {
		return true, nil
	}

	return false, nil
}

func (c *controller) getActiveDemotionTimeout(configMgr internal.ConfigManager) metav1.Duration {
	timeout := c.configs.ActivePromotion.DemotionTimeout
	if cfg := configMgr.Get(); cfg.ActivePromotion != nil && cfg.ActivePromotion.DemotionTimeout.Duration != 0 {
		timeout = cfg.ActivePromotion.DemotionTimeout
	}

	return timeout
}

func (c *controller) getActivePromotionTimeout(configMgr internal.ConfigManager) metav1.Duration {
	timeout := c.configs.ActivePromotion.Timeout
	if cfg := configMgr.Get(); cfg.ActivePromotion != nil && cfg.ActivePromotion.Timeout.Duration != 0 {
		timeout = cfg.ActivePromotion.Timeout
	}

	return timeout
}

func (c *controller) getActivePromotionRollbackTimeout(configMgr internal.ConfigManager) metav1.Duration {
	timeout := c.configs.ActivePromotion.RollbackTimeout
	if cfg := configMgr.Get(); cfg.ActivePromotion != nil && cfg.ActivePromotion.RollbackTimeout.Duration != 0 {
		timeout = cfg.ActivePromotion.RollbackTimeout
	}

	return timeout
}
