package msg

import (
	"fmt"
	"strings"

	"github.com/agoda-com/samsahai/internal/samsahai/component"
)

// ActivePromotion manages active promotion report
type ActivePromotion struct {
	Status                 string
	ServiceOwner           string
	CurrentActiveNamespace string
	Components             []component.OutdatedComponent
}

// NewActivePromotion creates a new active promotion
func NewActivePromotion(status, serviceOwner, currentActiveNamespace string, components []component.OutdatedComponent) *ActivePromotion {
	return &ActivePromotion{
		Status:                 status,
		ServiceOwner:           serviceOwner,
		CurrentActiveNamespace: currentActiveNamespace,
		Components:             components,
	}
}

// NewActivePromotionMessage creates an active promotion message
func (ap *ActivePromotion) NewActivePromotionMessage(showedDetails bool) string {
	statusText := getStatusText(ap.Status)
	message := getMainActivePromotionStatusMessage(ap, statusText)
	message += getComponentDetailsMessage(ap.Components, showedDetails)

	return message
}

func getMainActivePromotionStatusMessage(ap *ActivePromotion, status string) string {
	return fmt.Sprintf("*Active Promotion:* %s \n*Owner:* %s \n*Current Active Namespace:* %s\n", status, ap.ServiceOwner, ap.CurrentActiveNamespace)
}

// getStatusText gets the readable active promotion status
func getStatusText(status string) string {
	switch strings.ToLower(status) {
	case "success":
		return "SUCCESS"
	default:
		return "FAIL"
	}
}

// getComponentDetailsMessage gets outdated components message with or without up-to-date components
func getComponentDetailsMessage(components []component.OutdatedComponent, showedDetails bool) string {
	oc := NewOutdatedComponents(components, showedDetails)
	return oc.NewOutdatedComponentsMessage()
}
