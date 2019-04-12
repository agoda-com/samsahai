package comparator

import (
	"github.com/agoda-com/samsahai/internal/samsahai/component"
)

// GetChangedComponents gets changed components from values of updated and current components
func GetChangedComponents(updatedComponents map[string]component.ValuesFile, currentComponents map[string]component.ValuesFile) map[string]component.ValuesFile {
	changedComponents := make(map[string]component.ValuesFile)
	for updatedCompName, updatedCompImage := range updatedComponents {
		currentCompImage := currentComponents[updatedCompName].Image
		if currentCompImage.Tag != updatedCompImage.Image.Tag || currentCompImage.Repository != updatedCompImage.Image.Repository {
			changedComponents[updatedCompName] = updatedComponents[updatedCompName]
		}
	}

	return changedComponents
}
