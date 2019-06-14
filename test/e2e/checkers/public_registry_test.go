package checkers_test

import (
	"os"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/runtime/log"

	"github.com/agoda-com/samsahai/internal/desiredcomponent/checker/publicregistry"
)

func TestPublicRegistryChecker(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "public registry checker")
}

var _ = Describe("checker", func() {
	if os.Getenv("DEBUG") != "" {
		log.SetLogger(log.ZapLogger(true))
	}

	var checker = publicregistry.New()

	It("should returns non-empty name", func() {
		Expect(checker.GetName()).NotTo(BeEmpty())
	})

	Specify("unsupported gcr registry", func() {
		_, err := checker.GetVersion("gcr.io/google-samples/echo-go", "echo-go", ".*")
		Expect(err).NotTo(BeNil())
	})

	Specify("invalid registry", func() {
		_, err := checker.GetVersion("https://docker.io/phantomnat/curl", "echo-go", ".*")
		Expect(err).NotTo(BeNil())
	})

	Specify("invalid pattern", func() {
		_, err := checker.GetVersion("docker.io/phantomnat/curl", "echo-go", "[^0.+")
		Expect(err).NotTo(BeNil())
	})

	Describe("docker.io [slow] [e2e]", func() {
		It("should successfully get tag", func() {
			var tag string
			var err error

			tag, err = checker.GetVersion("alpine:3.9", "alpine", "^3\\.9\\.\\d+")
			Expect(err).To(BeNil())
			Expect(tag).NotTo(BeEmpty())

			tag, err = checker.GetVersion("docker.io/phantomnat/curl", "curl", "^0.+")
			Expect(err).To(BeNil())
			Expect(tag).NotTo(BeEmpty())
		})

		It("should not found the tag", func() {
			tag, err := checker.GetVersion("phantomnat/curl", "curl", "^3\\.9\\.\\d+")
			Expect(tag).To(BeEmpty())
			Expect(err).NotTo(BeNil())
		})
	})

	Describe("quay.io [slow] [e2e]", func() {
		It("should successfully get tag", func() {
			var tag string
			var err error

			tag, err = checker.GetVersion("quay.io/phantomnat/curl", "curl", "^0.+")
			Expect(err).To(BeNil())
			Expect(tag).NotTo(BeEmpty())

			tag, err = checker.GetVersion("quay.io/mhart/alpine-node", "alpine-node", "^11.14.0")
			Expect(err).To(BeNil())
			Expect(tag).NotTo(BeEmpty())
		})

		It("should not found the tag", func() {
			tag, err := checker.GetVersion("quay.io/phantomnat/curl", "curl", "^3\\.9\\.\\d+")
			Expect(tag).To(BeEmpty())
			Expect(err).NotTo(BeNil())
		})
	})
})
