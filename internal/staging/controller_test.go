package staging

import (
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	s2hv1 "github.com/agoda-com/samsahai/api/v1"
	"github.com/agoda-com/samsahai/internal"
	configctrl "github.com/agoda-com/samsahai/internal/config"
	"github.com/agoda-com/samsahai/internal/util"
	conf "github.com/agoda-com/samsahai/internal/util/config"
	"github.com/agoda-com/samsahai/internal/util/dotaccess"
	"github.com/agoda-com/samsahai/internal/util/unittest"
	"github.com/agoda-com/samsahai/internal/util/valuesutil"
)

func TestApplyEnvBasedConfig(t *testing.T) {
	unittest.InitGinkgo(t, "Apply Env Based Config")
}

var _ = Describe("Apply Env Based Config", func() {
	var err error
	var configCtrl internal.ConfigController
	g := NewWithT(GinkgoT())

	BeforeEach(func() {
		configCtrl = newMockConfigCtrl()
		g.Expect(err).NotTo(HaveOccurred())
	})

	It("Should successfully apply configuration based on queue type", func() {
		config, err := configCtrl.Get("mock")
		g.Expect(err).NotTo(HaveOccurred())

		comps, err := configCtrl.GetParentComponents("mock")
		g.Expect(err).NotTo(HaveOccurred())

		{
			values := util.CopyMap(comps["redis"].Values)
			values = applyEnvBaseConfig(&config.Spec, values, s2hv1.QueueTypeUpgrade, comps["redis"])
			v, err := dotaccess.Get(values, "master.service.nodePort")
			g.Expect(err).NotTo(HaveOccurred())
			port, ok := v.(float64)

			g.Expect(ok).To(BeTrue())
			g.Expect(int(port)).To(Equal(31001))
		}

		{
			values := util.CopyMap(comps["redis"].Values)
			values = applyEnvBaseConfig(&config.Spec, values, s2hv1.QueueTypePreActive, comps["redis"])
			v, err := dotaccess.Get(values, "master.service.nodePort")
			g.Expect(err).NotTo(HaveOccurred())
			port, ok := v.(float64)

			g.Expect(ok).To(BeTrue())
			g.Expect(int(port)).To(Equal(31002))
		}

		{
			values := util.CopyMap(comps["redis"].Values)
			values = applyEnvBaseConfig(&config.Spec, values, s2hv1.QueueTypePromoteToActive, comps["redis"])
			v, err := dotaccess.Get(values, "master.service.nodePort")
			g.Expect(err).NotTo(HaveOccurred())
			port, ok := v.(float64)

			g.Expect(ok).To(BeTrue())
			g.Expect(int(port)).To(Equal(31003))
		}

		{
			values := util.CopyMap(comps["redis"].Values)
			values = applyEnvBaseConfig(&config.Spec, values, s2hv1.QueueTypeDemoteFromActive, comps["redis"])
			val, err := dotaccess.Get(values, "master.service.nodePort")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(val).To(BeNil())
		}
	})

	It("Should correctly combine base values and config", func() {
		config, err := configCtrl.Get("mock")
		g.Expect(err).NotTo(HaveOccurred())

		comps, err := configCtrl.GetParentComponents("mock")
		g.Expect(err).NotTo(HaveOccurred())

		wordpress := comps["wordpress"]
		envValues, err := configctrl.GetEnvComponentValues(&config.Spec, "wordpress", s2hv1.EnvBase)
		g.Expect(err).NotTo(HaveOccurred())

		values := valuesutil.GenStableComponentValues(wordpress, nil, envValues)
		val, err := dotaccess.Get(values, "mariadb.enabled")
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(val).To(BeTrue())
		val, err = dotaccess.Get(values, "ingress.enabled")
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(val).To(BeTrue())
	})
})

type mockConfigCtrl struct{}

func newMockConfigCtrl() internal.ConfigController {
	return &mockConfigCtrl{}
}

