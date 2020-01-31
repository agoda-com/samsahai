package helmrelease

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fluxcd/flux/integrations/apis/flux.weave.works/v1beta1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/helm/pkg/chartutil"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/agoda-com/samsahai/internal"
	s2hlog "github.com/agoda-com/samsahai/internal/log"
	"github.com/agoda-com/samsahai/internal/util/unittest"
)

var cfg *rest.Config
var c internal.HelmReleaseClient

func TestMain(m *testing.M) {
	if os.Getenv("DEBUG") != "" {
		s2hlog.SetLogger(zap.New(func(o *zap.Options) {
			o.Development = true
		}))
	}

	t := &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "..", "..", "config", "manifests")},
	}

	err := v1beta1.SchemeBuilder.AddToScheme(scheme.Scheme)
	if err != nil {
		logger.Error(err, "addtoscheme error")
		os.Exit(1)
	}

	if cfg, err = t.Start(); err != nil {
		logger.Error(err, "start testenv error")
		os.Exit(1)
	}

	runtimeClient, err := crclient.New(cfg, crclient.Options{Scheme: scheme.Scheme})
	c = New("default", runtimeClient)
	//Expect(c).NotTo(BeNil())
	if c == nil {
		logger.Error(err, "create runtime client error")
		os.Exit(1)
	}
	//if c, err = New(cfg); e/rr != nil {
	//	logger.Error(err, "create runtime client error")
	//	os.Exit(1)
	//}

	code := m.Run()
	_ = t.Stop()
	os.Exit(code)
}

func TestUnit(t *testing.T) {
	unittest.InitGinkgo(t, "HelmRelease Rest Client")
}

var _ = Describe("HelmRelease Rest Client", func() {
	g := NewWithT(GinkgoT())

	It("Should successfully Create/Get/Update/List/Delete/DeleteCollection", func() {
		// Test Create
		//fetched := &Team{}
		var err error
		namespace := "default"
		created := v1beta1.HelmRelease{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: namespace,
			},
			Spec: v1beta1.HelmReleaseSpec{
				ForceUpgrade: false,
				ReleaseName:  "foo",
				ChartSource: v1beta1.ChartSource{
					RepoChartSource: &v1beta1.RepoChartSource{
						RepoURL: "https://kubernetes-charts.storage.googleapis.com",
						Name:    "redis",
						Version: "",
					},
				},
				HelmValues: v1beta1.HelmValues{
					Values: chartutil.Values{
						"cluster": map[string]interface{}{
							"enabled": false,
						},
						"master": map[string]interface{}{
							"persistence": map[string]interface{}{
								"enabled": false,
							},
						},
					},
				},
			},
		}

		By("create HelmRelease")
		createdObj := created
		_, err = c.Create(&createdObj)
		g.Expect(err).NotTo(HaveOccurred())

		By("get HelmRelease")
		fetched, err := c.Get("foo", metav1.GetOptions{})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(fetched.Spec).To(Equal(created.Spec))

		// Test Updating the Labels
		By("updating the labels")
		updated := fetched.DeepCopy()
		updated.Labels = map[string]string{"hello": "world"}
		_, err = c.Update(updated)
		g.Expect(err).NotTo(HaveOccurred())

		fetched, err = c.Get("foo", metav1.GetOptions{})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(fetched.Labels).To(Equal(updated.Labels))

		// Test Delete
		By("delete HelmRelease")
		g.Expect(c.Delete("foo", &metav1.DeleteOptions{})).NotTo(HaveOccurred())

		createdObj = created
		_, err = c.Create(&createdObj)
		g.Expect(err).NotTo(HaveOccurred())

		By("list HelmRelease")
		listed, err := c.List(metav1.ListOptions{})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(len(listed.Items)).To(Equal(1))

		By("delete collection")
		err = c.DeleteCollection(nil, metav1.ListOptions{})
		g.Expect(err).NotTo(HaveOccurred())

		listed, err = c.List(metav1.ListOptions{})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(len(listed.Items)).To(Equal(0))

		_, err = c.Get("foo", metav1.GetOptions{})
		g.Expect(err).To(HaveOccurred())
	})
})
