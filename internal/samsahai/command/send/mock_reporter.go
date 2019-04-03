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

// MakeComponentUpgradeFailReport mocks MakeComponentUpgradeFailReport function
func (r *mockReporter) MakeComponentUpgradeFailReport(cuf *reporter.ComponentUpgradeFail, options ...reporter.Option) string {
	return ""
}

// MakeActivePromotionStatusReport mocks MakeActivePromotionStatusReport function
func (r *mockReporter) MakeActivePromotionStatusReport(atv *reporter.ActivePromotion, options ...reporter.Option) string {
	return ""
}

// MakeOutdatedComponentsReport mocks MakeOutdatedComponentsReport function
func (r *mockReporter) MakeOutdatedComponentsReport(oc *reporter.OutdatedComponents, options ...reporter.Option) string {
	return ""
}

// MakeImageMissingListReport mocks MakeImageMissingListReport function
func (r *mockReporter) MakeImageMissingListReport(im *reporter.ImageMissing, options ...reporter.Option) string {
	return ""
}

// SendMessage mocks SendMessage function
func (r *mockReporter) SendMessage(message string, options ...reporter.Option) error {
	return nil
}

// SendComponentUpgradeFail mocks SendComponentUpgradeFail function
func (r *mockReporter) SendComponentUpgradeFail(component *component.Component, serviceOwner string, options ...reporter.Option) error {
	r.sendComponentUpgradeFailCalls++
	return nil
}

// SendActivePromotionStatus mocks SendActivePromotionStatus function
func (r *mockReporter) SendActivePromotionStatus(status, currentActiveNamespace, serviceOwner string, components []component.OutdatedComponent, options ...reporter.Option) error {
	r.sendActivePromotionStatusCalls++
	return nil
}

// SendOutdatedComponents mocks SendOutdatedComponents function
func (r *mockReporter) SendOutdatedComponents(components []component.OutdatedComponent, options ...reporter.Option) error {
	r.sendOutdatedComponentsCalls++
	return nil
}

// SendImageMissingList mocks SendImageMissingList function
func (r *mockReporter) SendImageMissingList(image []component.Image, options ...reporter.Option) error {
	r.sendImageMissingCalls++
	return nil
}
