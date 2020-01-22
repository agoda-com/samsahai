package staging

import (
	"path"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/agoda-com/samsahai/api/v1beta1"
	"github.com/agoda-com/samsahai/internal"
	"github.com/agoda-com/samsahai/internal/config"
	"github.com/agoda-com/samsahai/internal/util"
	"github.com/agoda-com/samsahai/internal/util/dotaccess"
	"github.com/agoda-com/samsahai/internal/util/unittest"
	"github.com/agoda-com/samsahai/internal/util/valuesutil"
)

func TestApplyEnvBasedConfig(t *testing.T) {
	unittest.InitGinkgo(t, "Apply Env Based Config")
}

var _ = Describe("Apply Env Based Config", func() {
	var err error
	var configMgr internal.ConfigManager
	g := NewWithT(GinkgoT())

	BeforeEach(func() {
		configMgr, err = config.NewWithGitClient(nil, "teamtest", path.Join("..", "..", "test", "data"))
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(configMgr.Sync()).NotTo(HaveOccurred())
	})

	It("Should successfully apply configuration based on queue type", func() {
		cfg := configMgr.Get()
		comps := configMgr.GetParentComponents()

		{
			values := util.CopyMap(comps["redis"].Values)
			values = applyEnvBaseConfig(cfg, values, v1beta1.QueueTypeUpgrade, comps["redis"])
			v, err := dotaccess.Get(values, "master.service.nodePort")
			g.Expect(err).NotTo(HaveOccurred())
			port, ok := v.(float64)

			g.Expect(ok).To(BeTrue())
			g.Expect(int(port)).To(Equal(31001))
		}

		{
			values := util.CopyMap(comps["redis"].Values)
			values = applyEnvBaseConfig(cfg, values, v1beta1.QueueTypePreActive, comps["redis"])
			v, err := dotaccess.Get(values, "master.service.nodePort")
			g.Expect(err).NotTo(HaveOccurred())
			port, ok := v.(float64)

			g.Expect(ok).To(BeTrue())
			g.Expect(int(port)).To(Equal(31002))
		}

		{
			values := util.CopyMap(comps["redis"].Values)
			values = applyEnvBaseConfig(cfg, values, v1beta1.QueueTypePromoteToActive, comps["redis"])
			v, err := dotaccess.Get(values, "master.service.nodePort")
			g.Expect(err).NotTo(HaveOccurred())
			port, ok := v.(float64)

			g.Expect(ok).To(BeTrue())
			g.Expect(int(port)).To(Equal(31003))
		}

		{
			values := util.CopyMap(comps["redis"].Values)
			values = applyEnvBaseConfig(cfg, values, v1beta1.QueueTypeDemoteFromActive, comps["redis"])
			val, err := dotaccess.Get(values, "master.service.nodePort")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(val).To(BeNil())
		}
	})

	It("Should correctly combine base values and config", func() {
		cfg := configMgr.Get()
		comps := configMgr.GetParentComponents()
		wordpress := comps["wordpress"]
		//comps := configMgr.GetParentComponents()
		values := valuesutil.GenStableComponentValues(wordpress, nil, cfg.Envs["base"]["wordpress"])
		val, err := dotaccess.Get(values, "mariadb.enabled")
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(val).To(BeTrue())
		val, err = dotaccess.Get(values, "ingress.enabled")
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(val).To(BeTrue())
	})
})
