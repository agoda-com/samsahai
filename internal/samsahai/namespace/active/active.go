package active

import (
	"strings"
	"time"

	"github.com/agoda-com/samsahai/internal/samsahai/component"
)

func GetCurrentActiveComponents(currentActiveValuesFile, newValuesFile map[string]component.ValuesFile) ([]component.Component, error) {
	var components []component.Component
	for name, val := range currentActiveValuesFile {
		currentVersion := strings.TrimSpace(val.Image.Tag)
		var (
			newVersion   string
			outdatedDays int
		)
		if _, exist := newValuesFile[name]; exist {
			newVersion = strings.TrimSpace(newValuesFile[name].Image.Tag)
			if newVersion != currentVersion {
				outdatedDays = getOutdatedDays(newValuesFile[name].Image.Timestamp)
			}
		}

		component, err := component.NewComponent(name, currentVersion, newVersion, outdatedDays)
		if err != nil {
			return nil, err
		}

		components = append(components, *component)
	}

	return components, nil
}

// TODO: implements
func GetCurrentActiveNamespaceByOwner(owner string) (string, error) {
	return "", nil
}

func getOutdatedDays(newVersionTimestamp int64) int {
	now := time.Now()
	newDate := time.Unix(newVersionTimestamp, 0)
	days := now.Sub(newDate).Hours() / 24
	return int(days) + 1
}
