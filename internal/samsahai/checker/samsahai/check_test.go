package samsahai

import (
	"context"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"testing"

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
		s2hConfig := s2h.SamsahaiConfig{SamsahaiCredential: s2h.SamsahaiCredential{InternalAuthToken: "123456"}}
		s2hCtrl = samsahai.New(nil, namespace, s2hConfig,
			samsahai.WithClient(crClient),
			samsahai.WithDisableLoaders(true, true, true),
			samsahai.WithScheme(scheme.Scheme))
		check = New(s2hCtrl)

		yamlTeam, err := ioutil.ReadFile(path.Join("..", "..", "..", "..", "test", "data", "team", "team.yaml"))
		g.Expect(err).NotTo(HaveOccurred())
		obj, _ := util.MustParseYAMLtoRuntimeObject(yamlTeam)

		team, _ := obj.(*s2hv1beta1.Team)
		team.Status.StableComponents = []s2hv1beta1.StableComponent{
			{
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
			{
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
