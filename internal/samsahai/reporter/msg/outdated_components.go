package msg

import (
	"fmt"
	"sort"

	"github.com/agoda-com/samsahai/internal/samsahai/component"
)

// OutdatedComponents manages outdated components report
type OutdatedComponents struct {
	Components    []component.OutdatedComponent
	ShowedDetails bool
}

// NewImageMissing creates a new image missing
func NewOutdatedComponents(components []component.OutdatedComponent, showedDetails bool) *OutdatedComponents {
	return &OutdatedComponents{Components: components, ShowedDetails: showedDetails}
}

// NewOutdatedComponentsMessage creates an outdated components message
func (oc *OutdatedComponents) NewOutdatedComponentsMessage() string {
	message := fmt.Sprintf("*Outdated Components* \n%s", getOutdatedComponentsMessage(oc.Components, oc.ShowedDetails))
	return message
}

func getOutdatedComponentsMessage(components []component.OutdatedComponent, showedDetails bool) string {
	var text string
	sortComponentsByOutdatedDays(components)
	for _, comp := range components {
		compName := comp.CurrentComponent.Name
		compCurrentVersion := comp.CurrentComponent.Version
		compNewVersion := comp.NewComponent.Version
		if comp.OutdatedDays > 0 {
			text += fmt.Sprintf("*%s* \n>Not update for %d day(s) \n>Current version: %s \n>New Version: %s\n", compName, comp.OutdatedDays, compCurrentVersion, compNewVersion)
		} else if showedDetails {
			text += fmt.Sprintf("*%s* \n>Current version: %s\n", compName, compCurrentVersion)
		}
	}

	return text
}

// sortComponentsByOutdatedDays sorts components by outdated days descending order
func sortComponentsByOutdatedDays(components []component.OutdatedComponent) {
	sort.Slice(components, func(i, j int) bool { return components[i].OutdatedDays > components[j].OutdatedDays })
}
