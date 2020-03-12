package exporter

import (
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	s2hv1beta1 "github.com/agoda-com/samsahai/api/v1beta1"
	"github.com/agoda-com/samsahai/internal"
	"github.com/agoda-com/samsahai/internal/config"
	"github.com/agoda-com/samsahai/internal/config/git"
	"github.com/agoda-com/samsahai/internal/util/http"
	"github.com/agoda-com/samsahai/internal/util/unittest"
)

func TestExporter(t *testing.T) {
	unittest.InitGinkgo(t, "Samsahai Exporter")
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

var _ = Describe("Samsahai Exporter", func() {
	timeout := float64(3000)
	namespace := "default"
	g := NewWithT(GinkgoT())
	var wgStop *sync.WaitGroup
	var chStop chan struct{}
	var configMgr internal.ConfigManager
	var err error

	RegisterMetrics()

	BeforeEach(func(done Done) {
		defer GinkgoRecover()
		defer close(done)

		configMgr, err = config.NewWithGitClient(&git.Client{}, "example", path.Join("..", "..", "..", "test", "data", "wordpress-redis"))
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(configMgr).NotTo(BeNil())

		chStop = make(chan struct{})

		mgr, err := manager.New(cfg, manager.Options{Namespace: namespace, MetricsBindAddress: ":8008"})
		Expect(err).NotTo(HaveOccurred(), "should create manager successfully")

		t := map[string]internal.ConfigManager{
			"testQTeamName1": configMgr,
		}
		q := &s2hv1beta1.Queue{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "qName1",
				Namespace: namespace,
			},
			Spec: s2hv1beta1.QueueSpec{
				TeamName:  "testQTeamName1",
				Version:   "10.9.8.7",
				NoOfOrder: 0,
			},
			Status: s2hv1beta1.QueueStatus{
				NoOfProcessed: 1,
				State:         "waiting",
			},
		}
		ap := &s2hv1beta1.ActivePromotion{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "testAPName1",
				Namespace: namespace,
			},
			Status: s2hv1beta1.ActivePromotionStatus{
				State: s2hv1beta1.ActivePromotionWaiting,
			},
		}

		SetTeamNameMetric(t)
		SetQueueMetric(q)
		SetActivePromotionMetric(ap)
		SetHealthStatusMetric("9.9.9.8", "777888999", 234000)

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

	It("Should show team name correctly ", func() {
		data, err := http.Get("http://localhost:8008/metrics")
		g.Expect(err).NotTo(HaveOccurred())
		expectedData := strings.Contains(string(data), `samsahai_team{teamName="testQTeamName1"} 1`)
		g.Expect(expectedData).To(BeTrue())
	}, timeout)

	It("Should show queue metric correctly  ", func(done Done) {
		defer close(done)
		data, err := http.Get("http://localhost:8008/metrics")
		g.Expect(err).NotTo(HaveOccurred())
		expectedData := strings.Contains(string(data), `samsahai_queue{component="qName1",no_of_processed="1",order="0",state="waiting",teamName="testQTeamName1",version="10.9.8.7"} 1`)
		g.Expect(expectedData).To(BeTrue())
		expectedData = strings.Contains(string(data), `samsahai_queue{component="",`)
		g.Expect(expectedData).To(BeFalse())
	}, timeout)

	It("Should show active promotion correctly", func(done Done) {
		defer close(done)
		data, err := http.Get("http://localhost:8008/metrics")
		g.Expect(err).NotTo(HaveOccurred())
		expectedData := strings.Contains(string(data), `samsahai_active_promotion{state="waiting",teamName="testAPName1"} 1`)
		g.Expect(expectedData).To(BeTrue())
	}, timeout)

	It("Should show health metric correctly", func(done Done) {
		defer close(done)
		data, err := http.Get("http://localhost:8008/metrics")
		g.Expect(err).NotTo(HaveOccurred())
		expectedData := strings.Contains(string(data), `samsahai_health{gitCommit="777888999",version="9.9.9.8"} 234000`)
		g.Expect(expectedData).To(BeTrue())
	}, timeout)
})
