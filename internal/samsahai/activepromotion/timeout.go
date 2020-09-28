package activepromotion

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	s2hv1beta1 "github.com/agoda-com/samsahai/api/v1beta1"
	"github.com/agoda-com/samsahai/internal"
)

type timeoutType string

const (
	timeoutActivePromotion           timeoutType = "ActivePromotionTimeout"
	timeoutActiveDemotion            timeoutType = "ActiveDemotionTimeout"
	timeoutActivePromotionRollback   timeoutType = "ActivePromotionRollbackTimeout"
	timeoutActiveDemotionForRollback timeoutType = "ActiveDemotionForRollbackTimeout"
)

func (c *controller) isTimeoutFromConfig(atpComp *s2hv1beta1.ActivePromotion, timeoutType timeoutType) (bool, error) {
	configCtrl := c.s2hCtrl.GetConfigController()

	var timeout metav1.Duration
	var startedTime *metav1.Time
	now := metav1.Now()
	switch timeoutType {
	case timeoutActivePromotion:
		timeout = c.getActivePromotionTimeout(atpComp.Name, configCtrl)
		startedTime = atpComp.Status.GetConditionLatestTime(s2hv1beta1.ActivePromotionCondStarted)
	case timeoutActiveDemotion:
		timeout = c.getActiveDemotionTimeout(atpComp.Name, configCtrl)
		startedTime = atpComp.Status.GetConditionLatestTime(s2hv1beta1.ActivePromotionCondActiveDemotionStarted)
	case timeoutActivePromotionRollback:
		timeout = c.getActivePromotionRollbackTimeout(atpComp.Name, configCtrl)
		startedTime = atpComp.Status.GetConditionLatestTime(s2hv1beta1.ActivePromotionCondRollbackStarted)
	case timeoutActiveDemotionForRollback:
		timeout = c.getActiveDemotionTimeout(atpComp.Name, configCtrl)
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

func (c *controller) getActiveDemotionTimeout(teamName string, configCtrl internal.ConfigController) metav1.Duration {
	timeout := c.configs.ActivePromotion.DemotionTimeout
	config, err := configCtrl.Get(teamName)
	if err != nil {
		return timeout
	}

	if config.Status.Used.ActivePromotion != nil && config.Status.Used.ActivePromotion.DemotionTimeout.Duration != 0 {
		timeout = config.Status.Used.ActivePromotion.DemotionTimeout
	}

	return timeout
}

func (c *controller) getActivePromotionTimeout(teamName string, configCtrl internal.ConfigController) metav1.Duration {
	timeout := c.configs.ActivePromotion.Timeout
	config, err := configCtrl.Get(teamName)
	if err != nil {
		return timeout
	}

	if config.Status.Used.ActivePromotion != nil && config.Status.Used.ActivePromotion.Timeout.Duration != 0 {
		timeout = config.Status.Used.ActivePromotion.Timeout
	}

	return timeout
}

func (c *controller) getActivePromotionRollbackTimeout(teamName string, configCtrl internal.ConfigController) metav1.Duration {
	timeout := c.configs.ActivePromotion.RollbackTimeout
	config, err := configCtrl.Get(teamName)
	if err != nil {
		return timeout
	}

	if config.Status.Used.ActivePromotion != nil && config.Status.Used.ActivePromotion.RollbackTimeout.Duration != 0 {
		timeout = config.Status.Used.ActivePromotion.RollbackTimeout
	}

	return timeout
}
