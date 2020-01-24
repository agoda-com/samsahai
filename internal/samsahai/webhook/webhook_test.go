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
	s2hconfig "github.com/agoda-com/samsahai/internal/config"
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
		configMgr, err := s2hconfig.NewWithGitClient(nil, teamName, path.Join("..", "..", "..", "test", "data"))
		g.Expect(err).NotTo(HaveOccurred())
		s2hConfig := s2h.SamsahaiConfig{
			PluginsDir: path.Join("..", "plugin"),
			SamsahaiCredential: s2h.SamsahaiCredential{
				InternalAuthToken: "123456",
			},
		}

		s2hCtrl = samsahai.New(nil, namespace, s2hConfig,
			samsahai.WithClient(c),
			samsahai.WithConfigManager(teamName, configMgr),
			samsahai.WithDisableLoaders(true, false, true),
			samsahai.WithScheme(scheme.Scheme))

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

		//c.Create(ctx, )
		yamlTeam, err := ioutil.ReadFile(path.Join("..", "..", "..", "test", "data", "github", "team.yaml"))
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

	It("Should successfully get version", func(done Done) {
		defer close(done)
		data, err := http.Get(server.URL + s2h.URIVersion)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(data).NotTo(BeEmpty())
		g.Expect(gjson.ValidBytes(data)).To(BeTrue())
	}, timeout)

	It("Should successfully get health check", func(done Done) {
		defer close(done)
		data, err := http.Get(server.URL + s2h.URIHealthz)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(data).NotTo(BeEmpty())
		g.Expect(gjson.ValidBytes(data)).To(BeTrue())
	}, timeout)

	Describe("Github", func() {
		It("Should successfully receive webhook from github", func(done Done) {
			defer close(done)

			reqData := map[string]interface{}{
				"ref": "/refs/head/master",
				"repository": map[string]interface{}{
					"name":      "samsahai-example",
					"full_name": "agoda-com/samsahai-example",
				},
			}
			b, err := json.Marshal(reqData)
			g.Expect(err).NotTo(HaveOccurred())
			_, err = http.Post(server.URL+"/webhook/github", b)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(s2hCtrl.QueueLen()).To(Equal(1))
		}, timeout)

		It("Should error if invalid response", func(done Done) {
			defer close(done)

			reqData := map[string]interface{}{
				"ref":        "/refs/head/master",
				"repository": map[string]interface{}{},
			}
			b, err := json.Marshal(reqData)
			g.Expect(err).NotTo(HaveOccurred())
			_, err = http.Post(server.URL+"/webhook/github", b)
			g.Expect(err).NotTo(BeNil())
		}, timeout)
	})

	Describe("Plugin", func() {
		It("Should successfully receive webhook from plugin", func(done Done) {
			defer close(done)
			pluginName := "example"

			reqData := map[string]interface{}{
				"component": "Kubernetes",
			}
			b, err := json.Marshal(reqData)
			g.Expect(err).NotTo(HaveOccurred())
			_, err = http.Post(server.URL+"/webhook/"+pluginName, b)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(s2hCtrl.QueueLen()).To(Equal(1))
		}, timeout)
	})

	Describe("Team", func() {
		It("Should successfully list teams", func(done Done) {
			defer close(done)
			defer GinkgoRecover()

			data, err := http.Get(server.URL + "/teams")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(data).NotTo(BeNil())
		}, timeout)

		It("Should successfully get team", func(done Done) {
			defer close(done)

			data, err := http.Get(server.URL + "/teams/" + teamName)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(data).NotTo(BeNil())
		}, timeout)

		Specify("Unknown team", func(done Done) {
			defer close(done)

			_, err := http.Get(server.URL + "/teams/" + "unknown")
			g.Expect(err).To(HaveOccurred())
		}, timeout)

		It("Should successfully get team configuration", func(done Done) {
			defer close(done)

			data, err := http.Get(server.URL + "/teams/" + teamName + "/config")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(data).NotTo(BeNil())
		}, timeout)

		It("Should successfully get team component", func(done Done) {
			defer close(done)

			data, err := http.Get(server.URL + "/teams/" + teamName + "/components")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(data).NotTo(BeNil())
		}, timeout)

		It("Should successfully get stable values from team", func(done Done) {
			defer close(done)

			data, err := http.Get(server.URL + "/teams/" + teamName + "/components/redis/values")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(data).NotTo(BeNil())
		}, timeout)

		It("Should successfully get zip log from queue history", func(done Done) {
			defer close(done)

			data, err := http.Get(server.URL + "/teams/" + teamName + "/queue/histories/test-history/log")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(data).NotTo(BeNil())
		}, timeout)

		It("Should successfully get zip log from active promotion history", func(done Done) {
			defer close(done)

			data, err := http.Get(server.URL + "/teams/" + teamName + "/activepromotions/histories/activepromotion-history/log")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(data).NotTo(BeNil())
		}, timeout)
	})

	Describe("ActivePromotion", func() {
		It("Should successfully list activepromotions", func(done Done) {
			defer close(done)
			defer GinkgoRecover()

			data, err := http.Get(server.URL + "/activepromotions")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(data).NotTo(BeNil())
		}, timeout)

		It("Should successfully get team activepromotion", func(done Done) {
			defer close(done)

			data, err := http.Get(server.URL + "/teams/" + teamName + "/activepromotions")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(data).NotTo(BeNil())
		}, timeout)

		It("Should successfully get team activepromotion histories", func(done Done) {
			defer close(done)

			data, err := http.Get(server.URL + "/teams/" + teamName + "/activepromotions/histories")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(data).NotTo(BeNil())
		}, timeout)

		Specify("Unknown active promotion", func(done Done) {
			defer close(done)

			_, err := http.Get(server.URL + "/teams/unknown/activepromotions")
			g.Expect(err).To(HaveOccurred())
		}, timeout)
	})
})
