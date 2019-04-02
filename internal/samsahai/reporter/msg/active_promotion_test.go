package msg

import (
	"testing"

	"github.com/agoda-com/samsahai/internal/samsahai/component"
	. "github.com/onsi/gomega"
)

func TestNewActivePromotion(t *testing.T) {
	g := NewGomegaWithT(t)
	atvPromotion := NewActivePromotion("success", "owner", "namespace-1", []component.OutdatedComponent{})
	g.Expect(atvPromotion).Should(Equal(&ActivePromotion{Status: "success", ServiceOwner: "owner", CurrentActiveNamespace: "namespace-1", Components: []component.OutdatedComponent{}}))
}

func TestNewActivePromotionMessage(t *testing.T) {
	g := NewGomegaWithT(t)
	ap := &ActivePromotion{
		Status:                 "success",
		ServiceOwner:           "owner",
		CurrentActiveNamespace: "namespace-1",
		Components: []component.OutdatedComponent{
			{
				CurrentComponent: &component.Component{Name: "comp1", Version: "1.1.0"},
				NewComponent:     &component.Component{Name: "comp1", Version: "1.1.2"},
				OutdatedDays:     1,
			},
		},
	}

	message := ap.NewActivePromotionMessage(true)
	g.Expect(message).Should(Equal("*Active Promotion:* SUCCESS \n*Owner:* owner \n*Current Active Namespace:* namespace-1\n*Outdated Components* \n*comp1* \n>Not update for 1 day(s) \n>Current version: 1.1.0 \n>New Version: 1.1.2\n"))
}

func TestGetStatusText(t *testing.T) {
	g := NewGomegaWithT(t)
	tests := map[string]struct {
		in  string
		out string
	}{
		"should get success status": {
			in:  "Success",
			out: "SUCCESS",
		},
		"should get fail status": {
			in:  "Failed",
			out: "FAIL",
		},
	}

	for desc, test := range tests {
		statusText := getStatusText(test.in)
		g.Expect(statusText).Should(Equal(test.out), desc)
	}
}
