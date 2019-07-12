package samsahai

import (
	"github.com/pkg/errors"

	"github.com/agoda-com/samsahai/internal"
	s2herrors "github.com/agoda-com/samsahai/internal/errors"
)

func (c *controller) NotifyActivePromotion(atpRpt *internal.ActivePromotionReporter) error {
	configMgr, ok := c.GetTeamConfigManager(atpRpt.TeamName)
	if !ok {
		return errors.Wrap(s2herrors.ErrLoadConfiguration, "cannot load configuration")
	}

	for _, reporter := range c.reporters {
		if err := reporter.SendActivePromotionStatus(configMgr, atpRpt); err != nil {
			logger.Error(err, "cannot send component upgrade failure report")
		}
	}

	return nil
}
