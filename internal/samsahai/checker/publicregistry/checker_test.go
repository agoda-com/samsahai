package publicregistry

import (
	"os"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	s2herrors "github.com/agoda-com/samsahai/internal/errors"
	s2hlog "github.com/agoda-com/samsahai/internal/log"
	"github.com/agoda-com/samsahai/internal/util/unittest"
)

func TestChecker(t *testing.T) {
	unittest.InitGinkgo(t, "Public Registry Checker")
}

var _ = Describe("Public Registry Checker", func() {
	if os.Getenv("DEBUG") != "" {
		s2hlog.SetLogger(zap.New(func(o *zap.Options) {
			o.Development = true
		}))
	}

	var checker = New()

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
			Expect(tag).To(Equal("^3\\.9\\.\\d+"))
			Expect(err).NotTo(BeNil())
		})

		It("should correctly ensure version", func(done Done) {
			defer close(done)

			err := checker.EnsureVersion("phantomnat/curl", "curl", "3.9.1")
			Expect(err).NotTo(BeNil())

			err = checker.EnsureVersion("phantomnat/curl", "curl", "3.9.1-missing")
			Expect(s2herrors.IsImageNotFound(err)).To(BeTrue())
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
			Expect(tag).NotTo(Equal("^11.14.0"))
		})

		It("should not found the tag", func() {
			tag, err := checker.GetVersion("quay.io/phantomnat/curl", "curl", "^3\\.9\\.\\d+")
			Expect(tag).To(Equal("^3\\.9\\.\\d+"))
			Expect(err).NotTo(BeNil())
		})
	})
})
