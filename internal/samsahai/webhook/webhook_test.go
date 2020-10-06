package webhook

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tidwall/gjson"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	s2hv1beta1 "github.com/agoda-com/samsahai/api/v1beta1"
	s2h "github.com/agoda-com/samsahai/internal"
	"github.com/agoda-com/samsahai/internal/samsahai"
	"github.com/agoda-com/samsahai/internal/util"
	"github.com/agoda-com/samsahai/internal/util/http"
	"github.com/agoda-com/samsahai/internal/util/unittest"
)

func TestWebhook(t *testing.T) {
	unittest.InitGinkgo(t, "Samsahai Webhook")
}

var cfg *rest.Config
var c client.Client

func TestMain(m *testing.M) {
	var err error
	t := &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "..", "..", "config", "crds")},
	}

	err = s2hv1beta1.SchemeBuilder.AddToScheme(scheme.Scheme)
	if err != nil {
		log.Fatal(err)
	}

	if cfg, err = t.Start(); err != nil {
		logger.Error(err, "start testenv error")
		os.Exit(1)
	}

	if c, err = client.New(cfg, client.Options{Scheme: scheme.Scheme}); err != nil {
		log.Fatal(err)
	}

	code := m.Run()
	_ = t.Stop()
	os.Exit(code)
}

var _ = Describe("Samsahai Webhook", func() {
	var server *httptest.Server
	var s2hCtrl s2h.SamsahaiController
	timeout := float64(60)
	teamName := "example"
	athName := "activepromotion-history"
	qhName := "test-history"
	namespace := "default"
	g := NewWithT(GinkgoT())

	BeforeEach(func(done Done) {
		defer close(done)

		configCtrl := newMockConfigCtrl()
		s2hConfig := s2h.SamsahaiConfig{
			PluginsDir: path.Join("..", "plugin"),
			SamsahaiCredential: s2h.SamsahaiCredential{
				InternalAuthToken: "123456",
			},
		}
		s2hCtrl = samsahai.New(nil, namespace, s2hConfig,
			samsahai.WithClient(c),
			samsahai.WithDisableLoaders(true, false, true),
			samsahai.WithScheme(scheme.Scheme),
			samsahai.WithConfigCtrl(configCtrl))

		r := New(s2hCtrl)

		qh := &s2hv1beta1.QueueHistory{
			ObjectMeta: metav1.ObjectMeta{
				Name:      qhName,
				Namespace: namespace,
			},
			Spec: s2hv1beta1.QueueHistorySpec{
				Queue: &s2hv1beta1.Queue{
					Status: s2hv1beta1.QueueStatus{
						KubeZipLog: "UEsDBAoAAAAAAEaVdU_5775xAQAAAAEAAAABABwAYVVUCQADFHjWXRR41l11eAsAAQRfQcJQBF9BwlBiUEsBAh4DCgAAAAAARpV1T_nvvnEBAAAAAQAAAAEAGAAAAAAAAQAAAKSBAAAAAGFVVAUAAxR41l11eAsAAQRfQcJQBF9BwlBQSwUGAAAAAAEAAQBHAAAAPAAAAAAA",
					},
				},
			},
		}
		ath := &s2hv1beta1.ActivePromotionHistory{
			ObjectMeta: metav1.ObjectMeta{
				Name: athName,
				Labels: map[string]string{
					"samsahai.io/teamname": teamName,
				},
			},
			Spec: s2hv1beta1.ActivePromotionHistorySpec{
				ActivePromotion: &s2hv1beta1.ActivePromotion{
					Status: s2hv1beta1.ActivePromotionStatus{
						PreActiveQueue: qh.Spec.Queue.Status,
					},
				},
			},
		}

		Expect(c.Create(context.TODO(), qh)).NotTo(HaveOccurred())
		Expect(c.Create(context.TODO(), ath)).NotTo(HaveOccurred())

		yamlTeam, err := ioutil.ReadFile(path.Join("..", "..", "..", "test", "data", "team", "team.yaml"))
		g.Expect(err).NotTo(HaveOccurred())
		obj, _ := util.MustParseYAMLtoRuntimeObject(yamlTeam)

		_ = c.Create(context.TODO(), obj)

		server = httptest.NewServer(r)
	}, timeout)

	AfterEach(func(done Done) {
		defer close(done)
		team := &s2hv1beta1.Team{}
		qh := &s2hv1beta1.QueueHistory{}
		ath := &s2hv1beta1.ActivePromotionHistory{}
		ctx := context.TODO()
		_ = c.Get(ctx, client.ObjectKey{Name: teamName, Namespace: namespace}, team)
		_ = c.Delete(ctx, team)
		_ = c.Get(ctx, client.ObjectKey{Name: qhName, Namespace: namespace}, qh)
		_ = c.Delete(ctx, qh)
		_ = c.Get(ctx, client.ObjectKey{Name: athName}, ath)
		_ = c.Delete(ctx, ath)
		server.Close()
	}, timeout)

	It("should successfully get version", func(done Done) {
		defer close(done)
		_, data, err := http.Get(server.URL + s2h.URIVersion)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(data).NotTo(BeEmpty())
		g.Expect(gjson.ValidBytes(data)).To(BeTrue())
	}, timeout)

	It("should successfully get health check", func(done Done) {
		defer close(done)
		_, data, err := http.Get(server.URL + s2h.URIHealthz)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(data).NotTo(BeEmpty())
		g.Expect(gjson.ValidBytes(data)).To(BeTrue())
	}, timeout)

	Describe("Plugin", func() {
		It("should successfully receive webhook from plugin", func(done Done) {
			defer close(done)
			pluginName := "example"

			reqData := map[string]interface{}{
				"component": "Kubernetes",
			}
			b, err := json.Marshal(reqData)
			g.Expect(err).NotTo(HaveOccurred())
			_, _, err = http.Post(server.URL+"/webhook/"+pluginName, b)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(s2hCtrl.QueueLen()).To(Equal(1))
		}, timeout)
	})

	Describe("Team", func() {
		It("should successfully list teams", func(done Done) {
			defer close(done)
			defer GinkgoRecover()

			_, data, err := http.Get(server.URL + "/teams")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(data).NotTo(BeNil())
		}, timeout)

		It("should successfully get team", func(done Done) {
			defer close(done)

			_, data, err := http.Get(server.URL + "/teams/" + teamName)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(data).NotTo(BeNil())
		}, timeout)

		Specify("Unknown team", func(done Done) {
			defer close(done)

			_, _, err := http.Get(server.URL + "/teams/" + "unknown")
			g.Expect(err).To(HaveOccurred())
		}, timeout)

		It("should successfully get team configuration", func(done Done) {
			defer close(done)

			_, data, err := http.Get(server.URL + "/teams/" + teamName + "/config")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(data).NotTo(BeNil())
		}, timeout)

		It("should successfully get team component", func(done Done) {
			defer close(done)

			_, data, err := http.Get(server.URL + "/teams/" + teamName + "/components")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(data).NotTo(BeNil())
		}, timeout)

		It("should successfully get stable values from team", func(done Done) {
			defer close(done)

			_, data, err := http.Get(server.URL + "/teams/" + teamName + "/components/redis/values")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(data).NotTo(BeNil())
		}, timeout)

		It("should successfully get zip log from queue history", func(done Done) {
			defer close(done)

			_, data, err := http.Get(server.URL + "/teams/" + teamName + "/queue/histories/test-history/log")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(data).NotTo(BeNil())
		}, timeout)

		It("should successfully get zip log from active promotion history", func(done Done) {
			defer close(done)

			_, data, err := http.Get(server.URL + "/teams/" + teamName + "/activepromotions/histories/activepromotion-history/log")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(data).NotTo(BeNil())
		}, timeout)
	})

	Describe("ActivePromotion", func() {
		It("should successfully list activepromotions", func(done Done) {
			defer close(done)
			defer GinkgoRecover()

			_, data, err := http.Get(server.URL + "/activepromotions")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(data).NotTo(BeNil())
		}, timeout)

		It("should successfully get team activepromotion", func(done Done) {
			defer close(done)

			_, data, err := http.Get(server.URL + "/teams/" + teamName + "/activepromotions")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(data).NotTo(BeNil())
		}, timeout)

		It("should successfully get team activepromotion histories", func(done Done) {
			defer close(done)

			_, data, err := http.Get(server.URL + "/teams/" + teamName + "/activepromotions/histories")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(data).NotTo(BeNil())
		}, timeout)

		Specify("Unknown active promotion", func(done Done) {
			defer close(done)

			_, _, err := http.Get(server.URL + "/teams/unknown/activepromotions")
			g.Expect(err).To(HaveOccurred())
		}, timeout)
	})
})

