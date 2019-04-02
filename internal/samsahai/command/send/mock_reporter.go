package send

import (
	"github.com/agoda-com/samsahai/internal/samsahai/component"
	"github.com/agoda-com/samsahai/internal/samsahai/reporter"
)

// Ensure mockReporter implements Reporter
var _ reporter.Reporter = &mockReporter{}

// mockReporter mocks Reporter interface
type mockReporter struct {
	sendActivePromotionStatusCalls int
	sendImageMissingCalls          int
	sendOutdatedComponentsCalls    int
	sendComponentUpgradeFailCalls  int
}

// SendMessage mocks SendMessage function
func (r *mockReporter) SendMessage(message string) error {
	return nil
}

// SendComponentUpgradeFail mocks SendComponentUpgradeFail function
func (r *mockReporter) SendComponentUpgradeFail(component *component.Component, serviceOwner string, options ...reporter.Option) error {
	r.sendComponentUpgradeFailCalls++
	return nil
}

// SendActivePromotionStatus mocks SendActivePromotionStatus function
func (r *mockReporter) SendActivePromotionStatus(status, currentActiveNamespace, serviceOwner string, components []component.OutdatedComponent, showedDetails bool) error {
	r.sendActivePromotionStatusCalls++
	return nil
}

// SendOutdatedComponents mocks SendOutdatedComponents function
func (r *mockReporter) SendOutdatedComponents(components []component.OutdatedComponent) error {
	r.sendOutdatedComponentsCalls++
	return nil
}

// SendImageMissingList mocks SendImageMissingList function
func (r *mockReporter) SendImageMissingList(image []component.Image) error {
	r.sendImageMissingCalls++
	return nil
}
