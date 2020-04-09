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
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/scheme"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	rclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	s2hv1beta1 "github.com/agoda-com/samsahai/api/v1beta1"
	"github.com/agoda-com/samsahai/internal"
	configctrl "github.com/agoda-com/samsahai/internal/config"
	"github.com/agoda-com/samsahai/internal/util"
	"github.com/agoda-com/samsahai/internal/util/stringutils"
)

var _ = Describe("config controller [e2e]", func() {
	var (
		controller    internal.ConfigController
		client        rclient.Client
		chStop        chan struct{}
		mgr           manager.Manager
		wgStop        *sync.WaitGroup
		namespace     = os.Getenv("POD_NAMESPACE")
		teamName      = "teamtest"
		redisComp     = "redis"
		mariaComp     = "mariadb"
		wordpressComp = "wordpress"
		testLabels    = map[string]string{
			"created-for": "s2h-testing",
		}

		mockTeam = s2hv1beta1.Team{
			ObjectMeta: metav1.ObjectMeta{
				Name:   teamName,
				Labels: testLabels,
			},
			Status: s2hv1beta1.TeamStatus{
				Namespace: s2hv1beta1.TeamNamespace{},
				DesiredComponentImageCreatedTime: map[string]map[string]s2hv1beta1.DesiredImageTime{
					mariaComp: {
						stringutils.ConcatImageString("bitnami/mariadb", "10.3.18-debian-9-r30"): s2hv1beta1.DesiredImageTime{
							Image:       &s2hv1beta1.Image{Repository: "bitnami/mariadb", Tag: "10.3.18-debian-9-r30"},
							CreatedTime: metav1.Time{Time: time.Date(2019, 10, 1, 9, 0, 0, 0, time.UTC)},
						},
						stringutils.ConcatImageString("bitnami/mariadb", "10.3.18-debian-9-r32"): s2hv1beta1.DesiredImageTime{
							Image:       &s2hv1beta1.Image{Repository: "bitnami/mariadb", Tag: "10.3.18-debian-9-r32"},
							CreatedTime: metav1.Time{Time: time.Date(2019, 10, 5, 9, 0, 0, 0, time.UTC)},
						},
					},
					redisComp: {
						stringutils.ConcatImageString("bitnami/redis", "5.0.7-debian-9-r56"): s2hv1beta1.DesiredImageTime{
							Image:       &s2hv1beta1.Image{Repository: "bitnami/redis", Tag: "5.0.7-debian-9-r56"},
							CreatedTime: metav1.Time{Time: time.Date(2019, 10, 1, 9, 0, 0, 0, time.UTC)},
						},
					},
					wordpressComp: {
						stringutils.ConcatImageString("bitnami/wordpress", "5.2.4-debian-9-r18"): s2hv1beta1.DesiredImageTime{
							Image:       &s2hv1beta1.Image{Repository: "bitnami/wordpress", Tag: "5.2.4-debian-9-r18"},
							CreatedTime: metav1.Time{Time: time.Date(2019, 10, 1, 9, 0, 0, 0, time.UTC)},
						},
					},
				},
			},
		}

		mockDesiredCompList = &s2hv1beta1.DesiredComponentList{
			Items: []s2hv1beta1.DesiredComponent{
				{
					ObjectMeta: metav1.ObjectMeta{Name: mariaComp, Namespace: namespace},
					Spec:       s2hv1beta1.DesiredComponentSpec{Name: mariaComp, Repository: "bitnami/mariadb", Version: "10.3.18-debian-9-r32"},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: redisComp, Namespace: namespace},
					Spec:       s2hv1beta1.DesiredComponentSpec{Name: redisComp, Repository: "bitnami/redis", Version: "5.0.7-debian-9-r56"},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: wordpressComp, Namespace: namespace},
					Spec:       s2hv1beta1.DesiredComponentSpec{Name: wordpressComp, Repository: "bitnami/wordpress", Version: "5.2.4-debian-9-r18"},
				},
			},
		}

		mockQueueList = &s2hv1beta1.QueueList{
			Items: []s2hv1beta1.Queue{
				{
					ObjectMeta: metav1.ObjectMeta{Name: mariaComp, Namespace: namespace},
					Spec:       s2hv1beta1.QueueSpec{Name: mariaComp, Repository: "bitnami/mariadb", Version: "10.3.18-debian-9-r32"},
					Status:     s2hv1beta1.QueueStatus{},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: redisComp, Namespace: namespace},
					Spec:       s2hv1beta1.QueueSpec{Name: redisComp, Repository: "bitnami/redis", Version: "5.0.7-debian-9-r56"},
					Status:     s2hv1beta1.QueueStatus{},
				},
			},
		}

		mockStableCompList = &s2hv1beta1.StableComponentList{
			Items: []s2hv1beta1.StableComponent{
				{
					ObjectMeta: metav1.ObjectMeta{Name: mariaComp, Namespace: namespace},
					Spec:       s2hv1beta1.StableComponentSpec{Name: mariaComp, Repository: "bitnami/mariadb", Version: "10.3.18-debian-9-r32"},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: redisComp, Namespace: namespace},
					Spec:       s2hv1beta1.StableComponentSpec{Name: redisComp, Repository: "bitnami/redis", Version: "5.0.7-debian-9-r56"},
				},
			},
		}
	)

	BeforeEach(func(done Done) {
		defer GinkgoRecover()
		defer close(done)

		Expect(namespace).NotTo(BeEmpty(), "Please provided POD_NAMESPACE")

		chStop = make(chan struct{})
		cfg, err := config.GetConfig()
		Expect(err).To(BeNil(), "Please provide credential for accessing k8s cluster")

		mgr, err = manager.New(cfg, manager.Options{Namespace: namespace, MetricsBindAddress: "0"})
		Expect(err).NotTo(HaveOccurred(), "should create manager successfully")

		client, err = rclient.New(cfg, rclient.Options{Scheme: scheme.Scheme})
		Expect(err).NotTo(HaveOccurred())

		controller = configctrl.New(mgr, configctrl.WithClient(client))
		Expect(controller).NotTo(BeNil(), "Should successfully init Config controller")

		wgStop = &sync.WaitGroup{}
		wgStop.Add(1)
		go func() {
			defer GinkgoRecover()
			defer wgStop.Done()
			Expect(mgr.Start(chStop)).To(BeNil())
		}()
	}, 5)

	AfterEach(func(done Done) {
		defer close(done)
		ctx := context.TODO()

		By("Deleting all DesiredComponents")
		err := client.DeleteAllOf(ctx, &s2hv1beta1.DesiredComponent{}, crclient.InNamespace(namespace))
		Expect(err).NotTo(HaveOccurred())

		By("Deleting all Queues")
		err = client.DeleteAllOf(ctx, &s2hv1beta1.Queue{}, crclient.InNamespace(namespace))
		Expect(err).NotTo(HaveOccurred())

		By("Deleting all StableComponents")
		err = client.DeleteAllOf(ctx, &s2hv1beta1.StableComponent{}, crclient.InNamespace(namespace))
		Expect(err).NotTo(HaveOccurred())

		By("Deleting all Teams")
		err = client.DeleteAllOf(ctx, &s2hv1beta1.Team{}, crclient.MatchingLabels(testLabels))
		Expect(err).NotTo(HaveOccurred())
		err = wait.PollImmediate(1*time.Second, 10*time.Second, func() (ok bool, err error) {
			teamList := s2hv1beta1.TeamList{}
			listOpt := &crclient.ListOptions{LabelSelector: labels.SelectorFromSet(testLabels)}
			err = client.List(ctx, &teamList, listOpt)
			if err != nil && k8serrors.IsNotFound(err) {
				return true, nil
			}

			if len(teamList.Items) == 0 {
				return true, nil
			}

			return false, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Delete all Teams error")

		_ = controller.Delete(teamName)

		close(chStop)
		wgStop.Wait()
	}, 5)

	It("should successfully get/delete Config", func(done Done) {
		defer close(done)
		ctx := context.TODO()

		By("Creating Config")
		yamlTeam, err := ioutil.ReadFile(path.Join("..", "data", "wordpress-redis", "config.yaml"))
		Expect(err).NotTo(HaveOccurred())

		obj, _ := util.MustParseYAMLtoRuntimeObject(yamlTeam)
		config, _ := obj.(*s2hv1beta1.Config)
		Expect(client.Create(ctx, config)).To(BeNil())

		By("Get Config")
		cfg, err := controller.Get(teamName)
		Expect(err).To(BeNil())
		Expect(cfg.Spec).NotTo(BeNil())
		Expect(len(cfg.Spec.Components)).To(Equal(2))
		Expect(len(cfg.Spec.Envs)).To(Equal(4))
		Expect(cfg.Spec.Staging).NotTo(BeNil())
		Expect(cfg.Spec.ActivePromotion).NotTo(BeNil())

		By("Get components")
		comps, err := controller.GetComponents(teamName)
		Expect(err).To(BeNil())
		Expect(len(comps)).To(Equal(3))

		By("Get parent components")
		parentComps, err := controller.GetParentComponents(teamName)
		Expect(err).To(BeNil())
		Expect(len(parentComps)).To(Equal(2))

		By("Delete Config")
		_ = controller.Delete(teamName)

		config = &s2hv1beta1.Config{}
		err = client.Get(context.TODO(), types.NamespacedName{Name: teamName}, config)
		Expect(err).To(HaveOccurred())

	}, 10)

	It("should successfully detect changed components", func(done Done) {
		defer close(done)
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

		By("Creating DesiredComponents")
		for _, d := range mockDesiredCompList.Items {
			Expect(client.Create(ctx, &d)).To(BeNil())
		}

		By("Creating Queues")
		for _, q := range mockQueueList.Items {
			Expect(client.Create(ctx, &q)).To(BeNil())
		}

		By("Creating StableComponents")
		for _, s := range mockStableCompList.Items {
			Expect(client.Create(ctx, &s)).To(BeNil())
		}

		By("Updating components config")
		configComp := s2hv1beta1.Config{}
		Expect(client.Get(ctx, types.NamespacedName{Name: teamName}, &configComp)).To(BeNil())
		configComp.Spec.Components = []*s2hv1beta1.Component{{Name: redisComp}}
		Expect(client.Update(ctx, &configComp)).To(BeNil())

		time.Sleep(1 * time.Second)
		By("Checking DesiredComponents")
		dRedis := s2hv1beta1.DesiredComponent{}
		Expect(client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: redisComp}, &dRedis)).To(BeNil())
		dMaria := s2hv1beta1.DesiredComponent{}
		err = client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: mariaComp}, &dMaria)
		Expect(k8serrors.IsNotFound(err)).To(BeTrue())
		dWordpress := s2hv1beta1.DesiredComponent{}
		err = client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: wordpressComp}, &dWordpress)
		Expect(k8serrors.IsNotFound(err)).To(BeTrue())

		By("Checking TeamDesired")
		team = s2hv1beta1.Team{}
		Expect(client.Get(ctx, types.NamespacedName{Name: teamName}, &team)).To(BeNil())
		Expect(team.Status.DesiredComponentImageCreatedTime[redisComp]).ToNot(BeNil())
		Expect(team.Status.DesiredComponentImageCreatedTime[mariaComp]).To(BeNil())
		Expect(team.Status.DesiredComponentImageCreatedTime[wordpressComp]).To(BeNil())

		By("Checking Queues")
		qRedis := s2hv1beta1.Queue{}
		Expect(client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: redisComp}, &qRedis)).To(BeNil())
		qMaria := s2hv1beta1.Queue{}
		err = client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: mariaComp}, &qMaria)
		Expect(k8serrors.IsNotFound(err)).To(BeTrue())

		By("Checking StableComponents")
		sRedis := s2hv1beta1.StableComponent{}
		Expect(client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: redisComp}, &sRedis)).To(BeNil())
		sMaria := s2hv1beta1.StableComponent{}
		err = client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: mariaComp}, &sMaria)
		Expect(k8serrors.IsNotFound(err)).To(BeTrue())

	}, 10)
})
