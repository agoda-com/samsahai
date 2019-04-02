package send

import (
	"testing"

	"github.com/agoda-com/samsahai/internal/samsahai/component"

	. "github.com/onsi/gomega"
)

func TestSendActivePromotionStatus(t *testing.T) {
	g := NewGomegaWithT(t)
	mockReporter := mockReporter{}
	err := sendActivePromotionStatus(&mockReporter, []component.OutdatedComponent{})
	g.Expect(mockReporter.sendActivePromotionStatusCalls).Should(Equal(1), "should call send active promotion status func")
	g.Expect(err).Should(BeNil())
}
