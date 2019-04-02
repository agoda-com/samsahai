package active

import (
	"strings"

	"github.com/agoda-com/samsahai/internal/samsahai/component"
)

// GetCurrentActiveComponents get current components in active namespace
// compares version with new components
func GetCurrentActiveComponents(currentActiveValuesFile, newValuesFile map[string]component.ValuesFile) ([]component.OutdatedComponent, error) {
	outdatedComponents := make([]component.OutdatedComponent, 0)
	for name, val := range currentActiveValuesFile {
		currentVersion := strings.TrimSpace(val.Image.Tag)
		var (
			newVersion   string
			newTimestamp int64
		)
		if _, exist := newValuesFile[name]; exist {
			newVersion = strings.TrimSpace(newValuesFile[name].Image.Tag)
			newTimestamp = newValuesFile[name].Image.Timestamp
		}

		component, err := component.NewOutdatedComponent(name, currentVersion, component.NewOptionNewVersion(newVersion, newTimestamp))
		if err != nil {
			return nil, err
		}

		outdatedComponents = append(outdatedComponents, *component)
	}

	return outdatedComponents, nil
}

// TODO: implements
// GetCurrentActiveNamespaceByOwner gets current active namespace by service owner
func GetCurrentActiveNamespaceByOwner(owner string) (string, error) {
	return "", nil
}
