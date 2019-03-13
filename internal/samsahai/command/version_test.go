package command

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestGetAppName(t *testing.T) {
	g := NewGomegaWithT(t)
	g.Expect(getAppName()).ShouldNot(Equal(""), "app name shouldn't empty")
}

func TestGetAppVersion(t *testing.T) {
	g := NewGomegaWithT(t)
	g.Expect(getVersion()).ShouldNot(Equal("app version shouldn't empty"))
}
