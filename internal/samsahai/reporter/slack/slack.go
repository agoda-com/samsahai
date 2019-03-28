package slack

import (
	"github.com/agoda-com/samsahai/internal/samsahai/component"
	"github.com/agoda-com/samsahai/internal/samsahai/reporter"
	"github.com/agoda-com/samsahai/internal/samsahai/reporter/msg"
	"github.com/agoda-com/samsahai/internal/samsahai/util/slack"
)

// Ensure Slack implements Reporter
var _ reporter.Reporter = &Slack{}

// Slack implements the Reporter interface
type Slack struct {
	Token    string   `required:"true"`
	Channels []string `required:"true"`
	Username string
}

func NewSlack(token, username string, channels []string) *Slack {
	return &Slack{
		Token:    token,
		Channels: channels,
		Username: username,
	}
}

func (s *Slack) SendMessage(message string) error {
	if err := postSlackMessage(s, message); err != nil {
		return err
	}

	return nil
}

// TODO: implements
func (s *Slack) SendFailedComponentUpgrade() error {
	s.Username = getSlackUsername(reporter.FailedComponentUpgrade, s.Username)
	return nil
}

func (s *Slack) SendActivePromotionStatus(status, currentActiveNamespace, serviceOwner string, components []component.Component, showedDetails bool) error {
	atvPromotion := msg.ActivePromotion{
		Status:                 status,
		ServiceOwner:           serviceOwner,
		CurrentActiveNamespace: currentActiveNamespace,
		Components:             components,
	}

	message := atvPromotion.NewActivePromotionMessage(showedDetails)
	s.Username = getSlackUsername(reporter.ActivePromotion, s.Username)
	if err := postSlackMessage(s, message); err != nil {
		return err
	}

	return nil
}

// TODO: implements
func (s *Slack) SendOutdatedComponents() error {
	s.Username = getSlackUsername(reporter.OutdatedComponent, s.Username)
	return nil
}

// TODO: implements
func (s *Slack) SendImageMissingList(images []component.Image) error {
	return nil
}

func getSlackUsername(notiType, username string) string {
	if username != "" {
		return username
	}

	switch notiType {
	case reporter.ImageMissing:
		return "Image Missing Alert"
	case reporter.OutdatedComponent:
		return "Component Outdated Summary"
	default:
		return "Samsahai Notification"
	}
}

func postSlackMessage(s *Slack, message string) error {
	slackCli := slack.NewClient(s.Token)

	for _, channel := range s.Channels {
		if _, _, err := slackCli.PostMessage(channel, message, s.Username); err != nil {
			return err
		}
	}

	return nil
}
