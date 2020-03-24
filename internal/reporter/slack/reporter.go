package slack

import (
	"strings"

	s2hv1beta1 "github.com/agoda-com/samsahai/api/v1beta1"
	"github.com/agoda-com/samsahai/internal"
	s2herrors "github.com/agoda-com/samsahai/internal/errors"
	s2hlog "github.com/agoda-com/samsahai/internal/log"
	"github.com/agoda-com/samsahai/internal/util/slack"
	"github.com/agoda-com/samsahai/internal/util/template"
	"github.com/agoda-com/samsahai/pkg/samsahai/rpc"
)

var logger = s2hlog.Log.WithName(ReporterName)

const (
	ReporterName = "slack"
	username     = "Samsahai Notification"

	componentUpgradeInterval = s2hv1beta1.IntervalRetry
	componentUpgradeCriteria = s2hv1beta1.CriteriaFailure
)

type reporter struct {
	slack slack.Slack
}

// NewOption allows specifying various configuration
type NewOption func(*reporter)

// WithSlackClient specifies slack client to override when create slack reporter
func WithSlackClient(slack slack.Slack) NewOption {
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

// newSlack returns reporter for sending report to slack at specific `channels`
func newSlack(token string) slack.Slack {
	return slack.NewClient(token)
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

	if err := r.checkMatchingInterval(slackConfig, comp.IsReverify); err != nil {
		return nil
	}

	if err := r.checkMatchingCriteria(slackConfig, comp.Status); err != nil {
		return nil
	}

	message := r.makeComponentUpgradeReport(comp)
	if len(comp.ImageMissingList) > 0 {
		message += "\n"
		message += r.makeImageMissingListReport(comp.ImageMissingList)
	}

	return r.post(slackConfig, message, internal.ComponentUpgradeType)
}

func (r *reporter) checkMatchingInterval(slackConfig *s2hv1beta1.Slack, isReverify bool) error {
	interval := componentUpgradeInterval
	if slackConfig.ComponentUpgrade != nil && slackConfig.ComponentUpgrade.Interval != "" {
		interval = slackConfig.ComponentUpgrade.Interval
	}

	switch interval {
	case s2hv1beta1.IntervalEveryTime:
	default:
		if !isReverify {
			return s2herrors.New("interval was not matched")
		}
	}

	return nil
}

func (r *reporter) checkMatchingCriteria(slackConfig *s2hv1beta1.Slack, status rpc.ComponentUpgrade_UpgradeStatus) error {
	criteria := componentUpgradeCriteria
	if slackConfig.ComponentUpgrade != nil && slackConfig.ComponentUpgrade.Criteria != "" {
		criteria = slackConfig.ComponentUpgrade.Criteria
	}

	switch criteria {
	case s2hv1beta1.CriteriaBoth:
	case s2hv1beta1.CriteriaSuccess:
		if status != rpc.ComponentUpgrade_UpgradeStatus_SUCCESS {
			return s2herrors.New("criteria was not matched")
		}
	default:
		if status != rpc.ComponentUpgrade_UpgradeStatus_FAILURE {
			return s2herrors.New("criteria was not matched")
		}
	}

	return nil
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
		message += r.makeImageMissingListReport(convertImageListToRPCImageList(imageMissingList))
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

func convertImageListToRPCImageList(images []s2hv1beta1.Image) []*rpc.Image {
	rpcImages := make([]*rpc.Image, 0)
	for _, img := range images {
		rpcImages = append(rpcImages, &rpc.Image{
			Repository: img.Repository,
			Tag:        img.Tag,
		})
	}

	return rpcImages
}

// SendImageMissing implements the reporter SendImageMissing function
func (r *reporter) SendImageMissing(teamName string, configCtrl internal.ConfigController, images *rpc.Image) error {
	slackConfig, err := r.getSlackConfig(teamName, configCtrl)
	if err != nil {
		return nil
	}

	message := r.makeImageMissingListReport([]*rpc.Image{images})

	return r.post(slackConfig, message, internal.ImageMissingType)
}

func (r *reporter) makeComponentUpgradeReport(comp *internal.ComponentUpgradeReporter) string {
	message := `
*Component Upgrade{{ if eq .Status 1 }} Successfully {{ else }} Failed {{ end }}*
>*Owner:* {{ .TeamName }}
>*Namespace:* {{ .Namespace }}
>*Run:*{{ if .IsReverify }} Reverify {{ else }} #{{ .Runs }} {{ end }}
>*Component:* {{ .Name }}
>*Version:* {{ .Image.Tag }}
>*Repository:* {{ .Image.Repository }}
{{- if eq .Status 0 }}
>*Issue type:* {{ .IssueTypeStr }}
  {{- if .TestRunner.Teamcity.BuildURL }}
>*Teamcity url:* <{{ .TestRunner.Teamcity.BuildURL }}|Click here>
  {{- end }}
>*Deployment Logs:* <{{ .SamsahaiExternalURL }}/teams/{{ .TeamName }}/queue/histories/{{ .QueueHistoryName }}/log|Download here>
>*Deployment history:* <{{ .SamsahaiExternalURL }}/teams/{{ .TeamName }}/queue/histories/{{ .QueueHistoryName }}|Click here>
{{- end}}
`
	return strings.TrimSpace(template.TextRender("SlackComponentUpgradeFailure", message, comp))
}

func (r *reporter) makeActivePromotionStatusReport(comp *internal.ActivePromotionReporter) string {
	var message = `
*Active Promotion:* {{ .Result }}
{{- if ne .Result "Success" }}
{{- range .Conditions }}
  {{- if eq .Type "` + string(s2hv1beta1.ActivePromotionCondActivePromoted) + `" }}
*Reason:* {{ .Message }}
  {{- end }}
{{- end }}
{{- end }}
*Owner:* {{ .TeamName }}
*Current Active Namespace:* {{ .CurrentActiveNamespace }}
{{- if and .PreActiveQueue.TestRunner (and .PreActiveQueue.TestRunner.Teamcity .PreActiveQueue.TestRunner.Teamcity.BuildURL) }}
*Teamcity url:* <{{ .PreActiveQueue.TestRunner.Teamcity.BuildURL }}|Click here>
{{- end }}
{{- if eq .Result "Failure" }}
*Deployment Logs:* <{{ .SamsahaiExternalURL }}/teams/{{ .TeamName }}/activepromotions/histories/{{ .ActivePromotionHistoryName }}/log|Download here>
{{- end }}
*Active Promotion History:* <{{ .SamsahaiExternalURL }}/teams/{{ .TeamName }}/activepromotions/histories/{{ .ActivePromotionHistoryName }}|Click here>
`

	return strings.TrimSpace(template.TextRender("SlackActivePromotionStatus", message, comp))
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
	var message = "*NOTES:* previous active namespace `{{ .PreviousActiveNamespace }}` will be destroyed at `{{ .DestroyedTime }}`"

	return strings.TrimSpace(template.TextRender("DestroyedTime", message, status))
}

// makeImageMissingListReport implements the reporter makeImageMissingListReport function
func (r *reporter) makeImageMissingListReport(images []*rpc.Image) string {
	var message = `
*Image Missing List*
{{- range .Images }}
- {{ .Repository }}:{{ .Tag }}
{{- end }}
`

	imagesObj := struct{ Images []*rpc.Image }{Images: images}
	return strings.TrimSpace(template.TextRender("SlackImageMissingList", message, imagesObj))
}

func (r *reporter) post(slackConfig *s2hv1beta1.Slack, message string, event internal.EventType) error {
	logger.Debug("start sending message to slack channels",
		"event", event, "channels", slackConfig.Channels)
	for _, channel := range slackConfig.Channels {
		if _, _, err := r.slack.PostMessage(channel, message, username); err != nil {
			logger.Error(err, "cannot post message to slack", "channel", channel)
			continue
		}
	}
	return nil
}

func (r *reporter) getSlackConfig(teamName string, configCtrl internal.ConfigController) (*s2hv1beta1.Slack, error) {
	cfg, err := configCtrl.Get(teamName)
	if err != nil {
		return nil, err
	}

	// no slack configuration
	if cfg.Reporter == nil || cfg.Reporter.Slack == nil {
		return nil, s2herrors.New("slack configuration not found")
	}

	return cfg.Reporter.Slack, nil
}
