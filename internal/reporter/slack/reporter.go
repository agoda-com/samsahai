package slack

import (
	"strings"

	s2hv1beta1 "github.com/agoda-com/samsahai/api/v1beta1"
	"github.com/agoda-com/samsahai/internal"
	s2hlog "github.com/agoda-com/samsahai/internal/log"
	"github.com/agoda-com/samsahai/internal/util/slack"
	"github.com/agoda-com/samsahai/internal/util/template"
	"github.com/agoda-com/samsahai/pkg/samsahai/rpc"
)

var logger = s2hlog.Log.WithName(ReporterName)

const (
	ReporterName = "slack"
	username     = "Samsahai Notification"
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
func (r *reporter) SendComponentUpgrade(configMgr internal.ConfigManager, comp *internal.ComponentUpgradeReporter) error {
	message := r.makeComponentUpgradeFailureReport(comp)
	if len(comp.ImageMissingList) > 0 {
		message += "\n"
		message += r.makeImageMissingListReport(comp.ImageMissingList)
	}

	return r.post(configMgr, message, internal.ComponentUpgradeType)
}

// SendActivePromotionStatus implements the reporter SendActivePromotionStatus function
func (r *reporter) SendActivePromotionStatus(configMgr internal.ConfigManager, atpRpt *internal.ActivePromotionReporter) error {
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

	return r.post(configMgr, message, internal.ActivePromotionType)
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
func (r *reporter) SendImageMissing(configMgr internal.ConfigManager, images *rpc.Image) error {
	message := r.makeImageMissingListReport([]*rpc.Image{images})

	return r.post(configMgr, message, internal.ImageMissingType)
}

func (r *reporter) makeComponentUpgradeFailureReport(comp *internal.ComponentUpgradeReporter) string {
	message := `
*Component Upgrade Failed*
>*Component:* {{ .Name }}
>*Version:* {{ .Image.Tag }}
>*Repository:* {{ .Image.Repository }}
>*Issue type:* {{ .IssueTypeStr }}
{{- if .TestRunner.Teamcity.BuildURL }}
>*Teamcity url:* <{{ .TestRunner.Teamcity.BuildURL }}|Click here>
{{- end }}
>*Deployment Logs:* <{{ .SamsahaiExternalURL }}/teams/{{ .TeamName }}/queue/histories/{{ .QueueHistoryName }}/log|Download here>
>*Deployment history:* <{{ .SamsahaiExternalURL }}/teams/{{ .TeamName }}/queue/histories/{{ .QueueHistoryName }}|Click here>
>*Owner:* {{ .TeamName }}
>*Namespace:* {{ .Namespace }}
`
	return strings.TrimSpace(template.TextRender("SlackComponentUpgradeFailure", message, comp))
}

func (r *reporter) makeActivePromotionStatusReport(comp *internal.ActivePromotionReporter) string {
	var message = `
*Active Promotion:* {{ .Result }}
{{- if ne .Result "Success" }}
{{- range .Conditions }}
  {{- if eq .Type "PreActiveVerified" }}
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

func (r *reporter) makeOutdatedComponentsReport(comps []*s2hv1beta1.OutdatedComponent) string {
	var message = `
*Outdated Components:*
{{- range .Components }}
{{- if gt .OutdatedDuration 0 }}
*{{ .Name }}*
>Not update for {{ .OutdatedDuration | FmtDurationToStr }}
>Current Version: <{{ .CurrentImage.Repository | ConcatHTTPStr }}|{{ .CurrentImage.Tag }}>
>Latest Version: <{{ .LatestImage.Repository | ConcatHTTPStr }}|{{ .LatestImage.Tag }}>
{{- end }}
{{- end }}
`

	ocObj := struct {
		Components []*s2hv1beta1.OutdatedComponent
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

func (r *reporter) post(configMgr internal.ConfigManager, message string, event internal.EventType) error {
	cfg := configMgr.Get()

	// no slack configuration
	if cfg.Reporter == nil || cfg.Reporter.Slack == nil {
		return nil
	}

	logger.Debug("start sending message to slack channels",
		"event", event, "channels", cfg.Reporter.Slack.Channels)
	for _, channel := range cfg.Reporter.Slack.Channels {
		if _, _, err := r.slack.PostMessage(channel, message, username); err != nil {
			return err
		}
	}
	return nil
}
