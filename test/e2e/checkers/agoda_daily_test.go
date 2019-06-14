package checkers_test

import (
	"os"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/runtime/log"

	"github.com/agoda-com/samsahai/internal"
	"github.com/agoda-com/samsahai/internal/desiredcomponent/checker/agodadaily"
)

func TestAgodaDailyChecker(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "agoda daily checker")
}

var _ = XDescribe("checker", func() {
	if os.Getenv("DEBUG") != "" {
		log.SetLogger(log.ZapLogger(true))
	}
	var harborURL string
	var checker internal.DesiredComponentChecker

	BeforeEach(func() {
		checker = agodadaily.New()
		harborURL = os.Getenv("HARBOR_URL")
		Expect(harborURL).NotTo(BeEmpty(), "please specify HARBOR_URL env")
	})

	It("should returns non-empty name", func() {
		Expect(checker.GetName()).NotTo(BeEmpty())
	})

	Describe("internal harbor [slow] [e2e]", func() {
		It("should successfully get tag", func() {
			var tag string
			var err error

			tag, err = checker.GetVersion(harborURL, "test", "")
			Expect(err).To(BeNil())
			Expect(tag).NotTo(BeEmpty())
		})
	})
})
