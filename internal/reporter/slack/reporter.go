package slack

import (
	"strings"

	"github.com/nlopes/slack"

	s2hv1beta1 "github.com/agoda-com/samsahai/api/v1beta1"
	"github.com/agoda-com/samsahai/internal"
	s2herrors "github.com/agoda-com/samsahai/internal/errors"
	s2hlog "github.com/agoda-com/samsahai/internal/log"
	slackutil "github.com/agoda-com/samsahai/internal/util/slack"
	"github.com/agoda-com/samsahai/internal/util/template"
	"github.com/agoda-com/samsahai/pkg/samsahai/rpc"
)

var logger = s2hlog.Log.WithName(ReporterName)

const (
	ReporterName = "slack"
	username     = "Samsahai Notification"

	statusSuccess = "success"
	statusFailure = "failure"
)

type reporter struct {
	slack slackutil.Slack
}

// NewOption allows specifying various configuration
type NewOption func(*reporter)

// WithSlackClient specifies slack client to override when creating slack reporter
func WithSlackClient(slack slackutil.Slack) NewOption {
	if slack == nil {
		panic("Slack client should not be nil")
	}

	return func(r *reporter) {
		r.slack = slack
	}
}

// New creates a new slack reporter
func New(token string, opts ...NewOption) internal.Reporter {
	r := &reporter{
		slack: newSlack(token),
	}

	// apply the new options
	for _, opt := range opts {
		opt(r)
	}

	return r
}

// newSlack returns reporter for sending report via slack into specific channels
func newSlack(token string) slackutil.Slack {
	return slackutil.NewClient(token)
}

// GetName returns slack type
func (r *reporter) GetName() string {
	return ReporterName
}

// SendComponentUpgrade implements the reporter SendComponentUpgrade function
func (r *reporter) SendComponentUpgrade(configCtrl internal.ConfigController, comp *internal.ComponentUpgradeReporter) error {
	slackConfig, err := r.getSlackConfig(comp.TeamName, configCtrl)
	if err != nil {
		return nil
	}

	if slackConfig.ComponentUpgrade != nil {
		if err := r.checkMatchingInterval(slackConfig.ComponentUpgrade.Interval, comp.IsReverify); err != nil {
			return nil
		}

		if err := r.checkMatchingCriteria(slackConfig.ComponentUpgrade.Criteria, string(comp.StatusStr)); err != nil {
			return nil
		}
	}

	message := r.makeComponentUpgradeReport(comp)
	if len(comp.ImageMissingList) > 0 {
		message += "\n"
		message += r.makeImageMissingListReport(convertRPCImageListToK8SImageList(comp.ImageMissingList))
	}

	return r.post(slackConfig, message, internal.ComponentUpgradeType)
}

// SendPullRequestQueue implements the reporter SendPullRequestQueue function
func (r *reporter) SendPullRequestQueue(configCtrl internal.ConfigController, comp *internal.ComponentUpgradeReporter) error {
	slackConfig, err := r.getSlackConfig(comp.TeamName, configCtrl)
	if err != nil {
		return nil
	}

	if slackConfig.PullRequestQueue != nil {
		if err := r.checkMatchingInterval(slackConfig.PullRequestQueue.Interval, comp.IsReverify); err != nil {
			return nil
		}

		if err := r.checkMatchingCriteria(slackConfig.PullRequestQueue.Criteria, string(comp.StatusStr)); err != nil {
			return nil
		}
	}

	message := r.makePullRequestQueueReport(comp)
	if len(comp.ImageMissingList) > 0 {
		message += "\n"
		message += r.makeImageMissingListReport(convertRPCImageListToK8SImageList(comp.ImageMissingList))
	}

	return r.post(slackConfig, message, internal.PullRequestQueueType)
}

