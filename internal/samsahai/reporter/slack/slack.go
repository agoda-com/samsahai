package slack

import (
	"fmt"
	"log"
	"strings"

	"github.com/agoda-com/samsahai/internal/samsahai/component"
	"github.com/agoda-com/samsahai/internal/samsahai/reporter"
	"github.com/agoda-com/samsahai/internal/samsahai/util/slack"
)

const (
	defaultUsername = "Samsahai Notification"
)

// Ensure Slack implements Reporter
var _ reporter.Reporter = &Slack{}

// Slack implements the Reporter interface
type Slack struct {
	Token      string   `required:"true"`
	Channels   []string `required:"true"`
	Username   string
	client     slack.Slack
	ReportType string
}

// NewSlack returns reporter for sending report to slack at specific `channels`
func NewSlack(token, username string, channels []string) *Slack {
	return NewSlackWithClient(token, username, channels, slack.NewClient(token))
}

// NewSlackWithClient returns reporter for sending report to slack at specific `channels`
// and also needs to manually provide slack `client`
func NewSlackWithClient(token, username string, channels []string, client slack.Slack) *Slack {
	if username == "" {
		username = defaultUsername
	}
	return &Slack{
		Token:    token,
		Channels: channels,
		Username: username,
		client:   client,
	}
}

// MakeComponentUpgradeFailReport returns formatted string for sending to slack
func (s *Slack) MakeComponentUpgradeFailReport(cuf *reporter.ComponentUpgradeFail, options ...reporter.Option) string {
	message := `
*Component Upgrade Failed*
"Stable version of component failed" - Please check your test
>*Component:* {{.Component.Name}}
>*Version:* {{.Component.Version}}
>*Repository:* {{.Component.Image.Repository}}
>*Issue type:* {{.IssueType}}
>*Values file url:* {{if .ValuesFileURL}}<{{.ValuesFileURL}}|Click here>{{else}}-{{end}}
>*CI url:* {{if .CIURL}}<{{.CIURL}}|Click here>{{else}}-{{end}}
>*Logs:* {{if .LogsURL}}<{{.LogsURL}}|Click here>{{else}}-{{end}}
>*Error:* {{if .ErrorURL}}<{{.ErrorURL}}|Click here>{{else}}-{{end}}
>*Owner:* {{.ServiceOwner}}
`
	return strings.TrimSpace(render("ComponentUpgradeFailed", message, cuf))
}

// MakeActivePromotionStatusReport implements the reporter MakeActivePromotionStatusReport function
func (s *Slack) MakeActivePromotionStatusReport(data *reporter.ActivePromotion, options ...reporter.Option) string {
	var showedDetails bool

	for _, opt := range options {
		switch opt.Key {
		case reporter.OptionShowedDetails:
			showedDetails = opt.Value.(bool)
		}
	}

	var template = `*Active Promotion:* ` + getStatusText(data.Status) + `
*Owner:* {{.ServiceOwner}}
*Current Active Namespace:* {{.CurrentActiveNamespace}}
`
	oc := reporter.NewOutdatedComponents(data.Components, showedDetails)

	return fmt.Sprintf("%s\n%s",
		strings.TrimSpace(render("ActivePromotionStatus", template, data)),
		s.MakeOutdatedComponentsReport(oc),
	)
}

// MakeOutdatedComponentsReport implements the reporter MakeOutdatedComponentsReport function
func (s *Slack) MakeOutdatedComponentsReport(oc *reporter.OutdatedComponents, options ...reporter.Option) string {
	var template = `
*Outdated Components*
{{- $showedDetails := .ShowedDetails }}
{{- range .Components }}
{{- if gt .OutdatedDays 0 }}
*{{.CurrentComponent.Name}}*
>Not update for {{.OutdatedDays}} day(s)
>Current version: {{.CurrentComponent.Version}}
>New Version: {{.NewComponent.Version}}
{{- else if $showedDetails }}
*{{.CurrentComponent.Name}}*
>Current version: {{.CurrentComponent.Version}}
{{- end }}
{{- end }}
`

	return strings.TrimSpace(render("OutdatedComponents", template, oc))
}

// MakeImageMissingListReport implements the reporter MakeImageMissingListReport function
func (s *Slack) MakeImageMissingListReport(im *reporter.ImageMissing, options ...reporter.Option) string {
	var template = `
{{- range .Images}}
{{.Repository}}:{{.Tag}} (image missing)
{{- end }}
`

	return strings.TrimSpace(render("ImageMissingList", template, im))
}

// SendMessage implements the reporter SendMessage function
func (s *Slack) SendMessage(message string, options ...reporter.Option) error {
	return s.post(message)
}

// SendComponentUpgradeFail implements the reporter SendComponentUpgradeFail function
func (s *Slack) SendComponentUpgradeFail(component *component.Component, serviceOwner string, options ...reporter.Option) error {
	var issueType, valuesFileURL, ciURL, logsURL, errorURL string
	for _, opt := range options {
		switch opt.Key {
		case reporter.OptionIssueType:
			issueType = opt.Value.(string)
		case reporter.OptionValuesFileURL:
			valuesFileURL = opt.Value.(string)
		case reporter.OptionCIURL:
			ciURL = opt.Value.(string)
		case reporter.OptionLogsURL:
			logsURL = opt.Value.(string)
		case reporter.OptionErrorURL:
			errorURL = opt.Value.(string)
		}
	}

	cuf := reporter.NewComponentUpgradeFail(component, serviceOwner, issueType, valuesFileURL, ciURL, logsURL, errorURL)
	message := s.MakeComponentUpgradeFailReport(cuf)
	s.ReportType = reporter.TypeComponentUpgradeFail

	return s.post(message)
}

// SendActivePromotionStatus implements the reporter SendActivePromotionStatus function
func (s *Slack) SendActivePromotionStatus(status, currentActiveNamespace, serviceOwner string, components []component.OutdatedComponent, options ...reporter.Option) error {
	atv := reporter.NewActivePromotion(status, serviceOwner, currentActiveNamespace, components)
	message := s.MakeActivePromotionStatusReport(atv, options...)
	s.ReportType = reporter.TypeActivePromotion

	return s.post(message)
}

// SendOutdatedComponents implements the reporter SendOutdatedComponents function
func (s *Slack) SendOutdatedComponents(components []component.OutdatedComponent, options ...reporter.Option) error {
	oc := reporter.NewOutdatedComponents(components, false)
	message := s.MakeOutdatedComponentsReport(oc)
	s.ReportType = reporter.TypeOutdatedComponent

	return s.post(message)
}

// SendImageMissingList implements the reporter SendImageMissingList function
func (s *Slack) SendImageMissingList(images []component.Image, options ...reporter.Option) error {
	im := reporter.NewImageMissing(images)
	message := s.MakeImageMissingListReport(im)
	s.ReportType = reporter.TypeImageMissing

	return s.post(message)
}

func (s *Slack) post(message string) error {
	log.Println("Start sending message to slack channels")

	username := s.Username
	if s.ReportType != "" {
		username = s.getReporterUsername()
	}

	for _, channel := range s.Channels {
		if _, _, err := s.client.PostMessage(channel, message, username); err != nil {
			return err
		}
	}
	return nil
}

func (s *Slack) getReporterUsername() string {
	switch s.ReportType {
	case reporter.TypeImageMissing:
		return "Image Missing Alert"
	case reporter.TypeOutdatedComponent:
		return "Components Outdated Summary"
	default:
		return defaultUsername
	}
}
