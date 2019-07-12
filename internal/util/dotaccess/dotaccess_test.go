package dotaccess_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/agoda-com/samsahai/internal/util/dotaccess"
	"github.com/agoda-com/samsahai/internal/util/unittest"
)

func TestUnit(t *testing.T) {
	unittest.InitGinkgo(t, "Dot Access")
}

var _ = Describe("Dot Access", func() {
	g := NewWithT(GinkgoT())

	It("Should successfully access map[string]interface{} by dot", func() {
		o := map[string]interface{}{
			"t": map[string]interface{}{
				"1": 1,
				"a": "a",
			},
		}

		g.Expect(dotaccess.Get(o, "t.1")).To(Equal(1))
		g.Expect(dotaccess.Get(o, "t.a")).To(Equal("a"))
		g.Expect(dotaccess.Get(o, "t")).To(Equal(map[string]interface{}{
			"1": 1,
			"a": "a",
		}))
	})
})
