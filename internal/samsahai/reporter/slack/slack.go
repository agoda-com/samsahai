package slack

import (
	"log"

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
	client   slack.Slack
}

// NewSlack creates a new slack
func NewSlack(token, username string, channels []string) *Slack {
	slackCli := slack.NewClient(token)
	return NewSlackWithClient(token, username, channels, slackCli)
}

// NewSlackWithClient creates a new slack with client
func NewSlackWithClient(token, username string, channels []string, client slack.Slack) *Slack {
	return &Slack{
		Token:    token,
		Channels: channels,
		Username: username,
		client:   client,
	}
}

// SendMessage implements the reporter SendMessage function
func (s *Slack) SendMessage(message string) error {
	if err := postSlackMessage(s, message); err != nil {
		return err
	}

	return nil
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

	upgradeFail := msg.NewComponentUpgradeFail(component, serviceOwner, issueType, valuesFileURL, ciURL, logsURL, errorURL)
	message := upgradeFail.NewComponentUpgradeFailMessage()
	s.Username = getSlackUsername(reporter.ComponentUpgradeFail, s.Username)
	if err := postSlackMessage(s, message); err != nil {
		return err
	}

	return nil
}

// SendActivePromotionStatus implements the reporter SendActivePromotionStatus function
func (s *Slack) SendActivePromotionStatus(status, currentActiveNamespace, serviceOwner string, components []component.OutdatedComponent, showedDetails bool) error {
	atvPromotion := msg.NewActivePromotion(status, serviceOwner, currentActiveNamespace, components)
	message := atvPromotion.NewActivePromotionMessage(showedDetails)
	s.Username = getSlackUsername(reporter.ActivePromotion, s.Username)
	if err := postSlackMessage(s, message); err != nil {
		return err
	}

	return nil
}

// SendOutdatedComponents implements the reporter SendOutdatedComponents function
func (s *Slack) SendOutdatedComponents(components []component.OutdatedComponent) error {
	oc := msg.OutdatedComponents{Components: components}
	message := oc.NewOutdatedComponentsMessage()
	s.Username = getSlackUsername(reporter.OutdatedComponent, s.Username)
	if err := postSlackMessage(s, message); err != nil {
		return err
	}

	return nil
}

// SendImageMissingList implements the reporter SendImageMissingList function
func (s *Slack) SendImageMissingList(images []component.Image) error {
	im := msg.NewImageMissing(images)
	message := im.NewImageMissingListMessage()
	s.Username = getSlackUsername(reporter.ImageMissing, s.Username)
	if err := postSlackMessage(s, message); err != nil {
		return err
	}

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
		return "Components Outdated Summary"
	default:
		return "Samsahai Notification"
	}
}

func postSlackMessage(s *Slack, message string) error {
	log.Printf("Start sending message to slack channels")

	for _, channel := range s.Channels {
		if _, _, err := s.client.PostMessage(channel, message, s.Username); err != nil {
			return err
		}
	}

	return nil
}
