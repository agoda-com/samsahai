package samsahai

import (
	"github.com/agoda-com/samsahai/internal"
)

func (c *controller) NotifyActivePromotion(atpRpt *internal.ActivePromotionReporter) {
	configCtrl := c.GetConfigController()

	for _, reporter := range c.reporters {
		if err := reporter.SendActivePromotionStatus(configCtrl, atpRpt); err != nil {
			logger.Error(err, "cannot send component upgrade failure report")
		}
	}
}
