package e2e

import (
	"os"
	"testing"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/reporters"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/log"

	s2hv1 "github.com/agoda-com/samsahai/api/v1"
	s2hlog "github.com/agoda-com/samsahai/internal/log"

	_ "github.com/agoda-com/samsahai/test/e2e/checkers"
	_ "github.com/agoda-com/samsahai/test/e2e/config"
	_ "github.com/agoda-com/samsahai/test/e2e/queue"
	_ "github.com/agoda-com/samsahai/test/e2e/samsahai"
	_ "github.com/agoda-com/samsahai/test/e2e/staging"
)

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)

	switch {
	case os.Getenv("TEAMCITY_VERSION") != "":
		fallthrough
	case os.Getenv("CI") != "":
		specReporters := []Reporter{reporters.NewJUnitReporter("e2e.unit-test.xml")}
		RunSpecsWithCustomReporters(t, "E2E", specReporters)
	default:
		RunSpecs(t, "E2E")
	}
}

var _ = BeforeSuite(func(done Done) {
	defer close(done)
	var err error

	if os.Getenv("DEBUG") == "1" {
		l := s2hlog.GetLogger(true)
		log.SetLogger(l)
		s2hlog.SetLogger(l)
	}

	err = corev1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred(), "should register scheme `corev1` successfully")

	err = s2hv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred(), "should register scheme `samsahai` successfully")

	Expect(os.Getenv("POD_NAMESPACE")).NotTo(BeEmpty(), "POD_NAMESPACE should be provided")

	_, err = config.GetConfig()
	Expect(err).To(BeNil(), "Please provide credential for accessing k8s cluster")

}, 5)
