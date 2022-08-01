package msteams

import (
	"fmt"
	"strings"

	s2hv1 "github.com/agoda-com/samsahai/api/v1"
	"github.com/agoda-com/samsahai/internal"
	s2herrors "github.com/agoda-com/samsahai/internal/errors"
	s2hlog "github.com/agoda-com/samsahai/internal/log"
	"github.com/agoda-com/samsahai/internal/reporter/util"
	"github.com/agoda-com/samsahai/internal/util/msteams"
	"github.com/agoda-com/samsahai/internal/util/template"
	"github.com/agoda-com/samsahai/pkg/samsahai/rpc"
)

var logger = s2hlog.Log.WithName(ReporterName)

const (
	ReporterName = "msteams"

	styleDanger  = `style="color:#EE2828"`
	styleWarning = `style="color:#EEA328"`
	styleInfo    = `style="color:#2EB44E"`
)

type reporter struct {
	msTeams msteams.MSTeams
}

// NewOption allows specifying various configuration
type NewOption func(*reporter)

// WithMSTeamsClient specifies msteams client to override when creating Microsoft Teams reporter
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
		msTeams: newMSTeamsClient(tenantID, clientID, clientSecret, username, password),
	}

	// apply the new options
	for _, opt := range opts {
		opt(r)
	}

	return r
}

// newMSTeamsClient returns a msteams client for sending report via Microsoft Teams into specific groups and channels
func newMSTeamsClient(tenantID, clientID, clientSecret, username, password string) msteams.MSTeams {
	return msteams.NewClient(tenantID, clientID, clientSecret, username, password)
}

// GetName returns a reporter type
func (r *reporter) GetName() string {
	return ReporterName
}

// SendComponentUpgrade implements the reporter SendComponentUpgrade function
func (r *reporter) SendComponentUpgrade(configCtrl internal.ConfigController, comp *internal.ComponentUpgradeReporter) error {
	msTeamsConfig, err := r.getMSTeamsConfig(comp.TeamName, configCtrl)
	if err != nil {
		return nil
	}

	if msTeamsConfig.ComponentUpgrade != nil {
		if err := util.CheckMatchingInterval(msTeamsConfig.ComponentUpgrade.Interval, comp.IsReverify); err != nil {
			return nil
		}

		if err := util.CheckMatchingCriteria(msTeamsConfig.ComponentUpgrade.Criteria, string(comp.StatusStr)); err != nil {
			return nil
		}
	}

	message := r.makeComponentUpgradeReport(comp)
	if len(comp.ImageMissingList) > 0 {
		message += "<hr/>"
		message += r.makeImageMissingListReport(convertRPCImageListToK8SImageList(comp.ImageMissingList), "")
	}

	return r.post(msTeamsConfig, message, internal.ComponentUpgradeType)
}

// SendPullRequestQueue implements the reporter SendPullRequestQueue function
func (r *reporter) SendPullRequestQueue(configCtrl internal.ConfigController, comp *internal.ComponentUpgradeReporter) error {
	msTeamsConfig, err := r.getMSTeamsConfig(comp.TeamName, configCtrl)
	if err != nil {
		return nil
	}

	if msTeamsConfig.PullRequestQueue != nil {
		if err := util.CheckMatchingInterval(msTeamsConfig.PullRequestQueue.Interval, comp.IsReverify); err != nil {
			return nil
		}

		if err := util.CheckMatchingCriteria(msTeamsConfig.PullRequestQueue.Criteria, string(comp.StatusStr)); err != nil {
			return nil
		}
	}

	message := r.makePullRequestQueueReport(comp)
	if len(comp.ImageMissingList) > 0 {
		message += "\n"
		message += r.makeImageMissingListReport(convertRPCImageListToK8SImageList(comp.ImageMissingList), "")
	}

	return r.post(msTeamsConfig, message, internal.PullRequestQueueType)
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
		message += r.makeImageMissingListReport(imageMissingList, "")
	}

	if atpRpt.HasOutdatedComponent {
		message += "<hr/>"
		message += r.makeOutdatedComponentsReport(atpRpt.OutdatedComponents)
	} else {
		message += "<br/>"
		message += r.makeNoOutdatedComponentsReport()
	}

	message += "<br/>"

	isDemotionFailed := atpRpt.DemotionStatus == s2hv1.ActivePromotionDemotionFailure
	if isDemotionFailed {
		message += "<br/>"
		message += r.makeActiveDemotingFailureReport()
	}

	if atpRpt.RollbackStatus == s2hv1.ActivePromotionRollbackFailure {
		message += "<br/>"
		message += r.makeActivePromotionRollbackFailureReport()
	}

	hasPreviousActiveNamespace := atpRpt.PreviousActiveNamespace != ""
	if atpRpt.Result == s2hv1.ActivePromotionSuccess && hasPreviousActiveNamespace && !isDemotionFailed {
		message += "<br/>"
		message += r.makeDestroyedPreviousActiveTimeReport(&atpRpt.ActivePromotionStatus)
	}

	return r.post(msTeamsConfig, message, internal.ActivePromotionType)
}

