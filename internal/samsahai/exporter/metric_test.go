package exporter

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	s2hv1 "github.com/agoda-com/samsahai/api/v1"
	"github.com/agoda-com/samsahai/internal"
	conf "github.com/agoda-com/samsahai/internal/util/config"
	"github.com/agoda-com/samsahai/internal/util/http"
	"github.com/agoda-com/samsahai/internal/util/unittest"
)

func TestExporter(t *testing.T) {
	unittest.InitGinkgo(t, "Samsahai Exporter")
}

var cfg *rest.Config

func TestMain(m *testing.M) {
	var err error
	t := &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "..", "..", "test", "data", "crds")},
	}

	err = s2hv1.SchemeBuilder.AddToScheme(scheme.Scheme)
	if err != nil {
		log.Fatal(err)
	}

	if cfg, err = t.Start(); err != nil {
		logger.Error(err, "start testenv error")
		os.Exit(1)
	}

	if _, err := client.New(cfg, client.Options{Scheme: scheme.Scheme}); err != nil {
		log.Fatal(err)
	}

	code := m.Run()
	_ = t.Stop()
	os.Exit(code)
}

var _ = Describe("Samsahai Exporter", func() {
	timeout := float64(3000)
	namespace := "default"
	g := NewWithT(GinkgoT())
	var (
		wgStop     *sync.WaitGroup
		chStop     chan struct{}
		configCtrl internal.ConfigController
		err        error
	)

	RegisterMetrics()

	BeforeEach(func(done Done) {
		defer GinkgoRecover()
		defer close(done)

		configCtrl = newMockConfigCtrl()
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(configCtrl).NotTo(BeNil())

		mgr, err := manager.New(cfg, manager.Options{Namespace: namespace, MetricsBindAddress: ":8008"})
		Expect(err).NotTo(HaveOccurred(), "should create manager successfully")

		teamList := &s2hv1.TeamList{
			Items: []s2hv1.Team{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "testQTeamName1",
					},
				},
			},
		}
		queue := &s2hv1.Queue{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "group",
				Namespace: namespace,
			},
			Spec: s2hv1.QueueSpec{
				TeamName: "testQTeamName1",
				Components: s2hv1.QueueComponents{
					{
						Name:    "qName1",
						Version: "10.9.8.7",
					},
					{
						Name:    "qName2",
						Version: "10.9.8.7.2",
					},
				},
				NoOfOrder: 0,
			},
			Status: s2hv1.QueueStatus{
				NoOfProcessed: 1,
				State:         "waiting",
			},
		}
		activePromotion := &s2hv1.ActivePromotion{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "testAPName1",
				Namespace: namespace,
			},
			Status: s2hv1.ActivePromotionStatus{
				State: s2hv1.ActivePromotionWaiting,
			},
		}

		SetTeamNameMetric(teamList)
		SetQueueMetric(queue)
		SetActivePromotionMetric(activePromotion)
		SetHealthStatusMetric("9.9.9.8", "777888999", 234000)

		chStop = make(chan struct{})
		wgStop = &sync.WaitGroup{}
		wgStop.Add(1)
		go func() {
			defer wgStop.Done()
			Expect(mgr.Start(chStop)).To(BeNil())
		}()
	}, timeout)

	AfterEach(func(done Done) {
		defer close(done)
		close(chStop)
		wgStop.Wait()
	}, timeout)

	It("should show team name correctly ", func() {
		_, data, err := http.Get("http://localhost:8008/metrics")
		g.Expect(err).NotTo(HaveOccurred())
		expectedData := strings.Contains(string(data), `samsahai_team{teamName="testQTeamName1"} 1`)
		g.Expect(expectedData).To(BeTrue())
	}, timeout)

	It("should show queue metric correctly  ", func(done Done) {
		defer close(done)
		_, data, err := http.Get("http://localhost:8008/metrics")
		g.Expect(err).NotTo(HaveOccurred())
		expectedData := strings.Contains(string(data), `samsahai_queue{component="qName1",no_of_processed="1",order="0",queueName="group",state="waiting",teamName="testQTeamName1",version="10.9.8.7"} 1`)
		g.Expect(expectedData).To(BeTrue())
		expectedData = strings.Contains(string(data), `samsahai_queue{component="qName2",no_of_processed="1",order="0",queueName="group",state="waiting",teamName="testQTeamName1",version="10.9.8.7.2"} 1`)
		g.Expect(expectedData).To(BeTrue())
		expectedData = strings.Contains(string(data), `samsahai_queue{component="",`)
		g.Expect(expectedData).To(BeFalse())
	}, timeout)

	It("should show active promotion correctly", func(done Done) {
		defer close(done)
		_, data, err := http.Get("http://localhost:8008/metrics")
		g.Expect(err).NotTo(HaveOccurred())
		expectedData := strings.Contains(string(data), `samsahai_active_promotion{state="waiting",teamName="testAPName1"} 1`)
		g.Expect(expectedData).To(BeTrue())
	}, timeout)

	It("should show health metric correctly", func(done Done) {
		defer close(done)
		_, data, err := http.Get("http://localhost:8008/metrics")
		g.Expect(err).NotTo(HaveOccurred())
		expectedData := strings.Contains(string(data), `samsahai_health{gitCommit="777888999",version="9.9.9.8"} 234000`)
		g.Expect(expectedData).To(BeTrue())
	}, timeout)
})

type mockConfigCtrl struct{}

func newMockConfigCtrl() internal.ConfigController {
	return &mockConfigCtrl{}
}

func (c *mockConfigCtrl) Get(configName string) (*s2hv1.Config, error) {
	engine := "helm3"
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
			Repository: "https://charts.helm.sh/stable",
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
			Repository: "https://charts.helm.sh/stable",
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
		Status: s2hv1.ConfigStatus{
			Used: s2hv1.ConfigSpec{
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
		},
	}

	return mockConfig, nil
}

func (c *mockConfigCtrl) GetComponents(configName string) (map[string]*s2hv1.Component, error) {
	config, _ := c.Get(configName)

	comps := map[string]*s2hv1.Component{
		"redis":     config.Status.Used.Components[0],
		"wordpress": config.Status.Used.Components[1],
		"mariadb":   conf.Convert(config.Spec.Components[1].Dependencies[0], nil),
	}

	comps["mariadb"].Parent = "wordpress"

	return comps, nil
}

func (c *mockConfigCtrl) GetParentComponents(configName string) (map[string]*s2hv1.Component, error) {
	return map[string]*s2hv1.Component{}, nil
}

func (c *mockConfigCtrl) GetPullRequestComponents(configName, prBundleName string, depIncluded bool) (map[string]*s2hv1.Component, error) {
	return map[string]*s2hv1.Component{}, nil
}

func (c *mockConfigCtrl) GetBundles(configName string) (s2hv1.ConfigBundles, error) {
	return s2hv1.ConfigBundles{}, nil
}

func (c *mockConfigCtrl) GetPriorityQueues(configName string) ([]string, error) {
	return nil, nil
}

func (c *mockConfigCtrl) GetPullRequestConfig(configName string) (*s2hv1.ConfigPullRequest, error) {
	return nil, nil
}

func (c *mockConfigCtrl) GetPullRequestBundleDependencies(configName, prBundleName string) ([]string, error) {
	return nil, nil
}

func (c *mockConfigCtrl) Update(config *s2hv1.Config) error {
	return nil
}

func (c *mockConfigCtrl) Delete(configName string) error {
	return nil
}

func (c *mockConfigCtrl) EnsureConfigTemplateChanged(config *s2hv1.Config) error {
	return nil
}
