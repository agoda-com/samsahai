package compare

import (
	"io/ioutil"
	"log"

	"github.com/agoda-com/samsahai/internal/samsahai/command/util"

	"gopkg.in/yaml.v2"

	"github.com/agoda-com/samsahai/internal/samsahai/component/comparator"

	"github.com/spf13/cobra"
)

// versionComponentsArgs defines arguments of version-components command
type versionComponentsArgs struct {
	updatedValuesPath string
	currentValuesPath string
	outputValuesPath  string
}

// versionComponents is global var for binding value
var versionComponents versionComponentsArgs

// variables for binding external functions
var (
	utilParseValuesFileToStruct    = util.ParseValuesFileToStruct
	comparatorGetChangedComponents = comparator.GetChangedComponents
	ioutilWriteFile                = ioutil.WriteFile
)

func versionComponentsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version-components",
		Short: "compare version components to get changes",
		RunE:  compareVersionComponentsCmd,
	}

	cmd.Flags().StringVar(&versionComponents.updatedValuesPath, "updated-values-file", "", "Updated values file (Required)")
	cmd.Flags().StringVar(&versionComponents.currentValuesPath, "current-values-file", "", "Current values file (Required)")
	cmd.Flags().StringVar(&versionComponents.outputValuesPath, "output-values-file", "", "Output values file (Required)")

	if err := cmd.MarkFlagRequired("updated-values-file"); err != nil {
		log.Fatalf("updated-values-file argument not found [%v]", err)
	}
	if err := cmd.MarkFlagRequired("current-values-file"); err != nil {
		log.Fatalf("current-values-file argument not found [%v]", err)
	}
	if err := cmd.MarkFlagRequired("output-values-file"); err != nil {
		log.Fatalf("output-values-file argument not found [%v]", err)
	}

	return cmd
}

// compareVersionComponentsCmd runs when command is executed
func compareVersionComponentsCmd(cmd *cobra.Command, args []string) error {
	newValues, err := utilParseValuesFileToStruct(versionComponents.updatedValuesPath)
	if err != nil {
		return err
	}

	currentValues, err := utilParseValuesFileToStruct(versionComponents.currentValuesPath)
	if err != nil {
		return err
	}

	changedValues := comparatorGetChangedComponents(newValues, currentValues)
	data, err := yaml.Marshal(&changedValues)
	if err != nil {
		return err
	}

	if err := exportOutputToFile(versionComponents.outputValuesPath, data); err != nil {
		return err
	}

	return nil
}

// exportOutputToFile runs to export the result to file
func exportOutputToFile(outputValuesPath string, data []byte) error {
	err := ioutilWriteFile(outputValuesPath, data, 0644)
	if err != nil {
		return err
	}

	return nil
}