func convertRPCImageListToK8SImageList(images []*rpc.Image) []s2hv1.Image {
	k8sImages := make([]s2hv1.Image, 0)
	for _, img := range images {
		k8sImages = append(k8sImages, s2hv1.Image{
			Repository: img.Repository,
			Tag:        img.Tag,
		})
	}

	return k8sImages
}

// SendImageMissing implements the reporter SendImageMissing function
func (r *reporter) SendImageMissing(configCtrl internal.ConfigController, imageMissingRpt *internal.ImageMissingReporter) error {
	msTeamsConfig, err := r.getMSTeamsConfig(imageMissingRpt.TeamName, configCtrl)
	if err != nil {
		return nil
	}

	message := r.makeImageMissingListReport([]s2hv1.Image{imageMissingRpt.Image}, imageMissingRpt.Reason)

	return r.post(msTeamsConfig, message, internal.ImageMissingType)
}

// SendPullRequestTriggerResult implements the reporter SendPullRequestTriggerResult function
func (r *reporter) SendPullRequestTriggerResult(configCtrl internal.ConfigController, prTriggerRpt *internal.PullRequestTriggerReporter) error {
	msTeamsConfig, err := r.getMSTeamsConfig(prTriggerRpt.TeamName, configCtrl)
	if err != nil {
		return nil
	}

	if msTeamsConfig.PullRequestTrigger != nil {
		err := util.CheckMatchingCriteria(msTeamsConfig.PullRequestTrigger.Criteria, prTriggerRpt.Result)
		if err != nil {
			return nil
		}
	}

	message := r.makePullRequestTriggerResultReport(prTriggerRpt)
	if len(prTriggerRpt.ImageMissingList) > 0 {
		message += "\n"
		message += r.makeImageMissingListReport(prTriggerRpt.ImageMissingList, "")
	}

	return r.post(msTeamsConfig, message, internal.PullRequestTriggerType)
}

// SendPullRequestTestRunnerPendingResult send pull request test runner pending status
func (r *reporter) SendPullRequestTestRunnerPendingResult(configCtrl internal.ConfigController, prTestRunnerRpt *internal.PullRequestTestRunnerPendingReporter) error {

	// does not support
	return nil
}

// SendActiveEnvironmentDeleted implements the reporter SendActiveEnvironmentDeleted function
func (r *reporter) SendActiveEnvironmentDeleted(configCtrl internal.ConfigController,
	activeNsDeletedRpt *internal.ActiveEnvironmentDeletedReporter) error {

	// does not support
	return nil
}

func (r *reporter) makeComponentUpgradeReport(comp *internal.ComponentUpgradeReporter) string {
	queueHistURL := `{{ .SamsahaiExternalURL }}/teams/{{ .TeamName }}/queue/histories/{{ .QueueHistoryName }}`
	queueLogURL := `{{ .SamsahaiExternalURL }}/teams/{{ .TeamName }}/queue/histories/{{ .QueueHistoryName }}/log`

	message := `
<b>Component Upgrade:</b><span {{ if eq .Status 1 }}` + styleInfo + `> Success {{ else }}` + styleDanger + `> Failure{{ end }}</span>
` + r.makeDeploymentQueueReport(comp, queueHistURL, queueLogURL)
	return strings.TrimSpace(template.TextRender("MSTeamsComponentUpgrade", message, comp))
}

