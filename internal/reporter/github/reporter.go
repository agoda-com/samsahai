package github

import (
	"fmt"

	s2hv1 "github.com/agoda-com/samsahai/api/v1"
	"github.com/agoda-com/samsahai/internal"
	s2herrors "github.com/agoda-com/samsahai/internal/errors"
	s2hlog "github.com/agoda-com/samsahai/internal/log"
	"github.com/agoda-com/samsahai/internal/util/github"
	"github.com/agoda-com/samsahai/pkg/samsahai/rpc"
)

var logger = s2hlog.Log.WithName(ReporterName)

const (
	ReporterName = "github"

	LabelNameLogs    = "Samsahai Deployment - Logs"
	LabelNameHistory = "Samsahai Deployment - History"
)

type reporter struct {
	github      github.Github
	githubURL   string
	githubToken string
}

// NewOption allows specifying various configuration
type NewOption func(*reporter)

// WithGithubClient specifies a github client to override when creating Github reporter
func WithGithubClient(github github.Github) NewOption {
	if github == nil {
		panic("Github client should not be nil")
	}

	return func(r *reporter) {
		r.github = github
	}
}

// WithGithubURL specifies a github url to override when creating Github reporter
func WithGithubURL(url string) NewOption {
	return func(r *reporter) {
		r.githubURL = url
	}
}

// WithGithubToken specifies a github access token to override when creating Github reporter
func WithGithubToken(token string) NewOption {
	return func(r *reporter) {
		r.githubToken = token
	}
}

// New creates a new Github reporter
func New(opts ...NewOption) internal.Reporter {
	r := &reporter{}

	// apply the new options
	for _, opt := range opts {
		opt(r)
	}

	return r
}

// NewGithubClient returns a github client for publishing commit status to github
func NewGithubClient(baseURL, token string) github.Github {
	return github.NewClient(baseURL, token)
}

// GetName returns a reporter type
func (r *reporter) GetName() string {
	return ReporterName
}

// SendComponentUpgrade implements the reporter SendComponentUpgrade function
func (r *reporter) SendComponentUpgrade(configCtrl internal.ConfigController,
	comp *internal.ComponentUpgradeReporter) error {

	// does not support
	return nil
}

// SendPullRequestQueue implements the reporter SendPullRequestQueue function
func (r *reporter) SendPullRequestQueue(configCtrl internal.ConfigController,
	comp *internal.ComponentUpgradeReporter) error {

	githubConfig, err := r.getGithubConfig(comp.TeamName, configCtrl)
	if err != nil {
		return nil
	}

	repository := r.getGithubRepository(comp, configCtrl)
	r.overrideGithubCredential(comp, githubConfig)

	commitSHA := comp.PullRequestComponent.CommitSHA
	commitStatus := r.convertCommitStatus(comp.Status)

	// send pull request history URL
	prHistURL := r.getPRHistoryURL(comp)
	prHistDesc := "Samsahai pull request deployment history"
	err = r.post(githubConfig, repository, commitSHA, LabelNameHistory, prHistURL, prHistDesc, commitStatus,
		internal.PullRequestQueueType)
	if err != nil {
		return err
	}

	// send pull request logs URL
	prLogsURL := r.getPRLogsURL(comp)
	prLogsDesc := "Samsahai pull request deployment logs"
	err = r.post(githubConfig, repository, commitSHA, LabelNameLogs, prLogsURL, prLogsDesc, commitStatus,
		internal.PullRequestQueueType)
	if err != nil {
		return err
	}

	return nil
}

// SendActiveEnvironmentDeleted implements the reporter SendActiveEnvironmentDeleted function
func (r *reporter) SendActiveEnvironmentDeleted(configCtrl internal.ConfigController,
	activeNsDeletedRpt *internal.ActiveEnvironmentDeletedReporter) error {

	// does not support
	return nil
}

func (r *reporter) convertCommitStatus(rpcStatus rpc.ComponentUpgrade_UpgradeStatus) github.CommitStatus {
	switch rpcStatus {
	case rpc.ComponentUpgrade_UpgradeStatus_SUCCESS:
		return github.CommitStatusSuccess
	default:
		return github.CommitStatusFailure
	}
}

