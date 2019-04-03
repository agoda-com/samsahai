package reporter

import (
	"github.com/agoda-com/samsahai/internal/samsahai/component"
)

// Defines types of report
const (
	TypeComponentUpgradeFail = "component-upgrade-fail"
	TypeActivePromotion      = "active-promotion"
	TypeOutdatedComponent    = "outdated-component"
	TypeImageMissing         = "image-missing"
)

// Option provides option when sending reporter
type Option struct {
	Key   string
	Value interface{}
}

const (
	// Defines type of send message options
	OptionSubject = "subject"
	// Defines type of send component upgrade fail options
	OptionIssueType     = "issue-type"
	OptionValuesFileURL = "values-file-url"
	OptionCIURL         = "ci-url"
	OptionLogsURL       = "logs-url"
	OptionErrorURL      = "error-url"
	// Defines type of send active promotion status options
	OptionShowedDetails = "showed-details"
)

// NewOptionSubject set the subject for send message
func NewOptionSubject(subject string) Option {
	return Option{Key: OptionSubject, Value: subject}
}

// NewOptionShowedDetails set the showed details flag for send active promotion status
func NewOptionShowedDetails(showedDetails bool) Option {
	return Option{Key: OptionShowedDetails, Value: showedDetails}
}

// NewOptionIssueType set the issue type for send component upgrade fail
func NewOptionIssueType(issueType string) Option {
	return Option{Key: OptionIssueType, Value: issueType}
}

// NewOptionValuesFileURL set the values file url for send component upgrade fail
func NewOptionValuesFileURL(valuesFile string) Option {
	return Option{Key: OptionValuesFileURL, Value: valuesFile}
}

// NewOptionCIURL set the ci url for send component upgrade fail
func NewOptionCIURL(ciURL string) Option {
	return Option{Key: OptionCIURL, Value: ciURL}
}

// NewOptionLogsURL set the logs url for send component upgrade fail
func NewOptionLogsURL(logsURL string) Option {
	return Option{Key: OptionLogsURL, Value: logsURL}
}

// NewOptionErrorURL set the error url for send component upgrade fail
func NewOptionErrorURL(errorURL string) Option {
	return Option{Key: OptionErrorURL, Value: errorURL}
}

// ComponentUpgradeFail manages components upgrade fail report
type ComponentUpgradeFail struct {
	Component     *component.Component `required:"true"`
	ServiceOwner  string               `required:"true"`
	IssueType     string
	ValuesFileURL string
	CIURL         string
	LogsURL       string
	ErrorURL      string
}

// NewComponentUpgradeFail creates a new component upgrade fail
func NewComponentUpgradeFail(component *component.Component, serviceOwner, issueType, valuesFileURL, ciURL, logsURL, errorURL string) *ComponentUpgradeFail {
	upgradeFail := &ComponentUpgradeFail{
		Component:     component,
		ServiceOwner:  serviceOwner,
		IssueType:     issueType,
		ValuesFileURL: valuesFileURL,
		CIURL:         ciURL,
		LogsURL:       logsURL,
		ErrorURL:      errorURL,
	}

	return upgradeFail
}

// ActivePromotion manages active promotion report
type ActivePromotion struct {
	Status                 string
	ServiceOwner           string
	CurrentActiveNamespace string
	Components             []component.OutdatedComponent
}

// NewActivePromotion creates a new active promotion
func NewActivePromotion(status, serviceOwner, currentActiveNamespace string, components []component.OutdatedComponent) *ActivePromotion {
	return &ActivePromotion{
		Status:                 status,
		ServiceOwner:           serviceOwner,
		CurrentActiveNamespace: currentActiveNamespace,
		Components:             components,
	}
}

// OutdatedComponents manages outdated components report
type OutdatedComponents struct {
	Components    []component.OutdatedComponent
	ShowedDetails bool
}

// NewOutdatedComponents creates a new outdated components
func NewOutdatedComponents(components []component.OutdatedComponent, showedDetails bool) *OutdatedComponents {
	return &OutdatedComponents{Components: components, ShowedDetails: showedDetails}
}

// ImageMissing manages image missing report
type ImageMissing struct {
	Images []component.Image
}

// NewImageMissing creates a new image missing
func NewImageMissing(images []component.Image) *ImageMissing {
	return &ImageMissing{Images: images}
}

// Reporter is the interface of reporter
type Reporter interface {
	// SendMessage send generic message
	SendMessage(message string, options ...Option) error

	// SendComponentUpgradeFail send details of component upgrade fail
	SendComponentUpgradeFail(component *component.Component, serviceOwner string, options ...Option) error

	// SendActivePromotionStatus send active promotion status
	SendActivePromotionStatus(status, currentActiveNamespace, serviceOwner string, components []component.OutdatedComponent, options ...Option) error

	// SendOutdatedComponent send outdated components
	SendOutdatedComponents(components []component.OutdatedComponent, options ...Option) error

	// SendImageMissingList send image missing list
	SendImageMissingList(images []component.Image, options ...Option) error
}
