package template_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/agoda-com/samsahai/internal/util/template"
	"github.com/agoda-com/samsahai/internal/util/unittest"
)

func TestUnit(t *testing.T) {
	unittest.InitGinkgo(t, "Reporter utils")
}

var _ = Describe("render template", func() {
	g := NewGomegaWithT(GinkgoT())

	It("should fail to render", func() {
		message := "Fail to render {{ .Value | MissingFunc }}"
		out := template.TextRender("FailToRender", message, nil)
		g.Expect(out).To(BeEmpty())
	})
})
