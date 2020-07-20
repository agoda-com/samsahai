package staging

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tidwall/gjson"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
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
	s2hlog "github.com/agoda-com/samsahai/internal/log"
	"github.com/agoda-com/samsahai/internal/queue"
	"github.com/agoda-com/samsahai/internal/samsahai"
	"github.com/agoda-com/samsahai/internal/staging"
	"github.com/agoda-com/samsahai/internal/staging/deploy/helm3"
	httputil "github.com/agoda-com/samsahai/internal/util/http"
	samsahairpc "github.com/agoda-com/samsahai/pkg/samsahai/rpc"
)

var _ = Describe("[e2e] Staging controller", func() {
	const (
		verifyTime1s  = 1 * time.Second
		verifyTime10s = 10 * time.Second
	)

	var (
		stagingCtrl internal.StagingController
		queueCtrl   internal.QueueController
		namespace   string
		cfgCtrl     internal.ConfigController
		client      rclient.Client
		restCfg     *rest.Config
		wgStop      *sync.WaitGroup
		chStop      chan struct{}
		mgr         manager.Manager
		err         error
	)

	logger := s2hlog.Log.WithName(fmt.Sprintf("%s-test", internal.StagingCtrlName))

	redisCompName := "redis"
	mariaDBCompName := "mariadb"
	wordpressCompName := "wordpress"

	stableWordPress := s2hv1beta1.StableComponent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      wordpressCompName,
			Namespace: namespace,
		},
		Spec: s2hv1beta1.StableComponentSpec{
			Name:       wordpressCompName,
			Version:    "5.3.2-debian-10-r32",
			Repository: "bitnami/wordpress",
		},
	}
	stableMariaDB := s2hv1beta1.StableComponent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mariaDBCompName,
			Namespace: namespace,
		},
		Spec: s2hv1beta1.StableComponentSpec{
			Name:       mariaDBCompName,
			Version:    "10.3.16-debian-9-r9",
			Repository: "bitnami/mariadb",
		},
	}

	nginxReplicas := int32(1)
	nginxLabels := map[string]string{"app": "nginx", "release": "samsahai-system-redis"}
	deployNginx := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nginx",
			Namespace: namespace,
			Labels:    nginxLabels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &nginxReplicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: nginxLabels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: nginxLabels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "nginx",
							Image: "nginx:stable-alpine",
							Ports: []corev1.ContainerPort{{ContainerPort: 80}},
						},
					},
				},
			},
		},
	}

	namespace = os.Getenv("POD_NAMESPACE")
	testLabels := map[string]string{
		"created-for": "s2h-testing",
	}
	teamName := "teamtest"
	mockTeam := s2hv1beta1.Team{
		ObjectMeta: metav1.ObjectMeta{
			Name:   teamName,
			Labels: testLabels,
		},
		Spec: s2hv1beta1.TeamSpec{
			Description: "team for testing",
			Owners:      []string{"samsahai@samsahai.io"},
			StagingCtrl: &s2hv1beta1.StagingCtrl{
				IsDeploy: false,
			},
		},
		Status: s2hv1beta1.TeamStatus{
			Namespace: s2hv1beta1.TeamNamespace{
				Staging: "s2h-teamtest",
			},
		},
	}

	engine := "helm3"
	deployConfig := s2hv1beta1.ConfigDeploy{
		Timeout:                 metav1.Duration{Duration: 5 * time.Minute},
		ComponentCleanupTimeout: metav1.Duration{Duration: 2 * time.Second},
		Engine:                  &engine,
		TestRunner: &s2hv1beta1.ConfigTestRunner{
			TestMock: &s2hv1beta1.ConfigTestMock{
				Result: true,
			},
		},
	}
	compSource := s2hv1beta1.UpdatingSource("public-registry")
	configCompRedis := s2hv1beta1.Component{
		Name: redisCompName,
		Chart: s2hv1beta1.ComponentChart{
			Repository: "https://kubernetes-charts.storage.googleapis.com",
			Name:       redisCompName,
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

	configCompWordpress := s2hv1beta1.Component{
		Name: wordpressCompName,
		Chart: s2hv1beta1.ComponentChart{
			Repository: "https://kubernetes-charts.storage.googleapis.com",
			Name:       wordpressCompName,
		},
		Image: s2hv1beta1.ComponentImage{
			Repository: "bitnami/wordpress",
			Pattern:    "5\\.2.*debian-9.*",
		},
		Source: &compSource,
		Dependencies: []*s2hv1beta1.Component{
			{
				Name: mariaDBCompName,
				Image: s2hv1beta1.ComponentImage{
					Repository: "bitnami/mariadb",
					Pattern:    "10\\.3.*debian-9.*",
				},
			},
		},
		Values: s2hv1beta1.ComponentValues{
			"resources": nil,
			"service": map[string]interface{}{
				"type": "NodePort",
			},
			"persistence": map[string]interface{}{
				"enabled": false,
			},
			"mariadb": map[string]interface{}{
				"enabled": true,
				"replication": map[string]interface{}{
					"enabled": false,
				},
				"master": map[string]interface{}{
					"persistence": map[string]interface{}{
						"enabled": false,
					},
				},
			},
		},
	}

	bundleName := "db"
	mockConfig := s2hv1beta1.Config{
		ObjectMeta: metav1.ObjectMeta{
			Name:   teamName,
			Labels: testLabels,
		},
		Spec: s2hv1beta1.ConfigSpec{
			Envs: map[s2hv1beta1.EnvType]s2hv1beta1.ChartValuesURLs{
				"base": map[string][]string{
					wordpressCompName: {"https://raw.githubusercontent.com/agoda-com/samsahai-example/master/envs/base/wordpress.yaml"},
				},
				"staging": map[string][]string{
					redisCompName: {"https://raw.githubusercontent.com/agoda-com/samsahai/master/test/data/wordpress-redis/envs/staging/redis.yaml"},
				},
				"pre-active": map[string][]string{
					redisCompName: {"https://raw.githubusercontent.com/agoda-com/samsahai/master/test/data/wordpress-redis/envs/pre-active/redis.yaml"},
				},
				"active": map[string][]string{
					redisCompName: {"https://raw.githubusercontent.com/agoda-com/samsahai/master/test/data/wordpress-redis/envs/active/redis.yaml"},
				},
			},
			Staging: &s2hv1beta1.ConfigStaging{
				Deployment: &deployConfig,
			},
			ActivePromotion: &s2hv1beta1.ConfigActivePromotion{
				Timeout:          metav1.Duration{Duration: 5 * time.Minute},
				TearDownDuration: metav1.Duration{Duration: 10 * time.Second},
				Deployment:       &deployConfig,
			},
			Reporter: &s2hv1beta1.ConfigReporter{
				ReportMock: true,
			},
			Bundles: s2hv1beta1.ConfigBundles{
				bundleName: []string{redisCompName, mariaDBCompName},
			},
			PriorityQueues: []string{wordpressCompName, redisCompName},
			Components:     []*s2hv1beta1.Component{&configCompRedis, &configCompWordpress},
		},
	}

	BeforeEach(func(done Done) {
		defer GinkgoRecover()
		defer close(done)
		var err error

		namespace = os.Getenv("POD_NAMESPACE")
		Expect(namespace).NotTo(BeEmpty(), "POD_NAMESPACE should be provided")
		stableWordPress.ObjectMeta.Namespace = namespace
		stableMariaDB.ObjectMeta.Namespace = namespace
		deployNginx.ObjectMeta.Namespace = namespace

		chStop = make(chan struct{})
		restCfg, err = config.GetConfig()
		Expect(err).NotTo(HaveOccurred(), "Please provide credential for accessing k8s cluster")

		mgr, err = manager.New(restCfg, manager.Options{Namespace: namespace, MetricsBindAddress: "0"})
		Expect(err).NotTo(HaveOccurred(), "should create manager successfully")

		client, err = rclient.New(restCfg, rclient.Options{Scheme: scheme.Scheme})
		Expect(err).NotTo(HaveOccurred(), "should create runtime client successfully")

		queueCtrl = queue.New(namespace, client)
		Expect(queueCtrl).ToNot(BeNil())

		cfgCtrl = configctrl.New(mgr)
		Expect(cfgCtrl).ToNot(BeNil())

		wgStop = &sync.WaitGroup{}
		wgStop.Add(1)
		go func() {
			defer GinkgoRecover()
			defer wgStop.Done()
			Expect(mgr.Start(chStop)).To(BeNil())
		}()
	}, 10)

	AfterEach(func(done Done) {
		defer close(done)

		By("Deleting nginx deployment")
		deploy := &deployNginx
		ctx := context.Background()
		_ = client.Delete(ctx, deploy)

		By("Deleting all teams")
		err = client.DeleteAllOf(ctx, &s2hv1beta1.Team{}, rclient.MatchingLabels(testLabels))
		Expect(err).NotTo(HaveOccurred())
		err = wait.PollImmediate(verifyTime1s, verifyTime10s, func() (ok bool, err error) {
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
		Expect(err).NotTo(HaveOccurred(), "Delete all teams error")

		By("Deleting all Configs")
		err = client.DeleteAllOf(ctx, &s2hv1beta1.Config{}, rclient.MatchingLabels(testLabels))
		Expect(err).NotTo(HaveOccurred())
		err = wait.PollImmediate(verifyTime1s, verifyTime10s, func() (ok bool, err error) {
			configList := s2hv1beta1.ConfigList{}
			listOpt := &rclient.ListOptions{LabelSelector: labels.SelectorFromSet(testLabels)}
			err = client.List(ctx, &configList, listOpt)
			if err != nil && errors.IsNotFound(err) {
				return true, nil
			}
			if len(configList.Items) == 0 {
				return true, nil
			}

			return false, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Deleting all configs error")

		By("Deleting all StableComponents")
		err = client.DeleteAllOf(ctx, &s2hv1beta1.StableComponent{}, rclient.InNamespace(namespace))
		Expect(err).NotTo(HaveOccurred())

		By("Deleting all Queues")
		err = client.DeleteAllOf(ctx, &s2hv1beta1.Queue{}, rclient.InNamespace(namespace))
		Expect(err).NotTo(HaveOccurred())

		By("Deleting all QueueHistories")
		err = client.DeleteAllOf(ctx, &s2hv1beta1.QueueHistory{}, rclient.InNamespace(namespace))
		Expect(err).NotTo(HaveOccurred())

		ql := &s2hv1beta1.QueueList{}
		err = client.List(context.Background(), ql, &rclient.ListOptions{Namespace: namespace})
		Expect(err).NotTo(HaveOccurred())
		Expect(ql.Items).To(BeEmpty())

		sl := &s2hv1beta1.StableComponentList{}
		err = client.List(context.Background(), sl, &rclient.ListOptions{Namespace: namespace})
		Expect(err).NotTo(HaveOccurred())
		Expect(sl.Items).To(BeEmpty())

		By("Deleting all helm3 releases")
		err = helm3.DeleteAllReleases(namespace, true)
		Expect(err).NotTo(HaveOccurred())

		close(chStop)
		wgStop.Wait()
	}, 10)

	It("should successfully start and stop", func(done Done) {
		defer close(done)
		ctx := context.Background()

		By("Creating Config")
		config := mockConfig
		Expect(client.Create(ctx, &config)).To(BeNil())

		By("Verifying config has been created")
		err = wait.PollImmediate(verifyTime1s, verifyTime10s, func() (ok bool, err error) {
			config := &s2hv1beta1.Config{}
			err = client.Get(ctx, types.NamespacedName{Name: teamName}, config)
			if err != nil {
				return false, nil
			}

			return true, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Verify config error")

		authToken := "12345"
		stagingCfgCtrl := configctrl.New(mgr)
		stagingCtrl = staging.NewController(teamName, namespace, authToken, nil, mgr, queueCtrl, stagingCfgCtrl,
			"", "", "", internal.StagingConfig{})

		go stagingCtrl.Start(chStop)

		cfg, err := cfgCtrl.Get(teamName)
		Expect(err).NotTo(HaveOccurred())

		deployTimeout := cfg.Spec.Staging.Deployment.Timeout.Duration
		testingTimeout := cfg.Spec.Staging.Deployment.TestRunner.Timeout

		swp := stableWordPress
		Expect(client.Create(context.TODO(), &swp)).To(BeNil())

		By("Creating 2 Queue")
		redisQueue := queue.NewUpgradeQueue(teamName, namespace, bundleName, bundleName,
			s2hv1beta1.QueueComponents{{Name: redisCompName, Repository: "bitnami/redis", Version: "5.0.5-debian-9-r160"}},
		)
		mariaDBQueue := queue.NewUpgradeQueue(teamName, namespace, bundleName, bundleName,
			s2hv1beta1.QueueComponents{{Name: mariaDBCompName, Repository: "bitnami/mariadb", Version: "10.3.18-debian-9-r32"}},
		)
		Expect(queueCtrl.Add(redisQueue, nil)).To(BeNil())
		Expect(queueCtrl.Add(mariaDBQueue, nil)).To(BeNil())

		By("Deploying")
		err = wait.PollImmediate(2*time.Second, deployTimeout, func() (ok bool, err error) {
			queue := &s2hv1beta1.Queue{}
			// bundle queue
			err = client.Get(context.TODO(), types.NamespacedName{Namespace: namespace, Name: redisQueue.Name}, queue)
			if err != nil {
				return false, nil
			}

			if queue.Status.IsConditionTrue(s2hv1beta1.QueueDeployStarted) {
				ok = true
				return
			}
			return
		})
		Expect(err).NotTo(HaveOccurred(), "Deploying error")

		By("Testing")
		err = wait.PollImmediate(2*time.Second, testingTimeout.Duration, func() (ok bool, err error) {
			return !stagingCtrl.IsBusy(), nil
		})
		Expect(err).NotTo(HaveOccurred(), "Testing error")

		By("Collecting")
		err = wait.PollImmediate(2*time.Second, 60*time.Second, func() (ok bool, err error) {
			redisStableComp := &s2hv1beta1.StableComponent{}
			err = client.Get(context.TODO(), types.NamespacedName{Namespace: namespace, Name: redisCompName},
				redisStableComp)
			if err != nil {
				return false, nil
			}

			mariaDBStableComp := &s2hv1beta1.StableComponent{}
			err = client.Get(context.TODO(), types.NamespacedName{Namespace: namespace, Name: mariaDBCompName},
				mariaDBStableComp)
			if err != nil {
				return false, nil
			}

			ok = true
			return
		})
		Expect(err).NotTo(HaveOccurred(), "Collecting error")

		By("Updating Config to deploy only one component")
		config = s2hv1beta1.Config{}
		err = client.Get(context.TODO(), types.NamespacedName{Namespace: namespace, Name: teamName}, &config)
		config.Spec.Components = []*s2hv1beta1.Component{&configCompRedis}
		Expect(client.Update(context.TODO(), &config)).To(BeNil())

		By("Ensure Pre Active Components")
		redisServiceName := fmt.Sprintf("%s-redis-master", namespace)
		skipTest := true

		err = wait.PollImmediate(2*time.Second, deployTimeout, func() (ok bool, err error) {
			queue, err := queue.EnsurePreActiveComponents(client, teamName, namespace, skipTest)
			if err != nil {
				logger.Error(err, "cannot ensure pre-active components")
				return false, nil
			}

			if queue.Status.State != s2hv1beta1.Finished {
				return
			}

			svc := corev1.Service{}
			err = client.Get(context.TODO(), rclient.ObjectKey{Name: redisServiceName, Namespace: namespace}, &svc)
			if err != nil {
				return
			}

			for _, p := range svc.Spec.Ports {
				if p.NodePort == 31002 {
					ok = true
					return
				}
			}

			return
		})
		Expect(err).NotTo(HaveOccurred(), "Ensure Pre Active error")

		q, err := queue.EnsurePreActiveComponents(client, teamName, namespace, skipTest)
		Expect(err).NotTo(HaveOccurred())
		Expect(q.IsDeploySuccess()).To(BeTrue())
		Expect(q.IsTestSuccess()).To(BeTrue())

		By("Delete Pre Active Queue")
		Expect(queue.DeletePreActiveQueue(client, namespace))

		By("Demote from Active")
		err = wait.PollImmediate(2*time.Second, deployTimeout, func() (ok bool, err error) {
			queue, err := queue.EnsureDemoteFromActiveComponents(client, teamName, namespace)
			if err != nil {
				logger.Error(err, "cannot ensure demote from active components")
				return false, nil
			}

			if queue.Status.State != s2hv1beta1.Finished {
				return
			}

			ok = true
			return

		})
		Expect(err).NotTo(HaveOccurred(), "Demote from Active error")

		By("Delete Demote from Active Queue")
		Expect(queue.DeleteDemoteFromActiveQueue(client, namespace))

		By("Promote to Active")
		err = wait.PollImmediate(2*time.Second, deployTimeout, func() (ok bool, err error) {
			queue, err := queue.EnsurePromoteToActiveComponents(client, teamName, namespace)
			if err != nil {
				logger.Error(err, "cannot ensure promote to active components")
				return false, nil
			}

			if queue.Status.State != s2hv1beta1.Finished {
				return
			}

			ok = true
			return

		})
		Expect(err).NotTo(HaveOccurred(), "Promote to Active error")

		By("Delete Promote to Active Queue")
		Expect(queue.DeletePromoteToActiveQueue(client, namespace))

	}, 300)

	It("should create error log in case of deploy failed", func(done Done) {
		defer close(done)
		ctx := context.Background()

		By("Creating Config")
		config := mockConfig
		config.Spec.Staging.MaxRetry = 0
		config.Spec.Staging.Deployment.Timeout = metav1.Duration{Duration: 10 * time.Second}
		config.Spec.Components[0].Values["master"].(map[string]interface{})["command"] = "exit 1"
		Expect(client.Create(ctx, &config)).To(BeNil())

		By("Creating Team")
		team := mockTeam
		Expect(client.Create(context.TODO(), &team)).To(BeNil())

		authToken := "12345"
		s2hConfig := internal.SamsahaiConfig{SamsahaiCredential: internal.SamsahaiCredential{InternalAuthToken: authToken}}
		samsahaiCtrl := samsahai.New(mgr, namespace, s2hConfig,
			samsahai.WithClient(client),
			samsahai.WithDisableLoaders(true, true, true))
		server := httptest.NewServer(samsahaiCtrl)
		defer server.Close()

		samsahaiClient := samsahairpc.NewRPCProtobufClient(server.URL, &http.Client{})

		stagingCfgCtrl := configctrl.New(mgr)
		stagingCtrl = staging.NewController(teamName, namespace, authToken, samsahaiClient, mgr, queueCtrl, stagingCfgCtrl,
			"", "", "", internal.StagingConfig{})
		go stagingCtrl.Start(chStop)

		redis := queue.NewUpgradeQueue(teamName, namespace, redisCompName, "",
			s2hv1beta1.QueueComponents{{Name: redisCompName, Repository: "bitnami/redis", Version: "5.0.5-debian-9-r160"}},
		)
		Expect(client.Create(context.TODO(), redis)).To(BeNil())

		qhl := &s2hv1beta1.QueueHistoryList{}
		err = wait.PollImmediate(1*time.Second, 120*time.Second, func() (ok bool, err error) {
			err = client.List(context.TODO(), qhl, &rclient.ListOptions{})
			if err != nil || len(qhl.Items) < 1 {
				return false, nil
			}
			return true, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Create queue history error")

		Expect(qhl.Items[0].Spec.Queue.IsDeploySuccess()).To(BeFalse(), "Should deploy failed")
		Expect(qhl.Items[0].Spec.Queue.Status.KubeZipLog).NotTo(BeEmpty(), "KubeZipLog should not be empty")

		err = wait.PollImmediate(2*time.Second, 60*time.Second, func() (ok bool, err error) {
			q := &s2hv1beta1.Queue{}
			err = client.Get(context.TODO(), types.NamespacedName{Namespace: namespace, Name: redisCompName}, q)
			if err != nil || q.Status.State != s2hv1beta1.Waiting || q.Spec.Type != s2hv1beta1.QueueTypeUpgrade {
				return false, nil
			}
			return true, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Should have waiting queue")
	}, 120)

	It("should successfully get health check", func(done Done) {
		defer close(done)

		stagingCtrl = staging.NewController(teamName, namespace, "", nil, mgr, queueCtrl,
			nil, "", "", "", internal.StagingConfig{})

		server := httptest.NewServer(stagingCtrl)
		defer server.Close()

		_, data, err := httputil.Get(server.URL + internal.URIHealthz)
		Expect(err).NotTo(HaveOccurred())
		Expect(data).NotTo(BeEmpty())
		Expect(gjson.ValidBytes(data)).To(BeTrue())

	}, 5)
})
