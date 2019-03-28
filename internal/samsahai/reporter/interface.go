package reporter

import (
	"github.com/agoda-com/samsahai/internal/samsahai/component"
)

const (
	FailedComponentUpgrade = "failed-component-upgrade"
	ActivePromotion        = "active-promotion"
	OutdatedComponent      = "outdated-component"
	ImageMissing           = "image-missing"
)

type Reporter interface {
	// SendMessage send generic message
	SendMessage(message string) error

	// SendFailedComponentUpgrade send details of failed component upgrade
	SendFailedComponentUpgrade() error

	// SendActivePromotionStatus send active promotion status
	SendActivePromotionStatus(status, currentActiveNamespace, serviceOwner string, components []component.Component, showedDetails bool) error

	// SendOutdatedComponent send outdated components
	SendOutdatedComponents() error

	// SendImageMissingList send image missing list
	SendImageMissingList(images []component.Image) error
}
