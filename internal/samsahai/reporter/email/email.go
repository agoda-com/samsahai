package email

import (
	"log"

	"github.com/agoda-com/samsahai/internal/samsahai/component"
	"github.com/agoda-com/samsahai/internal/samsahai/reporter"
	"github.com/agoda-com/samsahai/internal/samsahai/util/email"
)

// Ensure Email implements Reporter
var _ reporter.Reporter = &Email{}

// Email implements the Reporter interface
type Email struct {
	Server string   `required:"true"`
	Port   int      `required:"true"`
	From   []string `required:"true"`
	To     []string `required:"true"`
	client email.Email
}

// NewEmail creates a new email
func NewEmail(server string, port int, from, to []string) *Email {
	emailCli := email.NewClient(server, port)
	return NewEmailWithClient(server, port, from, to, emailCli)
}

// NewEmailWithClient creates a new email with client
func NewEmailWithClient(server string, port int, from, to []string, client email.Email) *Email {
	return &Email{
		Server: server,
		Port:   port,
		To:     to,
		From:   from,
		client: client,
	}
}

// MakeComponentUpgradeFailReport mocks MakeComponentUpgradeFailReport function
func (e *Email) MakeComponentUpgradeFailReport(cuf *reporter.ComponentUpgradeFail, options ...reporter.Option) string {
	text, err := getComponentUpgradeFailWithTemplate(cuf)
	if err != nil {
		log.Printf("Cannot get component upgrade fail report!!,   %v", err)
		return "Cannot get component upgrade fail report!!"
	}

	return text
}

// MakeActivePromotionStatusReport implements the reporter MakeActivePromotionStatusReport function
func (e *Email) MakeActivePromotionStatusReport(atv *reporter.ActivePromotion, options ...reporter.Option) string {
	component.SortComponentsByOutdatedDays(atv.Components)
	text, err := getActivePromotionWithTemplate(atv)
	if err != nil {
		log.Printf("Cannot get active promotion status report!!,   %v", err)
		return "Cannot get active promotion status report!!"
	}

	return text
}

// MakeOutdatedComponentsReport implements the reporter MakeOutdatedComponentsReport function
func (e *Email) MakeOutdatedComponentsReport(oc *reporter.OutdatedComponents, options ...reporter.Option) string {
	component.SortComponentsByOutdatedDays(oc.Components)
	text, err := getOutdatedComponentWithTemplate(oc)
	if err != nil {
		log.Printf("Cannot get outdated components report!!,   %v", err)
		return "Cannot get outdated components report!!"
	}

	return text
}

// MakeImageMissingListReport mocks MakeImageMissingListReport function
func (e *Email) MakeImageMissingListReport(im *reporter.ImageMissing, options ...reporter.Option) string {
	text, err := getImageMissingWithTemplate(im)
	if err != nil {
		log.Printf("Cannot get image missing report!!,   %v", err)
		return "Cannot get image missing report!!"
	}

	return text
}

// SendMessage implements the reporter SendMessage function
func (e *Email) SendMessage(body string, options ...reporter.Option) error {
	var subject string
	for _, opt := range options {
		switch opt.Key {
		case reporter.OptionSubject:
			subject = opt.Value.(string)
		}
	}

	if err := sendMessage(e, subject, body); err != nil {
		return err
	}

	return nil
}

// SendComponentUpgradeFail implements the reporter SendComponentUpgradeFail function
func (e *Email) SendComponentUpgradeFail(component *component.Component, serviceOwner string, options ...reporter.Option) error {
	var overridenSubject, issueType, valuesFileURL, ciURL, logsURL, errorURL string
	for _, opt := range options {
		switch opt.Key {
		case reporter.OptionSubject:
			overridenSubject = opt.Value.(string)
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
	message := e.MakeComponentUpgradeFailReport(cuf, options...)
	subject := getSubject(reporter.TypeComponentUpgradeFail, "", overridenSubject)
	if err := sendMessage(e, subject, message); err != nil {
		return err
	}

	return nil
}

// SendActivePromotionStatus implements the reporter SendActivePromotionStatus function
func (e *Email) SendActivePromotionStatus(status, currentActiveNamespace, serviceOwner string, components []component.OutdatedComponent, options ...reporter.Option) error {
	var overridenSubject string
	for _, opt := range options {
		switch opt.Key {
		case reporter.OptionSubject:
			overridenSubject = opt.Value.(string)
		}
	}

	statusText := getStatusText(status)
	atv := reporter.NewActivePromotion(statusText, serviceOwner, currentActiveNamespace, components)
	message := e.MakeActivePromotionStatusReport(atv, options...)
	subject := getSubject(reporter.TypeActivePromotion, statusText, overridenSubject)
	if err := sendMessage(e, subject, message); err != nil {
		return err
	}

	return nil
}

// SendOutdatedComponents implements the reporter SendOutdatedComponents function
func (e *Email) SendOutdatedComponents(components []component.OutdatedComponent, options ...reporter.Option) error {
	var overridenSubject string
	for _, opt := range options {
		switch opt.Key {
		case reporter.OptionSubject:
			overridenSubject = opt.Value.(string)
		}
	}

	oc := reporter.NewOutdatedComponents(components, false)
	message := e.MakeOutdatedComponentsReport(oc, options...)
	subject := getSubject(reporter.TypeOutdatedComponent, "", overridenSubject)
	if err := sendMessage(e, subject, message); err != nil {
		return err
	}

	return nil
}

// SendImageMissingList implements the reporter SendImageMissingList function
func (e *Email) SendImageMissingList(images []component.Image, options ...reporter.Option) error {
	var overridenSubject string
	for _, opt := range options {
		switch opt.Key {
		case reporter.OptionSubject:
			overridenSubject = opt.Value.(string)
		}
	}

	im := reporter.NewImageMissing(images)
	message := e.MakeImageMissingListReport(im, options...)
	subject := getSubject(reporter.TypeImageMissing, "", overridenSubject)
	if err := sendMessage(e, subject, message); err != nil {
		return err
	}

	return nil
}

func sendMessage(e *Email, subject, body string) error {
	log.Println("Start sending message via email")

	if err := e.client.SendMessage(e.From, e.To, subject, body); err != nil {
		return err
	}

	return nil
}
