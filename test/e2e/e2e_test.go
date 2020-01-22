package e2e

import (
	"os"
	"testing"

	fluxv1beta1 "github.com/fluxcd/flux/integrations/apis/flux.weave.works/v1beta1"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/reporters"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	s2hv1beta1 "github.com/agoda-com/samsahai/api/v1beta1"
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
		//specReporters := []Reporter{reporters.NewTeamCityReporter(os.Stdout)}
		//RunSpecsWithCustomReporters(t, "E2E", specReporters)
		fallthrough
	case os.Getenv("CIRCLECI") != "":
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
		log.SetLogger(zap.New(func(o *zap.Options) {
			o.Development = true
		}))
		s2hlog.SetLogger(zap.New(func(o *zap.Options) {
			o.Development = true
		}))
	}

	err = corev1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred(), "should register scheme `corev1` successfully")

	err = s2hv1beta1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred(), "should register scheme `samsahai` successfully")

	err = fluxv1beta1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred(), "should register scheme `flux` successfully")

	Expect(os.Getenv("POD_NAMESPACE")).NotTo(BeEmpty(), "POD_NAMESPACE should be provided")

	_, err = config.GetConfig()
	Expect(err).To(BeNil(), "Please provide credential for accessing k8s cluster")

}, 5)
