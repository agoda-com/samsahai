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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/agoda-com/samsahai/internal"
	"github.com/agoda-com/samsahai/internal/staging/deploy/helm3"

	"github.com/agoda-com/samsahai/api/v1beta1"
	s2hv1beta1 "github.com/agoda-com/samsahai/api/v1beta1"
	configctrl "github.com/agoda-com/samsahai/internal/config"
	"github.com/agoda-com/samsahai/internal/k8s/helmrelease"
	s2hlog "github.com/agoda-com/samsahai/internal/log"
	"github.com/agoda-com/samsahai/internal/queue"
	"github.com/agoda-com/samsahai/internal/samsahai"
	"github.com/agoda-com/samsahai/internal/staging"
	samsahairpc "github.com/agoda-com/samsahai/pkg/samsahai/rpc"
)

var _ = Describe("Staging Controller [e2e]", func() {
	var (
		stagingCtrl   internal.StagingController
		queueCtrl     internal.QueueController
		namespace     string
		cfgCtrl       internal.ConfigController
		runtimeClient crclient.Client
		restCfg       *rest.Config
		hrClient      internal.HelmReleaseClient
		wgStop        *sync.WaitGroup
		chStop        chan struct{}
		mgr           manager.Manager
		err           error
	)

	logger := s2hlog.Log.WithName(fmt.Sprintf("%s-test", internal.StagingCtrlName))

	stableWordPress := v1beta1.StableComponent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "wordpress",
			Namespace: namespace,
		},
		Spec: v1beta1.StableComponentSpec{
			Name:       "wordpress",
			Version:    "5.2.2-debian-9-r2",
			Repository: "bitnami/wordpress",
		},
	}
	stableMariaDB := v1beta1.StableComponent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mariadb",
			Namespace: namespace,
		},
		Spec: v1beta1.StableComponentSpec{
			Name:       "mariadb",
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

	mockConfig := s2hv1beta1.Config{
		ObjectMeta: metav1.ObjectMeta{
			Name: teamName,
		},
		Spec: s2hv1beta1.ConfigSpec{
			Envs: map[s2hv1beta1.EnvType]s2hv1beta1.ChartValuesURLs{
				"staging": map[string][]string{
					"redis": {"https://raw.githubusercontent.com/agoda-com/samsahai/master/test/data/wordpress-redis/envs/staging/redis.yaml"},
				},
				"pre-active": map[string][]string{
					"redis": {"https://raw.githubusercontent.com/agoda-com/samsahai/master/test/data/wordpress-redis/envs/pre-active/redis.yaml"},
				},
				"active": map[string][]string{
					"redis": {"https://raw.githubusercontent.com/agoda-com/samsahai/master/test/data/wordpress-redis/envs/active/redis.yaml"},
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
			Components: []*s2hv1beta1.Component{&redisConfigComp},
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

		runtimeClient, err = crclient.New(restCfg, crclient.Options{Scheme: scheme.Scheme})
		Expect(err).NotTo(HaveOccurred(), "should create runtime client successfully")

		queueCtrl = queue.New(namespace, runtimeClient)
		Expect(queueCtrl).ToNot(BeNil())

		cfgCtrl = configctrl.New(mgr)
		Expect(cfgCtrl).ToNot(BeNil())

		hrClient = helmrelease.New(namespace, runtimeClient)
		Expect(hrClient).NotTo(BeNil())

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

		ctx := context.Background()

		By("Deleting nginx deployment")
		deploy := &deployNginx
		_ = runtimeClient.Delete(ctx, deploy)

		By("Deleting all teams")
		err = runtimeClient.DeleteAllOf(ctx, &s2hv1beta1.Team{}, crclient.MatchingLabels(testLabels))
		Expect(err).NotTo(HaveOccurred())
		err = wait.PollImmediate(1*time.Second, 10*time.Second, func() (ok bool, err error) {
			teamList := s2hv1beta1.TeamList{}
			listOpt := &crclient.ListOptions{LabelSelector: labels.SelectorFromSet(testLabels)}
			err = runtimeClient.List(ctx, &teamList, listOpt)
			if err != nil && errors.IsNotFound(err) {
				return true, nil
			}
			if len(teamList.Items) == 0 {
				return true, nil
			}

			return false, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Delete all teams error")

		By("Deleting all StableComponents")
		err = runtimeClient.DeleteAllOf(ctx, &s2hv1beta1.StableComponent{}, crclient.InNamespace(namespace))
		Expect(err).NotTo(HaveOccurred())

		By("Deleting all Queues")
		err = runtimeClient.DeleteAllOf(ctx, &s2hv1beta1.Queue{}, crclient.InNamespace(namespace))
		Expect(err).NotTo(HaveOccurred())

		By("Deleting all QueueHistories")
		err = runtimeClient.DeleteAllOf(ctx, &s2hv1beta1.QueueHistory{}, crclient.InNamespace(namespace))
		Expect(err).NotTo(HaveOccurred())

		ql := &v1beta1.QueueList{}
		err = runtimeClient.List(context.Background(), ql, &crclient.ListOptions{Namespace: namespace})
		Expect(err).NotTo(HaveOccurred())
		Expect(ql.Items).To(BeEmpty())

		sl := &v1beta1.StableComponentList{}
		err = runtimeClient.List(context.Background(), sl, &crclient.ListOptions{Namespace: namespace})
		Expect(err).NotTo(HaveOccurred())
		Expect(sl.Items).To(BeEmpty())

		By("Deleting all HelmReleases")
		err = hrClient.DeleteCollection(nil, metav1.ListOptions{})
		Expect(err).NotTo(HaveOccurred())

		By("Deleting all helm3 releases")
		err = helm3.DeleteAllReleases(namespace, true)
		Expect(err).NotTo(HaveOccurred())

		close(chStop)
		wgStop.Wait()
	}, 10)

	It("should successfully start and stop", func(done Done) {
		defer close(done)

		authToken := "12345"

		By("Creating Config")
		config := mockConfig
		ctx := context.Background()
		_ = runtimeClient.Delete(ctx, &config)
		Expect(runtimeClient.Create(ctx, &config)).To(BeNil())

		stagingCfgCtrl := configctrl.New(mgr)
		stagingCtrl = staging.NewController(teamName, namespace, authToken, nil, mgr, queueCtrl, stagingCfgCtrl,
			"", "", "", internal.StagingConfig{})

		go stagingCtrl.Start(chStop)

		cfg, err := cfgCtrl.Get(teamName)
		Expect(err).NotTo(HaveOccurred())

		deployTimeout := cfg.Spec.Staging.Deployment.Timeout.Duration
		testingTimeout := cfg.Spec.Staging.Deployment.TestRunner.Timeout

		swp := stableWordPress
		Expect(runtimeClient.Create(context.TODO(), &swp)).To(BeNil())

		By("creating queue")
		newQueue := queue.NewUpgradeQueue(teamName, namespace,
			"redis", "bitnami/redis", "5.0.5-debian-9-r160")
		Expect(queueCtrl.Add(newQueue)).To(BeNil())

		By("deploying")

		err = wait.PollImmediate(2*time.Second, deployTimeout, func() (ok bool, err error) {
			queue := &v1beta1.Queue{}
			err = runtimeClient.Get(context.TODO(), types.NamespacedName{Namespace: namespace, Name: newQueue.Name}, queue)
			if err != nil {
				return false, nil
			}
			if queue.Status.IsConditionTrue(v1beta1.QueueDeployStarted) {
				ok = true
				return
			}
			return
		})
		Expect(err).NotTo(HaveOccurred(), "Deploying error")

		By("testing")

		err = wait.PollImmediate(2*time.Second, testingTimeout.Duration, func() (ok bool, err error) {
			return !stagingCtrl.IsBusy(), nil
		})
		Expect(err).NotTo(HaveOccurred(), "Testing error")

		By("collecting")

		err = wait.PollImmediate(2*time.Second, 30*time.Second, func() (ok bool, err error) {
			stableComp := &v1beta1.StableComponent{}
			err = runtimeClient.Get(context.TODO(), types.NamespacedName{Namespace: namespace, Name: newQueue.Name}, stableComp)
			if err != nil {
				return false, nil
			}
			ok = true
			return
		})
		Expect(err).NotTo(HaveOccurred(), "Collecting error")

		By("Ensure Pre Active Components")

		redisServiceName := fmt.Sprintf("%s-redis-master", namespace)

		err = wait.PollImmediate(2*time.Second, deployTimeout, func() (ok bool, err error) {
			queue, err := queue.EnsurePreActiveComponents(runtimeClient, teamName, namespace)
			if err != nil {
				logger.Error(err, "cannot ensure pre-active components")
				return false, nil
			}

			if queue.Status.State != v1beta1.Finished {
				return
			}

			svc := corev1.Service{}
			err = runtimeClient.Get(context.TODO(), crclient.ObjectKey{Name: redisServiceName, Namespace: namespace}, &svc)
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

		q, err := queue.EnsurePreActiveComponents(runtimeClient, teamName, namespace)
		Expect(err).NotTo(HaveOccurred())
		Expect(q.IsDeploySuccess()).To(BeTrue())
		Expect(q.IsTestSuccess()).To(BeTrue())

		By("Delete Pre Active Queue")
		Expect(queue.DeletePreActiveQueue(runtimeClient, namespace))

		By("Demote from Active")

		err = wait.PollImmediate(2*time.Second, deployTimeout, func() (ok bool, err error) {
			queue, err := queue.EnsureDemoteFromActiveComponents(runtimeClient, teamName, namespace)
			if err != nil {
				logger.Error(err, "cannot ensure demote from active components")
				return false, nil
			}

			if queue.Status.State != v1beta1.Finished {
				return
			}

			ok = true
			return

		})
		Expect(err).NotTo(HaveOccurred(), "Demote from Active error")

		By("Delete Demote from Active Queue")
		Expect(queue.DeleteDemoteFromActiveQueue(runtimeClient, namespace))

		By("Promote to Active")

		err = wait.PollImmediate(2*time.Second, deployTimeout, func() (ok bool, err error) {
			queue, err := queue.EnsurePromoteToActiveComponents(runtimeClient, teamName, namespace)
			if err != nil {
				logger.Error(err, "cannot ensure promote to active components")
				return false, nil
			}

			if queue.Status.State != v1beta1.Finished {
				return
			}

			ok = true
			return

		})
		Expect(err).NotTo(HaveOccurred(), "Promote to Active error")

		By("Delete Promote to Active Queue")
		Expect(queue.DeletePromoteToActiveQueue(runtimeClient, namespace))

	}, 300)

	It("Should create error log in case of deploy failed", func(done Done) {
		defer close(done)

		By("Creating Config")
		config := mockConfig
		config.Spec.Staging.MaxRetry = 0
		config.Spec.Staging.Deployment.Timeout = metav1.Duration{Duration: 10 * time.Second}
		config.Spec.Components[0].Values["master"].(map[string]interface{})["command"] = "exit 1"
		ctx := context.Background()
		_ = runtimeClient.Delete(ctx, &config)
		Expect(runtimeClient.Create(ctx, &config)).To(BeNil())

		By("Creating Team")
		team := mockTeam
		Expect(runtimeClient.Create(context.TODO(), &team)).To(BeNil())

		authToken := "12345"
		s2hConfig := internal.SamsahaiConfig{SamsahaiCredential: internal.SamsahaiCredential{InternalAuthToken: authToken}}
		samsahaiCtrl := samsahai.New(mgr, namespace, s2hConfig, cfgCtrl,
			samsahai.WithClient(runtimeClient),
			samsahai.WithDisableLoaders(true, true, true))
		server := httptest.NewServer(samsahaiCtrl)
		defer server.Close()

		samsahaiClient := samsahairpc.NewRPCProtobufClient(server.URL, &http.Client{})

		stagingCfgCtrl := configctrl.New(mgr)
		stagingCtrl = staging.NewController(teamName, namespace, authToken, samsahaiClient, mgr, queueCtrl, stagingCfgCtrl,
			"", "", "", internal.StagingConfig{})
		go stagingCtrl.Start(chStop)

		redis := queue.NewUpgradeQueue(teamName, namespace, "redis", "bitnami/redis", "5.0.5-debian-9-r160")
		Expect(runtimeClient.Create(context.TODO(), redis)).To(BeNil())

		qhl := &v1beta1.QueueHistoryList{}
		err = wait.PollImmediate(1*time.Second, 60*time.Second, func() (ok bool, err error) {
			err = runtimeClient.List(context.TODO(), qhl, &crclient.ListOptions{})
			if err != nil || len(qhl.Items) < 1 {
				return false, nil
			}
			return true, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Create queue history error")

		Expect(qhl.Items[0].Spec.Queue.IsDeploySuccess()).To(BeFalse(), "Should deploy failed")
		Expect(qhl.Items[0].Spec.Queue.Status.KubeZipLog).NotTo(BeEmpty(), "KubeZipLog should not be empty")

		err = wait.PollImmediate(2*time.Second, 60*time.Second, func() (ok bool, err error) {
			q := &v1beta1.Queue{}
			err = runtimeClient.Get(context.TODO(), types.NamespacedName{Namespace: namespace, Name: "redis"}, q)
			if err != nil || q.Status.State != v1beta1.Waiting || q.Spec.Type != v1beta1.QueueTypeUpgrade {
				return false, nil
			}
			return true, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Should have waiting queue")
	}, 120)

	// TODO: disable by phantomnat
	XIt("should successfully clean all k8s resources in case of there are zombie pods", func(done Done) {
		defer close(done)

		authToken := "12345"

		stagingCfgCtrl := configctrl.New(mgr)
		stagingCtrl = staging.NewController(teamName, namespace, authToken, nil, mgr, queueCtrl, stagingCfgCtrl,
			"", "", "", internal.StagingConfig{})

		go stagingCtrl.Start(chStop)

		By("Creating Config")
		config := mockConfig
		ctx := context.Background()
		_ = runtimeClient.Delete(ctx, &config)
		Expect(runtimeClient.Create(ctx, &config)).To(BeNil())

		By("deploying nginx deployment, to pretend as zombie pod with same release name")
		// TODO: create pod instead of deployment
		nginx := deployNginx
		Expect(runtimeClient.Create(context.TODO(), &nginx)).To(BeNil())

		time.Sleep(5 * time.Second)

		By("creating queue")
		newQueue := queue.NewUpgradeQueue(teamName, namespace, "redis", "bitnami/redis", "5.0.5-debian-9-r160")
		Expect(queueCtrl.Add(newQueue)).To(BeNil())

		By("cleaning before")
		err = wait.PollImmediate(1*time.Second, 30*time.Second, func() (ok bool, err error) {
			queue := &v1beta1.Queue{}
			err = runtimeClient.Get(context.TODO(), types.NamespacedName{Namespace: namespace, Name: newQueue.Name}, queue)
			if err != nil {
				return false, nil
			}
			if queue.Status.IsConditionTrue(v1beta1.QueueCleanedBefore) {
				ok = true
				return
			}
			return
		})
		Expect(err).NotTo(HaveOccurred(), "Cleaning error")
	}, 60)
})