func (r *reporter) makePullRequestQueueReport(comp *internal.ComponentUpgradeReporter) string {
	queueHistURL := `{{ .SamsahaiExternalURL }}/teams/{{ .TeamName }}/pullrequest/queue/histories/{{ .QueueHistoryName }}`
	queueLogURL := `{{ .SamsahaiExternalURL }}/teams/{{ .TeamName }}/pullrequest/queue/histories/{{ .QueueHistoryName }}/log`

	message := `
<b>Pull Request Queue:</b><span {{ if eq .Status 1 }}` + styleInfo + `> Success {{ else }}` + styleDanger + `> Failure{{ end }}</span>
{{- if .PullRequestComponent }}
<br/><b>Component:</b> {{ .PullRequestComponent.BundleName }}
<br/><b>PR Number:</b> {{ .PullRequestComponent.PRNumber }}
{{- end }}
` + r.makeDeploymentQueueReport(comp, queueHistURL, queueLogURL)
	return strings.TrimSpace(template.TextRender("MSTeamsPullRequestQueue", message, comp))
}

func (r *reporter) makeDeploymentQueueReport(comp *internal.ComponentUpgradeReporter, queueHistURL, queueLogURL string) string {
	message := `
{{- if eq .Status 0 }}
<br/><b>Issue type:</b> {{ .IssueTypeStr }}
{{- end }}
<br/><b>Run:</b>{{ if .PullRequestComponent }} #{{ .Runs }}{{ else if .IsReverify }} Reverify {{ else }} #{{ .Runs }}{{ end }}
<br/><b>Queue:</b> {{ .Name }}
{{- if .Components }}
<br/><b>Components:</b>
{{- range .Components }}
<li><b>- Name:</b> {{ .Name }}</li>
<li><b>&nbsp;&nbsp;Version:</b> {{ if .Image.Tag }}{{ .Image.Tag }}{{ else }}<code>no stable/active image tag found, using from values file</code>{{ end }}</li>
<li><b>&nbsp;&nbsp;Repository:</b> {{ if .Image.Repository }}{{ .Image.Repository }}{{ else }}<code>no stable/active image repository found, using from values file</code>{{ end }}</li>
{{- end }}
{{- end }}
<br/><b>Owner:</b> {{ .TeamName }}
<br/><b>Namespace:</b> {{ .Namespace }}
{{- if eq .Status 0 }}
{{- if .ComponentUpgrade.DeploymentIssues }}
<br/><b>Deployment Issues:</b>
{{- range .ComponentUpgrade.DeploymentIssues }}
<li><b>- Issue type:</b> {{ .IssueType }}</li>
<li><b>&nbsp;&nbsp;Components:</b> {{ range .FailureComponents }}{{ .ComponentName }},{{ end }}
    {{- if eq .IssueType "WaitForInitContainer" }}
<li><b>&nbsp;&nbsp;Wait for:</b> {{ range .FailureComponents }}{{ .FirstFailureContainerName }},{{ end }}
    {{- end }}
{{- end }} 
{{- end }}
{{- if .TestRunner.Teamcity.BuildURL }}
<br/><b>Teamcity URL:</b> <a href="{{ .TestRunner.Teamcity.BuildURL }}">#{{ .TestRunner.Teamcity.BuildNumber }}</a>
 {{- end }}
{{- if .TestRunner.Gitlab.PipelineURL }}
<br/><b>GitLab URL:</b> <a href="{{ .TestRunner.Gitlab.PipelineURL }}">#{{ .TestRunner.Gitlab.PipelineNumber }}</a>
{{- end }}
<br/><b>Deployment Logs:</b> <a href="` + queueLogURL + `">Download here</a>
<br/><b>Deployment History:</b> <a href="` + queueHistURL + `">Click here</a>
{{- end}}
`
	return strings.TrimSpace(template.TextRender("MSTeamsDeploymentQueue", message, comp))
}

