package reportermock

import (
	"github.com/agoda-com/samsahai/internal"
	"github.com/agoda-com/samsahai/pkg/samsahai/rpc"
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

// SendActivePromotionStatus implements the reporter SendActivePromotionStatus function
func (r *reporterMock) SendActivePromotionStatus(configCtrl internal.ConfigController, atpRpt *internal.ActivePromotionReporter) error {
	return nil
}

// SendImageMissing implements the reporter SendImageMissingList function
func (r *reporterMock) SendImageMissing(teamName string, configCtrl internal.ConfigController, image *rpc.Image) error {
	return nil
}
