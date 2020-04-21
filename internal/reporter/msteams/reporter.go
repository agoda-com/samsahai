package msteams

import (
	"strings"

	s2hv1beta1 "github.com/agoda-com/samsahai/api/v1beta1"
	"github.com/agoda-com/samsahai/internal"
	s2herrors "github.com/agoda-com/samsahai/internal/errors"
	s2hlog "github.com/agoda-com/samsahai/internal/log"
	"github.com/agoda-com/samsahai/internal/util/msteams"
	"github.com/agoda-com/samsahai/internal/util/template"
	"github.com/agoda-com/samsahai/pkg/samsahai/rpc"
)

var logger = s2hlog.Log.WithName(ReporterName)

const (
	ReporterName = "msteams"

	componentUpgradeInterval = s2hv1beta1.IntervalRetry
	componentUpgradeCriteria = s2hv1beta1.CriteriaFailure

	styleDanger  = `style="color:#FF0000"`
	styleWarning = `style="color:#FFCC00"`
	styleInfo    = `style="color:#00FF00"`
)

type reporter struct {
	msTeams msteams.MSTeams
}

// NewOption allows specifying various configuration
type NewOption func(*reporter)

// WithMSTeamsClient specifies msteams client to override when create Microsoft Teams reporter
func WithMSTeamsClient(msTeams msteams.MSTeams) NewOption {
	if msTeams == nil {
		panic("Microsoft Teams client should not be nil")
	}

	return func(r *reporter) {
		r.msTeams = msTeams
	}
}

// New creates a new Microsoft Teams reporter
func New(tenantID, clientID, clientSecret, username, password string, opts ...NewOption) internal.Reporter {
	r := &reporter{
		msTeams: newMSTeams(tenantID, clientID, clientSecret, username, password),
	}

	// apply the new options
	for _, opt := range opts {
		opt(r)
	}

	return r
}

// newMSTeams returns reporter for sending report via Microsoft Teams into specific groups and channels
func newMSTeams(tenantID, clientID, clientSecret, username, password string) msteams.MSTeams {
	return msteams.NewClient(tenantID, clientID, clientSecret, username, password)
}

// GetName returns msteams type
func (r *reporter) GetName() string {
	return ReporterName
}

// SendComponentUpgrade implements the reporter SendComponentUpgrade function
func (r *reporter) SendComponentUpgrade(configCtrl internal.ConfigController, comp *internal.ComponentUpgradeReporter) error {
	msTeamsConfig, err := r.getMSTeamsConfig(comp.TeamName, configCtrl)
	if err != nil {
		return nil
	}

	if err := r.checkMatchingInterval(msTeamsConfig, comp.IsReverify); err != nil {
		return nil
	}

	if err := r.checkMatchingCriteria(msTeamsConfig, comp.Status); err != nil {
		return nil
	}

	message := r.makeComponentUpgradeReport(comp)
	if len(comp.ImageMissingList) > 0 {
		message += "<hr/>"
		message += r.makeImageMissingListReport(comp.ImageMissingList)
	}

	return r.post(msTeamsConfig, message, internal.ComponentUpgradeType)
}