func (c *mockConfigCtrl) Get(configName string) (*s2hv1.Config, error) {
	engine := "flux-helm"
	deployConfig := s2hv1.ConfigDeploy{
		Timeout: metav1.Duration{Duration: 5 * time.Minute},
		Engine:  &engine,
		TestRunner: &s2hv1.ConfigTestRunner{
			TestMock: &s2hv1.ConfigTestMock{
				Result: true,
			},
		},
	}
	compSource := s2hv1.UpdatingSource("public-registry")
	redisConfigComp := s2hv1.Component{
		Name: "redis",
		Chart: s2hv1.ComponentChart{
			Repository: "https://kubernetes-charts.storage.googleapis.com",
			Name:       "redis",
		},
		Image: s2hv1.ComponentImage{
			Repository: "bitnami/redis",
			Pattern:    "5.*debian-9.*",
		},
		Source: &compSource,
		Values: s2hv1.ComponentValues{
			"image": map[string]interface{}{
				"repository": "bitnami/redis",
				"pullPolicy": "IfNotPresent",
			},
			"cluster": map[string]interface{}{
				"enabled": false,
			},
			"usePassword": false,
			"master": map[string]interface{}{
				"persistence": map[string]interface{}{
					"enabled": false,
				},
			},
		},
	}
	wordpressConfigComp := s2hv1.Component{
		Name: "wordpress",
		Chart: s2hv1.ComponentChart{
			Repository: "https://kubernetes-charts.storage.googleapis.com",
			Name:       "wordpress",
		},
		Image: s2hv1.ComponentImage{
			Repository: "bitnami/wordpress",
			Pattern:    "5\\.2.*debian-9.*",
		},
		Source: &compSource,
		Dependencies: []*s2hv1.Dependency{
			{
				Name: "mariadb",
				Image: s2hv1.ComponentImage{
					Repository: "bitnami/mariadb",
					Pattern:    "10\\.3.*debian-9.*",
				},
			},
		},
	}

	mockConfig := &s2hv1.Config{
		Spec: s2hv1.ConfigSpec{
			Envs: map[s2hv1.EnvType]s2hv1.ChartValuesURLs{
				"staging": map[string][]string{
					"redis": {"https://raw.githubusercontent.com/agoda-com/samsahai/master/test/data/wordpress-redis/envs/staging/redis.yaml"},
				},
				"pre-active": map[string][]string{
					"redis": {"https://raw.githubusercontent.com/agoda-com/samsahai/master/test/data/wordpress-redis/envs/pre-active/redis.yaml"},
				},
				"active": map[string][]string{
					"redis": {"https://raw.githubusercontent.com/agoda-com/samsahai/master/test/data/wordpress-redis/envs/active/redis.yaml"},
				},
				"base": map[string][]string{
					"wordpress": {"https://raw.githubusercontent.com/agoda-com/samsahai/master/test/data/wordpress-redis/envs/base/wordpress.yaml"},
				},
			},
			Staging: &s2hv1.ConfigStaging{
				MaxRetry:   3,
				Deployment: &deployConfig,
			},
			ActivePromotion: &s2hv1.ConfigActivePromotion{
				Timeout:          metav1.Duration{Duration: 10 * time.Minute},
				TearDownDuration: metav1.Duration{Duration: 10 * time.Second},
				Deployment:       &deployConfig,
			},
			Components: []*s2hv1.Component{
				&redisConfigComp,
				&wordpressConfigComp,
			},
		},
	}

	return mockConfig, nil
}

func (c *mockConfigCtrl) GetComponents(configName string) (map[string]*s2hv1.Component, error) {
	config, _ := c.Get(configName)

	comps := map[string]*s2hv1.Component{
		"redis":     config.Spec.Components[0],
		"wordpress": config.Spec.Components[1],
		"mariadb":   conf.New(config.Spec.Components[1].Dependencies[0], nil),
	}

	comps["mariadb"].Parent = "wordpress"

	return comps, nil
}

func (c *mockConfigCtrl) GetParentComponents(configName string) (map[string]*s2hv1.Component, error) {
	config, _ := c.Get(configName)

	comps := map[string]*s2hv1.Component{
		"redis":     config.Spec.Components[0],
		"wordpress": config.Spec.Components[1],
	}

	return comps, nil
}

func (c *mockConfigCtrl) Update(config *s2hv1.Config) error {
	return nil
}

func (c *mockConfigCtrl) Delete(configName string) error {
	return nil
}