func (r *reporter) makeActivePromotionStatusReport(comp *internal.ActivePromotionReporter) string {
	var message = `
<b>Active Promotion:</b> <span {{ if eq .Result "Success" }}` + styleInfo + `{{ else if eq .Result "Failure" }}` + styleDanger + `{{ end }}>{{ .Result }}</span>
{{- if ne .Result "Success" }}
{{- range .Conditions }}
 {{- if eq .Type "` + string(s2hv1.ActivePromotionCondActivePromoted) + `" }}
<br/><b>Reason:</b> {{ .Message }}
 {{- end }}
{{- end }}
{{- end }}
<br/><b>Run:</b> #{{ .Runs }}
<br/><b>Current Active Namespace:</b> {{ .CurrentActiveNamespace }}
<br/><b>Owner:</b> {{ .TeamName }}
{{- if eq .Result "Failure" }}
  {{- if .PreActiveQueue.DeploymentIssues }}
<br/><b>Deployment Issues:</b>
  {{- range .PreActiveQueue.DeploymentIssues }}
<li><b>- Issue type:</b> {{ .IssueType }}</li>
<li><b>&nbsp;&nbsp;Components:</b> {{ range .FailureComponents }}{{ .ComponentName }},{{ end }}
    {{- if eq .IssueType "WaitForInitContainer" }}
<li><b>&nbsp;&nbsp;Wait for:</b> {{ range .FailureComponents }}{{ .FirstFailureContainerName }},{{ end }}
    {{- end }}
  {{- end }} 
  {{- end }} 
{{- end }}
{{- if .PreActiveQueue.TestRunner }}
{{- if and .PreActiveQueue.TestRunner.Teamcity .PreActiveQueue.TestRunner.Teamcity.BuildURL }}
<br/><b>Teamcity URL:</b> <a href="{{ .PreActiveQueue.TestRunner.Teamcity.BuildURL }}">#{{ .PreActiveQueue.TestRunner.Teamcity.BuildNumber }}</a>
{{- end }}
{{- if and .PreActiveQueue.TestRunner.Gitlab .PreActiveQueue.TestRunner.Gitlab.PipelineURL }}
<br/><b>GitLab URL:</b> <a href="{{ .PreActiveQueue.TestRunner.Gitlab.PipelineURL }}">#{{ .PreActiveQueue.TestRunner.Gitlab.PipelineNumber }}</a>
{{- end }}
{{- end }}
{{- if eq .Result "Failure" }}
<br/><b>Deployment Logs:</b> <a href="{{ .SamsahaiExternalURL }}/teams/{{ .TeamName }}/activepromotions/histories/{{ .ActivePromotionHistoryName }}/log">Download here</a>
{{- end }}
<br/><b>Active Promotion History:</b> <a href="{{ .SamsahaiExternalURL }}/teams/{{ .TeamName }}/activepromotions/histories/{{ .ActivePromotionHistoryName }}">Click here</a>
`

	return strings.TrimSpace(template.TextRender("MSTeamsActivePromotionStatus", message, comp))
}

