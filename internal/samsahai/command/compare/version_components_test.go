package compare

import (
	"os"
	"testing"

	"github.com/agoda-com/samsahai/internal/samsahai/component"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
)

var (
	utilParseValuesFileToStructCalls    int
	comparatorGetChangedComponentsCalls int
	ioutilWriteFileCalls                int
)

func TestVersionComponentsCmd(t *testing.T) {
	testArgs := map[string]struct {
		flagName          string
		flagExpectedValue string
	}{
		"updated-values-file flag": {
			flagName:          "updated-values-file",
			flagExpectedValue: "updated-values.yaml",
		},
		"current-values-file flag": {
			flagName:          "current-values-file",
			flagExpectedValue: "current-values.yaml",
		},
		"output-values-file flag": {
			flagName:          "output-values-file",
			flagExpectedValue: "output-values.yaml",
		},
	}

	cmd := versionComponentsCmd()
	versionComponents.updatedValuesPath = testArgs["updated-values-file flag"].flagExpectedValue
	versionComponents.currentValuesPath = testArgs["current-values-file flag"].flagExpectedValue
	versionComponents.outputValuesPath = testArgs["output-values-file flag"].flagExpectedValue

	g := NewGomegaWithT(t)
	for _, arg := range testArgs {
		updatedValuesFlag, err := cmd.Flags().GetString(arg.flagName)
		g.Expect(updatedValuesFlag).Should(Equal(arg.flagExpectedValue))
		g.Expect(err).Should(BeNil())
	}
}

func TestCompareVersionComponentsCmd(t *testing.T) {
	oldUtilParseValuesFileToStruct := utilParseValuesFileToStruct
	defer func() { utilParseValuesFileToStruct = oldUtilParseValuesFileToStruct }()
	utilParseValuesFileToStruct = func(valuesFilePath string) (files map[string]component.ValuesFile, e error) {
		utilParseValuesFileToStructCalls++
		return nil, nil
	}

	oldComparatorGetChangedComponents := comparatorGetChangedComponents
	defer func() { comparatorGetChangedComponents = oldComparatorGetChangedComponents }()
	comparatorGetChangedComponents = func(updatedComponents map[string]component.ValuesFile, currentComponents map[string]component.ValuesFile) map[string]component.ValuesFile {
		comparatorGetChangedComponentsCalls++
		return nil
	}

	oldIoutilWriteFile := ioutilWriteFile
	defer func() { ioutilWriteFile = oldIoutilWriteFile }()
	ioutilWriteFile = func(filename string, data []byte, perm os.FileMode) error {
		ioutilWriteFileCalls++
		return nil
	}

	var mockStrings []string
	var mockCmd *cobra.Command
	err := compareVersionComponentsCmd(mockCmd, mockStrings)

	g := NewGomegaWithT(t)
	g.Expect(utilParseValuesFileToStructCalls).Should(Equal(2))
	g.Expect(comparatorGetChangedComponentsCalls).Should(Equal(1))
	g.Expect(ioutilWriteFileCalls).Should(Equal(1))
	g.Expect(err).Should(BeNil())
}