// SendActivePromotionStatus implements the reporter SendActivePromotionStatus function
func (r *reporter) SendActivePromotionStatus(configCtrl internal.ConfigController, atpRpt *internal.ActivePromotionReporter) error {
	slackConfig, err := r.getSlackConfig(atpRpt.TeamName, configCtrl)
	if err != nil {
		return nil
	}

	message := r.makeActivePromotionStatusReport(atpRpt)

	imageMissingList := atpRpt.ActivePromotionStatus.PreActiveQueue.ImageMissingList
	if len(imageMissingList) > 0 {
		message += "\n"
		message += r.makeImageMissingListReport(imageMissingList)
	}

	message += "\n"
	if atpRpt.HasOutdatedComponent {
		message += r.makeOutdatedComponentsReport(atpRpt.OutdatedComponents)
	} else {
		message += r.makeNoOutdatedComponentsReport()
	}

	isDemotionFailed := atpRpt.DemotionStatus == s2hv1beta1.ActivePromotionDemotionFailure
	if isDemotionFailed {
		message += "\n"
		message += r.makeActiveDemotingFailureReport()
	}

	if atpRpt.RollbackStatus == s2hv1beta1.ActivePromotionRollbackFailure {
		message += "\n"
		message += r.makeActivePromotionRollbackFailureReport()
	}

	hasPreviousActiveNamespace := atpRpt.PreviousActiveNamespace != ""
	if atpRpt.Result == s2hv1beta1.ActivePromotionSuccess && hasPreviousActiveNamespace && !isDemotionFailed {
		message += "\n"
		message += r.makeDestroyedPreviousActiveTimeReport(&atpRpt.ActivePromotionStatus)
	}

	return r.post(slackConfig, message, internal.ActivePromotionType)
}

// SendImageMissing implements the reporter SendImageMissing function
func (r *reporter) SendImageMissing(configCtrl internal.ConfigController, imageMissingRpt *internal.ImageMissingReporter) error {
	slackConfig, err := r.getSlackConfig(imageMissingRpt.TeamName, configCtrl)
	if err != nil {
		return nil
	}

	message := r.makeImageMissingListReport([]s2hv1beta1.Image{imageMissingRpt.Image})

	return r.post(slackConfig, message, internal.ImageMissingType)
}

// SendPullRequestTriggerResult implements the reporter SendPullRequestTriggerResult function
func (r *reporter) SendPullRequestTriggerResult(configCtrl internal.ConfigController, prTriggerRpt *internal.PullRequestTriggerReporter) error {
	slackConfig, err := r.getSlackConfig(prTriggerRpt.TeamName, configCtrl)
	if err != nil {
		return nil
	}

	if slackConfig.PullRequestTrigger != nil {
		err := r.checkMatchingCriteria(slackConfig.PullRequestTrigger.Criteria, string(prTriggerRpt.Result))
		if err != nil {
			return nil
		}
	}

	message := r.makePullRequestTriggerResultReport(prTriggerRpt)

	return r.post(slackConfig, message, internal.PullRequestTriggerType)
}

func convertRPCImageListToK8SImageList(images []*rpc.Image) []s2hv1beta1.Image {
	k8sImages := make([]s2hv1beta1.Image, 0)
	for _, img := range images {
		k8sImages = append(k8sImages, s2hv1beta1.Image{
			Repository: img.Repository,
			Tag:        img.Tag,
		})
	}

	return k8sImages
}

func (r *reporter) makeComponentUpgradeReport(comp *internal.ComponentUpgradeReporter) string {
	queueHistURL := `{{ .SamsahaiExternalURL }}/teams/{{ .TeamName }}/queue/histories/{{ .QueueHistoryName }}`
	queueLogURL := `{{ .SamsahaiExternalURL }}/teams/{{ .TeamName }}/queue/histories/{{ .QueueHistoryName }}/log`

	message := `
*Component Upgrade:* {{ .StatusStr }}
` + r.makeDeploymentQueueReport(comp, queueHistURL, queueLogURL)
	return strings.TrimSpace(template.TextRender("SlackComponentUpgrade", message, comp))
}

func (r *reporter) makePullRequestQueueReport(comp *internal.ComponentUpgradeReporter) string {
	queueHistURL := `{{ .SamsahaiExternalURL }}/teams/{{ .TeamName }}/pullrequest/queue/histories/{{ .QueueHistoryName }}`
	queueLogURL := `{{ .SamsahaiExternalURL }}/teams/{{ .TeamName }}/pullrequest/queue/histories/{{ .QueueHistoryName }}/log`

	message := `
*Pull Request Queue:* {{ .StatusStr }}
{{- if .PullRequestComponent }}
*Component:* {{ .PullRequestComponent.ComponentName }}
*PR Number:* {{ .PullRequestComponent.PRNumber }}
{{- end }}
` + r.makeDeploymentQueueReport(comp, queueHistURL, queueLogURL)
	return strings.TrimSpace(template.TextRender("SlackPullRequestQueue", message, comp))
}

