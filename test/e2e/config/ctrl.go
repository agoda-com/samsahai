package config

import (
	"context"
	"io/ioutil"
	"os"
	"path"
	"sync"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	rclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	s2hv1beta1 "github.com/agoda-com/samsahai/api/v1beta1"
	"github.com/agoda-com/samsahai/internal"
	configctrl "github.com/agoda-com/samsahai/internal/config"
	"github.com/agoda-com/samsahai/internal/samsahai"
	s2hobject "github.com/agoda-com/samsahai/internal/samsahai/k8sobject"
	"github.com/agoda-com/samsahai/internal/util"
)

const (
	verifyTime1s           = 1 * time.Second
//	verifyTime5s           = 5 * time.Second
//	verifyTime10s          = 10 * time.Second
//	verifyTime15s          = 15 * time.Second
	verifyTime30s          = 30 * time.Second
)
var (
	samsahaiCtrl   internal.SamsahaiController
	configCtrl     internal.ConfigController
	wgStop         *sync.WaitGroup
	chStop         chan struct{}
	mgr            manager.Manager
	client         rclient.Client
	namespace      string
)

func setupSamsahai() {
	s2hConfig := samsahaiConfig

	samsahaiCtrl = samsahai.New(mgr, "samsahai-system", s2hConfig)
	Expect(samsahaiCtrl).ToNot(BeNil())

	configCtrl = configctrl.New(mgr, configctrl.WithClient(client), configctrl.WithS2hCtrl(samsahaiCtrl))
	Expect(configCtrl).NotTo(BeNil(), "Should successfully init Config controller")

	wgStop = &sync.WaitGroup{}
	wgStop.Add(1)
	go func() {
		defer wgStop.Done()
		Expect(mgr.Start(chStop)).To(BeNil())
	}()
}

