package send

import (
	"testing"

	"github.com/agoda-com/samsahai/internal/samsahai/component"

	. "github.com/onsi/gomega"
)

func TestSendOutdatedComponents(t *testing.T) {
	g := NewGomegaWithT(t)
	mockReporter := mockReporter{}
	err := sendOutdatedComponents(&mockReporter, []component.OutdatedComponent{})
	g.Expect(mockReporter.sendOutdatedComponentsCalls).Should(Equal(1), "should call send outdated components func")
	g.Expect(err).Should(BeNil())
}
