package samsahai

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/agoda-com/samsahai/internal"
	"github.com/agoda-com/samsahai/internal/util/unittest"
)

func TestController(t *testing.T) {
	unittest.InitGinkgo(t, "Samsamhai")
}

var _ = Describe("S2H Controller", func() {
	g := NewWithT(GinkgoT())

	Describe("plugins", func() {
		ctrl := controller{
			checkers: map[string]internal.DesiredComponentChecker{},
			plugins:  map[string]internal.Plugin{},
		}
		pluginName := "example"

		It("should successfully load plugins", func() {
			ctrl.loadPlugins("plugin")

			g.Expect(len(ctrl.plugins)).To(Equal(1))
			name := ctrl.plugins[pluginName].GetName()
			g.Expect(name).To(Equal(pluginName))

			g.Expect(len(ctrl.checkers)).To(Equal(1))
			name = ctrl.checkers[pluginName].GetName()
			g.Expect(name).To(Equal(pluginName))
		})

		Specify("Non-existing plugin", func(done Done) {
			close(done)
			ctrl.loadPlugins("non-existing")
		}, 1)
	})
})
