package msg

import (
	"fmt"
	"sort"
	"strings"

	"github.com/agoda-com/samsahai/internal/samsahai/component"
)

type ActivePromotion struct {
	Status                 string
	ServiceOwner           string
	CurrentActiveNamespace string
	Components             []component.Component
}

func (ap *ActivePromotion) NewActivePromotionMessage(showedDetails bool) string {
	statusText := getStatusText(ap.Status)
	message := getMainActivePromotionStatusMessage(ap, statusText)
	message += getOutdatedComponentsMessage(ap.Components, showedDetails)

	return message
}

func getMainActivePromotionStatusMessage(ap *ActivePromotion, status string) string {
	return fmt.Sprintf("*Active Promotion:* %s \n*Owner:* %s \n*Current Active Namespace:* %s\n", status, ap.ServiceOwner, ap.CurrentActiveNamespace)
}

func getStatusText(status string) string {
	switch strings.ToLower(status) {
	case "success":
		return "SUCCESS"
	default:
		return "FAIL"
	}
}

func getOutdatedComponentsMessage(components []component.Component, showedDetails bool) string {
	var text string
	sortComponentsByOutdatedDays(components)
	for _, comp := range components {
		if comp.OutdatedDays > 0 {
			text += fmt.Sprintf("*%s* \n>Not update for %d day(s) \n>Current version: %s \n>New Version: %s\n", comp.Name, comp.OutdatedDays, comp.CurrentVersion, comp.NewVersion)
		} else if showedDetails {
			text += fmt.Sprintf("*%s* \n>Current version: %s\n", comp.Name, comp.CurrentVersion)
		}
	}

	return text
}

func sortComponentsByOutdatedDays(components []component.Component) {
	sort.Slice(components, func(i, j int) bool { return components[i].OutdatedDays > components[j].OutdatedDays })
}
