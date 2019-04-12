package send

import (
	"github.com/agoda-com/samsahai/internal/samsahai/command/util"
	"github.com/agoda-com/samsahai/internal/samsahai/component"
	"github.com/agoda-com/samsahai/internal/samsahai/namespace/active"
)

func getActiveComponentsFromValuesFile(currentActiveValuesPath, newValuesPath string) ([]component.OutdatedComponent, error) {
	if currentActiveValuesPath == "" {
		return nil, nil
	}

	var (
		activeValues, newValues map[string]component.ValuesFile
		err                     error
	)

	activeValues, err = util.ParseValuesFileToStruct(currentActiveValuesPath)
	if err != nil {
		return nil, err
	}

	if newValuesPath != "" {
		newValues, err = util.ParseValuesFileToStruct(newValuesPath)
		if err != nil {
			return nil, err
		}
	}

	return active.GetCurrentActiveComponents(activeValues, newValues)
}
