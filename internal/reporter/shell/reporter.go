package shell

import (
	"context"
	"fmt"
	"time"

	s2hv1beta1 "github.com/agoda-com/samsahai/api/v1beta1"
	"github.com/agoda-com/samsahai/internal"
	s2herrors "github.com/agoda-com/samsahai/internal/errors"
	s2hlog "github.com/agoda-com/samsahai/internal/log"
	"github.com/agoda-com/samsahai/internal/util/cmd"
)

var logger = s2hlog.Log.WithName(ReporterName)

const (
	ReporterName = "shell"

	ExecutionTimeout = 60 * time.Second
)

type execCommand func(ctx context.Context, configPath string, cmdObj *s2hv1beta1.CommandAndArgs) ([]byte, error)

type reporter struct {
	timeout     time.Duration
	execCommand execCommand
}

// NewOption allows specifying various configuration
type NewOption func(*reporter)

func WithExecCommand(execCommand execCommand) NewOption {
	return func(r *reporter) {
		r.execCommand = execCommand
	}
}

// WithTimeout specifies timeout to override when executing shell command
func WithTimeout(timeout time.Duration) NewOption {
	return func(r *reporter) {
		r.timeout = timeout
	}
}

// New creates a new shell reporter
func New(opts ...NewOption) internal.Reporter {
	r := &reporter{
		timeout:     ExecutionTimeout,
		execCommand: cmd.ExecuteCommand,
	}

	// apply the new options
	for _, opt := range opts {
		opt(r)
	}

	return r
}

// GetName returns shell type
func (r *reporter) GetName() string {
	return ReporterName
}

// SendComponentUpgrade implements the reporter SendComponentUpgrade function
func (r *reporter) SendComponentUpgrade(configCtrl internal.ConfigController, comp *internal.ComponentUpgradeReporter) error {
	config, err := configCtrl.Get(comp.TeamName)
	if err != nil {
		return err
	}

	if config.Status.Used.Reporter == nil ||
		config.Status.Used.Reporter.Shell == nil ||
		config.Status.Used.Reporter.Shell.ComponentUpgrade == nil {
		return nil
	}

	cmdObj := cmd.RenderTemplate(config.Status.Used.Reporter.Shell.ComponentUpgrade.Command,
		config.Status.Used.Reporter.Shell.ComponentUpgrade.Args, comp)
	if err := r.execute(cmdObj, internal.ComponentUpgradeType); err != nil {
		return err
	}

	return nil
}

// SendPullRequestQueue implements the reporter SendPullRequestQueue function
func (r *reporter) SendPullRequestQueue(configCtrl internal.ConfigController, comp *internal.ComponentUpgradeReporter) error {
	config, err := configCtrl.Get(comp.TeamName)
	if err != nil {
		return err
	}

	if config.Spec.Reporter == nil ||
		config.Spec.Reporter.Shell == nil ||
		config.Spec.Reporter.Shell.PullRequestQueue == nil {
		return nil
	}

	cmdObj := cmd.RenderTemplate(config.Spec.Reporter.Shell.PullRequestQueue.Command,
		config.Spec.Reporter.Shell.PullRequestQueue.Args, comp)
	if err := r.execute(cmdObj, internal.PullRequestQueueType); err != nil {
		return err
	}

	return nil
}

// SendActivePromotionStatus implements the reporter SendActivePromotionStatus function
func (r *reporter) SendActivePromotionStatus(configCtrl internal.ConfigController, atpRpt *internal.ActivePromotionReporter) error {
	config, err := configCtrl.Get(atpRpt.TeamName)
	if err != nil {
		return err
	}

	if config.Status.Used.Reporter == nil ||
		config.Status.Used.Reporter.Shell == nil ||
		config.Status.Used.Reporter.Shell.ActivePromotion == nil {
		return nil
	}

	cmdObj := cmd.RenderTemplate(config.Status.Used.Reporter.Shell.ActivePromotion.Command,
		config.Status.Used.Reporter.Shell.ActivePromotion.Args, atpRpt)
	if err := r.execute(cmdObj, internal.ActivePromotionType); err != nil {
		return err
	}

	return nil

}

// SendImageMissing implements the reporter SendImageMissing function
func (r *reporter) SendImageMissing(configCtrl internal.ConfigController, imageMissingRpt *internal.ImageMissingReporter) error {
	config, err := configCtrl.Get(imageMissingRpt.TeamName)
	if err != nil {
		return err
	}

	if config.Status.Used.Reporter == nil ||
		config.Status.Used.Reporter.Shell == nil ||
		config.Status.Used.Reporter.Shell.ImageMissing == nil {
		return nil
	}

	cmdObj := cmd.RenderTemplate(config.Status.Used.Reporter.Shell.ImageMissing.Command,
		config.Status.Used.Reporter.Shell.ImageMissing.Args, imageMissingRpt)
	if err := r.execute(cmdObj, internal.ImageMissingType); err != nil {
		return err
	}

	return nil
}

// SendPullRequestTriggerResult implements the reporter SendPullRequestTriggerResult function
func (r *reporter) SendPullRequestTriggerResult(configCtrl internal.ConfigController, prTriggerRpt *internal.PullRequestTriggerReporter) error {
	config, err := configCtrl.Get(prTriggerRpt.TeamName)
	if err != nil {
		return err
	}

	if config.Spec.Reporter == nil ||
		config.Spec.Reporter.Shell == nil ||
		config.Spec.Reporter.Shell.PullRequestTrigger == nil {
		return nil
	}

	cmdObj := cmd.RenderTemplate(config.Spec.Reporter.Shell.PullRequestTrigger.Command,
		config.Spec.Reporter.Shell.PullRequestTrigger.Args, prTriggerRpt)
	if err := r.execute(cmdObj, internal.PullRequestTriggerType); err != nil {
		return err
	}

	return nil
}

func (r *reporter) execute(cmdObj *s2hv1beta1.CommandAndArgs, event internal.EventType) error {
	logger.Debug("start executing command", "event", event)

	ctx, cancelFunc := context.WithTimeout(context.Background(), r.timeout)
	defer cancelFunc()

	errCh := make(chan error)
	go func() {
		out, err := r.execCommand(context.TODO(), ".", cmdObj)
		logger.Debug(fmt.Sprintf("output: %s", out), "event", event)
		errCh <- err
	}()

	select {
	case <-ctx.Done():
		logger.Error(s2herrors.ErrExecutionTimeout, fmt.Sprintf("executing took more than %v", r.timeout))
		return s2herrors.ErrExecutionTimeout
	case err := <-errCh:
		return err
	}
}
