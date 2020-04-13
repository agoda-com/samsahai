package config_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	s2hv1beta1 "github.com/agoda-com/samsahai/api/v1beta1"
	configctrl "github.com/agoda-com/samsahai/internal/config"
	"github.com/agoda-com/samsahai/internal/util/unittest"
)

func TestConfig(t *testing.T) {
	unittest.InitGinkgo(t, "Config Controller")
}

var _ = Describe("Config Controller", func() {
	compSource := s2hv1beta1.UpdatingSource("public-registry")
	redisCompName := "redis"
	redisConfigComp := s2hv1beta1.Component{
		Name: redisCompName,
		Chart: s2hv1beta1.ComponentChart{
			Repository: "https://kubernetes-charts.storage.googleapis.com",
			Name:       redisCompName,
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

	wordpressCompName := "wordpress"
	wordpressComp := s2hv1beta1.Component{
		Name: wordpressCompName,
		Chart: s2hv1beta1.ComponentChart{
			Repository: "https://kubernetes-charts.storage.googleapis.com",
			Name:       wordpressCompName,
		},
		Image: s2hv1beta1.ComponentImage{
			Repository: "bitnami/redis",
			Pattern:    "5\\.2.*debian-9.*",
		},
		Source: &compSource,
		Values: s2hv1beta1.ComponentValues{
			"image": map[string]interface{}{
				"repository": "bitnami/wordpress",
			},
			"service": map[string]interface{}{
				"type": "NodePort",
			},
		},
	}

	mockConfig := s2hv1beta1.ConfigSpec{
		Envs: map[s2hv1beta1.EnvType]s2hv1beta1.ChartValuesURLs{
			"staging": map[string][]string{
				redisCompName: {"https://raw.githubusercontent.com/agoda-com/samsahai/master/test/data/wordpress-redis/envs/staging/redis.yaml"},
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
			redisCompName: {
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
		compValues, err := configctrl.GetEnvComponentValues(&config, redisCompName, s2hv1beta1.EnvStaging)
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

	It("Should detect new component correctly when there is a new component", func() {
		g := NewWithT(GinkgoT())
		mockDesiredCompList := &s2hv1beta1.DesiredComponentList{
			Items: []s2hv1beta1.DesiredComponent{
				{
					ObjectMeta: metav1.ObjectMeta{Name: redisCompName},
					Spec:       s2hv1beta1.DesiredComponentSpec{Name: redisCompName, Repository: "bitnami/redis", Version: "5.0.7-debian-9-r56"},
				},
			},
		}

		isNewComponent := configctrl.IsNewComponent(mockDesiredCompList, &wordpressComp)
		g.Expect(isNewComponent).To(BeTrue())
	})

	It("Should detect new component correctly when repository of component is changed", func() {
		g := NewWithT(GinkgoT())
		mockDesiredCompList := &s2hv1beta1.DesiredComponentList{
			Items: []s2hv1beta1.DesiredComponent{
				{
					ObjectMeta: metav1.ObjectMeta{Name: redisCompName},
					Spec:       s2hv1beta1.DesiredComponentSpec{Name: redisCompName, Repository: "bitnami/redis2", Version: "5.0.7-debian-9-r56"},
				},
			},
		}

		isNewComponent := configctrl.IsNewComponent(mockDesiredCompList, &redisConfigComp)
		g.Expect(isNewComponent).To(BeTrue())
	})

	It("Should not detect new component correctly", func() {
		g := NewWithT(GinkgoT())
		mockDesiredCompList := &s2hv1beta1.DesiredComponentList{
			Items: []s2hv1beta1.DesiredComponent{
				{
					ObjectMeta: metav1.ObjectMeta{Name: redisCompName},
					Spec:       s2hv1beta1.DesiredComponentSpec{Name: redisCompName, Repository: "bitnami/redis", Version: "5.0.7-debian-9-r56"},
				},
			},
		}

		isNewComponent := configctrl.IsNewComponent(mockDesiredCompList, &redisConfigComp)
		g.Expect(isNewComponent).To(BeFalse())
	})
})
