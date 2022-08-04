package gitlab

import (
	"fmt"

	s2hv1 "github.com/agoda-com/samsahai/api/v1"
	"github.com/agoda-com/samsahai/internal"
	s2herrors "github.com/agoda-com/samsahai/internal/errors"
	s2hlog "github.com/agoda-com/samsahai/internal/log"
	gitlab "github.com/agoda-com/samsahai/internal/util/gitlab"
	"github.com/agoda-com/samsahai/pkg/samsahai/rpc"
)

var logger = s2hlog.Log.WithName(ReporterName)

const (
	ReporterName = "gitlab"

	LabelNameLogs    = "Samsahai Deployment - Logs"
	LabelNameHistory = "Samsahai Deployment - History"
)

type reporter struct {
	gitlab      gitlab.Gitlab
	gitlabURL   string
	gitlabToken string
}

// NewOption allows specifying various configuration
type NewOption func(*reporter)

// WithGitlabClient specifies a gitlab client to override when creating gitlab reporter
func WithGitlabClient(gl gitlab.Gitlab) NewOption {
	if gl == nil {
		panic("Gitlab client should not be nil")
	}

	return func(r *reporter) {
		r.gitlab = gl
	}
}

// WithGitlabURL specifies a Gitlab url to override when creating gitlab reporter
func WithGitlabURL(url string) NewOption {
	return func(r *reporter) {
		r.gitlabURL = url
	}
}

// WithGitlabToken specifies a Gitlab access token to override when creating gitlab reporter
func WithGitlabToken(token string) NewOption {
	return func(r *reporter) {
		r.gitlabToken = token
	}
}

// New creates a new gitlab reporter
func New(opts ...NewOption) internal.Reporter {
	r := &reporter{}

	// apply the new options
	for _, opt := range opts {
		opt(r)
	}

	return r
}

// NewGitlabClient returns a Gitlab client for publishing commit status to Gitlab
func NewGitlabClient(baseURL, token string) gitlab.Gitlab {
	return gitlab.NewClient(baseURL, token)
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

	teamName := comp.TeamName
	gitlabConfig, err := r.getGitlabConfig(teamName, configCtrl)
	if err != nil {
		return nil
	}

	conf, err := configCtrl.Get(teamName)
	if err != nil {
		return nil
	}
	repository := ""
	for _, b := range conf.Spec.PullRequest.Bundles {
		if b.Name == comp.Name {
			repository = b.Deployment.TestRunner.Gitlab.ProjectID
		}
	}

	//prBundlName := ""
	//if comp.PullRequestComponent != nil {
	//	prBundlName = comp.PullRequestComponent.BundleName
	//}

	//repository := r.getGitlabRepository(teamName, prBundlName, configCtrl)
	r.overrideGitlabCredential(comp.Credential, gitlabConfig)

	commitSHA := comp.PullRequestComponent.CommitSHA
	commitStatus := r.convertCommitStatus(comp.Status)

	// send pull request history URL
	prHistURL := r.getPRHistoryURL(comp)
	prHistDesc := "Samsahai pull request deployment history"
	err = r.post(gitlabConfig, repository, commitSHA, LabelNameHistory, prHistURL, prHistDesc, commitStatus,
		internal.PullRequestQueueType)
	if err != nil {
		return err
	}

	// send pull request logs URL
	prLogsURL := r.getPRLogsURL(comp)
	prLogsDesc := "Samsahai pull request deployment logs"
	err = r.post(gitlabConfig, repository, commitSHA, LabelNameLogs, prLogsURL, prLogsDesc, commitStatus,
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

func (r *reporter) convertCommitStatus(rpcStatus rpc.ComponentUpgrade_UpgradeStatus) gitlab.CommitStatus {
	switch rpcStatus {
	case rpc.ComponentUpgrade_UpgradeStatus_SUCCESS:
		return gitlab.CommitStatusSuccess
	default:
		return gitlab.CommitStatusFailure
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

// SendPullRequestTestRunnerPendingResult implements the reporter SendPullRequestTestRunnerPendingResult function
func (r *reporter) SendPullRequestTestRunnerPendingResult(configCtrl internal.ConfigController,
	prTestRunnerRpt *internal.PullRequestTestRunnerPendingReporter) error {

	teamName := prTestRunnerRpt.TeamName
	gitlabConfig, err := r.getGitlabConfig(teamName, configCtrl)
	if err != nil {
		return nil
	}

	conf, err := configCtrl.Get(teamName)
	if err != nil {
		return nil
	}
	repository := ""
	for _, b := range conf.Spec.PullRequest.Bundles {
		if b.Name == prTestRunnerRpt.BundleName {
			repository = b.Deployment.TestRunner.Gitlab.ProjectID
		}
	}

	r.overrideGitlabCredential(prTestRunnerRpt.Credential, gitlabConfig)

	commitSHA := prTestRunnerRpt.CommitSHA

	// send pull request log status pending while testrunner pipeline is running
	err = r.post(gitlabConfig, repository, commitSHA, LabelNameLogs, "", "",
		gitlab.CommitStatusPending,
		internal.PullRequestQueueType,
	)
	if err != nil {
		return err
	}
	return nil
}

func (r *reporter) post(
	gitlabConfig *s2hv1.ReporterGitlab,
	repository,
	commitSHA,
	labelName,
	targetURL,
	description string,
	commitStatus gitlab.CommitStatus,
	event internal.EventType,
) error {

	if !gitlabConfig.Enabled || repository == "" {
		return nil
	}

	logger.Debug("start publishing commit status to Gitlab",
		"event", event, "repository", repository, "commitSHA", commitSHA, "status", commitStatus)

	gitlabCli := r.gitlab
	if r.gitlab == nil {
		gitlabCli = NewGitlabClient(r.gitlabURL, r.gitlabToken)
	}

	err := gitlabCli.PublishCommitStatus(repository, commitSHA, labelName, targetURL, description, commitStatus)
	if err != nil {
		logger.Error(err, "cannot publish commit status into gitlab", "repository", repository,
			"commitSHA", commitSHA, "labelName", labelName, "targetURL", targetURL, "status", commitStatus)
		return err
	}

	return nil
}

func (r *reporter) getGitlabConfig(teamName string, configCtrl internal.ConfigController) (
	*s2hv1.ReporterGitlab, error) {

	config, err := configCtrl.Get(teamName)
	if err != nil {
		return nil, err
	}

	// no Gitlab configuration
	if config.Status.Used.Reporter == nil || config.Status.Used.Reporter.Gitlab == nil {
		return nil, s2herrors.New("Gitlab configuration not found")
	}

	return config.Status.Used.Reporter.Gitlab, nil
}

func (r *reporter) getGitlabRepository(teamName, prBundleName string,
	configCtrl internal.ConfigController) string {

	config, err := configCtrl.Get(teamName)
	if err != nil {
		return ""
	}

	repository := ""
	if config.Status.Used.PullRequest != nil && prBundleName != "" &&
		len(config.Status.Used.PullRequest.Bundles) > 0 {
		for _, bundle := range config.Status.Used.PullRequest.Bundles {
			if bundle.Name == prBundleName {
				repository = bundle.GitRepository
				break
			}
		}
	}

	return repository
}

func (r *reporter) overrideGitlabCredential(credential s2hv1.Credential,
	gitlabConfig *s2hv1.ReporterGitlab) {

	if credential.Gitlab != nil && credential.Gitlab.Token != "" {
		r.gitlabToken = credential.Gitlab.Token
	}

	if gitlabConfig.BaseURL != "" {
		r.gitlabURL = gitlabConfig.BaseURL
	}
}
