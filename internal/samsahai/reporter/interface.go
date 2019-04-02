package reporter

import (
	"github.com/agoda-com/samsahai/internal/samsahai/component"
)

// Defines types of report
const (
	ComponentUpgradeFail = "component-upgrade-fail"
	ActivePromotion      = "active-promotion"
	OutdatedComponent    = "outdated-component"
	ImageMissing         = "image-missing"
)

// Option provides option when sending reporter
type Option struct {
	Key   string
	Value interface{}
}

// Defines type of send component upgrade fail options
const (
	OptionIssueType     = "issue-type"
	OptionValuesFileURL = "values-file-url"
	OptionCIURL         = "ci-url"
	OptionLogsURL       = "logs-url"
	OptionErrorURL      = "error-url"
)

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

// Reporter is the interface of reporter
type Reporter interface {
	// SendMessage send generic message
	SendMessage(message string) error

	// SendComponentUpgradeFail send details of component upgrade fail
	SendComponentUpgradeFail(component *component.Component, serviceOwner string, options ...Option) error

	// SendActivePromotionStatus send active promotion status
	SendActivePromotionStatus(status, currentActiveNamespace, serviceOwner string, components []component.OutdatedComponent, showedDetails bool) error

	// SendOutdatedComponent send outdated components
	SendOutdatedComponents(components []component.OutdatedComponent) error

	// SendImageMissingList send image missing list
	SendImageMissingList(images []component.Image) error
}
