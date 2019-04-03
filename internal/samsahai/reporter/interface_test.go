package reporter

import (
	"testing"

	"github.com/agoda-com/samsahai/internal/samsahai/component"

	. "github.com/onsi/gomega"
)

func TestNewComponentUpgradeFail(t *testing.T) {
	g := NewGomegaWithT(t)
	cuf := NewComponentUpgradeFail(&component.Component{}, "owner", "issue1", "values-url", "ci-url", "logs-url", "error-url")
	g.Expect(cuf).Should(Equal(&ComponentUpgradeFail{
		Component:     &component.Component{},
		ServiceOwner:  "owner",
		IssueType:     "issue1",
		ValuesFileURL: "values-url",
		CIURL:         "ci-url",
		LogsURL:       "logs-url",
		ErrorURL:      "error-url",
	}))
}

func TestNewActivePromotion(t *testing.T) {
	g := NewGomegaWithT(t)
	atvPromotion := NewActivePromotion("success", "owner", "namespace-1", []component.OutdatedComponent{})
	g.Expect(atvPromotion).Should(Equal(&ActivePromotion{Status: "success", ServiceOwner: "owner", CurrentActiveNamespace: "namespace-1", Components: []component.OutdatedComponent{}}))
}

func TestNewOutdatedComponents(t *testing.T) {
	g := NewGomegaWithT(t)
	oc := NewOutdatedComponents([]component.OutdatedComponent{}, true)
	g.Expect(oc).Should(Equal(&OutdatedComponents{Components: []component.OutdatedComponent{}, ShowedDetails: true}))
}

func TestNewImageMissing(t *testing.T) {
	g := NewGomegaWithT(t)
	im := NewImageMissing([]component.Image{})
	g.Expect(im).Should(Equal(&ImageMissing{Images: []component.Image{}}))
}
