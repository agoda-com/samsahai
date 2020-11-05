package reportermock

import (
	"github.com/agoda-com/samsahai/internal"
)

const (
	reporterName = "reportermock"
)

type reporterMock struct{}

// New creates a new mock reporter
func New() internal.Reporter {
	return &reporterMock{}
}

// GetName implements the staging testRunner GetName function
func (r *reporterMock) GetName() string {
	return reporterName
}

// SendMessage implements the reporter SendMessage function
func (r *reporterMock) SendMessage(configCtrl internal.ConfigController, message string) error {
	return nil
}

// SendComponentUpgrade implements the reporter SendComponentUpgrade function
func (r *reporterMock) SendComponentUpgrade(configCtrl internal.ConfigController, component *internal.ComponentUpgradeReporter) error {
	return nil
}

// SendPullRequestQueue implements the reporter SendPullRequestQueue function
func (r *reporterMock) SendPullRequestQueue(configCtrl internal.ConfigController, comp *internal.ComponentUpgradeReporter) error {
	return nil
}

// SendActivePromotionStatus implements the reporter SendActivePromotionStatus function
func (r *reporterMock) SendActivePromotionStatus(configCtrl internal.ConfigController, atpRpt *internal.ActivePromotionReporter) error {
	return nil
}

// SendImageMissing implements the reporter SendImageMissingList function
func (r *reporterMock) SendImageMissing(configCtrl internal.ConfigController, imageMissingRpt *internal.ImageMissingReporter) error {
	return nil
}

// SendPullRequestTriggerResult implements the reporter SendPullRequestTriggerResult function
func (r *reporterMock) SendPullRequestTriggerResult(configCtrl internal.ConfigController, prTriggerRpt *internal.PullRequestTriggerReporter) error {
	return nil
}