func (r *reporter) makeOutdatedComponentsReport(comps map[string]s2hv1.OutdatedComponent) string {
	var message = `
<b>Outdated Components:</b>
{{- range $name, $component := .Components }}
{{- if gt .OutdatedDuration 0 }}
<br/>
<b>{{ $name }}</b>
<li>Not update for {{ .OutdatedDuration | FmtDurationToStr }}</li>
<li>Current Version: <a href="{{ .CurrentImage.Repository | ConcatHTTPStr }}">{{ .CurrentImage.Tag }}</a></li>
<li>Latest Version: <a href="{{ .DesiredImage.Repository | ConcatHTTPStr }}">{{ .DesiredImage.Tag }}</a></li>
{{- end }}
{{- end }}
`

	ocObj := struct {
		Components map[string]s2hv1.OutdatedComponent
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
	var message = "<b " + styleDanger + ">ERROR:</b> cannot rollback an active promotion process due to timeout"

	return strings.TrimSpace(template.TextRender("RollbackFailure", message, ""))
}

func (r *reporter) makeActiveDemotingFailureReport() string {
	var message = "<b " + styleWarning + ">WARNING:</b> cannot demote a previous active environment, previous active namespace has been destroyed immediately"

	return strings.TrimSpace(template.TextRender("DemotionFailure", message, ""))
}

func (r *reporter) makeDestroyedPreviousActiveTimeReport(status *s2hv1.ActivePromotionStatus) string {
	var message = "<b " + styleWarning + ">NOTES:</b> previous active namespace <code>{{ .PreviousActiveNamespace }}</code> will be destroyed at <code>{{ .DestroyedTime | TimeFormat }}</code>"

	return strings.TrimSpace(template.TextRender("DestroyedTime", message, status))
}

func (r *reporter) makeImageMissingListReport(images []s2hv1.Image, reason string) string {
	var reasonMsg string
	if reason != "" {
		reasonMsg = fmt.Sprintf("   <code>%s</code>", reason)
	}

	var message = `
<b>Image Missing List:</b>
{{- range .Images }}
<li>{{ .Repository }}:{{ .Tag }}</li>
` + reasonMsg + `
{{- end }}
`

	imagesObj := struct{ Images []s2hv1.Image }{Images: images}
	return strings.TrimSpace(template.TextRender("MSTeamsImageMissingList", message, imagesObj))
}

func (r *reporter) makePullRequestTriggerResultReport(prTriggerRpt *internal.PullRequestTriggerReporter) string {
	var message = `
<b>Pull Request Trigger:</b>  <span {{ if eq .Result "Success" }}` + styleInfo + `{{ else if eq .Result "Failure" }}` + styleDanger + `{{ end }}>{{ .Result }}</span>
<br/><b>Bundle:</b> {{ .BundleName }}
<br/><b>PR Number:</b> {{ .PRNumber }}
<br/><b>Components:</b>
{{- if .Components }}
{{- range .Components }}
<li><b>- Name:</b> {{ .ComponentName }}</li>
<li><b>&nbsp;&nbsp;Image:</b> {{ if .Image }}{{ .Image.Repository }}:{{ .Image.Tag }}{{ else }}no image defined{{ end }}
{{- end }}
{{- else }}
<br/><code>no components defined</code>
{{- end }}
<br/><b>NO of Retry:</b> {{ .NoOfRetry }}
<br/><b>Owner:</b> {{ .TeamName }}
<br/><b>Start at:</b> {{ .CreatedAt | TimeFormat }}
`

	return strings.TrimSpace(template.TextRender("MSTeamsPullRequestTriggerResult", message, prTriggerRpt))
}

func (r *reporter) post(msTeamsConfig *s2hv1.ReporterMSTeams, message string, event internal.EventType) error {
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
		// get group id from group name
		groupID, err := r.msTeams.GetGroupID(group.GroupNameOrID, accessToken)
		if err != nil {
			logger.Error(err, "cannot get group id",
				"event", event, "group", group.GroupNameOrID)
			continue
		}

		for _, channelNameOrID := range group.ChannelNameOrIDs {
			// get channel id from channel name
			channelID, err := r.msTeams.GetChannelID(groupID, channelNameOrID, accessToken)
			if err != nil {
				logger.Error(err, "cannot get channel id",
					"event", event, "group", group.GroupNameOrID, "channel", channelNameOrID)
				continue
			}

			if err := r.msTeams.PostMessage(groupID, channelID, message, accessToken, msteams.WithContentType(msteams.HTML)); err != nil {
				logger.Error(err, "cannot post message to Microsoft Teams",
					"event", event, "group", group.GroupNameOrID, "channel", channelNameOrID)
				globalErr = err
				continue
			}
		}
	}
	return globalErr
}

func (r *reporter) getMSTeamsConfig(teamName string, configCtrl internal.ConfigController) (*s2hv1.ReporterMSTeams, error) {
	config, err := configCtrl.Get(teamName)
	if err != nil {
		return nil, err
	}

	// no Microsoft Teams configuration
	if config.Status.Used.Reporter == nil || config.Status.Used.Reporter.MSTeams == nil {
		return nil, s2herrors.New("msTeams configuration not found")
	}

	return config.Status.Used.Reporter.MSTeams, nil
}
