package plugin

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/agoda-com/samsahai/internal"
	"github.com/agoda-com/samsahai/internal/errors"
	s2hlog "github.com/agoda-com/samsahai/internal/log"
	"github.com/agoda-com/samsahai/internal/util/cmd"
)

const (
	DefaultVerifyTimeout       = 5 * time.Second
	DefaultGetVersionTimeout   = 60 * time.Second
	DefaultGetComponentTimeout = 60 * time.Second
	CmdGetNameArg              = "get-name"
	CmdGetVersionArg           = "get-version"
	CmdEnsureVersionArg        = "ensure-version"
	CmdGetComponentArg         = "get-component"
)

var logger = s2hlog.Log.WithName("plugin")

type plugin struct {
	name                string
	path                string
	cwd                 string
	verifyTimeout       time.Duration
	getVersionTimeout   time.Duration
	getComponentTimeout time.Duration
	logger              s2hlog.Logger
}

func New(path string) (internal.Plugin, error) {
	return NewWithTimeout(path, DefaultVerifyTimeout, DefaultGetVersionTimeout, DefaultGetComponentTimeout)
}

func NewWithTimeout(path string, verifyTimeout, getVersionTimeout, getComponentTimeout time.Duration) (internal.Plugin, error) {
	cwd, _ := os.Getwd()
	p := &plugin{
		path:                path,
		cwd:                 cwd,
		verifyTimeout:       verifyTimeout,
		getVersionTimeout:   getVersionTimeout,
		getComponentTimeout: getComponentTimeout,
	}

	// verify plugin
	name, err := p.verify()
	if err != nil {
		return nil, err
	}

	p.name = name
	p.logger = logger.WithName(name)

	return p, nil
}

func (p *plugin) GetName() string {
	return p.name
}

func (p *plugin) GetVersion(repository, name, pattern string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), p.getVersionTimeout)
	defer cancel()

	output, err := p.executeCmd(ctx, CmdGetVersionArg, repository, name, pattern)
	if err != nil {
		switch err.Error() {
		case errors.ErrNoDesiredComponentVersion.Error():
			return "", errors.ErrNoDesiredComponentVersion
		case errors.ErrRequestTimeout.Error():
			return "", errors.ErrRequestTimeout
		default:
			return "", err
		}
	}
	return output, nil
}

func (p *plugin) EnsureVersion(repository, name, version string) error {
	ctx, cancel := context.WithTimeout(context.Background(), p.getVersionTimeout)
	defer cancel()

	_, err := p.executeCmd(ctx, CmdEnsureVersionArg, repository, name, version)
	if err != nil {
		switch err.Error() {
		case errors.ErrNoDesiredComponentVersion.Error():
			return errors.ErrNoDesiredComponentVersion
		case errors.ErrRequestTimeout.Error():
			return errors.ErrRequestTimeout
		default:
			return err
		}
	}
	return nil
}

func (p *plugin) GetComponentName(name string) string {
	ctx, cancel := context.WithTimeout(context.Background(), p.getVersionTimeout)
	defer cancel()

	output, err := p.executeCmd(ctx, CmdGetComponentArg, name)
	if err != nil {
		logger.Warn(fmt.Sprintf("get-component error: %v", err), "plugin", p.name, "component", name)
		return name
	}
	return output
}

func (p *plugin) executeCmd(ctx context.Context, commandAndArgs ...string) (string, error) {
	outputCh := make(chan string)
	errCh := make(chan error)

	go func() {
		data, err := cmd.ExecuteCommand(ctx, p.cwd, &internal.CommandAndArgs{
			Command: []string{p.path},
			Args:    commandAndArgs,
		})
		if err != nil {
			switch err := err.(type) {
			case *exec.ExitError:
				errStr := strings.TrimRight(string(err.Stderr), "\n")
				errCh <- errors.New(errStr)
			default:
				errCh <- err
			}
			return
		}
		outputCh <- strings.TrimRight(string(data), "\n")
	}()

	// check timeout
	select {
	case <-ctx.Done():
		return "", errors.ErrRequestTimeout
	case err := <-errCh:
		return "", err
	case output := <-outputCh:
		return output, nil
	}
}

func (p *plugin) verify() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), p.verifyTimeout)
	defer cancel()

	// get name
	data, err := cmd.ExecuteCommand(ctx, p.cwd, &internal.CommandAndArgs{
		Command: []string{p.path},
		Args:    []string{CmdGetNameArg},
	})

	if err != nil {
		return "", err
	}

	return strings.TrimRight(string(data), "\n"), nil
}
