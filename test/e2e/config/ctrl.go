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

	s2hv1 "github.com/agoda-com/samsahai/api/v1"
	"github.com/agoda-com/samsahai/internal"
	configctrl "github.com/agoda-com/samsahai/internal/config"
	"github.com/agoda-com/samsahai/internal/samsahai"
	s2hobject "github.com/agoda-com/samsahai/internal/samsahai/k8sobject"
	"github.com/agoda-com/samsahai/internal/util"
)

const (
	verifyTime1s  = 1 * time.Second
	verifyTime10s = 10 * time.Second
	verifyTime15s = 15 * time.Second
)

var (
	samsahaiCtrl internal.SamsahaiController
	configCtrl   internal.ConfigController
	wgStop       *sync.WaitGroup
	mgr          manager.Manager
	client       rclient.Client
	namespace    string
	cancel       context.CancelFunc
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
		Expect(mgr.Start(ctx)).To(BeNil())
	}()
}

var _ = Describe("[e2e] Config controller", func() {
	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.TODO())

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

	AfterEach(func() {
		By("Deleting all Teams")
		err := client.DeleteAllOf(ctx, &s2hv1.Team{}, rclient.MatchingLabels(testLabels))
		Expect(err).NotTo(HaveOccurred())
		err = wait.PollImmediate(verifyTime1s, verifyTime15s, func() (ok bool, err error) {
			teamList := s2hv1.TeamList{}
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

		By("Deleting all Configs")
		err = wait.PollImmediate(verifyTime1s, verifyTime10s, func() (ok bool, err error) {
			config1 := &s2hv1.Config{}
			err = client.Get(ctx, types.NamespacedName{Name: teamTest}, config1)
			if err != nil && !errors.IsNotFound(err) {
				return false, nil
			}

			_ = client.Delete(ctx, config1)

			config2 := &s2hv1.Config{}
			err = client.Get(ctx, types.NamespacedName{Name: teamTest2}, config2)
			if err != nil && errors.IsNotFound(err) {
				return true, nil
			}

			_ = client.Delete(ctx, config2)
			return false, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Delete all Configs error")

		cancel()
		wgStop.Wait()
	}, 30)

	It("should successfully get/delete Config", func() {
		setupSamsahai()

		By("Creating Config")
		yamlTeam, err := ioutil.ReadFile(path.Join("..", "data", "wordpress-redis", "config.yaml"))
		Expect(err).NotTo(HaveOccurred())

		obj, _ := util.MustParseYAMLtoRuntimeObject(yamlTeam)
		config, _ := obj.(*s2hv1.Config)
		Expect(client.Create(ctx, config)).To(BeNil())

		By("Get Config")
		err = wait.PollImmediate(1*time.Second, 5*time.Second, func() (ok bool, err error) {
			config = &s2hv1.Config{}
			err = client.Get(ctx, types.NamespacedName{Name: teamTest}, config)
			if err != nil {
				return false, nil
			}

			return true, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Get config error")

		cfg, err := configCtrl.Get(teamTest)
		Expect(err).To(BeNil())
		Expect(cfg.Status.Used).NotTo(BeNil())
		Expect(len(cfg.Spec.Components)).To(Equal(2))
		Expect(len(cfg.Spec.Envs)).To(Equal(4))
		Expect(cfg.Spec.Staging).NotTo(BeNil())
		Expect(cfg.Spec.ActivePromotion).NotTo(BeNil())

		By("Get components")
		err = wait.PollImmediate(1*time.Second, 5*time.Second, func() (ok bool, err error) {
			if err = configCtrl.EnsureConfigTemplateChanged(config); err != nil {
				return false, nil
			}
			if err = configCtrl.Update(config); err != nil {
				return false, nil
			}
			if comps, err := configCtrl.GetComponents(teamTest); err != nil || len(comps) != 3 {
				return false, nil
			}
			return true, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Get components error")

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
			config = &s2hv1.Config{}
			err = client.Get(ctx, types.NamespacedName{Name: teamTest}, config)
			if err != nil && errors.IsNotFound(err) {
				return true, nil
			}

			return false, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Delete config error")
	}, 10)

	It("Should successfully apply/update config template", func() {
		setupSamsahai()

		By("Creating Config")
		yamlTeam, err := ioutil.ReadFile(path.Join("..", "data", "wordpress-redis", "config.yaml"))
		Expect(err).NotTo(HaveOccurred())

		obj, _ := util.MustParseYAMLtoRuntimeObject(yamlTeam)
		config, _ := obj.(*s2hv1.Config)
		Expect(client.Create(ctx, config)).To(BeNil())

		By("Creating Team")
		team := mockTeam
		Expect(client.Create(ctx, &team)).To(BeNil())

		By("Creating Config using template")
		yamlTeam2, err := ioutil.ReadFile(path.Join("..", "data", "template", "config.yaml"))
		Expect(err).NotTo(HaveOccurred())

		obj, _ = util.MustParseYAMLtoRuntimeObject(yamlTeam2)
		configUsingTemplate, _ := obj.(*s2hv1.Config)
		Expect(client.Create(ctx, configUsingTemplate)).To(BeNil())

		By("Creating Team2")
		team2 := mockTeam2
		Expect(client.Create(ctx, &team2)).To(BeNil())

		By("Verifying config template updated")
		err = wait.PollImmediate(1*time.Second, 5*time.Second, func() (ok bool, err error) {
			configUsingTemplate = &s2hv1.Config{}
			err = client.Get(ctx, types.NamespacedName{Name: teamTest2}, configUsingTemplate)
			if err != nil {
				return false, nil
			}

			if len(configUsingTemplate.Status.Used.Components) > 0 {
				return true, nil
			}

			return false, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Verifying config template updated errors")

		configUsingTemplate = &s2hv1.Config{}
		err = client.Get(ctx, types.NamespacedName{Name: teamTest2}, configUsingTemplate)
		Expect(err).NotTo(HaveOccurred())
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
	ctx = context.TODO()

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
	teamTest   = "teamtest"
	teamTest2  = "teamtest2"
	testLabels = map[string]string{
		"created-for": "s2h-testing",
	}

	mockTeamSpec = s2hv1.TeamSpec{
		Description: "team for testing",
		Owners:      []string{"samsahai@samsahai.io"},
		Credential: s2hv1.Credential{
			SecretName: s2hobject.GetTeamSecretName(teamTest),
		},
		StagingCtrl: &s2hv1.StagingCtrl{
			IsDeploy: false,
		},
	}

	mockTeam = s2hv1.Team{
		ObjectMeta: metav1.ObjectMeta{
			Name:   teamTest,
			Labels: testLabels,
		},
		Spec: mockTeamSpec,
		Status: s2hv1.TeamStatus{
			Namespace: s2hv1.TeamNamespace{},
			Used:      mockTeamSpec,
		},
	}
	mockTeam2 = s2hv1.Team{
		ObjectMeta: metav1.ObjectMeta{
			Name:   teamTest2,
			Labels: testLabels,
		},
		Spec: mockTeamSpec,
		Status: s2hv1.TeamStatus{
			Namespace: s2hv1.TeamNamespace{},
			Used:      mockTeamSpec,
		},
	}
)
