package samsahai

import (
	"context"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	s2hv1beta1 "github.com/agoda-com/samsahai/api/v1beta1"
	s2h "github.com/agoda-com/samsahai/internal"
	"github.com/agoda-com/samsahai/internal/samsahai"
	"github.com/agoda-com/samsahai/internal/util"
	"github.com/agoda-com/samsahai/internal/util/unittest"
)

func TestChecker(t *testing.T) {
	unittest.InitGinkgo(t, "Samsahai Stable Checker")
}

var crClient client.Client

func TestMain(m *testing.M) {
	var err error
	var cfg *rest.Config

	t := &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "..", "..", "..", "config", "crds")},
	}

	err = s2hv1beta1.SchemeBuilder.AddToScheme(scheme.Scheme)
	if err != nil {
		log.Fatal(err)
	}

	if cfg, err = t.Start(); err != nil {
		logger.Error(err, "start testenv error")
		os.Exit(1)
	}

	if crClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme}); err != nil {
		log.Fatal(err)
	}

	code := m.Run()
	_ = t.Stop()
	os.Exit(code)
}

var _ = Describe("", func() {
	var s2hCtrl s2h.SamsahaiController
	var check s2h.DesiredComponentChecker
	timeoutSec := float64(10)
	teamName := "example"
	namespace := "default"
	g := NewWithT(GinkgoT())

	BeforeEach(func(done Done) {
		defer close(done)
		configCtrl := newMockConfigCtrl()
		s2hConfig := s2h.SamsahaiConfig{SamsahaiCredential: s2h.SamsahaiCredential{InternalAuthToken: "123456"}}
		s2hCtrl = samsahai.New(nil, namespace, s2hConfig,
			samsahai.WithClient(crClient),
			samsahai.WithDisableLoaders(true, true, true),
			samsahai.WithScheme(scheme.Scheme),
			samsahai.WithConfigCtrl(configCtrl))
		check = New(s2hCtrl)

		yamlTeam, err := ioutil.ReadFile(path.Join("..", "..", "..", "..", "test", "data", "team", "team.yaml"))
		g.Expect(err).NotTo(HaveOccurred())
		obj, _ := util.MustParseYAMLtoRuntimeObject(yamlTeam)

		team, _ := obj.(*s2hv1beta1.Team)
		team.Status.StableComponents = map[string]s2hv1beta1.StableComponent{
			"redis": {
				ObjectMeta: metav1.ObjectMeta{
					Name:      "redis",
					Namespace: namespace,
				},
				Spec: s2hv1beta1.StableComponentSpec{
					Name:       "redis",
					Repository: "bitnami/redis",
					Version:    "10.0.0-r0",
				},
			},
			"mariadb": {
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mariadb",
					Namespace: namespace,
				},
				Spec: s2hv1beta1.StableComponentSpec{
					Name:       "mariadb",
					Repository: "bitnami/mariadb",
					Version:    "12.0.0-r0",
				},
			},
		}
		_ = crClient.Create(context.TODO(), team)
	}, timeoutSec)

	AfterEach(func(done Done) {
		defer close(done)
		team := &s2hv1beta1.Team{}
		ctx := context.TODO()
		_ = crClient.Get(ctx, client.ObjectKey{Name: teamName, Namespace: namespace}, team)
		_ = crClient.Delete(ctx, team)
	}, timeoutSec)

	It("Should return checker name", func() {
		Expect(check.GetName()).To(Equal(CheckerName))
	})

	It("Should successfully get version", func() {
		version, err := check.GetVersion("", "mariadb", "")
		Expect(err).NotTo(HaveOccurred())
		Expect(version).To(Equal("12.0.0-r0"))

		err = check.EnsureVersion("", "mariadb", "")
		Expect(err).NotTo(HaveOccurred())

		version, err = check.GetVersion("bitnami/redis", "redis", "ex.*")
		Expect(err).NotTo(HaveOccurred())
		Expect(version).To(Equal("10.0.0-r0"))
	})

	Describe("Bad path", func() {
		It("Should error when repository not matched", func() {
			_, err := check.GetVersion("mariadb", "mariadb", "")
			Expect(err).To(HaveOccurred())
		})

		It("Should error when team not matched", func() {
			_, err := check.GetVersion("", "mariadb", "should-not-exist")
			Expect(err).To(HaveOccurred())
		})

		It("Should error when pattern is invalid", func() {
			_, err := check.GetVersion("", "mariadb", "((")
			Expect(err).To(HaveOccurred())
		})
	})
})

type mockConfigCtrl struct{}

func newMockConfigCtrl() s2h.ConfigController {
	return &mockConfigCtrl{}
}

func (c *mockConfigCtrl) Get(configName string) (*s2hv1beta1.Config, error) {
	engine := "flux-helm"
	deployConfig := s2hv1beta1.ConfigDeploy{
		Timeout: metav1.Duration{Duration: 5 * time.Minute},
		Engine:  &engine,
		TestRunner: &s2hv1beta1.ConfigTestRunner{
			TestMock: &s2hv1beta1.ConfigTestMock{
				Result: true,
			},
		},
	}
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
	wordpressConfigComp := s2hv1beta1.Component{
		Name: "wordpress",
		Chart: s2hv1beta1.ComponentChart{
			Repository: "https://kubernetes-charts.storage.googleapis.com",
			Name:       "wordpress",
		},
		Image: s2hv1beta1.ComponentImage{
			Repository: "bitnami/wordpress",
			Pattern:    "5\\.2.*debian-9.*",
		},
		Source: &compSource,
		Dependencies: []*s2hv1beta1.Component{
			{
				Name: "mariadb",
				Image: s2hv1beta1.ComponentImage{
					Repository: "bitnami/mariadb",
					Pattern:    "10\\.3.*debian-9.*",
				},
				Source: &compSource,
			},
		},
	}

	mockConfig := &s2hv1beta1.Config{
		Spec: s2hv1beta1.ConfigSpec{
			Staging: &s2hv1beta1.ConfigStaging{
				MaxRetry:   3,
				Deployment: &deployConfig,
			},
			ActivePromotion: &s2hv1beta1.ConfigActivePromotion{
				Timeout:          metav1.Duration{Duration: 10 * time.Minute},
				TearDownDuration: metav1.Duration{Duration: 10 * time.Second},
				Deployment:       &deployConfig,
			},
			Components: []*s2hv1beta1.Component{
				&redisConfigComp,
				&wordpressConfigComp,
			},
		},
	}

	return mockConfig, nil
}

func (c *mockConfigCtrl) GetComponents(configName string) (map[string]*s2hv1beta1.Component, error) {
	config, _ := c.Get(configName)

	comps := map[string]*s2hv1beta1.Component{
		"redis":     config.Spec.Components[0],
		"wordpress": config.Spec.Components[1],
		"mariadb":   config.Spec.Components[1].Dependencies[0],
	}

	comps["mariadb"].Parent = "wordpress"

	return comps, nil
}

func (c *mockConfigCtrl) GetParentComponents(configName string) (map[string]*s2hv1beta1.Component, error) {
	return map[string]*s2hv1beta1.Component{}, nil
}

func (c *mockConfigCtrl) Update(config *s2hv1beta1.Config) error {
	return nil
}

func (c *mockConfigCtrl) Delete(configName string) error {
	return nil
}
