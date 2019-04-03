package rest

import (
	"encoding/json"
	"log"

	"github.com/agoda-com/samsahai/internal/samsahai/component"
	"github.com/agoda-com/samsahai/internal/samsahai/reporter"
	"github.com/agoda-com/samsahai/internal/samsahai/util/http"
)

// Rest implements the Reporter interface for http callback
type Rest struct {
	client *http.Client
}

// Ensure Slack implements Reporter
var _ reporter.Reporter = &Rest{}

// ReporterJSON represents generic json data for http POST report
type ReporterJSON struct {
	EventType          string                        `json:"event_type"`
	Message            string                        `json:"message,omitempty"`
	Status             string                        `json:"status,omitempty"`
	ServiceOwner       string                        `json:"service_owner,omitempty"`
	ActiveNamespace    string                        `json:"active_namespace,omitempty"`
	Component          *component.Component          `json:"component,omitempty"`
	Images             []component.Image             `json:"images,omitempty"`
	OutdatedComponents []component.OutdatedComponent `json:"outdated_components,omitempty"`
	IssueType          string                        `json:"issue_type,omitempty"`
	URLS               ReportURLJSON                 `json:"urls,omitempty"`
}

// ReporterURLJSON represents url json data in ReporterJSON for http POST report
type ReportURLJSON struct {
	ValuesFile string `json:"values_file,omitempty"`
	CI         string `json:"ci,omitempty"`
	Logs       string `json:"logs,omitempty"`
	Error      string `json:"error,omitempty"`
}

// NewRest creates Rest client for sending http POST
func NewRest(callbackURL string) *Rest {
	var r Rest
	r.client = http.NewClient(callbackURL)
	return &r
}

// SendMessage send generic message via http POST
func (r *Rest) SendMessage(message string, options ...reporter.Option) error {
	data := ReporterJSON{
		EventType: "message",
		Message:   message,
	}
	return r.send(data)
}

// SendComponentUpgradeFail send details of component upgrade fail via http POST
func (r *Rest) SendComponentUpgradeFail(component *component.Component, serviceOwner string, options ...reporter.Option) error {
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

	data := ReporterJSON{
		EventType:    "component_upgrade_failed",
		ServiceOwner: serviceOwner,
		Component:    component,
		IssueType:    issueType,
		URLS:         ReportURLJSON{ValuesFile: valuesFileURL, CI: ciURL, Logs: logsURL, Error: errorURL},
	}

	return r.send(data)
}

// SendActivePromotionStatus send active promotion status via http POST
func (r *Rest) SendActivePromotionStatus(status, currentActiveNamespace, serviceOwner string, components []component.OutdatedComponent, options ...reporter.Option) error {
	data := ReporterJSON{
		EventType:          "active_promotion",
		Status:             status,
		ActiveNamespace:    currentActiveNamespace,
		ServiceOwner:       serviceOwner,
		OutdatedComponents: components,
	}
	return r.send(data)
}

// SendOutdatedComponent send outdated components via http POST
func (r *Rest) SendOutdatedComponents(components []component.OutdatedComponent, options ...reporter.Option) error {
	data := ReporterJSON{
		EventType:          "outdated_components",
		OutdatedComponents: components,
	}
	return r.send(data)
}

// SendImageMissingList send image missing list via http POST
func (r *Rest) SendImageMissingList(images []component.Image, options ...reporter.Option) error {
	data := ReporterJSON{
		EventType: "missing_image",
		Component: nil,
		Images:    images,
	}
	return r.send(data)
}

// send provides handling convert ReporterJSON to []byte and sent it via http POST
func (r *Rest) send(data ReporterJSON) error {
	log.Println("Start sending data via http POST")

	var err error
	var body []byte

	body, err = json.Marshal(data)
	if err != nil {
		return err
	}

	_, err = r.client.Post("/", body)
	if err != nil {
		return err
	}

	return nil
}
