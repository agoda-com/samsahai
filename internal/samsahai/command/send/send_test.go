package send

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestGetMultipleValues(t *testing.T) {
	g := NewGomegaWithT(t)
	values := getMultipleValues("values-1,values-2")
	g.Expect(values).Should(Equal([]string{"values-1", "values-2"}))
}
