package config_test

import (
	"os"
	"path"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	s2hv1beta1 "github.com/agoda-com/samsahai/api/v1beta1"
	"github.com/agoda-com/samsahai/internal"
	"github.com/agoda-com/samsahai/internal/config"
	"github.com/agoda-com/samsahai/internal/config/git"
	s2hlog "github.com/agoda-com/samsahai/internal/log"
	"github.com/agoda-com/samsahai/internal/util/unittest"
)

func TestUnit(t *testing.T) {
	unittest.InitGinkgo(t, "Config Manager")
}

var _ = BeforeSuite(func() {
	if os.Getenv("DEBUG") != "" {
		s2hlog.SetLogger(zap.New(func(o *zap.Options) {
			o.Development = true
		}))
	}
})

var _ = Describe("Config Manager", func() {
	g := NewWithT(GinkgoT())

	var (
		configMgr internal.ConfigManager
		err       error
	)

	BeforeEach(func() {
		configMgr, err = config.NewWithGitClient(nil, "teamtest", path.Join("..", "..", "test", "data", "wordpress-redis"))
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(configMgr.Sync()).NotTo(HaveOccurred())
		g.Expect(configMgr).NotTo(BeNil())

		cfg := configMgr.Get()
		g.Expect(cfg).NotTo(BeNil())
		g.Expect(len(cfg.Components)).To(Equal(2))
		g.Expect(len(configMgr.GetComponents())).To(Equal(3), "expect 3 components from mock config")
	})

	It("should check changes correctly", func() {
		configMgr, err = config.NewWithGitClient(&git.Client{}, "teamtest", path.Join("..", "..", "test", "data", "wordpress-redis"))
		hasChanges := configMgr.HasGitChanges(s2hv1beta1.GitStorage{URL: "https://samsahai.org"})
		Expect(hasChanges).To(BeTrue())

		pwd, _ := os.Getwd()
		hasChanges = configMgr.HasGitChanges(s2hv1beta1.GitStorage{Path: path.Join(pwd, "..", "..", "test", "data", "wordpress-redis")})
		Expect(hasChanges).To(BeFalse())
	})
})
