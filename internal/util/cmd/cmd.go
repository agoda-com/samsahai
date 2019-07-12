package cmd

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/pkg/errors"

	"github.com/agoda-com/samsahai/internal"
	s2hlog "github.com/agoda-com/samsahai/internal/log"
	"github.com/agoda-com/samsahai/internal/util/template"
)

var logger = s2hlog.S2HLog.WithName("Shell-util")

// ExecuteCommand executes command at defined executed path
func ExecuteCommand(ctx context.Context, exePath string, cmdObj *internal.CommandAndArgs) ([]byte, error) {
	command, args, err := parseCommand(cmdObj)
	if err != nil {
		return []byte{}, err
	}

	return execute(ctx, exePath, command, args...)
}

func RenderTemplate(commands, args []string, obj interface{}) *internal.CommandAndArgs {
	// render from template
	cmdObj := &internal.CommandAndArgs{}
	for _, c := range commands {
		cmdObj.Command = append(cmdObj.Command, template.TextRender("CommandsRendering", c, obj))
	}

	for _, c := range args {
		cmdObj.Args = append(cmdObj.Args, template.TextRender("ArgsRendering", c, obj))
	}

	return cmdObj
}

func parseCommand(cmdObj *internal.CommandAndArgs) (command string, args []string, err error) {
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
		logger.Error(err, "cannot execute command", "command", command, "args", args)
		return []byte{}, err
	}

	return out, nil
}
