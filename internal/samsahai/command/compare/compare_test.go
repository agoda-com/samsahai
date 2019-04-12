package compare_test

import (
	"testing"

	"github.com/agoda-com/samsahai/internal/samsahai/command/compare"
	. "github.com/onsi/gomega"
)

func TestCmd(t *testing.T) {
	const requiredFieldAnnotation = "cobra_annotation_bash_completion_one_required_flag"
	cmd := compare.Cmd()
	subCommand := cmd.Commands()

	testArgs := map[string]struct {
		flagName                  string
		flagExpectedRequiredField string
	}{
		"updated-values-file flag": {
			flagName:                  "updated-values-file",
			flagExpectedRequiredField: "true",
		},
		"current-values-file flag": {
			flagName:                  "current-values-file",
			flagExpectedRequiredField: "true",
		},
		"output-values-file flag": {
			flagName:                  "output-values-file",
			flagExpectedRequiredField: "true",
		},
	}

	g := NewGomegaWithT(t)
	g.Expect(cmd.HasSubCommands()).Should(BeTrue())

	for _, arg := range testArgs {
		g.Expect(subCommand[0].Flag(arg.flagName)).ShouldNot(BeNil())
		g.Expect(subCommand[0].Flag(arg.flagName).Annotations[requiredFieldAnnotation][0]).Should(Equal(arg.flagExpectedRequiredField))
	}
}
