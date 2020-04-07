package samsahai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tidwall/gjson"
	"github.com/twitchtv/twirp"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
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

	s2hv1beta1 "github.com/agoda-com/samsahai/api/v1beta1"
	"github.com/agoda-com/samsahai/internal"
	configctrl "github.com/agoda-com/samsahai/internal/config"
	"github.com/agoda-com/samsahai/internal/queue"
	"github.com/agoda-com/samsahai/internal/samsahai"
	"github.com/agoda-com/samsahai/internal/samsahai/activepromotion"
	s2hobject "github.com/agoda-com/samsahai/internal/samsahai/k8sobject"
	s2hhttp "github.com/agoda-com/samsahai/internal/samsahai/webhook"
	"github.com/agoda-com/samsahai/internal/stablecomponent"
	"github.com/agoda-com/samsahai/internal/staging"
	utilhttp "github.com/agoda-com/samsahai/internal/util/http"
	"github.com/agoda-com/samsahai/internal/util/stringutils"
	samsahairpc "github.com/agoda-com/samsahai/pkg/samsahai/rpc"
)

const (
	verifyTimeout10        = 10 * time.Second
	verifyNSCreatedTimeout = 15 * time.Second
	promoteTimeOut         = 220 * time.Second
)

var (
	stableComponentCtrl  internal.StableComponentController
	cfgCtrl              internal.ConfigController
	activePromotionCtrl  internal.ActivePromotionController
	samsahaiCtrl         internal.SamsahaiController
	stagingPreActiveCtrl internal.StagingController
	runtimeClient        crclient.Client
	wgStop               *sync.WaitGroup
	chStop               chan struct{}
	mgr                  manager.Manager
	err                  error
	samsahaiServer       *httptest.Server
	samsahaiClient       samsahairpc.RPC
	restCfg              *rest.Config

	samsahaiAuthToken = "1234567890_"
	samsahaiSystemNs  = "samsahai-system"

	teamName  = "teamtest"
	teamForQ1 = teamName + "-q1"
	teamForQ2 = teamName + "-q2"
	teamForQ3 = teamName + "-q3"

	defaultLabels   = internal.GetDefaultLabels(teamName)
	defaultLabelsQ1 = internal.GetDefaultLabels(teamForQ1)
	defaultLabelsQ2 = internal.GetDefaultLabels(teamForQ2)
	defaultLabelsQ3 = internal.GetDefaultLabels(teamForQ3)

	stgNamespace = internal.AppPrefix + teamName
	atvNamespace = internal.AppPrefix + teamName + "-active"

	testLabels = map[string]string{
		"created-for": "s2h-testing",
	}

	mockTeam = s2hv1beta1.Team{
		ObjectMeta: metav1.ObjectMeta{
			Name:   teamName,
			Labels: testLabels,
		},
		Spec: s2hv1beta1.TeamSpec{
			Description: "team for testing",
			Owners:      []string{"samsahai@samsahai.io"},
			Credential: s2hv1beta1.Credential{
				SecretName: s2hobject.GetTeamSecretName(teamName),
			},
			StagingCtrl: &s2hv1beta1.StagingCtrl{
				IsDeploy: false,
			},
		},
		Status: s2hv1beta1.TeamStatus{
			Namespace: s2hv1beta1.TeamNamespace{},
			DesiredComponentImageCreatedTime: map[string]map[string]s2hv1beta1.DesiredImageTime{
				"mariadb": {
					stringutils.ConcatImageString("bitnami/mariadb", "10.3.18-debian-9-r32"): s2hv1beta1.DesiredImageTime{
						Image:       &s2hv1beta1.Image{Repository: "bitnami/mariadb", Tag: "10.3.18-debian-9-r32"},
						CreatedTime: metav1.Time{Time: time.Date(2019, 10, 1, 9, 0, 0, 0, time.UTC)},
					},
				},
			},
		},
	}

	mockActiveQueue = s2hv1beta1.Queue{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "active",
			Labels: testLabels,
		},
		Status: s2hv1beta1.QueueStatus{
			State: s2hv1beta1.Finished,
		},
	}

	mockDeActiveQueue = s2hv1beta1.Queue{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "de-active",
			Labels: testLabels,
		},
		Status: s2hv1beta1.QueueStatus{
			State: s2hv1beta1.Finished,
		},
	}

	activeNamespace = corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   atvNamespace,
			Labels: testLabels,
		},
	}

	activePromotion = s2hv1beta1.ActivePromotion{
		ObjectMeta: metav1.ObjectMeta{
			Name:   teamName,
			Labels: testLabels,
		},
	}

	activePromotionHistory = s2hv1beta1.ActivePromotionHistory{
		ObjectMeta: metav1.ObjectMeta{
			Name:   fmt.Sprintf("%s-20191010-111111", teamName),
			Labels: defaultLabels,
		},
	}

	stableMariaDB = s2hv1beta1.StableComponent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mariadb",
			Namespace: stgNamespace,
		},
		Spec: s2hv1beta1.StableComponentSpec{
			Name:       "mariadb",
			Version:    "10.3.18-debian-9-r32",
			Repository: "bitnami/mariadb",
		},
	}

	stableAtvMariaDB = s2hv1beta1.StableComponent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mariadb",
			Namespace: atvNamespace,
		},
		Spec: s2hv1beta1.StableComponentSpec{
			Name:       "mariadb",
			Version:    "10.3.18-debian-9-r32",
			Repository: "bitnami/mariadb",
		},
	}

	stableRedis = s2hv1beta1.StableComponent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "redis",
			Namespace: stgNamespace,
		},
		Spec: s2hv1beta1.StableComponentSpec{
			Name:       "redis",
			Version:    "5.0.5-debian-9-r160",
			Repository: "bitnami/redis",
		},
	}

	mockSecret = corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      s2hobject.GetTeamSecretName(teamName),
			Namespace: samsahaiSystemNs,
		},
		Data: map[string][]byte{},
		Type: "Opaque",
	}

	engine       = "helm3"
	deployConfig = s2hv1beta1.ConfigDeploy{
		Timeout: metav1.Duration{Duration: 5 * time.Minute},
		Engine:  &engine,
		TestRunner: &s2hv1beta1.ConfigTestRunner{
			TestMock: &s2hv1beta1.ConfigTestMock{
				Result: true,
			},
		},
	}
	compSource      = s2hv1beta1.UpdatingSource("public-registry")
	redisConfigComp = s2hv1beta1.Component{
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
	wordpressConfigComp = s2hv1beta1.Component{
		Name: "wordpress",
		Chart: s2hv1beta1.ComponentChart{
			Repository: "https://kubernetes-charts.storage.googleapis.com",
			Name:       "wordpress",
		},
		Image: s2hv1beta1.ComponentImage{
			Repository: "bitnami/wordpress",
			Pattern:    "5\\.2.*debian-9.*",
		},
		Source: &compSource,
		Dependencies: []*s2hv1beta1.Component{
			{
				Name: "mariadb",
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

	mockConfig = s2hv1beta1.Config{
		ObjectMeta: metav1.ObjectMeta{
			Name:   teamName,
			Labels: testLabels,
		},
		Spec: s2hv1beta1.ConfigSpec{
			Staging: &s2hv1beta1.ConfigStaging{
				Deployment: &deployConfig,
			},
			ActivePromotion: &s2hv1beta1.ConfigActivePromotion{
				Timeout:          metav1.Duration{Duration: 10 * time.Minute},
				MaxHistories:     2,
				TearDownDuration: metav1.Duration{Duration: 10 * time.Second},
				OutdatedNotification: &s2hv1beta1.OutdatedNotification{
					ExceedDuration:            metav1.Duration{Duration: 24 * time.Hour},
					ExcludeWeekendCalculation: true,
				},
				Deployment: &deployConfig,
			},
			Reporter: &s2hv1beta1.ConfigReporter{
				ReportMock: true,
			},
			Components: []*s2hv1beta1.Component{
				&redisConfigComp,
				&wordpressConfigComp,
			},
		},
	}
)

var _ = Describe("Main Controller [e2e]", func() {
	BeforeEach(func(done Done) {
		defer close(done)

		chStop = make(chan struct{})

		adminRestConfig, err := config.GetConfig()
		Expect(err).NotTo(HaveOccurred(), "Please provide credential for accessing k8s cluster")

		adminClient, err := crclient.New(adminRestConfig, crclient.Options{Scheme: scheme.Scheme})
		Expect(err).NotTo(HaveOccurred(), "should create runtime client successfully")

		ctx := context.TODO()
		// get token for samsahai user
		restCfg = rest.CopyConfig(adminRestConfig)
		restCfg.Username = ""
		// get token
		samsahaiSA := &corev1.ServiceAccount{}
		err = adminClient.Get(ctx, types.NamespacedName{Name: "samsahai", Namespace: samsahaiSystemNs}, samsahaiSA)
		Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("cannot get sa: %s/%s", samsahaiSystemNs, "samsahai"))
		Expect(len(samsahaiSA.Secrets)).To(BeNumerically(">=", 1))
		samsahaiSecret := &corev1.Secret{}
		err = adminClient.Get(ctx, types.NamespacedName{Namespace: samsahaiSystemNs, Name: samsahaiSA.Secrets[0].Name}, samsahaiSecret)
		Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("cannot get secret: %s/%s", samsahaiSystemNs, samsahaiSA.Secrets[0].Name))
		restCfg.BearerToken = string(samsahaiSecret.Data["token"])

		mgr, err = manager.New(restCfg, manager.Options{MetricsBindAddress: "0"})
		Expect(err).NotTo(HaveOccurred(), "should create manager successfully")

		runtimeClient, err = crclient.New(restCfg, crclient.Options{Scheme: scheme.Scheme})
		Expect(err).NotTo(HaveOccurred(), "should create runtime client successfully")

		Expect(os.Setenv("S2H_CONFIG_PATH", "../data/application.yaml")).NotTo(HaveOccurred(),
			"should sent samsahai file config path successfully")
		s2hConfig := internal.SamsahaiConfig{
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
		cfgCtrl = configctrl.New(mgr)
		Expect(cfgCtrl).ToNot(BeNil())

		samsahaiCtrl = samsahai.New(mgr, "samsahai-system", s2hConfig, cfgCtrl)
		Expect(samsahaiCtrl).ToNot(BeNil())

		activePromotionCtrl = activepromotion.New(mgr, samsahaiCtrl, s2hConfig)
		Expect(activePromotionCtrl).ToNot(BeNil())

		stableComponentCtrl = stablecomponent.New(mgr, samsahaiCtrl)
		Expect(stableComponentCtrl).ToNot(BeNil())

		wgStop = &sync.WaitGroup{}
		wgStop.Add(1)
		go func() {
			defer wgStop.Done()
			Expect(mgr.Start(chStop)).To(BeNil())
		}()

		mux := http.NewServeMux()
		mux.Handle(samsahaiCtrl.PathPrefix(), samsahaiCtrl)
		mux.Handle("/", s2hhttp.New(samsahaiCtrl))
		samsahaiServer = httptest.NewServer(mux)
		samsahaiClient = samsahairpc.NewRPCProtobufClient(samsahaiServer.URL, &http.Client{})

		By("Creating Secret")
		secret := mockSecret
		_ = runtimeClient.Delete(context.TODO(), &secret)
		_ = runtimeClient.Create(context.TODO(), &secret)
	}, 60)

	AfterEach(func(done Done) {
		defer close(done)
		ctx := context.TODO()

		By("Deleting all StableComponents")
		err = runtimeClient.DeleteAllOf(ctx, &s2hv1beta1.StableComponent{}, crclient.InNamespace(stgNamespace))
		Expect(err).NotTo(HaveOccurred())

		By("Deleting all Teams")
		err = runtimeClient.DeleteAllOf(ctx, &s2hv1beta1.Team{}, crclient.MatchingLabels(testLabels))
		Expect(err).NotTo(HaveOccurred())
		err = wait.PollImmediate(1*time.Second, verifyTimeout10, func() (ok bool, err error) {
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
		Expect(err).NotTo(HaveOccurred(), "Delete all Teams error")

		By("Deleting all Configs")
		err = runtimeClient.DeleteAllOf(ctx, &s2hv1beta1.Config{}, crclient.MatchingLabels(testLabels))
		Expect(err).NotTo(HaveOccurred())
		err = wait.PollImmediate(1*time.Second, verifyTimeout10, func() (ok bool, err error) {
			configList := s2hv1beta1.ConfigList{}
			listOpt := &crclient.ListOptions{LabelSelector: labels.SelectorFromSet(testLabels)}
			err = runtimeClient.List(ctx, &configList, listOpt)
			if err != nil && errors.IsNotFound(err) {
				return true, nil
			}
			if len(configList.Items) == 0 {
				return true, nil
			}

			return false, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Deleting all Configs error")

		By("Deleting active namespace")
		atvNs := activeNamespace
		_ = runtimeClient.Delete(context.TODO(), &atvNs)
		err = wait.PollImmediate(1*time.Second, verifyTimeout10, func() (ok bool, err error) {
			namespace := corev1.Namespace{}
			err = runtimeClient.Get(ctx, types.NamespacedName{Name: atvNamespace}, &namespace)
			if err != nil && errors.IsNotFound(err) {
				return true, nil
			}
			return false, nil
		})

		By("Deleting all ActivePromotions")
		err = runtimeClient.DeleteAllOf(ctx, &s2hv1beta1.ActivePromotion{}, crclient.MatchingLabels(testLabels))
		Expect(err).NotTo(HaveOccurred())
		err = wait.PollImmediate(1*time.Second, verifyTimeout10, func() (ok bool, err error) {
			atpList := s2hv1beta1.ActivePromotionList{}
			listOpt := &crclient.ListOptions{LabelSelector: labels.SelectorFromSet(testLabels)}
			err = runtimeClient.List(ctx, &atpList, listOpt)
			if err != nil && errors.IsNotFound(err) {
				return true, nil
			}
			if len(atpList.Items) == 0 {
				return true, nil
			}

			return false, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Delete all active promotions error")

		By("Deleting ActivePromotionHistories")
		err = runtimeClient.DeleteAllOf(ctx, &s2hv1beta1.ActivePromotionHistory{}, crclient.MatchingLabels(testLabels))
		Expect(err).NotTo(HaveOccurred())
		err = runtimeClient.DeleteAllOf(ctx, &s2hv1beta1.ActivePromotionHistory{}, crclient.MatchingLabels(defaultLabels))
		Expect(err).NotTo(HaveOccurred())
		err = runtimeClient.DeleteAllOf(ctx, &s2hv1beta1.ActivePromotionHistory{}, crclient.MatchingLabels(defaultLabelsQ1))
		Expect(err).NotTo(HaveOccurred())
		err = runtimeClient.DeleteAllOf(ctx, &s2hv1beta1.ActivePromotionHistory{}, crclient.MatchingLabels(defaultLabelsQ2))
		Expect(err).NotTo(HaveOccurred())
		err = runtimeClient.DeleteAllOf(ctx, &s2hv1beta1.ActivePromotionHistory{}, crclient.MatchingLabels(defaultLabelsQ3))
		Expect(err).NotTo(HaveOccurred())

		By("Deleting Secret")
		secret := mockSecret
		Expect(runtimeClient.Delete(context.TODO(), &secret)).NotTo(HaveOccurred())

		close(chStop)
		samsahaiServer.Close()
		wgStop.Wait()
	}, 60)

	It("should successfully promote an active environment", func(done Done) {
		defer close(done)
		ctx := context.TODO()
		preActiveNs := ""

		By("Creating Config")
		config := mockConfig
		Expect(runtimeClient.Create(ctx, &config)).To(BeNil())

		By("Creating Team")
		team := mockTeam
		team.Status.Namespace.Active = atvNamespace
		Expect(runtimeClient.Create(ctx, &team)).To(BeNil())

		By("Verifying staging related objects has been created")
		err = wait.PollImmediate(1*time.Second, verifyNSCreatedTimeout, func() (ok bool, err error) {
			namespace := corev1.Namespace{}
			err = runtimeClient.Get(ctx, types.NamespacedName{Name: stgNamespace}, &namespace)
			if err != nil {
				return false, nil
			}

			secret := corev1.Secret{}
			err = runtimeClient.Get(ctx, types.NamespacedName{Name: internal.StagingCtrlName, Namespace: stgNamespace}, &secret)
			if err != nil {
				return false, nil
			}

			// TODO: uncomment when staging can be successfully deployed
			//deployment := appv1.Deployment{}
			//err = runtimeClient.Get(ctx, types.NamespacedName{Name: internal.StagingCtrlName, Namespace: stgNamespace}, &deployment)
			//if err != nil || deployment.Status.AvailableReplicas != *deployment.Spec.Replicas {
			//	time.Sleep(500 * time.Millisecond)
			//	continue
			//}

			svc := corev1.Service{}
			err = runtimeClient.Get(ctx, types.NamespacedName{Name: internal.StagingCtrlName, Namespace: stgNamespace}, &svc)
			if err != nil {
				return false, nil
			}

			role := rbacv1.Role{}
			err = runtimeClient.Get(ctx, types.NamespacedName{Name: internal.StagingCtrlName, Namespace: stgNamespace}, &role)
			if err != nil {
				return false, nil
			}

			roleBinding := rbacv1.RoleBinding{}
			err = runtimeClient.Get(ctx, types.NamespacedName{Name: internal.StagingCtrlName, Namespace: stgNamespace}, &roleBinding)
			if err != nil {
				return false, nil
			}

			clusterRole := rbacv1.ClusterRole{}
			err = runtimeClient.Get(ctx, types.NamespacedName{Name: s2hobject.GenClusterRoleName(stgNamespace), Namespace: stgNamespace}, &clusterRole)
			if err != nil {
				return false, nil
			}

			clusterRoleBinding := rbacv1.ClusterRoleBinding{}
			err = runtimeClient.Get(ctx, types.NamespacedName{Name: s2hobject.GenClusterRoleName(stgNamespace), Namespace: stgNamespace}, &clusterRoleBinding)
			if err != nil {
				return false, nil
			}

			sa := corev1.ServiceAccount{}
			err = runtimeClient.Get(ctx, types.NamespacedName{Name: internal.StagingCtrlName, Namespace: stgNamespace}, &sa)
			if err != nil {
				return false, nil
			}

			config := s2hv1beta1.Config{}
			err = runtimeClient.Get(ctx, types.NamespacedName{Name: team.Name}, &config)
			if err != nil {
				return false, nil
			}

			return true, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Create staging related object objects error")

		By("Creating active namespace")
		atvNs := activeNamespace
		Expect(runtimeClient.Create(ctx, &atvNs)).To(BeNil())

		By("Creating StableComponent")
		smd := stableMariaDB
		Expect(runtimeClient.Create(ctx, &smd)).To(BeNil())

		time.Sleep(1 * time.Second)
		By("Checking stable component has been set")
		teamComp := s2hv1beta1.Team{}
		Expect(runtimeClient.Get(ctx, types.NamespacedName{Name: team.Name}, &teamComp))
		teamSpecStableComps := teamComp.Status.StableComponents[0].Spec
		Expect(teamSpecStableComps.Name).To(Equal(stableAtvMariaDB.Spec.Name))
		Expect(teamSpecStableComps.Repository).To(Equal(stableAtvMariaDB.Spec.Repository))
		Expect(teamSpecStableComps.Version).To(Equal(stableAtvMariaDB.Spec.Version))

		By("Creating ActivePromotionHistory 1")
		atpHist := activePromotionHistory
		atpHist.Name = atpHist.Name + "-1"
		Expect(runtimeClient.Create(ctx, &atpHist)).To(BeNil())

		time.Sleep(1 * time.Second)
		By("Creating ActivePromotionHistory 2")
		atpHist = activePromotionHistory
		atpHist.Name = atpHist.Name + "-2"
		Expect(runtimeClient.Create(ctx, &atpHist)).To(BeNil())

		By("Creating ActivePromotion")
		atp := activePromotion
		Expect(runtimeClient.Create(ctx, &atp)).To(BeNil())

		By("Creating mock de-active queue for active namespace")
		deActiveQ := mockDeActiveQueue
		deActiveQ.Namespace = atvNamespace
		Expect(runtimeClient.Create(ctx, &deActiveQ)).To(BeNil())

		By("Waiting pre-active environment is successfully created")
		atpResCh := make(chan s2hv1beta1.ActivePromotion)
		go func() {
			atpTemp := s2hv1beta1.ActivePromotion{}
			for {
				_ = runtimeClient.Get(ctx, types.NamespacedName{Name: atp.Name}, &atpTemp)
				if atpTemp.Status.IsConditionTrue(s2hv1beta1.ActivePromotionCondPreActiveCreated) {
					break
				}
				time.Sleep(500 * time.Millisecond)
			}
			atpResCh <- atpTemp
		}()
		atpRes := <-atpResCh

		By("Start staging controller for pre-active")
		preActiveNs = atpRes.Status.TargetNamespace
		{
			stagingCfg := rest.CopyConfig(restCfg)
			stagingCfg.Username = ""
			// get token
			stagingSA := &corev1.ServiceAccount{}
			err = runtimeClient.Get(ctx, types.NamespacedName{Name: internal.StagingCtrlName, Namespace: preActiveNs}, stagingSA)
			Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("cannot get sa: %s", internal.StagingCtrlName))
			Expect(len(stagingSA.Secrets)).To(BeNumerically(">=", 1))
			stagingSecret := &corev1.Secret{}
			err = runtimeClient.Get(ctx, types.NamespacedName{Namespace: preActiveNs, Name: stagingSA.Secrets[0].Name}, stagingSecret)
			Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("cannot get secret: %s", stagingSA.Secrets[0].Name))
			stagingCfg.BearerToken = string(stagingSecret.Data["token"])

			// create mgr from config
			stagingMgr, err := manager.New(stagingCfg, manager.Options{
				Namespace:          preActiveNs,
				MetricsBindAddress: "0",
			})
			Expect(err).NotTo(HaveOccurred())

			stagingCfgCtrl := configctrl.New(stagingMgr)
			qctrl := queue.New(preActiveNs, runtimeClient)
			stagingPreActiveCtrl = staging.NewController(teamName, preActiveNs, samsahaiAuthToken, samsahaiClient,
				stagingMgr, qctrl, stagingCfgCtrl, "", "", "",
				internal.StagingConfig{})
			go func() {
				defer GinkgoRecover()
				Expect(stagingMgr.Start(chStop)).NotTo(HaveOccurred())
			}()
			go stagingPreActiveCtrl.Start(chStop)
		}

		By("Checking pre-active namespace has been set")
		Expect(runtimeClient.Get(ctx, types.NamespacedName{Name: atp.Name}, &teamComp))

		Expect(teamComp.Status.Namespace.PreActive).ToNot(BeEmpty())
		Expect(atpRes.Status.TargetNamespace).To(Equal(teamComp.Status.Namespace.PreActive))
		Expect(atpRes.Status.PreviousActiveNamespace).To(Equal(atvNamespace))

		By("Checking stable components has been deployed to target namespace")
		stableComps := &s2hv1beta1.StableComponentList{}
		err = runtimeClient.List(ctx, stableComps, &crclient.ListOptions{Namespace: atpRes.Status.TargetNamespace})
		Expect(err).To(BeNil())
		Expect(len(stableComps.Items)).To(Equal(1))

		By("previous active namespace should be deleted")
		err = wait.PollImmediate(1*time.Second, promoteTimeOut, func() (ok bool, err error) {
			namespace := corev1.Namespace{}
			err = runtimeClient.Get(ctx, types.NamespacedName{Name: atvNamespace}, &namespace)
			if err != nil && errors.IsNotFound(err) {
				return true, nil
			}

			return false, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Delete previous namespace error")

		By("ActivePromotion should be deleted")
		err = wait.PollImmediate(1*time.Second, 45*time.Second, func() (ok bool, err error) {
			atpTemp := s2hv1beta1.ActivePromotion{}
			err = runtimeClient.Get(ctx, types.NamespacedName{Name: atp.Name}, &atpTemp)
			if err != nil && errors.IsNotFound(err) {
				return true, nil
			}

			return false, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Delete active promotion error")

		By("Checking active namespace and previous namespace has been reset")
		teamComp = s2hv1beta1.Team{}
		err = runtimeClient.Get(ctx, types.NamespacedName{Name: atp.Name}, &teamComp)
		Expect(err).To(BeNil())
		Expect(teamComp.Status.Namespace.Active).To(Equal(preActiveNs))

		err = runtimeClient.Get(ctx, types.NamespacedName{Name: atvNamespace}, &atvNs)
		Expect(errors.IsNotFound(err)).To(BeTrue())

		By("ActivePromotionHistory should be created")
		atpHists := &s2hv1beta1.ActivePromotionHistoryList{}
		listOpt := &crclient.ListOptions{LabelSelector: labels.SelectorFromSet(defaultLabels)}
		err = runtimeClient.List(context.TODO(), atpHists, listOpt)
		Expect(err).To(BeNil())
		Expect(len(atpHists.Items)).To(Equal(2))
		Expect(atpHists.Items[0].Name).ToNot(Equal(atpHist.Name + "-1"))
		Expect(atpHists.Items[1].Name).ToNot(Equal(atpHist.Name + "-1"))
		Expect(atpHists.Items[1].Spec.ActivePromotion.Status.OutdatedComponents).ToNot(BeNil())

		By("Current active components should be set")
		teamComp = s2hv1beta1.Team{}
		err = runtimeClient.Get(ctx, types.NamespacedName{Name: team.Name}, &teamComp)
		Expect(err).To(BeNil())
		Expect(len(teamComp.Status.ActiveComponents)).ToNot(BeZero())

		By("Public API")
		{
			By("Get team")
			{
				data, err := utilhttp.Get(samsahaiServer.URL + "/teams/" + team.Name)
				Expect(err).NotTo(HaveOccurred())
				Expect(data).NotTo(BeNil())

				Expect(gjson.GetBytes(data, "teamName").Str).To(Equal(team.Name))
			}

			By("Get team Queue")
			{
				data, err := utilhttp.Get(samsahaiServer.URL + "/teams/" + team.Name + "/queue")
				Expect(err).NotTo(HaveOccurred())
				Expect(data).NotTo(BeNil())
			}

			By("Get team Queue not found")
			{
				_, err := utilhttp.Get(samsahaiServer.URL + "/teams/" + team.Name + "/queue/histories/" + "unknown")
				Expect(err).To(HaveOccurred())
			}

			By("Get Stable Values")
			{
				parentComps, err := cfgCtrl.GetParentComponents(team.Name)
				Expect(err).NotTo(HaveOccurred())

				compName := ""
				for c := range parentComps {
					compName = c
				}

				url := fmt.Sprintf("%s/teams/%s/components/%s/values", samsahaiServer.URL, team.Name, compName)
				data, err := utilhttp.Get(url, utilhttp.WithHeader("Accept", "text/yaml"))
				Expect(err).NotTo(HaveOccurred())
				Expect(data).NotTo(BeNil())
			}
		}
	}, 230)

	It("should successfully promote an active environment even demote timeout", func(done Done) {
		defer close(done)

		ctx := context.TODO()

		By("Creating Config")
		config := mockConfig
		Expect(runtimeClient.Create(ctx, &config)).To(BeNil())

		By("Creating Team")
		team := mockTeam
		team.Status.Namespace.Active = atvNamespace
		Expect(runtimeClient.Create(ctx, &team)).To(BeNil())

		By("Creating active namespace")
		atvNs := activeNamespace
		Expect(runtimeClient.Create(ctx, &atvNs)).To(BeNil())

		By("Verifying namespace and config have been created")
		err = wait.PollImmediate(1*time.Second, verifyNSCreatedTimeout, func() (ok bool, err error) {
			namespace := corev1.Namespace{}
			if err := runtimeClient.Get(ctx, types.NamespacedName{Name: stgNamespace}, &namespace); err != nil {
				return false, nil
			}

			config := s2hv1beta1.Config{}
			err = runtimeClient.Get(ctx, types.NamespacedName{Name: team.Name}, &config)
			if err != nil {
				return false, nil
			}

			return true, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Verify namespace and config error")

		By("Creating ActivePromotion with `DemotingActiveEnvironment` state")
		atp := activePromotion
		atp.Status.State = s2hv1beta1.ActivePromotionDemoting
		atp.Status.PreviousActiveNamespace = atvNamespace
		atp.Status.SetCondition(s2hv1beta1.ActivePromotionCondActiveDemotionStarted, corev1.ConditionTrue, "start demoting")
		Expect(runtimeClient.Create(ctx, &atp)).To(BeNil())

		By("Waiting ActivePromotion state to be `PromotingActiveEnvironment`")
		err = wait.PollImmediate(1*time.Second, verifyTimeout10, func() (ok bool, err error) {
			atpComp := s2hv1beta1.ActivePromotion{}
			if err := runtimeClient.Get(ctx, types.NamespacedName{Name: teamName}, &atpComp); err != nil {
				return false, nil
			}

			if atpComp.Status.State == s2hv1beta1.ActivePromotionActiveEnvironment {
				return true, nil
			}

			return false, nil
		})
		Expect(err).NotTo(HaveOccurred(),
			"Waiting active promotion state to `PromotingActiveEnvironment` error")
	}, 40)

	It("should successfully add/remove/run active promotion from queue", func(done Done) {
		defer close(done)
		ctx := context.TODO()

		By("Creating Team for Q1")
		team1 := mockTeam
		team1.Name = teamForQ1
		Expect(runtimeClient.Create(ctx, &team1)).To(BeNil())

		By("Creating Config for Q1")
		config1 := mockConfig
		config1.Name = teamForQ1
		Expect(runtimeClient.Create(ctx, &config1)).To(BeNil())

		By("Creating Team for Q2")
		team2 := mockTeam
		team2.Name = teamForQ2
		Expect(runtimeClient.Create(ctx, &team2)).To(BeNil())

		By("Creating Config for Q2")
		config2 := mockConfig
		config2.Name = teamForQ2
		Expect(runtimeClient.Create(ctx, &config2)).To(BeNil())

		By("Creating Team for Q3")
		team3 := mockTeam
		team3.Name = teamForQ3
		Expect(runtimeClient.Create(ctx, &team3)).To(BeNil())
		By("Verifying configuration has been created")

		By("Creating Config for Q3")
		config3 := mockConfig
		config3.Name = teamForQ3
		Expect(runtimeClient.Create(ctx, &config3)).To(BeNil())

		By("Verifying all teams have been created")
		err = wait.PollImmediate(1*time.Second, verifyTimeout10, func() (ok bool, err error) {
			teamList := s2hv1beta1.TeamList{}
			listOpt := &crclient.ListOptions{LabelSelector: labels.SelectorFromSet(testLabels)}
			if err := runtimeClient.List(ctx, &teamList, listOpt); err != nil {
				return false, nil
			}

			if len(teamList.Items) == 3 {
				return true, nil
			}

			configList := s2hv1beta1.ConfigList{}
			if err := runtimeClient.List(ctx, &configList, listOpt); err != nil {
				return false, nil
			}

			if len(configList.Items) == 3 {
				return true, nil
			}

			return false, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Create teams error")

		By("Creating ActivePromotions")
		atpQ1 := activePromotion
		atpQ1.Name = teamForQ1
		Expect(runtimeClient.Create(ctx, &atpQ1)).To(BeNil())

		time.Sleep(1 * time.Second)

		atpQ2 := activePromotion
		atpQ2.Name = teamForQ2
		Expect(runtimeClient.Create(ctx, &atpQ2)).To(BeNil())

		time.Sleep(1 * time.Second)

		atpQ3 := activePromotion
		atpQ3.Name = teamForQ3
		Expect(runtimeClient.Create(ctx, &atpQ3)).To(BeNil())

		By("Waiting ActivePromotion Q1 state to be `Deploying`, other ActivePromotion states to be `Waiting`")
		err = wait.PollImmediate(1*time.Second, verifyTimeout10, func() (ok bool, err error) {
			atpCompQ1 := s2hv1beta1.ActivePromotion{}
			if err := runtimeClient.Get(ctx, types.NamespacedName{Name: teamForQ1}, &atpCompQ1); err != nil {
				return false, nil
			}

			atpCompQ2 := s2hv1beta1.ActivePromotion{}
			if err := runtimeClient.Get(ctx, types.NamespacedName{Name: teamForQ2}, &atpCompQ2); err != nil {
				return false, nil
			}

			atpCompQ3 := s2hv1beta1.ActivePromotion{}
			if err := runtimeClient.Get(ctx, types.NamespacedName{Name: teamForQ2}, &atpCompQ3); err != nil {
				return false, nil
			}

			if atpCompQ1.Status.State == s2hv1beta1.ActivePromotionDeployingComponents &&
				atpCompQ2.Status.State == s2hv1beta1.ActivePromotionWaiting &&
				atpCompQ3.Status.State == s2hv1beta1.ActivePromotionWaiting {
				return true, nil
			}

			return false, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Change active promotion state error")

		By("Deleting ActivePromotion Q2 from queue")
		atpCompQ2 := s2hv1beta1.ActivePromotion{}
		Expect(runtimeClient.Get(ctx, types.NamespacedName{Name: teamForQ2}, &atpCompQ2)).To(BeNil())
		Expect(runtimeClient.Delete(context.TODO(), &atpCompQ2)).NotTo(HaveOccurred())
		err = wait.PollImmediate(1*time.Second, verifyTimeout10, func() (ok bool, err error) {
			atpTemp := s2hv1beta1.ActivePromotion{}
			err = runtimeClient.Get(ctx, types.NamespacedName{Name: teamForQ2}, &atpTemp)
			if err != nil && errors.IsNotFound(err) {
				return true, nil
			}

			return false, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Delete active promotion for Team2 error")

		atpCompQ3 := s2hv1beta1.ActivePromotion{}
		Expect(runtimeClient.Get(ctx, types.NamespacedName{Name: teamForQ3}, &atpCompQ3)).To(BeNil())
		Expect(atpCompQ3.Status.State).To(Equal(s2hv1beta1.ActivePromotionWaiting))

		By("Deleting ActivePromotion Q1")
		atpCompQ1 := s2hv1beta1.ActivePromotion{}
		Expect(runtimeClient.Get(ctx, types.NamespacedName{Name: teamForQ1}, &atpCompQ1)).To(BeNil())
		Expect(runtimeClient.Delete(context.TODO(), &atpCompQ1)).NotTo(HaveOccurred())

		By("Creating mock de-active Q1")
		preActiveNs := atpCompQ1.Status.TargetNamespace
		deActiveQ := mockDeActiveQueue
		deActiveQ.Namespace = preActiveNs
		Expect(runtimeClient.Create(ctx, &deActiveQ)).To(BeNil())

		By("Verifying delete ActivePromotion Q1")
		err = wait.PollImmediate(1*time.Second, 30*time.Second, func() (ok bool, err error) {
			atpTemp := s2hv1beta1.ActivePromotion{}
			err = runtimeClient.Get(ctx, types.NamespacedName{Name: teamForQ1}, &atpTemp)
			if err != nil && errors.IsNotFound(err) {
				return true, nil
			}

			return false, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Delete active promotion for Team1 error")

		By("Checking ActivePromotion Q3 should be run")
		err = wait.PollImmediate(1*time.Second, verifyTimeout10, func() (ok bool, err error) {
			atpTemp := s2hv1beta1.ActivePromotion{}
			if err := runtimeClient.Get(ctx, types.NamespacedName{Name: teamForQ3}, &atpTemp); err != nil {
				return false, nil
			}

			if atpTemp.Status.State == s2hv1beta1.ActivePromotionDeployingComponents {
				return true, nil
			}

			return false, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Promote Team3 error")

	}, 60)

	It("should successfully rollback and delete active promotion", func(done Done) {
		defer close(done)

		ctx := context.TODO()

		By("Creating Config")
		config := mockConfig
		Expect(runtimeClient.Create(ctx, &config)).To(BeNil())

		By("Creating Team")
		team := mockTeam
		team.Status.Namespace.Active = atvNamespace
		Expect(runtimeClient.Create(ctx, &team)).To(BeNil())

		By("Creating active namespace")
		atvNs := activeNamespace
		Expect(runtimeClient.Create(ctx, &atvNs)).To(BeNil())

		By("Creating StableComponent in active namespace")
		smd := stableAtvMariaDB
		Expect(runtimeClient.Create(ctx, &smd)).To(BeNil())

		By("Verifying namespace and config have been created")
		err = wait.PollImmediate(1*time.Second, verifyNSCreatedTimeout, func() (ok bool, err error) {
			namespace := corev1.Namespace{}
			if err := runtimeClient.Get(ctx, types.NamespacedName{Name: stgNamespace}, &namespace); err != nil {
				return false, nil
			}

			config := s2hv1beta1.Config{}
			err = runtimeClient.Get(ctx, types.NamespacedName{Name: team.Name}, &config)
			if err != nil {
				return false, nil
			}

			return true, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Verify namespace and config error")

		By("Creating ActivePromotion")
		atp := activePromotion
		Expect(runtimeClient.Create(ctx, &atp)).To(BeNil())

		By("Waiting ActivePromotion state to be `Deploying`")
		err = wait.PollImmediate(1*time.Second, verifyTimeout10, func() (ok bool, err error) {
			atpComp := s2hv1beta1.ActivePromotion{}
			if err := runtimeClient.Get(ctx, types.NamespacedName{Name: teamName}, &atpComp); err != nil {
				return false, nil
			}

			if atpComp.Status.State == s2hv1beta1.ActivePromotionDeployingComponents {
				return true, nil
			}

			return false, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Change active promotion state to `Deploying` error")

		By("Updating ActivePromotion state to be `PromotingActiveEnvironment`")
		atpComp := s2hv1beta1.ActivePromotion{}
		Expect(runtimeClient.Get(ctx, types.NamespacedName{Name: teamName}, &atpComp))
		atpComp.Status.State = s2hv1beta1.ActivePromotionActiveEnvironment
		atpComp.Status.SetCondition(s2hv1beta1.ActivePromotionCondVerified, corev1.ConditionTrue, "verified")
		Expect(runtimeClient.Update(ctx, &atpComp)).To(BeNil())

		By("Delete ActivePromotion")
		atpComp = s2hv1beta1.ActivePromotion{}
		Expect(runtimeClient.Get(ctx, types.NamespacedName{Name: teamName}, &atpComp))
		Expect(runtimeClient.Delete(context.TODO(), &atpComp)).To(BeNil())

		By("Creating mock active queue for active namespace")
		activeQ := mockActiveQueue
		activeQ.Namespace = atvNamespace
		Expect(runtimeClient.Create(ctx, &activeQ)).To(BeNil())

		By("pre-active namespace should be deleted")
		preActiveNs := atpComp.Status.TargetNamespace
		err = wait.PollImmediate(1*time.Second, 15*time.Second, func() (ok bool, err error) {
			namespace := corev1.Namespace{}
			err = runtimeClient.Get(ctx, types.NamespacedName{Name: preActiveNs}, &namespace)
			if err != nil && errors.IsNotFound(err) {
				return true, nil
			}

			return false, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Delete pre-active namespace error")

		By("ActivePromotion should be deleted")
		err = wait.PollImmediate(1*time.Second, verifyTimeout10, func() (ok bool, err error) {
			atpTemp := s2hv1beta1.ActivePromotion{}
			err = runtimeClient.Get(ctx, types.NamespacedName{Name: atp.Name}, &atpTemp)
			if err != nil && errors.IsNotFound(err) {
				return true, nil
			}

			return false, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Delete active promotion error")

		atpHists := &s2hv1beta1.ActivePromotionHistoryList{}
		listOpt := &crclient.ListOptions{LabelSelector: labels.SelectorFromSet(defaultLabels)}
		err = runtimeClient.List(context.TODO(), atpHists, listOpt)
		Expect(err).To(BeNil())
		Expect(len(atpHists.Items)).To(Equal(1))
		Expect(atpHists.Items[0].Spec.ActivePromotion.Status.OutdatedComponents).ToNot(BeNil())

		By("Current active components should not be set")
		teamComp := s2hv1beta1.Team{}
		err = runtimeClient.Get(ctx, types.NamespacedName{Name: team.Name}, &teamComp)
		Expect(err).To(BeNil())
		Expect(len(teamComp.Status.ActiveComponents)).To(BeZero())
	}, 60)

	It("should rollback active environment timeout", func(done Done) {
		defer close(done)

		ctx := context.TODO()

		By("Creating Config")
		config := mockConfig
		Expect(runtimeClient.Create(ctx, &config)).To(BeNil())

		By("Creating Team")
		team := mockTeam
		Expect(runtimeClient.Create(ctx, &team)).To(BeNil())

		By("Verifying namespace and config have been created")
		err = wait.PollImmediate(1*time.Second, verifyNSCreatedTimeout, func() (ok bool, err error) {
			namespace := corev1.Namespace{}
			if err := runtimeClient.Get(ctx, types.NamespacedName{Name: stgNamespace}, &namespace); err != nil {
				return false, nil
			}

			config := s2hv1beta1.Config{}
			err = runtimeClient.Get(ctx, types.NamespacedName{Name: team.Name}, &config)
			if err != nil {
				return false, nil
			}

			return true, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Verify namespace and config error")

		By("Creating ActivePromotion with `Rollback` state")
		atp := activePromotion
		atp.Status.State = s2hv1beta1.ActivePromotionRollback
		atp.Status.SetCondition(s2hv1beta1.ActivePromotionCondRollbackStarted, corev1.ConditionTrue, "start rollback")
		startedTime := metav1.Now().Add(-10 * time.Second)
		atp.Status.Conditions[0].LastTransitionTime = metav1.Time{Time: startedTime}
		Expect(runtimeClient.Create(ctx, &atp)).To(BeNil())

		By("ActivePromotion should be deleted")
		err = wait.PollImmediate(1*time.Second, verifyTimeout10, func() (ok bool, err error) {
			atpTemp := s2hv1beta1.ActivePromotion{}
			err = runtimeClient.Get(ctx, types.NamespacedName{Name: atp.Name}, &atpTemp)
			if err != nil && errors.IsNotFound(err) {
				return true, nil
			}

			return false, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Delete active promotion error")
	}, 30)

	It("should successfully delete config when delete team", func(done Done) {
		defer close(done)

		ctx := context.TODO()

		By("Creating Config")
		config := mockConfig
		Expect(runtimeClient.Create(ctx, &config)).To(BeNil())

		By("Creating Team")
		team := mockTeam
		Expect(runtimeClient.Create(ctx, &team)).To(BeNil())

		By("Verifying namespace and config have been created")
		err = wait.PollImmediate(1*time.Second, verifyNSCreatedTimeout, func() (ok bool, err error) {
			team := s2hv1beta1.Team{}
			if err := runtimeClient.Get(ctx, types.NamespacedName{Name: teamName}, &team); err != nil {
				return false, nil
			}

			if len(team.ObjectMeta.Finalizers) == 0 {
				return false, nil
			}

			config := s2hv1beta1.Config{}
			err = runtimeClient.Get(ctx, types.NamespacedName{Name: teamName}, &config)
			if err != nil {
				return false, nil
			}

			return true, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Verify namespace and config error")

		By("Deleting Team")
		_ = runtimeClient.Get(ctx, types.NamespacedName{Name: teamName}, &team)
		Expect(runtimeClient.Delete(ctx, &team)).To(BeNil())

		By("Verifying Config should be deleted")
		err = wait.PollImmediate(1*time.Second, 15*time.Second, func() (ok bool, err error) {
			config := s2hv1beta1.Config{}
			err = runtimeClient.Get(ctx, types.NamespacedName{Name: teamName}, &config)
			if err != nil && errors.IsNotFound(err) {
				return true, nil
			}

			return false, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Config should be deleted")

	}, 30)

	It("should be error when creating team if config does not exist", func(done Done) {
		defer close(done)

		ctx := context.TODO()

		By("Creating Team")
		team := mockTeam
		Expect(runtimeClient.Create(ctx, &team)).To(BeNil())

		By("Team should be error if missing Config")
		err = wait.PollImmediate(1*time.Second, verifyTimeout10, func() (ok bool, err error) {
			team := s2hv1beta1.Team{}
			if err := runtimeClient.Get(ctx, types.NamespacedName{Name: teamName}, &team); err != nil {
				return false, nil
			}

			for i, c := range team.Status.Conditions {
				if c.Type == s2hv1beta1.TeamConfigExisted {
					if team.Status.Conditions[i].Status == corev1.ConditionFalse {
						return true, nil
					}
				}
			}

			return false, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Team should be error if missing Config")
	}, 15)

	It("should create DesiredComponent on team staging namespace", func(done Done) {
		defer close(done)

		By("Starting Samsahai internal process")
		go samsahaiCtrl.Start(chStop)

		By("Starting http server")
		mux := http.NewServeMux()
		mux.Handle(samsahaiCtrl.PathPrefix(), samsahaiCtrl)
		mux.Handle("/", s2hhttp.New(samsahaiCtrl))
		server := httptest.NewServer(mux)
		defer server.Close()

		ctx := context.TODO()

		By("Creating Config")
		config := mockConfig
		Expect(runtimeClient.Create(ctx, &config)).To(BeNil())

		By("Creating Team")
		team := mockTeam
		Expect(runtimeClient.Create(ctx, &team)).To(BeNil())

		By("Verifying namespace and config have been created")
		err = wait.PollImmediate(1*time.Second, verifyNSCreatedTimeout, func() (ok bool, err error) {
			namespace := corev1.Namespace{}
			if err := runtimeClient.Get(ctx, types.NamespacedName{Name: stgNamespace}, &namespace); err != nil {
				return false, nil
			}

			config := s2hv1beta1.Config{}
			err = runtimeClient.Get(ctx, types.NamespacedName{Name: team.Name}, &config)
			if err != nil {
				return false, nil
			}

			return true, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Verify namespace and config error")

		By("Send webhook")
		component := "redis"
		jsonData, err := json.Marshal(map[string]interface{}{
			"component": component,
		})
		Expect(err).NotTo(HaveOccurred())
		_, err = utilhttp.Post(server.URL+"/webhook/component", jsonData)
		Expect(err).NotTo(HaveOccurred())

		By("Verifying DesiredComponent has been created")
		err = wait.PollImmediate(1*time.Second, 30*time.Second, func() (ok bool, err error) {
			_, _ = utilhttp.Post(server.URL+"/webhook/component", jsonData)

			dc := s2hv1beta1.DesiredComponent{}
			if err = runtimeClient.Get(ctx, types.NamespacedName{Name: component, Namespace: stgNamespace}, &dc); err != nil {
				return false, nil
			}

			return true, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Verify DesiredComponent error")
	}, 60)

	It("should detect image missing and not create desired component", func(done Done) {
		defer close(done)

		By("Starting Samsahai internal process")
		go samsahaiCtrl.Start(chStop)

		By("Starting http server")
		mux := http.NewServeMux()
		mux.Handle(samsahaiCtrl.PathPrefix(), samsahaiCtrl)
		mux.Handle("/", s2hhttp.New(samsahaiCtrl))
		server := httptest.NewServer(mux)
		defer server.Close()

		ctx := context.TODO()

		By("Creating Config")
		config := mockConfig
		redisComp := redisConfigComp
		redisComp.Image.Repository = "bitnami/rediss"
		redisComp.Image.Pattern = "image-missing"
		redisComp.Values = map[string]interface{}{
			"image": map[string]interface{}{
				"repository": "bitnami/rediss",
			},
		}
		config.Spec.Components = []*s2hv1beta1.Component{&redisComp}
		Expect(runtimeClient.Create(ctx, &config)).To(BeNil())

		By("Creating Team")
		team := mockTeam
		Expect(runtimeClient.Create(ctx, &team)).To(BeNil())

		By("Verifying namespace and config have been created")
		err = wait.PollImmediate(1*time.Second, verifyNSCreatedTimeout, func() (ok bool, err error) {
			namespace := corev1.Namespace{}
			if err := runtimeClient.Get(ctx, types.NamespacedName{Name: stgNamespace}, &namespace); err != nil {
				return false, nil
			}

			config := s2hv1beta1.Config{}
			err = runtimeClient.Get(ctx, types.NamespacedName{Name: team.Name}, &config)
			if err != nil {
				return false, nil
			}

			return true, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Verify namespace and config error")

		components, err := cfgCtrl.GetComponents(team.Name)
		Expect(err).NotTo(HaveOccurred())

		By("Send webhook")
		component := "redis"
		jsonData, err := json.Marshal(map[string]interface{}{
			"component": component,
		})

		componentRepository := components[component].Image.Repository
		Expect(err).NotTo(HaveOccurred())
		_, err = utilhttp.Post(server.URL+"/webhook/component", jsonData)
		Expect(err).NotTo(HaveOccurred())
		Expect(componentRepository).NotTo(Equal(""))

		By("Get Team")
		Expect(runtimeClient.Get(ctx, types.NamespacedName{Name: teamName}, &team)).NotTo(HaveOccurred())

		By("Verifying DesiredComponentImageCreatedTime has been updated")
		err = wait.PollImmediate(1*time.Second, verifyTimeout10, func() (ok bool, err error) {
			teamComp := s2hv1beta1.Team{}
			if err := runtimeClient.Get(ctx, types.NamespacedName{Name: team.Name}, &teamComp); err != nil {
				return false, nil
			}

			image := stringutils.ConcatImageString(componentRepository, "image-missing")
			if _, ok = teamComp.Status.DesiredComponentImageCreatedTime["redis"][image]; !ok {
				return false, nil
			}

			return true, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Update DesiredComponentImageCreatedTime error")

		By("Verifying DesiredComponent has not been created")
		foundCh := make(chan bool)
		go func() {
			const maxCount = 2
			count := 0
			for count < maxCount {
				dc := s2hv1beta1.DesiredComponent{}
				err := runtimeClient.Get(
					ctx,
					types.NamespacedName{Name: component, Namespace: team.Status.Namespace.Staging},
					&dc)
				if err != nil {
					count++
					time.Sleep(time.Second)
					continue
				}

				foundCh <- true
				return
			}
			foundCh <- false
		}()
		found := <-foundCh
		Expect(found).To(BeFalse())

	}, 60)

	XIt("should correctly get image missing list", func(done Done) {
		defer close(done)

		// TODO: should check image missing faster, maybe use the image that have less tags than bitnami/*

		By("Starting Samsahai internal process")
		go samsahaiCtrl.Start(chStop)

		By("Starting http server")
		mux := http.NewServeMux()
		mux.Handle(samsahaiCtrl.PathPrefix(), samsahaiCtrl)
		mux.Handle("/", s2hhttp.New(samsahaiCtrl))
		server := httptest.NewServer(mux)
		defer server.Close()

		ctx := context.TODO()

		By("Creating Config")
		config := mockConfig
		Expect(runtimeClient.Create(ctx, &config)).To(BeNil())

		By("Creating Team")
		team := mockTeam
		Expect(runtimeClient.Create(ctx, &team)).To(BeNil())

		By("Verifying namespace and config have been created")
		err = wait.PollImmediate(1*time.Second, verifyTimeout10, func() (ok bool, err error) {
			namespace := corev1.Namespace{}
			if err := runtimeClient.Get(ctx, types.NamespacedName{Name: stgNamespace}, &namespace); err != nil {
				return false, nil
			}

			config := s2hv1beta1.Config{}
			err = runtimeClient.Get(ctx, types.NamespacedName{Name: team.Name}, &config)
			if err != nil {
				return false, nil
			}

			return true, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Verify namespace and config error")

		By("Creating StableComponent")
		smd := stableMariaDB
		smd.Spec.Version = "10.3.18-debian-9-r32-missing"
		Expect(runtimeClient.Create(ctx, &smd)).To(BeNil())

		By("Set up RPC")
		headers := make(http.Header)
		headers.Set(internal.SamsahaiAuthHeader, samsahaiAuthToken)
		ctx, err = twirp.WithHTTPRequestHeaders(ctx, headers)
		Expect(err).NotTo(HaveOccurred(), "should set request headers successfully")

		By("RPC GetMissingVersion")
		comp := &samsahairpc.TeamWithCurrentComponent{
			TeamName: team.Name,
			CompName: stableMariaDB.Name,
			Image:    &samsahairpc.Image{Repository: stableMariaDB.Spec.Repository, Tag: stableMariaDB.Spec.Version},
		}
		imgList, err := samsahaiClient.GetMissingVersion(ctx, comp)
		Expect(err).NotTo(HaveOccurred())
		Expect(imgList).NotTo(BeNil())
		Expect(imgList.Images).To(BeNil(), "should not get image missing list")

		comp = &samsahairpc.TeamWithCurrentComponent{
			TeamName: team.Name,
			CompName: stableRedis.Name,
			Image:    &samsahairpc.Image{Repository: stableRedis.Spec.Repository, Tag: stableRedis.Spec.Version},
		}
		imgList, err = samsahaiClient.GetMissingVersion(ctx, comp)
		Expect(err).NotTo(HaveOccurred())
		Expect(imgList).NotTo(BeNil())
		Expect(imgList.Images).NotTo(BeNil())
		Expect(len(imgList.Images)).To(Equal(1), "should get image missing list")

	}, 150)
})

var _ = Describe("Main Controller Promote On Team Creation [e2e]", func() {
	BeforeEach(func(done Done) {
		defer close(done)

		chStop = make(chan struct{})

		adminRestConfig, err := config.GetConfig()
		Expect(err).NotTo(HaveOccurred(), "Please provide credential for accessing k8s cluster")

		adminClient, err := crclient.New(adminRestConfig, crclient.Options{Scheme: scheme.Scheme})
		Expect(err).NotTo(HaveOccurred(), "should create runtime client successfully")

		ctx := context.TODO()
		// get token for samsahai user
		restCfg = rest.CopyConfig(adminRestConfig)
		restCfg.Username = ""
		// get token
		samsahaiSA := &corev1.ServiceAccount{}
		err = adminClient.Get(ctx, types.NamespacedName{Name: "samsahai", Namespace: samsahaiSystemNs}, samsahaiSA)
		Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("cannot get sa: %s/%s", samsahaiSystemNs, "samsahai"))
		Expect(len(samsahaiSA.Secrets)).To(BeNumerically(">=", 1))
		samsahaiSecret := &corev1.Secret{}
		err = adminClient.Get(ctx, types.NamespacedName{Namespace: samsahaiSystemNs, Name: samsahaiSA.Secrets[0].Name}, samsahaiSecret)
		Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("cannot get secret: %s/%s", samsahaiSystemNs, samsahaiSA.Secrets[0].Name))
		restCfg.BearerToken = string(samsahaiSecret.Data["token"])

		mgr, err = manager.New(restCfg, manager.Options{MetricsBindAddress: "0"})
		Expect(err).NotTo(HaveOccurred(), "should create manager successfully")

		runtimeClient, err = crclient.New(restCfg, crclient.Options{Scheme: scheme.Scheme})
		Expect(err).NotTo(HaveOccurred(), "should create runtime client successfully")

		Expect(os.Setenv("S2H_CONFIG_PATH", "../data/application.yaml")).NotTo(HaveOccurred(),
			"should sent samsahai file config path successfully")
		s2hConfig := internal.SamsahaiConfig{
			ActivePromotion: internal.ActivePromotionConfig{
				Concurrences:          1,
				Timeout:               metav1.Duration{Duration: 5 * time.Minute},
				DemotionTimeout:       metav1.Duration{Duration: 1 * time.Second},
				RollbackTimeout:       metav1.Duration{Duration: 10 * time.Second},
				TearDownDuration:      metav1.Duration{Duration: 1 * time.Second},
				MaxHistories:          2,
				PromoteOnTeamCreation: true,
			},
			SamsahaiCredential: internal.SamsahaiCredential{
				InternalAuthToken: samsahaiAuthToken,
			},
		}
		cfgCtrl = configctrl.New(mgr)
		Expect(cfgCtrl).ToNot(BeNil())

		samsahaiCtrl = samsahai.New(mgr, "samsahai-system", s2hConfig, cfgCtrl)
		Expect(samsahaiCtrl).ToNot(BeNil())

		activePromotionCtrl = activepromotion.New(mgr, samsahaiCtrl, s2hConfig)
		Expect(activePromotionCtrl).ToNot(BeNil())

		stableComponentCtrl = stablecomponent.New(mgr, samsahaiCtrl)
		Expect(stableComponentCtrl).ToNot(BeNil())

		wgStop = &sync.WaitGroup{}
		wgStop.Add(1)
		go func() {
			defer wgStop.Done()
			Expect(mgr.Start(chStop)).To(BeNil())
		}()

		mux := http.NewServeMux()
		mux.Handle(samsahaiCtrl.PathPrefix(), samsahaiCtrl)
		mux.Handle("/", s2hhttp.New(samsahaiCtrl))
		samsahaiServer = httptest.NewServer(mux)
		samsahaiClient = samsahairpc.NewRPCProtobufClient(samsahaiServer.URL, &http.Client{})

		By("Creating Secret")
		secret := mockSecret
		_ = runtimeClient.Delete(context.TODO(), &secret)
		_ = runtimeClient.Create(context.TODO(), &secret)
	}, 60)

	AfterEach(func(done Done) {
		defer close(done)
		ctx := context.TODO()

		By("Deleting all StableComponents")
		err = runtimeClient.DeleteAllOf(ctx, &s2hv1beta1.StableComponent{}, crclient.InNamespace(stgNamespace))
		Expect(err).NotTo(HaveOccurred())

		By("Deleting all Teams")
		err = runtimeClient.DeleteAllOf(ctx, &s2hv1beta1.Team{}, crclient.MatchingLabels(testLabels))
		Expect(err).NotTo(HaveOccurred())
		err = wait.PollImmediate(1*time.Second, verifyTimeout10, func() (ok bool, err error) {
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
		Expect(err).NotTo(HaveOccurred(), "Delete all Teams error")

		By("Deleting all Configs")
		err = runtimeClient.DeleteAllOf(ctx, &s2hv1beta1.Config{}, crclient.MatchingLabels(testLabels))
		Expect(err).NotTo(HaveOccurred())
		err = wait.PollImmediate(1*time.Second, verifyTimeout10, func() (ok bool, err error) {
			configList := s2hv1beta1.ConfigList{}
			listOpt := &crclient.ListOptions{LabelSelector: labels.SelectorFromSet(testLabels)}
			err = runtimeClient.List(ctx, &configList, listOpt)
			if err != nil && errors.IsNotFound(err) {
				return true, nil
			}
			if len(configList.Items) == 0 {
				return true, nil
			}

			return false, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Deleting all Configs error")

		By("Deleting all ActivePromotions")
		err = runtimeClient.DeleteAllOf(ctx, &s2hv1beta1.ActivePromotion{}, crclient.MatchingLabels(testLabels))
		Expect(err).NotTo(HaveOccurred())
		err = wait.PollImmediate(1*time.Second, verifyTimeout10, func() (ok bool, err error) {
			atpList := s2hv1beta1.ActivePromotionList{}
			listOpt := &crclient.ListOptions{LabelSelector: labels.SelectorFromSet(testLabels)}
			err = runtimeClient.List(ctx, &atpList, listOpt)
			if err != nil && errors.IsNotFound(err) {
				return true, nil
			}
			if len(atpList.Items) == 0 {
				return true, nil
			}

			return false, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Delete all active promotions error")

		By("Deleting ActivePromotionHistories")
		err = runtimeClient.DeleteAllOf(ctx, &s2hv1beta1.ActivePromotionHistory{}, crclient.MatchingLabels(testLabels))
		Expect(err).NotTo(HaveOccurred())
		err = runtimeClient.DeleteAllOf(ctx, &s2hv1beta1.ActivePromotionHistory{}, crclient.MatchingLabels(defaultLabels))
		Expect(err).NotTo(HaveOccurred())

		By("Deleting Secret")
		secret := mockSecret
		Expect(runtimeClient.Delete(context.TODO(), &secret)).NotTo(HaveOccurred())

		close(chStop)
		samsahaiServer.Close()
		wgStop.Wait()
	}, 60)

	It("should successfully set new active namespace on new team creation", func(done Done) {
		defer close(done)
		ctx := context.TODO()

		By("Creating Config")
		config := mockConfig
		Expect(runtimeClient.Create(ctx, &config)).To(BeNil())

		By("Creating Team")
		team := mockTeam
		Expect(runtimeClient.Create(ctx, &team)).To(BeNil())

		By("Verifying namespace and config have been created")
		err = wait.PollImmediate(1*time.Second, verifyNSCreatedTimeout, func() (ok bool, err error) {
			namespace := corev1.Namespace{}
			if err := runtimeClient.Get(ctx, types.NamespacedName{Name: stgNamespace}, &namespace); err != nil {
				return false, nil
			}

			config := s2hv1beta1.Config{}
			err = runtimeClient.Get(ctx, types.NamespacedName{Name: team.Name}, &config)
			if err != nil {
				return false, nil
			}

			return true, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Create staging related object objects error")

		teamComp := s2hv1beta1.Team{}
		Expect(runtimeClient.Get(ctx, types.NamespacedName{Name: team.Name}, &teamComp))

		By("Waiting pre-active environment is successfully created")
		atpResCh := make(chan s2hv1beta1.ActivePromotion)
		go func() {
			atpTemp := s2hv1beta1.ActivePromotion{}
			for {
				_ = runtimeClient.Get(ctx, types.NamespacedName{Name: team.Name}, &atpTemp)
				if atpTemp.Status.IsConditionTrue(s2hv1beta1.ActivePromotionCondPreActiveCreated) {
					break
				}
				time.Sleep(500 * time.Millisecond)
			}
			atpResCh <- atpTemp
		}()
		atpRes := <-atpResCh

		By("Checking pre-active namespace has been set")
		Expect(runtimeClient.Get(ctx, types.NamespacedName{Name: team.Name}, &teamComp))
		Expect(teamComp.Status.Namespace.PreActive).ToNot(BeEmpty())
		Expect(atpRes.Status.TargetNamespace).To(Equal(teamComp.Status.Namespace.PreActive))
		Expect(atpRes.Status.PreviousActiveNamespace).To(BeEmpty())

		By("Start staging controller for pre-active")
		preActiveNs := atpRes.Status.TargetNamespace
		{
			stagingCfg := rest.CopyConfig(restCfg)
			stagingCfg.Username = ""
			// get token
			stagingSA := &corev1.ServiceAccount{}
			err = runtimeClient.Get(ctx, types.NamespacedName{Name: internal.StagingCtrlName, Namespace: preActiveNs}, stagingSA)
			Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("cannot get sa: %s", internal.StagingCtrlName))
			Expect(len(stagingSA.Secrets)).To(BeNumerically(">=", 1))
			stagingSecret := &corev1.Secret{}
			err = runtimeClient.Get(ctx, types.NamespacedName{Namespace: preActiveNs, Name: stagingSA.Secrets[0].Name}, stagingSecret)
			Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("cannot get secret: %s", stagingSA.Secrets[0].Name))
			stagingCfg.BearerToken = string(stagingSecret.Data["token"])

			// create mgr from config
			stagingMgr, err := manager.New(stagingCfg, manager.Options{
				Namespace:          preActiveNs,
				MetricsBindAddress: "0",
			})
			Expect(err).NotTo(HaveOccurred())

			stagingCfgCtrl := configctrl.New(stagingMgr)
			qCtrl := queue.New(preActiveNs, runtimeClient)
			stagingPreActiveCtrl = staging.NewController(teamName, preActiveNs, samsahaiAuthToken, samsahaiClient,
				stagingMgr, qCtrl, stagingCfgCtrl, "", "", "",
				internal.StagingConfig{})
			go func() {
				defer GinkgoRecover()
				Expect(stagingMgr.Start(chStop)).NotTo(HaveOccurred())
			}()
			go stagingPreActiveCtrl.Start(chStop)
		}

		By("ActivePromotion should be deleted")
		err = wait.PollImmediate(1*time.Second, promoteTimeOut, func() (ok bool, err error) {
			atpTemp := s2hv1beta1.ActivePromotion{}
			err = runtimeClient.Get(ctx, types.NamespacedName{Name: team.Name}, &atpTemp)
			if err != nil && errors.IsNotFound(err) {
				return true, nil
			}

			return false, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Delete active promotion error")

		By("Checking active namespace has been set")
		teamComp = s2hv1beta1.Team{}
		err = runtimeClient.Get(ctx, types.NamespacedName{Name: team.Name}, &teamComp)
		Expect(err).To(BeNil())
		Expect(teamComp.Status.Namespace.Active).To(Equal(preActiveNs))

		By("ActivePromotionHistory should be created")
		atpHists := &s2hv1beta1.ActivePromotionHistoryList{}
		listOpt := &crclient.ListOptions{LabelSelector: labels.SelectorFromSet(defaultLabels)}
		err = runtimeClient.List(context.TODO(), atpHists, listOpt)
		Expect(err).To(BeNil())
		Expect(len(atpHists.Items)).To(Equal(1))
		Expect(atpHists.Items[0].Spec.ActivePromotion.Status.OutdatedComponents).To(BeNil())

		By("Current active components should not be set as it is first time promotion")
		teamComp = s2hv1beta1.Team{}
		err = runtimeClient.Get(ctx, types.NamespacedName{Name: team.Name}, &teamComp)
		Expect(err).To(BeNil())
		Expect(len(teamComp.Status.ActiveComponents)).To(BeZero())

		By("Public API")
		{
			By("Get team")
			{
				data, err := utilhttp.Get(samsahaiServer.URL + "/teams/" + team.Name)
				Expect(err).NotTo(HaveOccurred())
				Expect(data).NotTo(BeNil())
				Expect(gjson.GetBytes(data, "teamName").Str).To(Equal(team.Name))
			}

			By("Get team Queue")
			{
				data, err := utilhttp.Get(samsahaiServer.URL + "/teams/" + team.Name + "/queue")
				Expect(err).NotTo(HaveOccurred())
				Expect(data).NotTo(BeNil())
			}

			By("Get team QueueHistories not found")
			{
				_, err := utilhttp.Get(samsahaiServer.URL + "/teams/" + team.Name + "/queue/histories/" + "unknown")
				Expect(err).To(HaveOccurred())
			}
		}
	}, 230)

	It("should not set active namespace on new team creation", func(done Done) {
		defer close(done)
		ctx := context.TODO()

		By("Creating Config")
		config := mockConfig
		Expect(runtimeClient.Create(ctx, &config)).To(BeNil())

		By("Creating Team")
		team := mockTeam
		Expect(runtimeClient.Create(ctx, &team)).To(BeNil())

		By("Verifying namespace and config have been created")
		err = wait.PollImmediate(1*time.Second, verifyNSCreatedTimeout, func() (ok bool, err error) {
			namespace := corev1.Namespace{}
			if err := runtimeClient.Get(ctx, types.NamespacedName{Name: stgNamespace}, &namespace); err != nil {
				return false, nil
			}

			config := s2hv1beta1.Config{}
			err = runtimeClient.Get(ctx, types.NamespacedName{Name: team.Name}, &config)
			if err != nil {
				return false, nil
			}

			return true, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Create staging related object objects error")

		By("Waiting pre-active environment is successfully created")
		atpResCh := make(chan s2hv1beta1.ActivePromotion)
		go func() {
			atpTemp := s2hv1beta1.ActivePromotion{}
			for {
				_ = runtimeClient.Get(ctx, types.NamespacedName{Name: team.Name}, &atpTemp)
				if atpTemp.Status.IsConditionTrue(s2hv1beta1.ActivePromotionCondPreActiveCreated) {
					break
				}
				time.Sleep(500 * time.Millisecond)
			}
			atpResCh <- atpTemp
		}()
		atpRes := <-atpResCh

		By("Checking pre-active namespace has been set")
		teamComp := s2hv1beta1.Team{}
		err = runtimeClient.Get(ctx, types.NamespacedName{Name: team.Name}, &teamComp)
		Expect(err).To(BeNil())
		Expect(runtimeClient.Get(ctx, types.NamespacedName{Name: team.Name}, &teamComp))
		Expect(teamComp.Status.Namespace.PreActive).ToNot(BeEmpty())
		Expect(atpRes.Status.TargetNamespace).To(Equal(teamComp.Status.Namespace.PreActive))
		Expect(atpRes.Status.PreviousActiveNamespace).To(BeEmpty())

		By("Waiting ActivePromotion state to be `Deploying`")
		err = wait.PollImmediate(1*time.Second, 60*time.Second, func() (ok bool, err error) {
			atpComp := s2hv1beta1.ActivePromotion{}
			if err := runtimeClient.Get(ctx, types.NamespacedName{Name: atpRes.Name}, &atpComp); err != nil {
				return false, nil
			}

			if atpComp.Status.State == s2hv1beta1.ActivePromotionDeployingComponents {
				return true, nil
			}

			return false, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Change active promotion state to `Deploying` error")

		By("Updating ActivePromotion state to be `DestroyingPreActiveEnvironment`")
		atpComp := s2hv1beta1.ActivePromotion{}
		Expect(runtimeClient.Get(ctx, types.NamespacedName{Name: atpRes.Name}, &atpComp))
		atpComp.Status.State = s2hv1beta1.ActivePromotionDestroyingPreActive
		atpComp.Status.SetCondition(s2hv1beta1.ActivePromotionCondPreActiveDestroyed, corev1.ConditionTrue, "failed")
		Expect(runtimeClient.Update(ctx, &atpComp)).To(BeNil())

		By("Delete ActivePromotion")
		atpComp = s2hv1beta1.ActivePromotion{}
		Expect(runtimeClient.Get(ctx, types.NamespacedName{Name: atpRes.Name}, &atpComp))
		Expect(runtimeClient.Delete(context.TODO(), &atpComp)).To(BeNil())

		By("pre-active namespace should be deleted")
		preActiveNs := atpComp.Status.TargetNamespace
		err = wait.PollImmediate(1*time.Second, 15*time.Second, func() (ok bool, err error) {
			namespace := corev1.Namespace{}
			err = runtimeClient.Get(ctx, types.NamespacedName{Name: preActiveNs}, &namespace)
			if err != nil && errors.IsNotFound(err) {
				return true, nil
			}

			return false, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Delete pre-active namespace error")

		By("ActivePromotion should be deleted")
		err = wait.PollImmediate(1*time.Second, verifyTimeout10, func() (ok bool, err error) {
			atpTemp := s2hv1beta1.ActivePromotion{}
			err = runtimeClient.Get(ctx, types.NamespacedName{Name: team.Name}, &atpTemp)
			if err != nil && errors.IsNotFound(err) {
				return true, nil
			}

			return false, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Delete active promotion error")

		atpHists := &s2hv1beta1.ActivePromotionHistoryList{}
		listOpt := &crclient.ListOptions{LabelSelector: labels.SelectorFromSet(defaultLabels)}
		err = runtimeClient.List(context.TODO(), atpHists, listOpt)
		Expect(err).To(BeNil())
		Expect(len(atpHists.Items)).To(Equal(1))
		Expect(atpHists.Items[0].Spec.ActivePromotion.Status.OutdatedComponents).To(BeNil())

		By("Checking only staging namespace left")
		teamComp = s2hv1beta1.Team{}
		err = runtimeClient.Get(ctx, types.NamespacedName{Name: team.Name}, &teamComp)
		Expect(err).To(BeNil())
		Expect(teamComp.Status.Namespace.Staging).ToNot(BeEmpty())
		Expect(teamComp.Status.Namespace.Active).To(BeEmpty())
		Expect(teamComp.Status.Namespace.PreActive).To(BeEmpty())

		By("Current active components should not be set")
		Expect(len(teamComp.Status.ActiveComponents)).To(BeZero())
	}, 60)
})
