package util

import (
	"io/ioutil"

	"github.com/agoda-com/samsahai/internal/samsahai/component"
	"gopkg.in/yaml.v2"
)

// ParseValuesFileToStruct reads values file and parses to component
func ParseValuesFileToStruct(valuesFilePath string) (map[string]component.ValuesFile, error) {
	if valuesFilePath == "" {
		return nil, nil
	}

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
