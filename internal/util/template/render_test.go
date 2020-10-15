package template_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/agoda-com/samsahai/internal/util/template"
	"github.com/agoda-com/samsahai/internal/util/unittest"
)

func TestUnit(t *testing.T) {
	unittest.InitGinkgo(t, "Template utils")
}

var _ = Describe("render template", func() {
	g := NewGomegaWithT(GinkgoT())

	data := struct {
		Name string
	}{
		Name: "foo",
	}

	It("should correctly render", func() {
		message := "name: {{ .Name }}"
		out := template.TextRender("SuccessRender", message, data)
		g.Expect(out).To(Equal("name: foo"))
	})

	It("should ignore value if it is missing", func() {
		message := `
name: {{ .Name }}
url: {{ .Data.URL }}
`
		out := template.TextRender("IgnoreMissingValues", message, data)
		g.Expect(out).To(Equal(`
name: foo
url: {{.Data.URL}}
`))
	})

	It("should return same text if error", func() {
		message := `
name: {{ .Name }}
url: {{{ .Data.URL }}}
`
		out := template.TextRender("TemplateError", message, data)
		g.Expect(out).To(Equal(`
name: {{ .Name }}
url: {{{ .Data.URL }}}
`))
	})
})
