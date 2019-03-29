package send

import (
	"github.com/agoda-com/samsahai/internal/samsahai/component"
	"github.com/agoda-com/samsahai/internal/samsahai/reporter"
)

type mockReporter struct {
	sendActivePromotionStatusCalls int
}

// Ensure mockReporter implements Reporter
var _ reporter.Reporter = &mockReporter{}

func (r *mockReporter) SendMessage(message string) error {
	return nil
}

func (r *mockReporter) SendFailedComponentUpgrade() error {
	return nil
}

func (r *mockReporter) SendActivePromotionStatus(status, currentActiveNamespace, serviceOwner string, components []component.Component, showedDetails bool) error {
	r.sendActivePromotionStatusCalls++
	return nil
}

func (r *mockReporter) SendOutdatedComponents() error {
	return nil
}

func (r *mockReporter) SendImageMissingList(image []component.Image) error {
	return nil
}