func (r *reporter) makeDeploymentQueueReport(comp *internal.ComponentUpgradeReporter, queueHistURL, queueLogURL string) string {
	message := `
{{- if eq .Status 0 }}
*Issue type:* {{ .IssueTypeStr }}
{{- end }}
*Run:*{{ if .PullRequestComponent }} #{{ .Runs }}{{ else if .IsReverify }} Reverify {{ else }} #{{ .Runs }}{{ end }}
*Queue:* {{ .Name }}
*Components* 
{{- range .Components }}
>- *Name:* {{ .Name }}
>   *Version:* {{ .Image.Tag }}
>   *Repository:* {{ .Image.Repository }}
{{- end }}
*Owner:* {{ .TeamName }}
*Namespace:* {{ .Namespace }}
{{- if eq .Status 0 }}
  {{- if .ComponentUpgrade.DeploymentIssues }}
*Deployment Issues:*
  {{- range .ComponentUpgrade.DeploymentIssues }}
>- *Issue type:* {{ .IssueType }}
>   *Components:* {{ range .FailureComponents }}{{ .ComponentName }},{{ end }}
    {{- if eq .IssueType "WaitForInitContainer" }}
>   *Wait for:* {{ range .FailureComponents }}{{ .FirstFailureContainerName }},{{ end }}
    {{- end }}
  {{- end }} 
  {{- end }} 
  {{- if .TestRunner.Teamcity.BuildURL }}
*Teamcity URL:* <{{ .TestRunner.Teamcity.BuildURL }}|{{ .TestRunner.Teamcity.BuildNumber }}>
  {{- end }}
*Deployment Logs:* <` + queueLogURL + `|Download here>
*Deployment History:* <` + queueHistURL + `|Click here>
{{- end}}
`
	return strings.TrimSpace(template.TextRender("SlackDeploymentQueue", message, comp))
}

func (r *reporter) makeActivePromotionStatusReport(atpRpt *internal.ActivePromotionReporter) string {
	var message = `
*Active Promotion:* {{ .Result }}
{{- if ne .Result "Success" }}
{{- range .Conditions }}
  {{- if eq .Type "` + string(s2hv1beta1.ActivePromotionCondActivePromoted) + `" }}
*Reason:* {{ .Message }}
  {{- end }}
{{- end }}
{{- end }}
*Current Active Namespace:* {{ .CurrentActiveNamespace }}
*Owner:* {{ .TeamName }}
{{- if eq .Result "Failure" }}
  {{- if .PreActiveQueue.DeploymentIssues }}
*Deployment Issues:*
  {{- range .PreActiveQueue.DeploymentIssues }}
>- *Issue type:* {{ .IssueType }}
>   *Components:* {{ range .FailureComponents }}{{ .ComponentName }},{{ end }}
    {{- if eq .IssueType "WaitForInitContainer" }}
>   *Wait for:* {{ range .FailureComponents }}{{ .FirstFailureContainerName }},{{ end }}
    {{- end }}
  {{- end }} 
  {{- end }}
{{- end }}
{{- if and .PreActiveQueue.TestRunner (and .PreActiveQueue.TestRunner.Teamcity .PreActiveQueue.TestRunner.Teamcity.BuildURL) }}
*Teamcity URL:* <{{ .PreActiveQueue.TestRunner.Teamcity.BuildURL }}|{{ .PreActiveQueue.TestRunner.Teamcity.BuildNumber }}>
{{- end }}
{{- if eq .Result "Failure" }}
*Deployment Logs:* <{{ .SamsahaiExternalURL }}/teams/{{ .TeamName }}/activepromotions/histories/{{ .ActivePromotionHistoryName }}/log|Download here>
{{- end }}
*Active Promotion History:* <{{ .SamsahaiExternalURL }}/teams/{{ .TeamName }}/activepromotions/histories/{{ .ActivePromotionHistoryName }}|Click here>
`

	return strings.TrimSpace(template.TextRender("SlackActivePromotionStatus", message, atpRpt))
}

func (r *reporter) makeOutdatedComponentsReport(comps map[string]s2hv1beta1.OutdatedComponent) string {
	var message = `
*Outdated Components:*
{{- range $name, $component := .Components }}
{{- if gt .OutdatedDuration 0 }}
*{{ $name }}*
>Not update for {{ .OutdatedDuration | FmtDurationToStr }}
>Current Version: <{{ .CurrentImage.Repository | ConcatHTTPStr }}|{{ .CurrentImage.Tag }}>
>Latest Version: <{{ .DesiredImage.Repository | ConcatHTTPStr }}|{{ .DesiredImage.Tag }}>
{{- end }}
{{- end }}
`

	ocObj := struct {
		Components map[string]s2hv1beta1.OutdatedComponent
	}{Components: comps}
	return strings.TrimSpace(template.TextRender("SlackOutdatedComponents", message, ocObj))
}

