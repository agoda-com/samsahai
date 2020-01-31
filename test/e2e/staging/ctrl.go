package staging

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"sync"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/twitchtv/twirp"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/agoda-com/samsahai/internal"
	s2hconfig "github.com/agoda-com/samsahai/internal/config"
	s2herrors "github.com/agoda-com/samsahai/internal/errors"
	"github.com/agoda-com/samsahai/internal/staging/deploy/helm3"

	"github.com/agoda-com/samsahai/api/v1beta1"
	s2hv1beta1 "github.com/agoda-com/samsahai/api/v1beta1"
	"github.com/agoda-com/samsahai/internal/k8s/helmrelease"
	s2hlog "github.com/agoda-com/samsahai/internal/log"
	"github.com/agoda-com/samsahai/internal/queue"
	"github.com/agoda-com/samsahai/internal/samsahai"
	"github.com/agoda-com/samsahai/internal/staging"
	"github.com/agoda-com/samsahai/internal/staging/deploy/mock"
	samsahairpc "github.com/agoda-com/samsahai/pkg/samsahai/rpc"
	stagingrpc "github.com/agoda-com/samsahai/pkg/staging/rpc"
)

var _ = Describe("Staging Controller [e2e]", func() {
	var stagingCtrl internal.StagingController
	var queueCtrl internal.QueueController
	var namespace string
	var configMgr internal.ConfigManager
	var runtimeClient crclient.Client
	var restCfg *rest.Config
	var hrClient internal.HelmReleaseClient
	var wgStop *sync.WaitGroup
	var chStop chan struct{}
	var mgr manager.Manager
	var err error
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
	nginxLabels := map[string]string{"app": "nginx", "release": "teamtest-samsahai-system-redis"}
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

		By("Deleting mock team")
		t := &v1beta1.Team{}
		_ = runtimeClient.Get(ctx, crclient.ObjectKey{Name: teamName}, t)
		_ = runtimeClient.Delete(ctx, t)

		By("Deleting nginx deployment")
		deploy := &deployNginx
		_ = runtimeClient.Delete(ctx, deploy)

		By("Deleting all teams")
		err = runtimeClient.DeleteAllOf(ctx, &s2hv1beta1.Team{}, crclient.MatchingLabels(testLabels))
		Expect(err).NotTo(HaveOccurred())

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
		configMgr, err = s2hconfig.NewWithGitClient(nil, teamName, path.Join("..", "data", "redis"))

		stagingCtrl = staging.NewController(teamName, namespace, authToken, nil, mgr, queueCtrl, configMgr,
			"", "", "")

		go stagingCtrl.Start(chStop)

		cfg := configMgr.Get()
		deployTimeout := cfg.Staging.Deployment.Timeout.Duration
		testingTimeout := cfg.Staging.Deployment.TestRunner.Timeout

		swp := stableWordPress
		Expect(runtimeClient.Create(context.TODO(), &swp)).To(BeNil())

		By("creating queue")
		newQueue := queue.NewUpgradeQueue(teamName, namespace, "redis", "bitnami/redis", "5.0.5-debian-9-r160")
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

		redisServiceName := fmt.Sprintf("%s-%s-redis-master", teamName, namespace)

		err = wait.PollImmediate(2*time.Second, 40*time.Second, func() (ok bool, err error) {
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

		err = wait.PollImmediate(2*time.Second, 30*time.Second, func() (ok bool, err error) {
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

		err = wait.PollImmediate(2*time.Second, 30*time.Second, func() (ok bool, err error) {
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
		By("Creating Team")
		team := mockTeam
		Expect(runtimeClient.Create(context.TODO(), &team)).To(BeNil())

		authToken := "12345"

		configMgr, err = s2hconfig.NewWithGitClient(nil, teamName, path.Join("..", "data", "failed-fast"))

		s2hConfig := internal.SamsahaiConfig{SamsahaiCredential: internal.SamsahaiCredential{InternalAuthToken: authToken}}
		samsahaiCtrl := samsahai.New(nil, namespace, s2hConfig,
			samsahai.WithClient(runtimeClient),
			samsahai.WithConfigManager(teamName, configMgr),
			samsahai.WithDisableLoaders(true, true, true))
		server := httptest.NewServer(samsahaiCtrl)
		defer server.Close()

		samsahaiClient := samsahairpc.NewRPCProtobufClient(server.URL, &http.Client{})

		stagingCtrl = staging.NewController(teamName, namespace, authToken, samsahaiClient, mgr, queueCtrl, configMgr, "", "", "")
		go stagingCtrl.Start(chStop)

		redis := queue.NewUpgradeQueue(teamName, namespace, "redis", "bitnami/redis", "5.0.5-debian-9-r160")
		Expect(runtimeClient.Create(context.TODO(), redis)).To(BeNil())

		qhl := &v1beta1.QueueHistoryList{}
		err = wait.PollImmediate(1*time.Second, 30*time.Second, func() (ok bool, err error) {
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
	}, 90)

	// TODO: disable by phantomnat
	XIt("should successfully clean all k8s resources in case of there are zombie pods", func(done Done) {
		defer close(done)

		authToken := "12345"
		configMgr, err = s2hconfig.NewWithGitClient(nil, teamName, path.Join("..", "data", "redis"))

		stagingCtrl = staging.NewController(teamName, namespace, authToken, nil, mgr, queueCtrl, configMgr,
			"", "", "")

		go stagingCtrl.Start(chStop)

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

	Describe("gRPC", func() {
		It("Should error if client not specify the authentication", func(done Done) {
			defer close(done)

			authToken := "12345"
			configMgr, err = s2hconfig.NewWithGitClient(nil, teamName, path.Join("..", "data", "redis"))
			s2hConfig := internal.SamsahaiConfig{SamsahaiCredential: internal.SamsahaiCredential{InternalAuthToken: authToken}}
			samsahaiCtrl := samsahai.New(nil, namespace, s2hConfig,
				samsahai.WithClient(runtimeClient),
				samsahai.WithConfigManager(teamName, configMgr),
				samsahai.WithDisableLoaders(true, true, true))
			samsahaiServer := httptest.NewServer(samsahaiCtrl)
			defer samsahaiServer.Close()

			samsahaiClient := samsahairpc.NewRPCProtobufClient(samsahaiServer.URL, &http.Client{})
			configMgr, err = s2hconfig.NewWithSamsahaiClient(samsahaiClient, teamName, authToken)
			Expect(err).ToNot(HaveOccurred())

			stagingCtrl = staging.NewController(teamName, namespace, authToken, samsahaiClient, mgr, queueCtrl, configMgr, "", "", "")
			server := httptest.NewServer(stagingCtrl)
			defer server.Close()

			rpcClient := stagingrpc.NewRPCProtobufClient(server.URL, &http.Client{})

			go stagingCtrl.Start(chStop)

			cfg := configMgr.Get()
			data, err := json.Marshal(cfg)
			Expect(err).NotTo(HaveOccurred())
			_, err = rpcClient.UpdateConfiguration(context.TODO(), &stagingrpc.Configuration{
				GitRevision: "abc123",
				Config:      data,
			})
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(Equal(twirp.InternalErrorWith(s2herrors.ErrUnauthorized).Error()))

		}, 30)

		It("Should success communicate with gRPC", func(done Done) {
			defer close(done)

			authToken := "12345"
			configMgr, err = s2hconfig.NewWithGitClient(nil, teamName, path.Join("..", "data", "redis"))
			s2hConfig := internal.SamsahaiConfig{SamsahaiCredential: internal.SamsahaiCredential{InternalAuthToken: authToken}}
			samsahaiCtrl := samsahai.New(nil, namespace, s2hConfig,
				samsahai.WithClient(runtimeClient),
				samsahai.WithConfigManager(teamName, configMgr),
				samsahai.WithDisableLoaders(true, true, true))
			samsahaiServer := httptest.NewServer(samsahaiCtrl)
			defer samsahaiServer.Close()

			samsahaiClient := samsahairpc.NewRPCProtobufClient(samsahaiServer.URL, &http.Client{})
			configMgr, err = s2hconfig.NewWithSamsahaiClient(samsahaiClient, teamName, authToken)
			Expect(err).ToNot(HaveOccurred())

			stagingCtrl = staging.NewController(teamName, namespace, authToken, samsahaiClient, mgr, queueCtrl, configMgr, "", "", "")
			server := httptest.NewServer(stagingCtrl)
			defer server.Close()

			rpcClient := stagingrpc.NewRPCProtobufClient(server.URL, &http.Client{})

			go stagingCtrl.Start(chStop)

			By("specify auth header")
			headers := make(http.Header)
			headers.Set(internal.SamsahaiAuthHeader, authToken)
			ctx := context.TODO()
			ctx, err = twirp.WithHTTPRequestHeaders(ctx, headers)
			Expect(err).ToNot(HaveOccurred())

			cfg := configMgr.Get()
			data, err := json.Marshal(cfg)
			Expect(err).NotTo(HaveOccurred())
			_, err = rpcClient.UpdateConfiguration(ctx, &stagingrpc.Configuration{
				GitRevision: "abc123",
				Config:      data,
			})
			Expect(err).NotTo(HaveOccurred())
		}, 30)
	})

	XDescribe("Failure Testing", func() {
		testFile := "../data/failed.yaml"
		mariadbq := queue.NewUpgradeQueue(teamName, namespace, "mariadb", "bitnami/mariadb", "10.3.16-debian-9-r23")

		BeforeEach(func(done Done) {
			defer close(done)

			pwd, err := os.Getwd()
			Expect(err).NotTo(HaveOccurred())
			filepath := path.Join(pwd, testFile)
			data, err := ioutil.ReadFile(filepath)
			Expect(err).NotTo(HaveOccurred())

			configMgr = s2hconfig.NewWithBytes(data)
			Expect(configMgr).NotTo(BeNil())

			//stagingCtrl = staging.NewController(namespace, mgr, restClient, queueCtrl, configMgr)
			Expect(stagingCtrl).NotTo(BeNil())

			mariadbq.ObjectMeta.Namespace = namespace

			By("Creating StableComponents")
			swp := stableWordPress
			Expect(runtimeClient.Create(context.TODO(), &swp)).To(BeNil())
			smd := stableMariaDB
			Expect(runtimeClient.Create(context.TODO(), &smd)).To(BeNil())
			chStop = make(chan struct{})
			go stagingCtrl.Start(chStop)
		}, 10)

		AfterEach(func(done Done) {
			defer close(done)

			By("Deleting all StableComponents")
			err = runtimeClient.DeleteAllOf(context.TODO(), &s2hv1beta1.StableComponent{}, crclient.InNamespace(namespace))
			Expect(err).NotTo(HaveOccurred())

			By("Deleting all Queues")
			err = runtimeClient.DeleteAllOf(context.TODO(), &s2hv1beta1.Queue{}, crclient.InNamespace(namespace))
			Expect(err).NotTo(HaveOccurred())

			close(chStop)
		}, 10)

		XIt("Should retry queue on failed test and create Reverify queue after reach max failed threshold", func(done Done) {
			defer close(done)

			cfg := configMgr.Get()

			wgUpgrade := sync.WaitGroup{}
			wgReverify := sync.WaitGroup{}
			wgUpgrade.Add(cfg.Staging.MaxRetry + 1)
			wgReverify.Add(1)

			mockDeployEngine := mock.NewWithCallback(func(refName string, comp *internal.Component, parentComp *internal.Component, values map[string]interface{}) {
				defer GinkgoRecover()

				//if q.Spec.Type == v1beta1.QueueTypeReverify {
				//	defer wgReverify.Done()
				//
				//	Expect(dotaccess.Get(values, "mariadb.image.tag")).To(Equal(stableMariaDB.Spec.Version))
				//	Expect(dotaccess.Get(values, "mariadb.image.repository")).To(Equal(stableMariaDB.Spec.Repository))
				//} else {
				//	defer wgUpgrade.Done()
				//
				//	Expect(dotaccess.Get(values, "mariadb.image.tag")).To(Equal(mariadb.Spec.Version))
				//	Expect(dotaccess.Get(values, "mariadb.image.repository")).To(Equal(mariadb.Spec.Repository))
				//}
			}, nil)

			stagingCtrl.LoadDeployEngine(mockDeployEngine)

			By("Creating Queue")
			qmdb := *mariadbq
			Expect(queueCtrl.Add(&qmdb)).To(BeNil())

			wgUpgrade.Wait()
			wgReverify.Wait()

			for stagingCtrl.IsBusy() {
				time.Sleep(50 * time.Millisecond)
			}
		}, 60)

		// TODO: test is flaky
		XSpecify("Cancelling Queue", func(done Done) {
			defer close(done)

			wgDone := sync.WaitGroup{}
			wgDone.Add(1)

			Eventually(stagingCtrl.IsBusy, time.Second, 50*time.Millisecond).Should(BeFalse())

			mockDeployEngine := mock.NewWithCallback(func(refName string, comp *internal.Component, parentComp *internal.Component, values map[string]interface{}) {
				defer GinkgoRecover()
				// skip other components that aren't Queue
				if comp.Name != mariadbq.Name {
					return
				}
				defer wgDone.Done()

				fetched := &v1beta1.Queue{}
				err := runtimeClient.Get(
					context.Background(),
					types.NamespacedName{Namespace: mariadbq.Namespace, Name: mariadbq.Name},
					fetched)
				Expect(err).NotTo(HaveOccurred())

				err = runtimeClient.Delete(context.Background(), fetched)
				Expect(err).NotTo(HaveOccurred())
			}, nil)

			stagingCtrl.LoadDeployEngine(mockDeployEngine)

			By("Creating queue")
			qmdb := *mariadbq
			Expect(queueCtrl.Add(&qmdb)).To(BeNil())

			wgDone.Wait()

			Eventually(stagingCtrl.IsBusy, 5*time.Second, 50*time.Millisecond).Should(BeFalse())
		}, 30)
	})
})
