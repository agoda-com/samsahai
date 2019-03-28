package send

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestGetGetSlackChannels(t *testing.T) {
	g := NewGomegaWithT(t)
	values := getMultipleValues("#channel-1,#channel-2")
	g.Expect(values).Should(Equal([]string{"#channel-1", "#channel-2"}))
}