type mockConfigCtrl struct{}

func newMockConfigCtrl() s2h.ConfigController {
	return &mockConfigCtrl{}
}

func (c *mockConfigCtrl) Get(configName string) (*s2hv1beta1.Config, error) {
	engine := "helm3"
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
		Status: s2hv1beta1.ConfigStatus{
			Used: s2hv1beta1.ConfigSpec{
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
		},
	}

	return mockConfig, nil
}

func (c *mockConfigCtrl) GetComponents(configName string) (map[string]*s2hv1beta1.Component, error) {
	config, _ := c.Get(configName)

	comps := map[string]*s2hv1beta1.Component{
		"redis":     config.Status.Used.Components[0],
		"wordpress": config.Status.Used.Components[1],
		"mariadb":   config.Status.Used.Components[1].Dependencies[0],
	}

	comps["mariadb"].Parent = "wordpress"

	return comps, nil
}

func (c *mockConfigCtrl) GetParentComponents(configName string) (map[string]*s2hv1beta1.Component, error) {
	config, _ := c.Get(configName)

	comps := map[string]*s2hv1beta1.Component{
		"redis":     config.Status.Used.Components[0],
		"wordpress": config.Status.Used.Components[1],
	}

	return comps, nil
}

func (c *mockConfigCtrl) GetBundles(configName string) (s2hv1beta1.ConfigBundles, error) {
	return s2hv1beta1.ConfigBundles{}, nil
}

func (c *mockConfigCtrl) GetPriorityQueues(configName string) ([]string, error) {
	return nil, nil
}

func (c *mockConfigCtrl) Update(config *s2hv1beta1.Config) error {
	return nil
}

func (c *mockConfigCtrl) Delete(configName string) error {
	return nil
}

func (c *mockConfigCtrl) EnsureConfigTemplateChanged(config *s2hv1beta1.Config) error {
	return nil
}
