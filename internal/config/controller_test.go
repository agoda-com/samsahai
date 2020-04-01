package config_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	s2hv1beta1 "github.com/agoda-com/samsahai/api/v1beta1"
	configctrl "github.com/agoda-com/samsahai/internal/config"
	"github.com/agoda-com/samsahai/internal/util/unittest"
)

func TestConfig(t *testing.T) {
	unittest.InitGinkgo(t, "Config Controller")
}

var _ = Describe("Config Controller", func() {
	compSource := s2hv1beta1.UpdatingSource("public-registry")
	redisConfigComp := s2hv1beta1.Component{
		Name: "redis",
		Chart: s2hv1beta1.ComponentChart{
			Repository: "https://kubernetes-charts.storage.googleapis.com",
			Name:       "redis",
		},
		Image: s2hv1beta1.ComponentImage{
			Repository: "bitnami/redis",
			Pattern:    "5.*debian-9.*",
		},
		Source: &compSource,
		Values: s2hv1beta1.ComponentValues{
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

	mockConfig := s2hv1beta1.ConfigSpec{
		Envs: map[s2hv1beta1.EnvType]s2hv1beta1.ChartValuesURLs{
			"staging": map[string][]string{
				"redis": {"https://raw.githubusercontent.com/agoda-com/samsahai/master/test/data/wordpress-redis/envs/staging/redis.yaml"},
			},
		},
		Components: []*s2hv1beta1.Component{
			&redisConfigComp,
		},
	}

	It("Should get env values by the env type correctly", func() {
		g := NewWithT(GinkgoT())

		config := mockConfig
		compValues, err := configctrl.GetEnvValues(&config, s2hv1beta1.EnvStaging)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(compValues).To(Equal(map[string]s2hv1beta1.ComponentValues{
			"redis": {
				"master": map[string]interface{}{
					"service": map[string]interface{}{
						"nodePort": float64(31001),
						"type":     "NodePort",
					},
				},
			},
		}))
	})

	It("Should get env values by the env type and component name correctly", func() {
		g := NewWithT(GinkgoT())

		config := mockConfig
		compValues, err := configctrl.GetEnvComponentValues(&config, "redis", s2hv1beta1.EnvStaging)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(compValues).To(Equal(s2hv1beta1.ComponentValues{
			"master": map[string]interface{}{
				"service": map[string]interface{}{
					"nodePort": float64(31001),
					"type":     "NodePort",
				},
			},
		}))
	})
})
