package unittest

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/reporters"
	"github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/runtime/log"

	s2hlog "github.com/agoda-com/samsahai/internal/log"
)

var re = regexp.MustCompile("[^a-z0-9]+")

func slug(s string) string {
	return strings.Trim(re.ReplaceAllString(strings.ToLower(s), "-"), "-")
}

func InitGinkgo(t *testing.T, desc string) {
	gomega.RegisterFailHandler(ginkgo.Fail)

	if os.Getenv("DEBUG") != "" {
		s2hlog.SetLogger(log.ZapLogger(true))
	}

	slugName := slug(desc)

	switch {
	case os.Getenv("TEAMCITY_VERSION") != "":
		//specReporters := []ginkgo.Reporter{reporters.NewTeamCityReporter(os.Stdout)}
		//ginkgo.RunSpecsWithCustomReporters(t, desc, specReporters)
		fallthrough
	case os.Getenv("CIRCLECI") != "":
		specReporters := []ginkgo.Reporter{reporters.NewJUnitReporter(slugName + ".unit-test.xml")}
		ginkgo.RunSpecsWithCustomReporters(t, desc, specReporters)
	default:
		ginkgo.RunSpecs(t, desc)
	}
}