var _ = Describe("[e2e] Config controller", func() {
	BeforeEach(func(done Done) {
		defer close(done)
		chStop = make(chan struct{})

		adminRestConfig, err := config.GetConfig()
		Expect(err).NotTo(HaveOccurred(), "Please provide credential for accessing k8s cluster")

		restCfg := rest.CopyConfig(adminRestConfig)
		mgr, err = manager.New(restCfg, manager.Options{MetricsBindAddress: "0"})
		Expect(err).NotTo(HaveOccurred(), "should create manager successfully")

		namespace = os.Getenv("POD_NAMESPACE")
		Expect(namespace).NotTo(BeEmpty(), "Please provided POD_NAMESPACE")

		cfg, err := config.GetConfig()
		Expect(err).To(BeNil(), "Please provide credential for accessing k8s cluster")

		client, err = rclient.New(cfg, rclient.Options{Scheme: scheme.Scheme})
		Expect(err).NotTo(HaveOccurred())
		Expect(client).NotTo(BeNil())

	}, 5)

	AfterEach(func(done Done) {
		defer close(done)
		ctx := context.TODO()

		By("Deleting Config")
		Expect(configCtrl.Delete(teamTest)).NotTo(HaveOccurred())
		Expect(configCtrl.Delete(teamTest2)).NotTo(HaveOccurred())

		By("Deleting all Teams")
		err := client.DeleteAllOf(ctx, &s2hv1beta1.Team{}, rclient.MatchingLabels(testLabels))
		Expect(err).NotTo(HaveOccurred())
		err = wait.PollImmediate(verifyTime1s, verifyTime30s, func() (ok bool, err error) {
			teamList := s2hv1beta1.TeamList{}
			listOpt := &rclient.ListOptions{LabelSelector: labels.SelectorFromSet(testLabels)}
			err = client.List(ctx, &teamList, listOpt)
			if err != nil && errors.IsNotFound(err) {
				return true, nil
			}
			if len(teamList.Items) == 0 {
				return true, nil
			}

			return false, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Delete all Teams error")

	}, 5)

	It("should successfully get/delete Config", func(done Done) {
		defer close(done)
		setupSamsahai()
		ctx := context.TODO()

		By("Creating Config")
		yamlTeam, err := ioutil.ReadFile(path.Join("..", "data", "wordpress-redis", "config.yaml"))
		Expect(err).NotTo(HaveOccurred())

		obj, _ := util.MustParseYAMLtoRuntimeObject(yamlTeam)
		config, _ := obj.(*s2hv1beta1.Config)
		Expect(client.Create(ctx, config)).To(BeNil())

		By("Get Config")
		cfg, err := configCtrl.Get(teamTest)
		Expect(err).To(BeNil())
		Expect(cfg.Status.Used).NotTo(BeNil())
		Expect(len(cfg.Spec.Components)).To(Equal(2))
		Expect(len(cfg.Spec.Envs)).To(Equal(4))
		Expect(cfg.Spec.Staging).NotTo(BeNil())
		Expect(cfg.Spec.ActivePromotion).NotTo(BeNil())

		By("Get components")
		Expect(configCtrl.EnsureConfigTemplateChanged(config)).To(BeNil())
		Expect(configCtrl.Update(config)).To(BeNil())
		comps, err := configCtrl.GetComponents(teamTest)
		Expect(err).To(BeNil())
		Expect(len(comps)).To(Equal(3))

		By("Get parent components")
		parentComps, err := configCtrl.GetParentComponents(teamTest)
		Expect(err).To(BeNil())
		Expect(len(parentComps)).To(Equal(2))

		By("Get bundles")
		bundles, err := configCtrl.GetBundles(teamTest)
		Expect(err).To(BeNil())
		dbs, ok := bundles["db"]
		Expect(ok).To(BeTrue())
		Expect(len(dbs)).To(Equal(2))

		By("Delete Config")
		err = configCtrl.Delete(teamTest)

		By("Config should be deleted")
		err = wait.PollImmediate(1*time.Second, 5*time.Second, func() (ok bool, err error) {
			config = &s2hv1beta1.Config{}
			err = client.Get(context.TODO(), types.NamespacedName{Name: teamTest}, config)
			if err != nil && errors.IsNotFound(err) {
				return true, nil
			}

			return false, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Delete config error")
	}, 10)

	It("Should successfully apply/update config template", func(done Done) {
		defer close(done)
		setupSamsahai()
		ctx := context.TODO()

		By("Creating Config")
		yamlTeam, err := ioutil.ReadFile(path.Join("..", "data", "wordpress-redis", "config.yaml"))
		Expect(err).NotTo(HaveOccurred())

		obj, _ := util.MustParseYAMLtoRuntimeObject(yamlTeam)
		config, _ := obj.(*s2hv1beta1.Config)
		Expect(client.Create(ctx, config)).To(BeNil())

		By("Creating Team")
		team := mockTeam
		Expect(client.Create(ctx, &team)).To(BeNil())

		By("Creating Config using template")
		yamlTeam2, err := ioutil.ReadFile(path.Join("..", "data", "template", "config.yaml"))
		Expect(err).NotTo(HaveOccurred())

		obj, _ = util.MustParseYAMLtoRuntimeObject(yamlTeam2)
		configUsingTemplate, _ := obj.(*s2hv1beta1.Config)
		Expect(client.Create(ctx, configUsingTemplate)).To(BeNil())

		By("Creating Team2")
		team2 := mockTeam2
		Expect(client.Create(ctx, &team2)).To(BeNil())

		By("Apply config template")
		Expect(configCtrl.EnsureConfigTemplateChanged(configUsingTemplate)).To(BeNil())
		Expect(configUsingTemplate.Status.Used).NotTo(BeNil())
		Expect(len(configUsingTemplate.Status.Used.Components)).To(Equal(2))
		Expect(len(configUsingTemplate.Status.Used.Envs)).To(Equal(4))
		Expect(configUsingTemplate.Status.Used.Staging).NotTo(BeNil())
		Expect(configUsingTemplate.Status.Used.ActivePromotion).NotTo(BeNil())
		mockEngine := "mock"
		Expect(configUsingTemplate.Status.Used.Staging.Deployment.Engine).To(Equal(&mockEngine))

		By("Update config template")
		config, err = configCtrl.Get(teamTest)
		Expect(err).NotTo(HaveOccurred())

		config.Spec.ActivePromotion.Deployment.Engine = &mockEngine
		Expect(configCtrl.Update(config)).To(BeNil())
		configUsingTemplate, err = configCtrl.Get(teamTest2)
		Expect(err).NotTo(HaveOccurred())
		Expect(configCtrl.EnsureConfigTemplateChanged(configUsingTemplate)).To(BeNil())
		Expect(configUsingTemplate.Status.Used.ActivePromotion.Deployment.Engine).To(Equal(&mockEngine))
	}, 10)

})

var (
	samsahaiAuthToken = "1234567890_"
	samsahaiConfig    = internal.SamsahaiConfig{
		ActivePromotion: internal.ActivePromotionConfig{
			Concurrences:          1,
			Timeout:               metav1.Duration{Duration: 5 * time.Minute},
			DemotionTimeout:       metav1.Duration{Duration: 1 * time.Second},
			RollbackTimeout:       metav1.Duration{Duration: 10 * time.Second},
			TearDownDuration:      metav1.Duration{Duration: 1 * time.Second},
			MaxHistories:          2,
			PromoteOnTeamCreation: false,
		},

		SamsahaiCredential: internal.SamsahaiCredential{
			InternalAuthToken: samsahaiAuthToken,
		},
	}
	teamTest  = "teamtest"
	teamTest2 = "teamtest2"
	testLabels = map[string]string{
		"created-for": "s2h-testing",
	}


	mockTeam = s2hv1beta1.Team{
		ObjectMeta: metav1.ObjectMeta{
			Name:   teamTest,
			Labels: testLabels,
		},
		Spec: s2hv1beta1.TeamSpec{
			Description: "team for testing",
			Owners:      []string{"samsahai@samsahai.io"},
			Credential: s2hv1beta1.Credential{
				SecretName: s2hobject.GetTeamSecretName(teamTest),
			},
			StagingCtrl: &s2hv1beta1.StagingCtrl{
				IsDeploy: false,
			},
		},
		Status: s2hv1beta1.TeamStatus{
			Namespace: s2hv1beta1.TeamNamespace{},
			Used: s2hv1beta1.TeamSpec{
				Description: "team for testing",
				Owners:      []string{"samsahai@samsahai.io"},
				Credential: s2hv1beta1.Credential{
					SecretName: s2hobject.GetTeamSecretName(teamTest),
				},
				StagingCtrl: &s2hv1beta1.StagingCtrl{
					IsDeploy: false,
				},
			},
		},
	}
	mockTeam2 = s2hv1beta1.Team{
		ObjectMeta: metav1.ObjectMeta{
			Name:   teamTest2,
			Labels: testLabels,
		},
		Spec: s2hv1beta1.TeamSpec{
			Description: "team for testing",
			Owners:      []string{"samsahai@samsahai.io"},
			Credential: s2hv1beta1.Credential{
				SecretName: s2hobject.GetTeamSecretName(teamTest2),
			},
			StagingCtrl: &s2hv1beta1.StagingCtrl{
				IsDeploy: false,
			},
		},
		Status: s2hv1beta1.TeamStatus{
			Namespace: s2hv1beta1.TeamNamespace{},
			Used: s2hv1beta1.TeamSpec{
				Description: "team for testing",
				Owners:      []string{"samsahai@samsahai.io"},
				Credential: s2hv1beta1.Credential{
					SecretName: s2hobject.GetTeamSecretName(teamTest2),
				},
				StagingCtrl: &s2hv1beta1.StagingCtrl{
					IsDeploy: false,
				},
			},
		},
	}
)