func (r *reporter) checkMatchingInterval(msTeamsConfig *s2hv1beta1.MSTeams, isReverify bool) error {
	interval := componentUpgradeInterval
	if msTeamsConfig.ComponentUpgrade != nil && msTeamsConfig.ComponentUpgrade.Interval != "" {
		interval = msTeamsConfig.ComponentUpgrade.Interval
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

func (r *reporter) checkMatchingCriteria(msTeamsConfig *s2hv1beta1.MSTeams, status rpc.ComponentUpgrade_UpgradeStatus) error {
	criteria := componentUpgradeCriteria
	if msTeamsConfig.ComponentUpgrade != nil && msTeamsConfig.ComponentUpgrade.Criteria != "" {
		criteria = msTeamsConfig.ComponentUpgrade.Criteria
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
	msTeamsConfig, err := r.getMSTeamsConfig(atpRpt.TeamName, configCtrl)
	if err != nil {
		return nil
	}

	message := r.makeActivePromotionStatusReport(atpRpt)

	imageMissingList := atpRpt.ActivePromotionStatus.PreActiveQueue.ImageMissingList
	if len(imageMissingList) > 0 {
		message += "<hr/>"
		message += r.makeImageMissingListReport(convertImageListToRPCImageList(imageMissingList))
	}

	if atpRpt.HasOutdatedComponent {
		message += "<hr/>"
		message += r.makeOutdatedComponentsReport(atpRpt.OutdatedComponents)
	} else {
		message += "<br/>"
		message += r.makeNoOutdatedComponentsReport()
	}

	message += "<br/>"

	isDemotionFailed := atpRpt.DemotionStatus == s2hv1beta1.ActivePromotionDemotionFailure
	if isDemotionFailed {
		message += "<br/>"
		message += r.makeActiveDemotingFailureReport()
	}

	if atpRpt.RollbackStatus == s2hv1beta1.ActivePromotionRollbackFailure {
		message += "<br/>"
		message += r.makeActivePromotionRollbackFailureReport()
	}

	hasPreviousActiveNamespace := atpRpt.PreviousActiveNamespace != ""
	if atpRpt.Result == s2hv1beta1.ActivePromotionSuccess && hasPreviousActiveNamespace && !isDemotionFailed {
		message += "<br/>"
		message += r.makeDestroyedPreviousActiveTimeReport(&atpRpt.ActivePromotionStatus)
	}

	return r.post(msTeamsConfig, message, internal.ActivePromotionType)
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
	msTeamsConfig, err := r.getMSTeamsConfig(teamName, configCtrl)
	if err != nil {
		return nil
	}

	message := r.makeImageMissingListReport([]*rpc.Image{images})

	return r.post(msTeamsConfig, message, internal.ImageMissingType)
}

func (r *reporter) makeComponentUpgradeReport(comp *internal.ComponentUpgradeReporter) string {
	message := `
<b>Component Upgrade:</b><span {{ if eq .Status 1 }}` + styleInfo + `> Success {{ else }}` + styleDanger + `> Failure{{ end }}</span>
<li><b>Issue type:</b> {{ .IssueTypeStr }}</li>
<li><b>Run:</b>{{ if .IsReverify }} Reverify {{ else }} #{{ .Runs }} {{ end }}</li>
<li><b>Component:</b> {{ .Name }}</li>
<li><b>Version:</b> {{ .Image.Tag }}</li>
<li><b>Repository:</b> {{ .Image.Repository }}</li>
<li><b>Owner:</b> {{ .TeamName }}</li>
<li><b>Namespace:</b> {{ .Namespace }}</li>
{{- if eq .Status 0 }}
 {{- if .TestRunner.Teamcity.BuildURL }}
<li><b>Teamcity url:</b> <a href="{{ .TestRunner.Teamcity.BuildURL }}">Click here</a></li>
 {{- end }}
<li><b>Deployment Logs:</b> <a href="{{ .SamsahaiExternalURL }}/teams/{{ .TeamName }}/queue/histories/{{ .QueueHistoryName }}/log">Download here</a></li>
<li><b>Deployment history:</b> <a href="{{ .SamsahaiExternalURL }}/teams/{{ .TeamName }}/queue/histories/{{ .QueueHistoryName }}">Click here</a></li>
{{- end}}
`
	return strings.TrimSpace(template.TextRender("MSTeamsComponentUpgradeFailure", message, comp))
}

func (r *reporter) makeActivePromotionStatusReport(comp *internal.ActivePromotionReporter) string {
	var message = `
<b>Active Promotion:</b> <span {{ if eq .Result "Success" }}` + styleInfo + `{{ else if eq .Result "Failure" }}` + styleDanger + `{{ end }}>{{ .Result }}</span>
{{- if ne .Result "Success" }}
{{- range .Conditions }}
 {{- if eq .Type "` + string(s2hv1beta1.ActivePromotionCondActivePromoted) + `" }}
<br/><b>Reason:</b> {{ .Message }}
 {{- end }}
{{- end }}
{{- end }}
<li><b>Current Active Namespace:</b> {{ .CurrentActiveNamespace }}</li>
<li><b>Owner:</b> {{ .TeamName }}</li>
{{- if and .PreActiveQueue.TestRunner (and .PreActiveQueue.TestRunner.Teamcity .PreActiveQueue.TestRunner.Teamcity.BuildURL) }}
<li><b>Teamcity url:</b> <a href="{{ .PreActiveQueue.TestRunner.Teamcity.BuildURL }}">Click here</a></li>
{{- end }}
{{- if eq .Result "Failure" }}
<li><b>Deployment Logs:</b> <a href="{{ .SamsahaiExternalURL }}/teams/{{ .TeamName }}/activepromotions/histories/{{ .ActivePromotionHistoryName }}/log">Download here</a></li>
{{- end }}
<li><b>Active Promotion History:</b> <a href="{{ .SamsahaiExternalURL }}/teams/{{ .TeamName }}/activepromotions/histories/{{ .ActivePromotionHistoryName }}">Click here</a></li>
`

	return strings.TrimSpace(template.TextRender("MSTeamsActivePromotionStatus", message, comp))
}

func (r *reporter) makeOutdatedComponentsReport(comps map[string]s2hv1beta1.OutdatedComponent) string {
	var message = `
<b>Outdated Components:</b>
{{- range $name, $component := .Components }}
{{- if gt .OutdatedDuration 0 }}
<b>{{ $name }}</b>
<li>Not update for {{ .OutdatedDuration | FmtDurationToStr }}</li>
<li>Current Version: <a href="{{ .CurrentImage.Repository | ConcatHTTPStr }}">{{ .CurrentImage.Tag }}</a></li>
<li>Latest Version: <a href="{{ .DesiredImage.Repository | ConcatHTTPStr }}">{{ .DesiredImage.Tag }}</a></li>
{{- end }}
{{- end }}
`

	ocObj := struct {
		Components map[string]s2hv1beta1.OutdatedComponent
	}{Components: comps}
	return strings.TrimSpace(template.TextRender("MSTeamsOutdatedComponents", message, ocObj))
}

func (r *reporter) makeNoOutdatedComponentsReport() string {
	var message = `
<li><b>All components are up to date!</b></li>
`

	return strings.TrimSpace(template.TextRender("MSTeamsNoOutdatedComponents", message, ""))
}

func (r *reporter) makeActivePromotionRollbackFailureReport() string {
	var message = "<b " + styleDanger + ">ERROR:</b> cannot rollback an active promotion process due to timeout`"

	return strings.TrimSpace(template.TextRender("RollbackFailure", message, ""))
}

func (r *reporter) makeActiveDemotingFailureReport() string {
	var message = "<b " + styleWarning + ">WARNING:</b> cannot demote a previous active environment, previous active namespace has been destroyed immediately"

	return strings.TrimSpace(template.TextRender("DemotionFailure", message, ""))
}

func (r *reporter) makeDestroyedPreviousActiveTimeReport(status *s2hv1beta1.ActivePromotionStatus) string {
	var message = "<b " + styleWarning + ">NOTES:</b> previous active namespace <code>{{ .PreviousActiveNamespace }}</code> will be destroyed at <code>{{ .DestroyedTime }}</code>"

	return strings.TrimSpace(template.TextRender("DestroyedTime", message, status))
}

// makeImageMissingListReport implements the reporter makeImageMissingListReport function
func (r *reporter) makeImageMissingListReport(images []*rpc.Image) string {
	var message = `
<b>Image Missing List</b>
<br/>
{{- range .Images }}
- {{ .Repository }}:{{ .Tag }}
{{- end }}
`

	imagesObj := struct{ Images []*rpc.Image }{Images: images}
	return strings.TrimSpace(template.TextRender("MSTeamsImageMissingList", message, imagesObj))
}

func (r *reporter) post(msTeamsConfig *s2hv1beta1.MSTeams, message string, event internal.EventType) error {
	logger.Debug("start sending message to Microsoft Teams groups and channels",
		"event", event, "groups", msTeamsConfig.Groups)

	accessToken, err := r.msTeams.GetAccessToken()
	if err != nil {
		logger.Error(err, "cannot get Microsoft access token",
			"event", event, "groups", msTeamsConfig.Groups)
		return err
	}

	var globalErr error
	for _, group := range msTeamsConfig.Groups {
		for _, chanID := range group.ChannelIDs {
			err := r.msTeams.PostMessage(group.GroupID, chanID, message,
				msteams.WithAccessToken(accessToken), msteams.WithContentType("html"))
			if err != nil {
				logger.Error(err, "cannot post message to Microsoft Teams",
					"event", event, "groupID", group.GroupID, "channelID", chanID)
				globalErr = err
				continue
			}
		}
	}
	return globalErr
}

func (r *reporter) getMSTeamsConfig(teamName string, configCtrl internal.ConfigController) (*s2hv1beta1.MSTeams, error) {
	config, err := configCtrl.Get(teamName)
	if err != nil {
		return nil, err
	}

	// no Microsoft Teams configuration
	if config.Spec.Reporter == nil || config.Spec.Reporter.MSTeams == nil {
		return nil, s2herrors.New("msTeams configuration not found")
	}

	return config.Spec.Reporter.MSTeams, nil
}
