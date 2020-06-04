package util

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/agoda-com/samsahai/internal/util/unittest"
)

func TestCopyMap(t *testing.T) {
	unittest.InitGinkgo(t, "Copy Map")
}

var _ = Describe("Copy Map", func() {
	g := NewWithT(GinkgoT())

	It("should successfully copy map of string of interface{}", func() {
		m1 := map[string]interface{}{
			"a": "bbb",
			"b": map[string]interface{}{
				"c": 123,
			},
		}
		m2 := CopyMap(m1)

		m1["a"] = "zzz"
		delete(m1, "b")

		g.Expect(map[string]interface{}{"a": "zzz"}).To(Equal(m1))
		g.Expect(map[string]interface{}{
			"a": "bbb",
			"b": map[string]interface{}{
				"c": 123,
			},
		}).To(Equal(m2))
	})
})