func (r *reporter) makeNoOutdatedComponentsReport() string {
	var message = `
>*All components are up to date!*
`

	return strings.TrimSpace(template.TextRender("SlackNoOutdatedComponents", message, ""))
}

func (r *reporter) makeActivePromotionRollbackFailureReport() string {
	var message = "`ERROR: cannot rollback an active promotion process due to timeout`"

	return strings.TrimSpace(template.TextRender("RollbackFailure", message, ""))
}

func (r *reporter) makeActiveDemotingFailureReport() string {
	var message = "`WARNING: cannot demote a previous active environment, previous active namespace has been destroyed immediately`"

	return strings.TrimSpace(template.TextRender("DemotionFailure", message, ""))
}

func (r *reporter) makeDestroyedPreviousActiveTimeReport(status *s2hv1beta1.ActivePromotionStatus) string {
	var message = "*NOTES:* previous active namespace `{{ .PreviousActiveNamespace }}` will be destroyed at `{{ .DestroyedTime | TimeFormat }}`"

	return strings.TrimSpace(template.TextRender("DestroyedTime", message, status))
}

func (r *reporter) makeImageMissingListReport(images []s2hv1beta1.Image) string {
	var message = `
*Image Missing List*
{{- range .Images }}
- {{ .Repository }}:{{ .Tag }}
{{- end }}
`

	imagesObj := struct{ Images []s2hv1beta1.Image }{Images: images}
	return strings.TrimSpace(template.TextRender("SlackImageMissingList", message, imagesObj))
}

func (r *reporter) makePullRequestTriggerResultReport(prTriggerRpt *internal.PullRequestTriggerReporter) string {
	var message = `
*Pull Request Trigger:* {{ .Result }}
*Component:* {{ .ComponentName }}
*PR Number:* {{ .PRNumber }}
*Image:* {{ if .Image }}{{ .Image.Repository }}:{{ .Image.Tag }}{{ else }}no image defined{{ end }}
*NO of Retry:* {{ if .NoOfRetry }}{{ .NoOfRetry }}{{ else }}0{{ end }}
*Owner:* {{ .TeamName }}
*Start at:* {{ .CreatedAt | TimeFormat }}
`

	return strings.TrimSpace(template.TextRender("SlackPullRequestTriggerResult", message, prTriggerRpt))
}

func (r *reporter) checkMatchingInterval(interval s2hv1beta1.ReporterInterval, isReverify bool) error {
	switch interval {
	case s2hv1beta1.IntervalEveryTime:
	default:
		if !isReverify {
			return s2herrors.New("interval was not matched")
		}
	}

	return nil
}

func (r *reporter) checkMatchingCriteria(criteria s2hv1beta1.ReporterCriteria, result string) error {
	lowerCaseResult := strings.ToLower(result)

	switch criteria {
	case s2hv1beta1.CriteriaBoth:
	case s2hv1beta1.CriteriaSuccess:
		if lowerCaseResult != statusSuccess {
			return s2herrors.New("criteria was not matched")
		}
	default:
		if lowerCaseResult != statusFailure {
			return s2herrors.New("criteria was not matched")
		}
	}

	return nil
}

func (r *reporter) isPullRequestQueue(comp *internal.ComponentUpgradeReporter) bool {
	return comp.PullRequestComponent != nil && comp.PullRequestComponent.PRNumber != ""
}

func (r *reporter) post(slackConfig *s2hv1beta1.Slack, message string, event internal.EventType) error {
	logger.Debug("start sending message to slack channels",
		"event", event, "channels", slackConfig.Channels)
	var globalErr error
	for _, channel := range slackConfig.Channels {
		if err := r.slack.PostMessage(channel, message, slack.MsgOptionUsername(username)); err != nil {
			logger.Error(err, "cannot post message to slack", "event", event, "channel", channel)
			globalErr = err
			continue
		}
	}
	return globalErr
}

func (r *reporter) getSlackConfig(teamName string, configCtrl internal.ConfigController) (*s2hv1beta1.Slack, error) {
	config, err := configCtrl.Get(teamName)
	if err != nil {
		return nil, err
	}

	// no slack configuration
	if config.Spec.Reporter == nil || config.Spec.Reporter.Slack == nil {
		return nil, s2herrors.New("slack configuration not found")
	}

	return config.Spec.Reporter.Slack, nil
}