func (r *reporter) getPRHistoryURL(comp *internal.ComponentUpgradeReporter) string {
	return fmt.Sprintf("%s/teams/%s/pullrequest/queue/histories/%s",
		comp.SamsahaiExternalURL, comp.TeamName, comp.QueueHistoryName)
}

func (r *reporter) getPRLogsURL(comp *internal.ComponentUpgradeReporter) string {
	return fmt.Sprintf("%s/teams/%s/pullrequest/queue/histories/%s/log",
		comp.SamsahaiExternalURL, comp.TeamName, comp.QueueHistoryName)
}

// SendActivePromotionStatus implements the reporter SendActivePromotionStatus function
func (r *reporter) SendActivePromotionStatus(configCtrl internal.ConfigController,
	atpRpt *internal.ActivePromotionReporter) error {

	// does not support
	return nil
}

// SendImageMissing implements the reporter SendImageMissing function
func (r *reporter) SendImageMissing(configCtrl internal.ConfigController,
	imageMissingRpt *internal.ImageMissingReporter) error {

	// does not support
	return nil
}

// SendPullRequestTriggerResult implements the reporter SendPullRequestTriggerResult function
func (r *reporter) SendPullRequestTriggerResult(configCtrl internal.ConfigController,
	prTriggerRpt *internal.PullRequestTriggerReporter) error {

	// does not support
	return nil
}

func (r *reporter) post(githubConfig *s2hv1.ReporterGithub,
	repository, commitSHA, labelName, targetURL, description string,
	commitStatus github.CommitStatus, event internal.EventType) error {

	if !githubConfig.Enabled || repository == "" {
		return nil
	}

	logger.Debug("start publishing commit status to Github",
		"event", event, "repository", repository, "commitSHA", commitSHA, "status", commitStatus)

	githubCli := r.github
	if r.github == nil {
		githubCli = NewGithubClient(r.githubURL, r.githubToken)
	}

	err := githubCli.PublishCommitStatus(repository, commitSHA, labelName, targetURL, description, commitStatus)
	if err != nil {
		logger.Error(err, "cannot publish commit status into github", "repository", repository,
			"commitSHA", commitSHA, "labelName", labelName, "targetURL", targetURL, "status", commitStatus)
		return err
	}

	return nil
}

func (r *reporter) getGithubConfig(teamName string, configCtrl internal.ConfigController) (
	*s2hv1.ReporterGithub, error) {

	config, err := configCtrl.Get(teamName)
	if err != nil {
		return nil, err
	}

	// no Github configuration
	if config.Status.Used.Reporter == nil || config.Status.Used.Reporter.Github == nil {
		return nil, s2herrors.New("github configuration not found")
	}

	return config.Status.Used.Reporter.Github, nil
}

func (r *reporter) getGithubRepository(comp *internal.ComponentUpgradeReporter,
	configCtrl internal.ConfigController) string {

	config, err := configCtrl.Get(comp.TeamName)
	if err != nil {
		return ""
	}

	// no Github configuration
	if comp.PullRequestComponent == nil {
		return ""
	}

	repository := ""
	prCompName := comp.PullRequestComponent.ComponentName
	//// TODO: pohfy, update here
	if config.Status.Used.PullRequest != nil && len(config.Status.Used.PullRequest.Components) > 0 {
		for _, comp := range config.Status.Used.PullRequest.Components {
			if comp.Name == prCompName {
				repository = comp.GitRepository
				break
			}
		}
	}

	return repository
}

func (r *reporter) overrideGithubCredential(comp *internal.ComponentUpgradeReporter,
	githubConfig *s2hv1.ReporterGithub) {

	if comp.Credential.Github != nil && comp.Credential.Github.Token != "" {
		r.githubToken = comp.Credential.Github.Token
	}

	if githubConfig.BaseURL != "" {
		r.githubURL = githubConfig.BaseURL
	}
}
