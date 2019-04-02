package send

import (
	"io/ioutil"

	"github.com/agoda-com/samsahai/internal/samsahai/component"
	"github.com/agoda-com/samsahai/internal/samsahai/namespace/active"
	"gopkg.in/yaml.v2"
)

// parseValuesfileToStruct reads values file and parses to component
func parseValuesfileToStruct(valuesFilePath string) (map[string]component.ValuesFile, error) {
	b, err := ioutil.ReadFile(valuesFilePath)
	if err != nil {
		return nil, err
	}

	values := make(map[string]component.ValuesFile)
	if err := yaml.Unmarshal(b, &values); err != nil {
		return nil, err
	}

	return values, nil
}

func getActiveComponentsFromValuesFile(currentActiveValuesPath, newValuesPath string) ([]component.OutdatedComponent, error) {
	if currentActiveValuesPath == "" {
		return nil, nil
	}

	var (
		activeValues, newValues map[string]component.ValuesFile
		err                     error
	)

	activeValues, err = parseValuesfileToStruct(currentActiveValuesPath)
	if err != nil {
		return nil, err
	}

	if newValuesPath != "" {
		newValues, err = parseValuesfileToStruct(newValuesPath)
		if err != nil {
			return nil, err
		}
	}

	return active.GetCurrentActiveComponents(activeValues, newValues)
}
