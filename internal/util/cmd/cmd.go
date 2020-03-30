package cmd

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/pkg/errors"

	s2hv1beta1 "github.com/agoda-com/samsahai/api/v1beta1"
	"github.com/agoda-com/samsahai/internal/util/template"
)

// ExecuteCommand executes command at defined executed path
func ExecuteCommand(ctx context.Context, exePath string, cmdObj *s2hv1beta1.CommandAndArgs) ([]byte, error) {
	command, args, err := parseCommand(cmdObj)
	if err != nil {
		return []byte{}, err
	}

	return execute(ctx, exePath, command, args...)
}

func RenderTemplate(commands, args []string, obj interface{}) *s2hv1beta1.CommandAndArgs {
	// render from template
	cmdObj := &s2hv1beta1.CommandAndArgs{}
	for _, c := range commands {
		cmdObj.Command = append(cmdObj.Command, template.TextRender("CommandsRendering", c, obj))
	}

	for _, c := range args {
		cmdObj.Args = append(cmdObj.Args, template.TextRender("ArgsRendering", c, obj))
	}

	return cmdObj
}

func parseCommand(cmdObj *s2hv1beta1.CommandAndArgs) (command string, args []string, err error) {
	if cmdObj == nil || len(cmdObj.Command) == 0 {
		err = errors.Wrap(fmt.Errorf("no command to execute"), "cannot parse data to command")
		return
	}

	command = cmdObj.Command[0]
	if len(cmdObj.Command) > 1 {
		args = cmdObj.Command[1:]
	}

	if len(cmdObj.Args) > 0 {
		args = append(args, cmdObj.Args...)
	}

	return
}

func execute(ctx context.Context, exePath, command string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = exePath

	out, err := cmd.Output()
	if err != nil {
		return out, err
	}

	return out, nil
}
