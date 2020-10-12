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
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
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
	verifyTime1s           = 1 * time.Second
	verifyTime5s           = 5 * time.Second
	verifyTime10s          = 10 * time.Second
	verifyTime15s          = 15 * time.Second
	verifyTime30s          = 30 * time.Second
	verifyTime45s          = 45 * time.Second
	verifyTime60s          = 60 * time.Second
	verifyNSCreatedTimeout = verifyTime15s
	promoteTimeOut         = 30 * time.Second
)

var (
	samsahaiCtrl   internal.SamsahaiController
	client         rclient.Client
	wgStop         *sync.WaitGroup
	chStop         chan struct{}
	mgr            manager.Manager
	samsahaiServer *httptest.Server
	samsahaiClient samsahairpc.RPC
	restCfg        *rest.Config
	err            error
)

func setupSamsahai(isPromoteOnTeamCreationDisabled bool) {
	s2hConfig := samsahaiConfig
	if isPromoteOnTeamCreationDisabled {
		s2hConfig.ActivePromotion.PromoteOnTeamCreation = false
	}

	samsahaiCtrl = samsahai.New(mgr, "samsahai-system", s2hConfig)
	Expect(samsahaiCtrl).ToNot(BeNil())

	activePromotionCtrl := activepromotion.New(mgr, samsahaiCtrl, s2hConfig)
	Expect(activePromotionCtrl).ToNot(BeNil())

	stableComponentCtrl := stablecomponent.New(mgr, samsahaiCtrl)
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
}

var _ = Describe("[e2e] Main controller", func() {
	BeforeEach(func(done Done) {
		defer close(done)
		chStop = make(chan struct{})

		adminRestConfig, err := config.GetConfig()
		Expect(err).NotTo(HaveOccurred(), "Please provide credential for accessing k8s cluster")

		restCfg = rest.CopyConfig(adminRestConfig)
		mgr, err = manager.New(restCfg, manager.Options{MetricsBindAddress: "0"})
		Expect(err).NotTo(HaveOccurred(), "should create manager successfully")

		client, err = rclient.New(restCfg, rclient.Options{Scheme: scheme.Scheme})
		Expect(err).NotTo(HaveOccurred(), "should create runtime client successfully")

		Expect(os.Setenv("S2H_CONFIG_PATH", "../data/application.yaml")).NotTo(HaveOccurred(),
			"should sent samsahai file config path successfully")

		ctx := context.TODO()
		By("Creating Secret")
		secret := mockSecret
		_ = client.Create(ctx, &secret)

		By("Deleting all ActivePromotions before running")
		err = client.DeleteAllOf(ctx, &s2hv1beta1.ActivePromotion{}, rclient.MatchingLabels(testLabels))
		Expect(err).NotTo(HaveOccurred())
		err = wait.PollImmediate(verifyTime1s, verifyTime30s, func() (ok bool, err error) {
			atpList := s2hv1beta1.ActivePromotionList{}
			listOpt := &rclient.ListOptions{LabelSelector: labels.SelectorFromSet(testLabels)}
			if err = client.List(ctx, &atpList, listOpt); err != nil {
				return false, nil
			}

			if len(atpList.Items) == 0 {
				return true, nil
			}

			return false, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Delete all active promotions before running error")
	}, 60)

	AfterEach(func(done Done) {
		defer close(done)
		ctx := context.TODO()

		By("Deleting all Teams")
		err = client.DeleteAllOf(ctx, &s2hv1beta1.Team{}, rclient.MatchingLabels(testLabels))
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
		Expect(err).NotTo(HaveOccurred(), "Deleting all Configs error")

		By("Deleting all DesiredComponents")
		err = client.DeleteAllOf(ctx, &s2hv1beta1.DesiredComponent{}, rclient.InNamespace(stgNamespace))
		Expect(err).NotTo(HaveOccurred())

		By("Deleting all Queues")
		err = client.DeleteAllOf(ctx, &s2hv1beta1.Queue{}, rclient.InNamespace(stgNamespace))
		Expect(err).NotTo(HaveOccurred())

		By("Deleting all StableComponents")
		err = client.DeleteAllOf(ctx, &s2hv1beta1.StableComponent{}, rclient.InNamespace(stgNamespace))
		Expect(err).NotTo(HaveOccurred())
		err = wait.PollImmediate(verifyTime1s, verifyTime10s, func() (ok bool, err error) {
			stableList := s2hv1beta1.StableComponentList{}
			err = client.List(ctx, &stableList, &rclient.ListOptions{Namespace: stgNamespace})
			if err != nil && errors.IsNotFound(err) {
				return true, nil
			}
			if len(stableList.Items) == 0 {
				return true, nil
			}

			return false, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Deleting all StableComponents error")

		By("Deleting active namespace")
		atvNs := activeNamespace
		_ = client.Delete(context.TODO(), &atvNs)
		err = wait.PollImmediate(verifyTime1s, verifyTime10s, func() (ok bool, err error) {
			namespace := corev1.Namespace{}
			err = client.Get(ctx, types.NamespacedName{Name: atvNamespace}, &namespace)
			if err != nil && errors.IsNotFound(err) {
				return true, nil
			}
			return false, nil
		})

		By("Deleting all ActivePromotions")
		err = client.DeleteAllOf(ctx, &s2hv1beta1.ActivePromotion{}, rclient.MatchingLabels(testLabels))
		Expect(err).NotTo(HaveOccurred())
		err = wait.PollImmediate(verifyTime1s, verifyTime30s, func() (ok bool, err error) {
			atpList := s2hv1beta1.ActivePromotionList{}
			listOpt := &rclient.ListOptions{LabelSelector: labels.SelectorFromSet(testLabels)}
			if err = client.List(ctx, &atpList, listOpt); err != nil {
				return false, nil
			}

			if len(atpList.Items) == 0 {
				return true, nil
			}

			return false, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Delete all active promotions error")

		By("Deleting ActivePromotionHistories")
		err = client.DeleteAllOf(ctx, &s2hv1beta1.ActivePromotionHistory{}, rclient.MatchingLabels(testLabels))
		Expect(err).NotTo(HaveOccurred())
		err = client.DeleteAllOf(ctx, &s2hv1beta1.ActivePromotionHistory{}, rclient.MatchingLabels(defaultLabels))
		Expect(err).NotTo(HaveOccurred())
		err = client.DeleteAllOf(ctx, &s2hv1beta1.ActivePromotionHistory{}, rclient.MatchingLabels(defaultLabelsQ1))
		Expect(err).NotTo(HaveOccurred())
		err = client.DeleteAllOf(ctx, &s2hv1beta1.ActivePromotionHistory{}, rclient.MatchingLabels(defaultLabelsQ2))
		Expect(err).NotTo(HaveOccurred())
		err = client.DeleteAllOf(ctx, &s2hv1beta1.ActivePromotionHistory{}, rclient.MatchingLabels(defaultLabelsQ3))
		Expect(err).NotTo(HaveOccurred())

		By("Deleting Secret")
		secret := mockSecret
		Expect(client.Delete(context.TODO(), &secret)).NotTo(HaveOccurred())

		By("Deleting Config")
		Expect(samsahaiCtrl.GetConfigController().Delete(teamName)).NotTo(HaveOccurred())

		close(chStop)
		samsahaiServer.Close()
		wgStop.Wait()
	}, 90)

	It("should successfully promote an active environment without doing retry", func(done Done) {
		defer close(done)
		setupSamsahai(true)
		ctx := context.TODO()

		By("Creating Config")
		config := mockConfig
		config.Spec.ActivePromotion.MaxRetry = &maxActivePromotionRetry
		Expect(client.Create(ctx, &config)).To(BeNil())

		By("Creating Team")
		team := mockTeam
		team.Status.Namespace.Active = atvNamespace
		Expect(client.Create(ctx, &team)).To(BeNil())

		By("Verifying staging related objects has been created")
		err = wait.PollImmediate(verifyTime1s, verifyNSCreatedTimeout, func() (ok bool, err error) {
			namespace := corev1.Namespace{}
			err = client.Get(ctx, types.NamespacedName{Name: stgNamespace}, &namespace)
			if err != nil {
				return false, nil
			}

			secret := corev1.Secret{}
			err = client.Get(ctx, types.NamespacedName{Name: internal.StagingCtrlName, Namespace: stgNamespace}, &secret)
			if err != nil {
				return false, nil
			}

			// TODO: uncomment when staging can be successfully deployed
			//deployment := appv1.Deployment{}
			//err = client.Get(ctx, types.NamespacedName{Name: internal.StagingCtrlName, Namespace: stgNamespace}, &deployment)
			//if err != nil || deployment.Status.AvailableReplicas != *deployment.Spec.Replicas {
			//	time.Sleep(500 * time.Millisecond)
			//	continue
			//}

			svc := corev1.Service{}
			err = client.Get(ctx, types.NamespacedName{Name: internal.StagingCtrlName, Namespace: stgNamespace}, &svc)
			if err != nil {
				return false, nil
			}

			role := rbacv1.Role{}
			err = client.Get(ctx, types.NamespacedName{Name: internal.StagingCtrlName, Namespace: stgNamespace}, &role)
			if err != nil {
				return false, nil
			}

			roleBinding := rbacv1.RoleBinding{}
			err = client.Get(ctx, types.NamespacedName{Name: internal.StagingCtrlName, Namespace: stgNamespace}, &roleBinding)
			if err != nil {
				return false, nil
			}

			clusterRole := rbacv1.ClusterRole{}
			err = client.Get(ctx, types.NamespacedName{Name: s2hobject.GenClusterRoleName(stgNamespace), Namespace: stgNamespace}, &clusterRole)
			if err != nil {
				return false, nil
			}

			clusterRoleBinding := rbacv1.ClusterRoleBinding{}
			err = client.Get(ctx, types.NamespacedName{Name: s2hobject.GenClusterRoleName(stgNamespace), Namespace: stgNamespace}, &clusterRoleBinding)
			if err != nil {
				return false, nil
			}

			sa := corev1.ServiceAccount{}
			err = client.Get(ctx, types.NamespacedName{Name: internal.StagingCtrlName, Namespace: stgNamespace}, &sa)
			if err != nil {
				return false, nil
			}

			config := s2hv1beta1.Config{}
			err = client.Get(ctx, types.NamespacedName{Name: team.Name}, &config)
			if err != nil {
				return false, nil
			}

			return true, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Create staging related object objects error")

		By("Creating active namespace")
		atvNs := activeNamespace
		Expect(client.Create(ctx, &atvNs)).To(BeNil())

		By("Creating StableComponent")
		smd := stableMariaDB
		Expect(client.Create(ctx, &smd)).To(BeNil())

		time.Sleep(1 * time.Second)
		By("Checking stable component has been set")
		teamComp := s2hv1beta1.Team{}
		Expect(client.Get(ctx, types.NamespacedName{Name: team.Name}, &teamComp))
		teamSpecStableComps := teamComp.Status.StableComponents[mariaDBCompName].Spec
		Expect(teamSpecStableComps.Name).To(Equal(stableAtvMariaDB.Spec.Name))
		Expect(teamSpecStableComps.Repository).To(Equal(stableAtvMariaDB.Spec.Repository))
		Expect(teamSpecStableComps.Version).To(Equal(stableAtvMariaDB.Spec.Version))

		By("Creating ActivePromotionHistory 1")
		atpHist := activePromotionHistory
		atpHist.Name = atpHist.Name + "-1"
		Expect(client.Create(ctx, &atpHist)).To(BeNil())

		time.Sleep(1 * time.Second)
		By("Creating ActivePromotionHistory 2")
		atpHist = activePromotionHistory
		atpHist.Name = atpHist.Name + "-2"
		Expect(client.Create(ctx, &atpHist)).To(BeNil())

		By("Creating ActivePromotion")
		atp := activePromotion
		Expect(client.Create(ctx, &atp)).To(BeNil())

		By("Creating mock de-active queue for active namespace")
		deActiveQ := mockDeActiveQueue
		deActiveQ.Namespace = atvNamespace
		Expect(client.Create(ctx, &deActiveQ)).To(BeNil())

		By("Waiting pre-active environment is successfully created")
		err = wait.PollImmediate(verifyTime1s, verifyTime10s, func() (ok bool, err error) {
			atpTemp := s2hv1beta1.ActivePromotion{}
			err = client.Get(ctx, types.NamespacedName{Name: atp.Name}, &atpTemp)
			if err != nil {
				return false, nil
			}

			if atpTemp.Status.IsConditionTrue(s2hv1beta1.ActivePromotionCondPreActiveCreated) {
				return true, nil
			}

			return false, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Delete previous namespace error")

		atpRes := s2hv1beta1.ActivePromotion{}
		err = client.Get(ctx, types.NamespacedName{Name: atp.Name}, &atpRes)
		Expect(err).NotTo(HaveOccurred(), "Get active promotion error")

		By("Start staging controller for pre-active")
		preActiveNs := atpRes.Status.TargetNamespace
		{
			// create mgr from config
			stagingCfg := rest.CopyConfig(restCfg)
			stagingMgr, err := manager.New(stagingCfg, manager.Options{
				Namespace:          preActiveNs,
				MetricsBindAddress: "0",
			})
			Expect(err).NotTo(HaveOccurred())

			stagingCfgCtrl := configctrl.New(stagingMgr)
			qctrl := queue.New(preActiveNs, client)
			stagingPreActiveCtrl := staging.NewController(teamName, preActiveNs, samsahaiAuthToken, samsahaiClient,
				stagingMgr, qctrl, stagingCfgCtrl, "", "", "",
				internal.StagingConfig{})
			go func() {
				defer GinkgoRecover()
				Expect(stagingMgr.Start(chStop)).NotTo(HaveOccurred())
			}()
			go stagingPreActiveCtrl.Start(chStop)
		}

		By("Checking pre-active namespace has been set")
		Expect(client.Get(ctx, types.NamespacedName{Name: atp.Name}, &teamComp))
		Expect(teamComp.Status.Namespace.PreActive).ToNot(BeEmpty())
		Expect(atpRes.Status.TargetNamespace).To(Equal(teamComp.Status.Namespace.PreActive))
		Expect(atpRes.Status.PreviousActiveNamespace).To(Equal(atvNamespace))

		By("Checking stable components has been deployed to target namespace")
		stableComps := &s2hv1beta1.StableComponentList{}
		err = client.List(ctx, stableComps, &rclient.ListOptions{Namespace: atpRes.Status.TargetNamespace})
		Expect(err).To(BeNil())
		Expect(len(stableComps.Items)).To(Equal(1))

		By("Previous active namespace should be deleted")
		err = wait.PollImmediate(verifyTime1s, promoteTimeOut, func() (ok bool, err error) {
			namespace := corev1.Namespace{}
			err = client.Get(ctx, types.NamespacedName{Name: atvNamespace}, &namespace)
			if err != nil && errors.IsNotFound(err) {
				return true, nil
			}

			return false, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Delete previous namespace error")

		By("ActivePromotion should be deleted")
		err = wait.PollImmediate(verifyTime1s, verifyTime30s, func() (ok bool, err error) {
			atpTemp := s2hv1beta1.ActivePromotion{}
			err = client.Get(ctx, types.NamespacedName{Name: atp.Name}, &atpTemp)
			if err != nil && errors.IsNotFound(err) {
				return true, nil
			}

			return false, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Delete active promotion error")

		By("Checking active, previous namespace and active promoted by has been reset")
		teamComp = s2hv1beta1.Team{}
		err = client.Get(ctx, types.NamespacedName{Name: atp.Name}, &teamComp)
		Expect(err).To(BeNil())
		Expect(teamComp.Status.Namespace.Active).To(Equal(preActiveNs))
		Expect(teamComp.Status.ActivePromotedBy).To(Equal(activePromotion.Spec.PromotedBy))

		err = client.Get(ctx, types.NamespacedName{Name: atvNamespace}, &atvNs)
		Expect(errors.IsNotFound(err)).To(BeTrue())

		By("Current active components should be set")
		teamComp = s2hv1beta1.Team{}
		err = client.Get(ctx, types.NamespacedName{Name: team.Name}, &teamComp)
		Expect(err).To(BeNil())
		Expect(len(teamComp.Status.ActiveComponents)).ToNot(BeZero())

		By("ActivePromotionHistory should be created")
		atpHists := &s2hv1beta1.ActivePromotionHistoryList{}
		listOpt := &rclient.ListOptions{LabelSelector: labels.SelectorFromSet(defaultLabels)}
		err = client.List(context.TODO(), atpHists, listOpt)
		Expect(err).To(BeNil())
		Expect(len(atpHists.Items)).To(Equal(2))
		Expect(atpHists.Items[0].Name).ToNot(Equal(atpHist.Name + "-1"))
		Expect(atpHists.Items[1].Name).ToNot(Equal(atpHist.Name + "-1"))
		Expect(atpHists.Items[1].Spec.ActivePromotion.Status.OutdatedComponents).ToNot(BeNil())

		By("Public API")
		{
			By("Get team")
			{
				_, data, err := utilhttp.Get(samsahaiServer.URL + "/teams/" + team.Name)
				Expect(err).NotTo(HaveOccurred())
				Expect(data).NotTo(BeNil())
				Expect(gjson.GetBytes(data, "teamName").Str).To(Equal(team.Name))
			}

			By("Get team Queue")
			{
				_, data, err := utilhttp.Get(samsahaiServer.URL + "/teams/" + team.Name + "/queue")
				Expect(err).NotTo(HaveOccurred())
				Expect(data).NotTo(BeNil())
			}

			By("Get team Queue not found")
			{
				_, _, err := utilhttp.Get(samsahaiServer.URL + "/teams/" + team.Name + "/queue/histories/" + "unknown")
				Expect(err).To(HaveOccurred())
			}

			By("Get Stable Values")
			{
				parentComps, err := samsahaiCtrl.GetConfigController().GetParentComponents(team.Name)
				Expect(err).NotTo(HaveOccurred())

				compName := ""
				for c := range parentComps {
					compName = c
				}

				url := fmt.Sprintf("%s/teams/%s/components/%s/values", samsahaiServer.URL, team.Name, compName)
				_, data, err := utilhttp.Get(url, utilhttp.WithHeader("Accept", "text/yaml"))
				Expect(err).NotTo(HaveOccurred())
				Expect(data).NotTo(BeNil())
			}
		}
	}, 90)

	It("should successfully promote an active environment even demote timeout", func(done Done) {
		defer close(done)
		setupSamsahai(true)
		ctx := context.TODO()

		By("Creating Config")
		config := mockConfig
		Expect(client.Create(ctx, &config)).To(BeNil())

		By("Creating Team")
		team := mockTeam
		team.Status.Namespace.Active = atvNamespace
		Expect(client.Create(ctx, &team)).To(BeNil())

		By("Creating active namespace")
		atvNs := activeNamespace
		Expect(client.Create(ctx, &atvNs)).To(BeNil())

		By("Verifying namespace and config have been created")
		err = wait.PollImmediate(verifyTime1s, verifyNSCreatedTimeout, func() (ok bool, err error) {
			namespace := corev1.Namespace{}
			if err := client.Get(ctx, types.NamespacedName{Name: stgNamespace}, &namespace); err != nil {
				return false, nil
			}

			config := s2hv1beta1.Config{}
			err = client.Get(ctx, types.NamespacedName{Name: team.Name}, &config)
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
		Expect(client.Create(ctx, &atp)).To(BeNil())

		By("Waiting ActivePromotion state to be `PromotingActiveEnvironment`")
		err = wait.PollImmediate(verifyTime1s, verifyTime10s, func() (ok bool, err error) {
			atpComp := s2hv1beta1.ActivePromotion{}
			if err := client.Get(ctx, types.NamespacedName{Name: teamName}, &atpComp); err != nil {
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
		setupSamsahai(true)
		ctx := context.TODO()

		By("Creating Team for Q1")
		team1 := mockTeam
		team1.Name = teamForQ1
		Expect(client.Create(ctx, &team1)).To(BeNil())

		By("Creating Config for Q1")
		config1 := mockConfig
		config1.Name = teamForQ1
		Expect(client.Create(ctx, &config1)).To(BeNil())

		By("Creating Team for Q2")
		team2 := mockTeam
		team2.Name = teamForQ2
		Expect(client.Create(ctx, &team2)).To(BeNil())

		By("Creating Config for Q2")
		config2 := mockConfig
		config2.Name = teamForQ2
		Expect(client.Create(ctx, &config2)).To(BeNil())

		By("Creating Team for Q3")
		team3 := mockTeam
		team3.Name = teamForQ3
		Expect(client.Create(ctx, &team3)).To(BeNil())
		By("Verifying configuration has been created")

		By("Creating Config for Q3")
		config3 := mockConfig
		config3.Name = teamForQ3
		Expect(client.Create(ctx, &config3)).To(BeNil())

		By("Verifying all teams have been created")
		err = wait.PollImmediate(verifyTime1s, verifyTime10s, func() (ok bool, err error) {
			teamList := s2hv1beta1.TeamList{}
			listOpt := &rclient.ListOptions{LabelSelector: labels.SelectorFromSet(testLabels)}
			if err := client.List(ctx, &teamList, listOpt); err != nil {
				return false, nil
			}

			if len(teamList.Items) == 3 {
				return true, nil
			}

			configList := s2hv1beta1.ConfigList{}
			if err := client.List(ctx, &configList, listOpt); err != nil {
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
		Expect(client.Create(ctx, &atpQ1)).To(BeNil())

		time.Sleep(1 * time.Second)

		atpQ2 := activePromotion
		atpQ2.Name = teamForQ2
		Expect(client.Create(ctx, &atpQ2)).To(BeNil())

		time.Sleep(verifyTime1s)
		atpQ3 := activePromotion
		atpQ3.Name = teamForQ3
		Expect(client.Create(ctx, &atpQ3)).To(BeNil())

		By("Waiting ActivePromotion Q1 state to be `Deploying`, other ActivePromotion states to be waiting")
		err = wait.PollImmediate(verifyTime1s, verifyTime10s, func() (ok bool, err error) {
			atpCompQ1 := s2hv1beta1.ActivePromotion{}
			if err := client.Get(ctx, types.NamespacedName{Name: teamForQ1}, &atpCompQ1); err != nil {
				return false, nil
			}

			if atpCompQ1.Status.State != s2hv1beta1.ActivePromotionDeployingComponents {
				return false, nil
			}

			waitingAtpList := &s2hv1beta1.ActivePromotionList{}
			selectors := map[string]string{"state": "waiting"}
			listOpt := &rclient.ListOptions{LabelSelector: labels.SelectorFromSet(selectors)}
			if err := client.List(ctx, waitingAtpList, listOpt); err != nil {
				return false, nil
			}

			if len(waitingAtpList.Items) == 2 {
				return true, nil
			}

			return false, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Change active promotion state error")

		By("Deleting ActivePromotion Q2 from queue")
		atpCompQ2 := s2hv1beta1.ActivePromotion{}
		Expect(client.Get(ctx, types.NamespacedName{Name: teamForQ2}, &atpCompQ2)).To(BeNil())
		Expect(client.Delete(context.TODO(), &atpCompQ2)).NotTo(HaveOccurred())
		err = wait.PollImmediate(verifyTime1s, verifyTime10s, func() (ok bool, err error) {
			atpTemp := s2hv1beta1.ActivePromotion{}
			err = client.Get(ctx, types.NamespacedName{Name: teamForQ2}, &atpTemp)
			if err != nil && errors.IsNotFound(err) {
				return true, nil
			}

			return false, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Delete active promotion for Team2 error")

		atpCompQ3 := s2hv1beta1.ActivePromotion{}
		Expect(client.Get(ctx, types.NamespacedName{Name: teamForQ3}, &atpCompQ3)).To(BeNil())
		Expect(atpCompQ3.Status.State).To(Equal(s2hv1beta1.ActivePromotionWaiting))

		By("Deleting ActivePromotion Q1")
		atpCompQ1 := s2hv1beta1.ActivePromotion{}
		Expect(client.Get(ctx, types.NamespacedName{Name: teamForQ1}, &atpCompQ1)).To(BeNil())
		Expect(client.Delete(context.TODO(), &atpCompQ1)).NotTo(HaveOccurred())

		By("Creating mock de-active Q1")
		preActiveNs := atpCompQ1.Status.TargetNamespace
		deActiveQ := mockDeActiveQueue
		deActiveQ.Namespace = preActiveNs
		Expect(client.Create(ctx, &deActiveQ)).To(BeNil())

		By("Verifying delete ActivePromotion Q1")
		err = wait.PollImmediate(verifyTime1s, verifyTime30s, func() (ok bool, err error) {
			atpTemp := s2hv1beta1.ActivePromotion{}
			err = client.Get(ctx, types.NamespacedName{Name: teamForQ1}, &atpTemp)
			if err != nil && errors.IsNotFound(err) {
				return true, nil
			}

			return false, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Delete active promotion for Team1 error")

		By("Checking ActivePromotion Q3 should be run")
		err = wait.PollImmediate(verifyTime1s, verifyTime10s, func() (ok bool, err error) {
			atpTemp := s2hv1beta1.ActivePromotion{}
			if err := client.Get(ctx, types.NamespacedName{Name: teamForQ3}, &atpTemp); err != nil {
				return false, nil
			}

			if atpTemp.Status.State == s2hv1beta1.ActivePromotionDeployingComponents {
				return true, nil
			}

			return false, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Promote Team3 error")

	}, 60)

	It("should do retry if active promotion fail", func(done Done) {
		defer close(done)
		setupSamsahai(true)
		ctx := context.TODO()

		By("Creating Config")
		config := mockConfig
		config.Spec.ActivePromotion.MaxRetry = &maxActivePromotionRetry
		Expect(client.Create(ctx, &config)).To(BeNil())

		By("Creating Team")
		team := mockTeam
		Expect(client.Create(ctx, &team)).To(BeNil())

		By("Verifying namespace and config have been created")
		err = wait.PollImmediate(verifyTime1s, verifyNSCreatedTimeout, func() (ok bool, err error) {
			namespace := corev1.Namespace{}
			if err := client.Get(ctx, types.NamespacedName{Name: stgNamespace}, &namespace); err != nil {
				return false, nil
			}

			config := s2hv1beta1.Config{}
			err = client.Get(ctx, types.NamespacedName{Name: team.Name}, &config)
			if err != nil {
				return false, nil
			}

			return true, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Verify namespace and config error")

		By("Creating ActivePromotion with `Finished` state")
		atp := activePromotion
		atp.Spec.NoOfRetry = 1
		atp.Status.State = s2hv1beta1.ActivePromotionFinished
		atp.Status.Result = s2hv1beta1.ActivePromotionFailure
		Expect(client.Create(ctx, &atp)).To(BeNil())

		By("Waiting ActivePromotion state to be ready for doing retry")
		err = wait.PollImmediate(verifyTime1s, verifyTime10s, func() (ok bool, err error) {
			atpComp := s2hv1beta1.ActivePromotion{}
			if err := client.Get(ctx, types.NamespacedName{Name: teamName}, &atpComp); err != nil {
				return false, nil
			}

			if atpComp.Status.State == s2hv1beta1.ActivePromotionWaiting ||
				atpComp.Status.State == s2hv1beta1.ActivePromotionCreatingPreActive ||
				atpComp.Status.State == s2hv1beta1.ActivePromotionDeployingComponents {
				return true, nil
			}

			return false, nil
		})
		Expect(err).NotTo(HaveOccurred(),
			"Waiting active promotion state to be ready for doing retrys error")

		By("Verifying ActivePromotion status")
		atpComp := s2hv1beta1.ActivePromotion{}
		Expect(client.Get(ctx, types.NamespacedName{Name: teamName}, &atpComp)).To(BeNil())
		Expect(atpComp.Spec.NoOfRetry).To(Equal(2))
	}, 45)

	It("should successfully rollback and delete active promotion", func(done Done) {
		defer close(done)
		setupSamsahai(true)
		ctx := context.TODO()

		By("Creating Config")
		config := mockConfig
		Expect(client.Create(ctx, &config)).To(BeNil())

		By("Creating Team")
		team := mockTeam
		team.Status.Namespace.Active = atvNamespace
		Expect(client.Create(ctx, &team)).To(BeNil())

		By("Creating active namespace")
		atvNs := activeNamespace
		Expect(client.Create(ctx, &atvNs)).To(BeNil())

		By("Creating StableComponent in active namespace")
		smd := stableAtvMariaDB
		Expect(client.Create(ctx, &smd)).To(BeNil())

		By("Verifying namespace and config have been created")
		err = wait.PollImmediate(verifyTime1s, verifyNSCreatedTimeout, func() (ok bool, err error) {
			namespace := corev1.Namespace{}
			if err := client.Get(ctx, types.NamespacedName{Name: stgNamespace}, &namespace); err != nil {
				return false, nil
			}

			config := s2hv1beta1.Config{}
			err = client.Get(ctx, types.NamespacedName{Name: team.Name}, &config)
			if err != nil {
				return false, nil
			}

			return true, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Verify namespace and config error")

		By("Creating ActivePromotion")
		atp := activePromotion
		Expect(client.Create(ctx, &atp)).To(BeNil())

		By("Waiting ActivePromotion state to be `Deploying`")
		err = wait.PollImmediate(verifyTime1s, verifyTime10s, func() (ok bool, err error) {
			atpComp := s2hv1beta1.ActivePromotion{}
			if err := client.Get(ctx, types.NamespacedName{Name: teamName}, &atpComp); err != nil {
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
		Expect(client.Get(ctx, types.NamespacedName{Name: teamName}, &atpComp))
		atpComp.Status.State = s2hv1beta1.ActivePromotionActiveEnvironment
		atpComp.Status.SetCondition(s2hv1beta1.ActivePromotionCondVerified, corev1.ConditionTrue, "verified")
		Expect(client.Update(ctx, &atpComp)).To(BeNil())

		By("Delete ActivePromotion")
		atpComp = s2hv1beta1.ActivePromotion{}
		Expect(client.Get(ctx, types.NamespacedName{Name: teamName}, &atpComp))
		Expect(client.Delete(context.TODO(), &atpComp)).To(BeNil())

		By("Creating mock active queue for active namespace")
		activeQ := mockActiveQueue
		activeQ.Namespace = atvNamespace
		Expect(client.Create(ctx, &activeQ)).To(BeNil())

		By("Pre-active namespace should be deleted")
		preActiveNs := atpComp.Status.TargetNamespace
		err = wait.PollImmediate(verifyTime1s, verifyTime15s, func() (ok bool, err error) {
			namespace := corev1.Namespace{}
			err = client.Get(ctx, types.NamespacedName{Name: preActiveNs}, &namespace)
			if err != nil && errors.IsNotFound(err) {
				return true, nil
			}

			return false, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Delete pre-active namespace error")

		By("ActivePromotion should be deleted")
		err = wait.PollImmediate(verifyTime1s, verifyTime30s, func() (ok bool, err error) {
			atpTemp := s2hv1beta1.ActivePromotion{}
			err = client.Get(ctx, types.NamespacedName{Name: atp.Name}, &atpTemp)
			if err != nil && errors.IsNotFound(err) {
				return true, nil
			}

			return false, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Delete active promotion error")

		By("Current active components should not be set")
		teamComp := s2hv1beta1.Team{}
		err = client.Get(ctx, types.NamespacedName{Name: team.Name}, &teamComp)
		Expect(err).To(BeNil())
		Expect(len(teamComp.Status.ActiveComponents)).To(BeZero())

		atpHists := &s2hv1beta1.ActivePromotionHistoryList{}
		listOpt := &rclient.ListOptions{LabelSelector: labels.SelectorFromSet(defaultLabels)}
		err = client.List(context.TODO(), atpHists, listOpt)
		Expect(err).To(BeNil())
		Expect(len(atpHists.Items)).To(Equal(1))
		Expect(atpHists.Items[0].Spec.ActivePromotion.Status.OutdatedComponents).ToNot(BeNil())
	}, 60)

	It("should rollback active environment timeout", func(done Done) {
		defer close(done)
		setupSamsahai(true)
		ctx := context.TODO()

		By("Creating Config")
		config := mockConfig
		Expect(client.Create(ctx, &config)).To(BeNil())

		By("Creating Team")
		team := mockTeam
		Expect(client.Create(ctx, &team)).To(BeNil())

		By("Verifying namespace and config have been created")
		err = wait.PollImmediate(verifyTime1s, verifyNSCreatedTimeout, func() (ok bool, err error) {
			namespace := corev1.Namespace{}
			if err := client.Get(ctx, types.NamespacedName{Name: stgNamespace}, &namespace); err != nil {
				return false, nil
			}

			config := s2hv1beta1.Config{}
			err = client.Get(ctx, types.NamespacedName{Name: team.Name}, &config)
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
		Expect(client.Create(ctx, &atp)).To(BeNil())

		By("ActivePromotion should be deleted")
		err = wait.PollImmediate(verifyTime1s, verifyTime10s, func() (ok bool, err error) {
			atpTemp := s2hv1beta1.ActivePromotion{}
			err = client.Get(ctx, types.NamespacedName{Name: atp.Name}, &atpTemp)
			if err != nil && errors.IsNotFound(err) {
				return true, nil
			}

			return false, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Delete active promotion error")
	}, 30)

	It("should successfully delete config when delete team", func(done Done) {
		defer close(done)
		setupSamsahai(true)
		ctx := context.TODO()

		By("Creating Config")
		config := mockConfig
		Expect(client.Create(ctx, &config)).To(BeNil())

		By("Creating Team")
		team := mockTeam
		Expect(client.Create(ctx, &team)).To(BeNil())

		By("Verifying namespace and config have been created")
		err = wait.PollImmediate(verifyTime1s, verifyNSCreatedTimeout, func() (ok bool, err error) {
			team := s2hv1beta1.Team{}
			if err := client.Get(ctx, types.NamespacedName{Name: teamName}, &team); err != nil {
				return false, nil
			}

			if len(team.ObjectMeta.Finalizers) == 0 {
				return false, nil
			}

			config := s2hv1beta1.Config{}
			err = client.Get(ctx, types.NamespacedName{Name: teamName}, &config)
			if err != nil {
				return false, nil
			}

			return true, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Verify namespace and config error")

		By("Deleting Team")
		_ = client.Get(ctx, types.NamespacedName{Name: teamName}, &team)
		Expect(client.Delete(ctx, &team)).To(BeNil())

		By("Verifying Config should be deleted")
		err = wait.PollImmediate(verifyTime1s, verifyTime15s, func() (ok bool, err error) {
			config := s2hv1beta1.Config{}
			err = client.Get(ctx, types.NamespacedName{Name: teamName}, &config)
			if err != nil && errors.IsNotFound(err) {
				return true, nil
			}

			return false, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Config should be deleted")

	}, 30)

	It("should be error when creating team if config does not exist", func(done Done) {
		defer close(done)
		setupSamsahai(true)
		ctx := context.TODO()

		By("Creating Team")
		team := mockTeam
		Expect(client.Create(ctx, &team)).To(BeNil())

		By("Team should be error if missing Config")
		err = wait.PollImmediate(verifyTime1s, verifyTime10s, func() (ok bool, err error) {
			team := s2hv1beta1.Team{}
			if err := client.Get(ctx, types.NamespacedName{Name: teamName}, &team); err != nil {
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
		setupSamsahai(true)

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
		Expect(client.Create(ctx, &config)).To(BeNil())

		By("Creating Team")
		team := mockTeam
		Expect(client.Create(ctx, &team)).To(BeNil())

		By("Verifying namespace and config have been created")
		err = wait.PollImmediate(verifyTime1s, verifyNSCreatedTimeout, func() (ok bool, err error) {
			namespace := corev1.Namespace{}
			if err := client.Get(ctx, types.NamespacedName{Name: stgNamespace}, &namespace); err != nil {
				return false, nil
			}

			config := s2hv1beta1.Config{}
			err = client.Get(ctx, types.NamespacedName{Name: team.Name}, &config)
			if err != nil {
				return false, nil
			}

			return true, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Verify namespace and config error")

		By("Send webhook")
		jsonData, err := json.Marshal(map[string]interface{}{
			"component": redisCompName,
		})
		Expect(err).NotTo(HaveOccurred())
		_, _, err = utilhttp.Post(server.URL+"/webhook/component", jsonData)
		Expect(err).NotTo(HaveOccurred())

		By("Verifying DesiredComponent has been created")
		err = wait.PollImmediate(verifyTime1s, verifyTime45s, func() (ok bool, err error) {
			_, _, _ = utilhttp.Post(server.URL+"/webhook/component", jsonData)

			dc := s2hv1beta1.DesiredComponent{}
			if err = client.Get(ctx, types.NamespacedName{Name: redisCompName, Namespace: stgNamespace}, &dc); err != nil {
				return false, nil
			}

			return true, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Verify DesiredComponent error")
	}, 75)

	It("should detect image missing and not create desired component", func(done Done) {
		defer close(done)
		setupSamsahai(true)

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
		redisComp := configCompRedis
		redisComp.Image.Repository = "bitnami/redis-not-found"
		redisComp.Image.Pattern = "image-missing"
		redisComp.Values = map[string]interface{}{
			"image": map[string]interface{}{
				"repository": "bitnami/redis-not-found",
			},
		}
		config.Spec.Components = []*s2hv1beta1.Component{&redisComp}
		Expect(client.Create(ctx, &config)).To(BeNil())

		By("Creating Team")
		team := mockTeam
		Expect(client.Create(ctx, &team)).To(BeNil())

		By("Verifying namespace and config have been created")
		err = wait.PollImmediate(verifyTime1s, verifyNSCreatedTimeout, func() (ok bool, err error) {
			namespace := corev1.Namespace{}
			if err := client.Get(ctx, types.NamespacedName{Name: stgNamespace}, &namespace); err != nil {
				return false, nil
			}

			config := s2hv1beta1.Config{}
			err = client.Get(ctx, types.NamespacedName{Name: team.Name}, &config)
			if err != nil {
				return false, nil
			}

			return true, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Verify namespace and config error")

		components, err := samsahaiCtrl.GetConfigController().GetComponents(team.Name)
		Expect(err).NotTo(HaveOccurred())

		By("Sending webhook & Verifying DesiredComponentImageCreatedTime has been updated")
		err = wait.PollImmediate(verifyTime1s, verifyTime15s, func() (ok bool, err error) {
			jsonData, err := json.Marshal(map[string]interface{}{
				"component": redisCompName,
			})
			if err != nil {
				return false, nil
			}
			componentRepository := components[redisCompName].Image.Repository
			_, _, err = utilhttp.Post(server.URL+"/webhook/component", jsonData)
			if err != nil {
				return false, nil
			}

			teamComp := s2hv1beta1.Team{}
			if err := client.Get(ctx, types.NamespacedName{Name: team.Name}, &teamComp); err != nil {
				return false, nil
			}

			image := stringutils.ConcatImageString(componentRepository, "image-missing")
			if _, ok = teamComp.Status.DesiredComponentImageCreatedTime[redisCompName][image]; !ok {
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
				err := client.Get(
					ctx,
					types.NamespacedName{Name: redisCompName, Namespace: team.Status.Namespace.Staging},
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

	It("should successfully detect changed components", func(done Done) {
		defer close(done)
		setupSamsahai(true)

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
		Expect(client.Create(ctx, &config)).To(BeNil())

		By("Creating Team")
		teamComp := mockTeam
		Expect(client.Create(ctx, &teamComp)).To(BeNil())

		By("Verifying namespace and config have been created")
		err = wait.PollImmediate(verifyTime1s, verifyNSCreatedTimeout, func() (ok bool, err error) {
			namespace := corev1.Namespace{}
			if err := client.Get(ctx, types.NamespacedName{Name: stgNamespace}, &namespace); err != nil {
				return false, nil
			}

			config := s2hv1beta1.Config{}
			err = client.Get(ctx, types.NamespacedName{Name: teamComp.Name}, &config)
			if err != nil {
				return false, nil
			}

			return true, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Verify namespace and config error")

		By("Send webhook")
		jsonDataRedis, err := json.Marshal(map[string]interface{}{
			"component": redisCompName,
		})
		Expect(err).NotTo(HaveOccurred())

		jsonDataWordpress, err := json.Marshal(map[string]interface{}{
			"component": wordpressCompName,
		})
		Expect(err).NotTo(HaveOccurred())

		By("Verifying redis DesiredComponent has been created")
		err = wait.PollImmediate(verifyTime1s, verifyTime60s, func() (ok bool, err error) {
			_, _, _ = utilhttp.Post(server.URL+"/webhook/component", jsonDataRedis)
			dRedis := s2hv1beta1.DesiredComponent{}
			if err = client.Get(ctx, types.NamespacedName{Name: redisCompName, Namespace: stgNamespace}, &dRedis); err != nil {
				return false, nil
			}

			_, _, _ = utilhttp.Post(server.URL+"/webhook/component", jsonDataWordpress)
			dWordpress := s2hv1beta1.DesiredComponent{}
			if err = client.Get(ctx, types.NamespacedName{Name: wordpressCompName, Namespace: stgNamespace}, &dWordpress); err != nil {
				return false, nil
			}

			return true, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Verify DesiredComponent error")

		By("Checking all desired components have been set")
		desiredComps := &s2hv1beta1.DesiredComponentList{}
		Expect(client.List(ctx, desiredComps, &rclient.ListOptions{Namespace: stgNamespace}))
		Expect(len(desiredComps.Items)).To(Equal(2))

		By("Creating Queues")
		for _, q := range mockQueueList.Items {
			Expect(client.Create(ctx, &q)).To(BeNil())
		}

		By("Checking all queues have been set")
		queues := &s2hv1beta1.QueueList{}
		Expect(client.List(ctx, queues, &rclient.ListOptions{Namespace: stgNamespace}))
		Expect(len(queues.Items)).To(Equal(2))

		By("Creating StableComponents")
		for _, s := range mockStableCompList.Items {
			Expect(client.Create(ctx, &s)).To(BeNil())
		}

		By("Checking all stable components have been set")
		stableComps := &s2hv1beta1.StableComponentList{}
		Expect(client.List(ctx, stableComps, &rclient.ListOptions{Namespace: stgNamespace}))
		Expect(len(queues.Items)).To(Equal(2))

		By("Updating components config")
		configComp := s2hv1beta1.Config{}
		Expect(client.Get(ctx, types.NamespacedName{Name: teamName}, &configComp)).To(BeNil())
		configComp.Spec.Components = []*s2hv1beta1.Component{{Name: redisCompName}}
		Expect(client.Update(ctx, &configComp)).To(BeNil())

		time.Sleep(verifyTime1s)
		By("Checking DesiredComponents")
		dRedis := s2hv1beta1.DesiredComponent{}
		Expect(client.Get(ctx, types.NamespacedName{Namespace: stgNamespace, Name: redisCompName}, &dRedis)).To(BeNil())
		dWordpress := s2hv1beta1.DesiredComponent{}
		err = client.Get(ctx, types.NamespacedName{Namespace: stgNamespace, Name: wordpressCompName}, &dWordpress)
		Expect(errors.IsNotFound(err)).To(BeTrue())

		By("Checking TeamDesiredComponents")
		err = wait.PollImmediate(verifyTime1s, verifyTime5s, func() (ok bool, err error) {
			teamComp = s2hv1beta1.Team{}
			if err = client.Get(ctx, types.NamespacedName{Name: teamName}, &teamComp); err != nil {
				return false, nil
			}

			if _, ok := teamComp.Status.DesiredComponentImageCreatedTime[redisCompName]; !ok {
				return false, nil
			}

			if _, ok := teamComp.Status.DesiredComponentImageCreatedTime[wordpressCompName]; ok {
				return false, nil
			}

			return true, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Verify TeamDesiredComponent error")

		By("Checking Queues")
		err = wait.PollImmediate(verifyTime1s, verifyTime10s, func() (ok bool, err error) {
			qRedis := s2hv1beta1.Queue{}
			if err = client.Get(ctx, types.NamespacedName{Namespace: stgNamespace, Name: redisCompName}, &qRedis); err != nil {
				return false, nil
			}

			qWordpress := s2hv1beta1.Queue{}
			if err = client.Get(ctx, types.NamespacedName{Namespace: stgNamespace, Name: wordpressCompName}, &qWordpress); err != nil && !errors.IsNotFound(err) {
				return false, nil
			}

			return true, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Verify StableComponents error")

		By("Checking StableComponents")
		err = wait.PollImmediate(verifyTime1s, verifyTime10s, func() (ok bool, err error) {
			sRedis := s2hv1beta1.StableComponent{}
			if err = client.Get(ctx, types.NamespacedName{Namespace: stgNamespace, Name: redisCompName}, &sRedis); err != nil {
				return false, nil
			}

			sMaria := s2hv1beta1.StableComponent{}
			if err = client.Get(ctx, types.NamespacedName{Namespace: stgNamespace, Name: mariaDBCompName}, &sMaria); err != nil && !errors.IsNotFound(err) {
				return false, nil
			}

			return true, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Verify StableComponents error")
	}, 110)

	It("should successfully create outdated component when no any active namespace left but there are active components in team", func(done Done) {
		defer close(done)
		setupSamsahai(true)
		ctx := context.TODO()

		By("Creating Config")
		config := mockConfigOnlyRedis
		Expect(client.Create(ctx, &config)).To(BeNil())

		By("Creating Team")
		team := mockTeam
		team.Status.ActiveComponents = map[string]s2hv1beta1.StableComponent{
			redisCompName: {Spec: stableSpecRedis},
		}
		Expect(client.Create(ctx, &team)).To(BeNil())

		By("Current active components should be set to Team")
		teamComp := s2hv1beta1.Team{}
		err = client.Get(ctx, types.NamespacedName{Name: team.Name}, &teamComp)
		Expect(err).To(BeNil())
		Expect(len(teamComp.Status.ActiveComponents)).ToNot(BeZero())

		By("Creating ActivePromotion")
		atp := activePromotion
		Expect(client.Create(ctx, &atp)).To(BeNil())

		By("Waiting pre-active environment is successfully created")
		atpResCh := make(chan s2hv1beta1.ActivePromotion)
		go func() {
			atpTemp := s2hv1beta1.ActivePromotion{}
			for {
				_ = client.Get(ctx, types.NamespacedName{Name: team.Name}, &atpTemp)
				if atpTemp.Status.IsConditionTrue(s2hv1beta1.ActivePromotionCondPreActiveCreated) {
					break
				}
				time.Sleep(500 * time.Millisecond)
			}
			atpResCh <- atpTemp
		}()
		atpRes := <-atpResCh

		By("Checking pre-active namespace has been set")
		teamComp = s2hv1beta1.Team{}
		err = client.Get(ctx, types.NamespacedName{Name: team.Name}, &teamComp)
		Expect(err).To(BeNil())
		Expect(client.Get(ctx, types.NamespacedName{Name: team.Name}, &teamComp))
		Expect(teamComp.Status.Namespace.PreActive).ToNot(BeEmpty())
		Expect(atpRes.Status.TargetNamespace).To(Equal(teamComp.Status.Namespace.PreActive))
		Expect(atpRes.Status.PreviousActiveNamespace).To(BeEmpty())

		By("Waiting ActivePromotion state to be `Deploying`")
		err = wait.PollImmediate(verifyTime1s, verifyTime60s, func() (ok bool, err error) {
			atpComp := s2hv1beta1.ActivePromotion{}
			if err := client.Get(ctx, types.NamespacedName{Name: atpRes.Name}, &atpComp); err != nil {
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
		Expect(client.Get(ctx, types.NamespacedName{Name: atpRes.Name}, &atpComp))
		atpComp.Status.State = s2hv1beta1.ActivePromotionDestroyingPreActive
		atpComp.Status.SetCondition(s2hv1beta1.ActivePromotionCondPreActiveDestroyed, corev1.ConditionTrue, "failed")
		Expect(client.Update(ctx, &atpComp)).To(BeNil())

		By("Delete ActivePromotion")
		atpComp = s2hv1beta1.ActivePromotion{}
		Expect(client.Get(ctx, types.NamespacedName{Name: atpRes.Name}, &atpComp))
		Expect(client.Delete(context.TODO(), &atpComp)).To(BeNil())

		By("Pre-active namespace should be deleted")
		preActiveNs := atpComp.Status.TargetNamespace
		err = wait.PollImmediate(verifyTime1s, verifyTime15s, func() (ok bool, err error) {
			namespace := corev1.Namespace{}
			err = client.Get(ctx, types.NamespacedName{Name: preActiveNs}, &namespace)
			if err != nil && errors.IsNotFound(err) {
				return true, nil
			}

			return false, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Delete pre-active namespace error")

		By("ActivePromotion should be deleted")
		err = wait.PollImmediate(verifyTime1s, verifyTime10s, func() (ok bool, err error) {
			atpTemp := s2hv1beta1.ActivePromotion{}
			err = client.Get(ctx, types.NamespacedName{Name: team.Name}, &atpTemp)
			if err != nil && errors.IsNotFound(err) {
				return true, nil
			}

			return false, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Delete active promotion error")

		By("ActivePromotionHistory should be created")
		atpHists := &s2hv1beta1.ActivePromotionHistoryList{}
		listOpt := &rclient.ListOptions{LabelSelector: labels.SelectorFromSet(defaultLabels)}
		err = client.List(context.TODO(), atpHists, listOpt)
		Expect(err).To(BeNil())
		Expect(len(atpHists.Items)).To(Equal(1))
		Expect(atpHists.Items[0].Spec.ActivePromotion.Status.OutdatedComponents).ToNot(BeNil())
	}, 60)

	It("should successfully set new active namespace when success promotion on team creation", func(done Done) {
		defer close(done)
		setupSamsahai(false)
		ctx := context.TODO()

		By("Creating Config")
		config := mockConfigOnlyRedis
		Expect(client.Create(ctx, &config)).To(BeNil())

		By("Creating Team")
		team := mockTeam
		Expect(client.Create(ctx, &team)).To(BeNil())

		By("Verifying namespace and config have been created")
		err = wait.PollImmediate(verifyTime1s, verifyNSCreatedTimeout, func() (ok bool, err error) {
			namespace := corev1.Namespace{}
			if err := client.Get(ctx, types.NamespacedName{Name: stgNamespace}, &namespace); err != nil {
				return false, nil
			}

			config := s2hv1beta1.Config{}
			err = client.Get(ctx, types.NamespacedName{Name: team.Name}, &config)
			if err != nil {
				return false, nil
			}

			return true, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Create staging related object objects error")

		teamComp := s2hv1beta1.Team{}
		Expect(client.Get(ctx, types.NamespacedName{Name: team.Name}, &teamComp))

		By("Waiting pre-active environment is successfully created")
		atpResCh := make(chan s2hv1beta1.ActivePromotion)
		go func() {
			atpTemp := s2hv1beta1.ActivePromotion{}
			for {
				_ = client.Get(ctx, types.NamespacedName{Name: team.Name}, &atpTemp)
				if atpTemp.Status.IsConditionTrue(s2hv1beta1.ActivePromotionCondPreActiveCreated) {
					break
				}
				time.Sleep(500 * time.Millisecond)
			}
			atpResCh <- atpTemp
		}()
		atpRes := <-atpResCh

		By("Checking pre-active namespace has been set")
		Expect(client.Get(ctx, types.NamespacedName{Name: team.Name}, &teamComp))
		Expect(teamComp.Status.Namespace.PreActive).ToNot(BeEmpty())
		Expect(atpRes.Status.TargetNamespace).To(Equal(teamComp.Status.Namespace.PreActive))
		Expect(atpRes.Status.PreviousActiveNamespace).To(BeEmpty())

		By("Start staging controller for pre-active")
		preActiveNs := atpRes.Status.TargetNamespace
		{
			stagingCfg := rest.CopyConfig(restCfg)
			stagingMgr, err := manager.New(stagingCfg, manager.Options{
				Namespace:          preActiveNs,
				MetricsBindAddress: "0",
			})
			Expect(err).NotTo(HaveOccurred())

			stagingCfgCtrl := configctrl.New(stagingMgr)
			qCtrl := queue.New(preActiveNs, client)
			stagingPreActiveCtrl := staging.NewController(teamName, preActiveNs, samsahaiAuthToken, samsahaiClient,
				stagingMgr, qCtrl, stagingCfgCtrl, "", "", "",
				internal.StagingConfig{})
			go func() {
				defer GinkgoRecover()
				Expect(stagingMgr.Start(chStop)).NotTo(HaveOccurred())
			}()
			go stagingPreActiveCtrl.Start(chStop)
		}

		By("ActivePromotion should be deleted")
		err = wait.PollImmediate(verifyTime1s, promoteTimeOut, func() (ok bool, err error) {
			atpTemp := s2hv1beta1.ActivePromotion{}
			err = client.Get(ctx, types.NamespacedName{Name: team.Name}, &atpTemp)
			if err != nil && errors.IsNotFound(err) {
				return true, nil
			}

			return false, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Delete active promotion error")

		By("Active namespace should be set")
		err = wait.PollImmediate(verifyTime1s, verifyTime15s, func() (ok bool, err error) {
			teamComp = s2hv1beta1.Team{}
			if err := client.Get(ctx, types.NamespacedName{Name: team.Name}, &teamComp); err != nil {
				return false, nil
			}

			if teamComp.Status.Namespace.Active == preActiveNs {
				return true, nil
			}

			return false, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Active namespace is not set")

		By("Current active components should not be set as it is first time promotion")
		teamComp = s2hv1beta1.Team{}
		err = client.Get(ctx, types.NamespacedName{Name: team.Name}, &teamComp)
		Expect(err).To(BeNil())
		Expect(len(teamComp.Status.ActiveComponents)).To(BeZero())

		By("ActivePromotionHistory should be created")
		atpHists := &s2hv1beta1.ActivePromotionHistoryList{}
		listOpt := &rclient.ListOptions{LabelSelector: labels.SelectorFromSet(defaultLabels)}
		err = client.List(context.TODO(), atpHists, listOpt)
		Expect(err).To(BeNil())
		Expect(len(atpHists.Items)).To(Equal(1))
		Expect(atpHists.Items[0].Spec.ActivePromotion.Status.OutdatedComponents).To(BeNil())

		By("Public API")
		{
			By("Get team")
			{
				_, data, err := utilhttp.Get(samsahaiServer.URL + "/teams/" + team.Name)
				Expect(err).NotTo(HaveOccurred())
				Expect(data).NotTo(BeNil())
				Expect(gjson.GetBytes(data, "teamName").Str).To(Equal(team.Name))
			}

			By("Get team Queue")
			{
				_, data, err := utilhttp.Get(samsahaiServer.URL + "/teams/" + team.Name + "/queue")
				Expect(err).NotTo(HaveOccurred())
				Expect(data).NotTo(BeNil())
			}

			By("Get team QueueHistories not found")
			{
				_, _, err := utilhttp.Get(samsahaiServer.URL + "/teams/" + team.Name + "/queue/histories/" + "unknown")
				Expect(err).To(HaveOccurred())
			}
		}
	}, 250)

	It("should successfully not set active namespace when failed promotion on team creation", func(done Done) {
		defer close(done)
		setupSamsahai(false)
		ctx := context.TODO()

		By("Creating Config")
		config := mockConfigOnlyRedis
		Expect(client.Create(ctx, &config)).To(BeNil())

		By("Creating Team")
		team := mockTeam
		Expect(client.Create(ctx, &team)).To(BeNil())

		By("Verifying namespace and config have been created")
		err = wait.PollImmediate(verifyTime1s, verifyNSCreatedTimeout, func() (ok bool, err error) {
			namespace := corev1.Namespace{}
			if err := client.Get(ctx, types.NamespacedName{Name: stgNamespace}, &namespace); err != nil {
				return false, nil
			}

			config := s2hv1beta1.Config{}
			err = client.Get(ctx, types.NamespacedName{Name: team.Name}, &config)
			if err != nil {
				return false, nil
			}

			return true, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Create staging related objects error")

		By("Waiting pre-active environment is successfully created")
		atpResCh := make(chan s2hv1beta1.ActivePromotion)
		go func() {
			atpTemp := s2hv1beta1.ActivePromotion{}
			for {
				_ = client.Get(ctx, types.NamespacedName{Name: team.Name}, &atpTemp)
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
		err = client.Get(ctx, types.NamespacedName{Name: team.Name}, &teamComp)
		Expect(err).To(BeNil())
		Expect(client.Get(ctx, types.NamespacedName{Name: team.Name}, &teamComp))
		Expect(teamComp.Status.Namespace.PreActive).ToNot(BeEmpty())
		Expect(atpRes.Status.TargetNamespace).To(Equal(teamComp.Status.Namespace.PreActive))
		Expect(atpRes.Status.PreviousActiveNamespace).To(BeEmpty())

		By("Waiting ActivePromotion state to be `Deploying`")
		err = wait.PollImmediate(verifyTime1s, verifyTime60s, func() (ok bool, err error) {
			atpComp := s2hv1beta1.ActivePromotion{}
			if err := client.Get(ctx, types.NamespacedName{Name: atpRes.Name}, &atpComp); err != nil {
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
		Expect(client.Get(ctx, types.NamespacedName{Name: atpRes.Name}, &atpComp))
		atpComp.Status.State = s2hv1beta1.ActivePromotionDestroyingPreActive
		atpComp.Status.SetCondition(s2hv1beta1.ActivePromotionCondPreActiveDestroyed, corev1.ConditionTrue, "failed")
		Expect(client.Update(ctx, &atpComp)).To(BeNil())

		By("Delete ActivePromotion")
		atpComp = s2hv1beta1.ActivePromotion{}
		Expect(client.Get(ctx, types.NamespacedName{Name: atpRes.Name}, &atpComp))
		Expect(client.Delete(context.TODO(), &atpComp)).To(BeNil())

		By("Pre-active namespace should be deleted")
		preActiveNs := atpComp.Status.TargetNamespace
		err = wait.PollImmediate(verifyTime1s, verifyTime15s, func() (ok bool, err error) {
			namespace := corev1.Namespace{}
			err = client.Get(ctx, types.NamespacedName{Name: preActiveNs}, &namespace)
			if err != nil && errors.IsNotFound(err) {
				return true, nil
			}

			return false, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Delete pre-active namespace error")

		By("ActivePromotion should be deleted")
		err = wait.PollImmediate(verifyTime1s, verifyTime10s, func() (ok bool, err error) {
			atpTemp := s2hv1beta1.ActivePromotion{}
			err = client.Get(ctx, types.NamespacedName{Name: team.Name}, &atpTemp)
			if err != nil && errors.IsNotFound(err) {
				return true, nil
			}

			return false, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Delete active promotion error")

		atpHists := &s2hv1beta1.ActivePromotionHistoryList{}
		listOpt := &rclient.ListOptions{LabelSelector: labels.SelectorFromSet(defaultLabels)}
		err = client.List(context.TODO(), atpHists, listOpt)
		Expect(err).To(BeNil())
		Expect(len(atpHists.Items)).To(Equal(1))
		Expect(atpHists.Items[0].Spec.ActivePromotion.Status.OutdatedComponents).To(BeNil())

		By("Checking only staging namespace left")
		teamComp = s2hv1beta1.Team{}
		err = client.Get(ctx, types.NamespacedName{Name: team.Name}, &teamComp)
		Expect(err).To(BeNil())
		Expect(teamComp.Status.Namespace.Staging).ToNot(BeEmpty())
		Expect(teamComp.Status.Namespace.Active).To(BeEmpty())
		Expect(teamComp.Status.Namespace.PreActive).To(BeEmpty())

		By("Current active components should not be set")
		Expect(len(teamComp.Status.ActiveComponents)).To(BeZero())
	}, 60)

	It("should successfully notify component changed and promote active after creating team", func(done Done) {
		defer close(done)
		setupSamsahai(false)
		ctx := context.TODO()

		By("Creating Config")
		config := mockConfigOnlyRedis
		Expect(client.Create(ctx, &config)).To(BeNil())

		By("Creating Team")
		team := mockTeam
		Expect(client.Create(ctx, &team)).To(BeNil())

		By("Verifying namespace and config have been created")
		err = wait.PollImmediate(verifyTime1s, verifyNSCreatedTimeout, func() (ok bool, err error) {
			namespace := corev1.Namespace{}
			if err := client.Get(ctx, types.NamespacedName{Name: stgNamespace}, &namespace); err != nil {
				return false, nil
			}

			config := s2hv1beta1.Config{}
			err = client.Get(ctx, types.NamespacedName{Name: team.Name}, &config)
			if err != nil {
				return false, nil
			}

			return true, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Create staging related object objects error")

		teamComp := s2hv1beta1.Team{}
		Expect(client.Get(ctx, types.NamespacedName{Name: team.Name}, &teamComp))

		By("Waiting TeamFirstNotifyComponentChanged and TeamFirstActivePromotionRun conditions have been set")
		err = wait.PollImmediate(verifyTime1s, verifyTime5s, func() (ok bool, err error) {
			teamComp := s2hv1beta1.Team{}
			if err = client.Get(ctx, types.NamespacedName{Name: team.Name}, &teamComp); err != nil {
				return false, nil
			}

			var foundNotifying, foundPromotion bool
			for i, c := range teamComp.Status.Conditions {
				if c.Type == s2hv1beta1.TeamFirstNotifyComponentChanged {
					if teamComp.Status.Conditions[i].Status == corev1.ConditionTrue {
						foundNotifying = true
					}
				}

				if c.Type == s2hv1beta1.TeamFirstActivePromotionRun {
					if teamComp.Status.Conditions[i].Status == corev1.ConditionTrue {
						foundPromotion = true
					}
				}
			}

			if foundNotifying && foundPromotion {
				return true, nil
			}

			return false, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Notify component changed and promote the first active error")
	}, 20)

	It("should successfully create cronjob", func(done Done) {
		defer close(done)
		setupSamsahai(true)
		ctx := context.TODO()

		By("Creating Config that have Scheduler")
		configRedis := mockConfigOnlyRedis
		configRedis.Spec.Components[0].Schedules = []string{"0 4 * * *"}
		Expect(client.Create(ctx, &configRedis)).To(BeNil())

		By("Creating Team")
		team := mockTeam
		Expect(client.Create(ctx, &team)).To(BeNil())

		By("Verifying namespace and config have been created")
		err = wait.PollImmediate(verifyTime1s, verifyNSCreatedTimeout, func() (ok bool, err error) {
			namespace := corev1.Namespace{}
			if err := client.Get(ctx, types.NamespacedName{Name: stgNamespace}, &namespace); err != nil {
				return false, nil
			}

			config := s2hv1beta1.Config{}
			err = client.Get(ctx, types.NamespacedName{Name: team.Name}, &config)
			if err != nil {
				return false, nil
			}

			return true, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Verify namespace and config error")

		By("Verifying CronJob have been created")
		err = wait.PollImmediate(verifyTime1s, verifyTime10s, func() (ok bool, err error) {
			cronjobList := &batchv1beta1.CronJobList{}
			cronjobLabel := labels.SelectorFromSet(map[string]string{"component": configRedis.Spec.Components[0].Name})
			listOption := &rclient.ListOptions{Namespace: stgNamespace, LabelSelector: cronjobLabel}
			if err := client.List(ctx, cronjobList, listOption); err != nil {
				return false, err
			}

			if len(cronjobList.Items) == 0 || len(cronjobList.Items) != len(configRedis.Spec.Components[0].Schedules) {
				return false, nil
			}
			return true, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Verify creating CronJob error")

		By("Updating Config that have no Scheduler")
		configRedis = s2hv1beta1.Config{}
		_ = client.Get(ctx, types.NamespacedName{Name: teamName}, &configRedis)
		configRedis.Spec.Components[0].Schedules = []string{}
		Expect(client.Update(ctx, &configRedis)).To(BeNil())

		By("Verifying CronJob should be deleted")
		err = wait.PollImmediate(verifyTime1s, verifyTime10s, func() (ok bool, err error) {
			cronjobList := &batchv1beta1.CronJobList{}
			cronjobLabel := labels.SelectorFromSet(map[string]string{"component": configRedis.Spec.Components[0].Name})
			listOption := &rclient.ListOptions{Namespace: stgNamespace, LabelSelector: cronjobLabel}
			if err := client.List(ctx, cronjobList, listOption); err != nil {
				return false, nil
			}

			if len(cronjobList.Items) != 0 {
				return false, nil
			}
			return true, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Config should be deleted")
	}, 90)

	It("should successfully apply/update team template", func(done Done) {
		defer close(done)
		setupSamsahai(true)
		ctx := context.TODO()

		By("Creating Config")
		config := mockConfig
		Expect(client.Create(ctx, &config)).To(BeNil())

		By("Creating Team")
		team := mockTeam
		Expect(client.Create(ctx, &team)).To(BeNil())

		By("Creating Config using template")
		config2 := mockConfigUsingTemplate
		Expect(client.Create(ctx, &config2)).To(BeNil())

		By("Creating Team using template")
		team2 := mockTeam2
		Expect(client.Create(ctx, &team2)).To(BeNil())

		By("Apply team template")
		err = wait.PollImmediate(verifyTime1s, verifyTime5s, func() (ok bool, err error) {
			team := s2hv1beta1.Team{}
			teamUsingTemplate := s2hv1beta1.Team{}
			if err = client.Get(context.TODO(), types.NamespacedName{Name: mockTeam.Name}, &team); err != nil {
				return false, nil
			}
			if err = client.Get(context.TODO(), types.NamespacedName{Name: mockTeam2.Name}, &teamUsingTemplate); err != nil {
				return false, nil
			}
			if teamUsingTemplate.Status.Used.Credential == team.Status.Used.Credential ||
				teamUsingTemplate.Status.Used.StagingCtrl == team.Status.Used.StagingCtrl ||
				len(teamUsingTemplate.Status.Used.Owners) == len(team.Status.Used.Owners) {
				return true, nil
			}
			return false, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Apply team template error")

		By("Update team template")
		err = wait.PollImmediate(verifyTime1s, verifyTime5s, func() (ok bool, err error) {
			team := s2hv1beta1.Team{}
			teamUsingTemplate := s2hv1beta1.Team{}
			if err = client.Get(context.TODO(), types.NamespacedName{Name: mockTeam.Name}, &team); err != nil {
				return false, nil
			}
			team.Spec.StagingCtrl.Endpoint = "http://127.0.0.1"
			if err = client.Update(context.TODO(), &team); err != nil {
				return false, nil
			}
			if err = client.Get(context.TODO(), types.NamespacedName{Name: mockTeam2.Name}, &teamUsingTemplate); err != nil {
				return false, nil
			}
			if teamUsingTemplate.Status.Used.StagingCtrl.Endpoint == "http://127.0.0.1" &&
				teamUsingTemplate.Status.TemplateUID == team.Status.TemplateUID {
				return true, nil
			}
			return false, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Update team template error")

	}, 10)
})

var (
	samsahaiAuthToken = "1234567890_"
	samsahaiSystemNs  = "samsahai-system"
	samsahaiConfig    = internal.SamsahaiConfig{
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

	teamName  = "teamtest"
	teamName2 = "teamtest2"
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

	redisCompName     = "redis"
	mariaDBCompName   = "mariadb"
	wordpressCompName = "wordpress"

	maxActivePromotionRetry = 2

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
				mariaDBCompName: {
					stringutils.ConcatImageString("bitnami/mariadb", "10.3.18-debian-9-r32"): s2hv1beta1.DesiredImageTime{
						Image:       &s2hv1beta1.Image{Repository: "bitnami/mariadb", Tag: "10.3.18-debian-9-r32"},
						CreatedTime: metav1.Time{Time: time.Date(2019, 10, 1, 9, 0, 0, 0, time.UTC)},
					},
				},
				redisCompName: {
					stringutils.ConcatImageString("bitnami/redis", "5.0.5-debian-9-r160"): s2hv1beta1.DesiredImageTime{
						Image:       &s2hv1beta1.Image{Repository: "bitnami/redis", Tag: "5.0.5-debian-9-r160"},
						CreatedTime: metav1.Time{Time: time.Date(2019, 10, 1, 9, 0, 0, 0, time.UTC)},
					},
				},
				wordpressCompName: {
					stringutils.ConcatImageString("bitnami/wordpress", "5.2.4-debian-9-r18"): s2hv1beta1.DesiredImageTime{
						Image:       &s2hv1beta1.Image{Repository: "bitnami/wordpress", Tag: "5.2.4-debian-9-r18"},
						CreatedTime: metav1.Time{Time: time.Date(2019, 10, 1, 9, 0, 0, 0, time.UTC)},
					},
				},
			},
			Used: s2hv1beta1.TeamSpec{
				Description: "team for testing",
				Owners:      []string{"samsahai@samsahai.io"},
				Credential: s2hv1beta1.Credential{
					SecretName: s2hobject.GetTeamSecretName(teamName),
				},
				StagingCtrl: &s2hv1beta1.StagingCtrl{
					IsDeploy: false,
				},
			},
		},
	}

	mockTeam2 = s2hv1beta1.Team{
		ObjectMeta: metav1.ObjectMeta{
			Name:   teamName2,
			Labels: testLabels,
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
		Spec: s2hv1beta1.ActivePromotionSpec{
			PromotedBy: "user",
		},
	}

	activePromotionHistory = s2hv1beta1.ActivePromotionHistory{
		ObjectMeta: metav1.ObjectMeta{
			Name:   fmt.Sprintf("%s-20191010-111111", teamName),
			Labels: defaultLabels,
		},
	}

	stableSpecMariaDB = s2hv1beta1.StableComponentSpec{Name: mariaDBCompName, Version: "10.3.18-debian-9-r32", Repository: "bitnami/mariadb"}
	stableMariaDB     = s2hv1beta1.StableComponent{
		ObjectMeta: metav1.ObjectMeta{Name: mariaDBCompName, Namespace: stgNamespace},
		Spec:       stableSpecMariaDB,
	}
	stableAtvMariaDB = s2hv1beta1.StableComponent{
		ObjectMeta: metav1.ObjectMeta{Name: mariaDBCompName, Namespace: atvNamespace},
		Spec:       stableSpecMariaDB,
	}

	stableSpecRedis = s2hv1beta1.StableComponentSpec{Name: redisCompName, Version: "5.0.5-debian-9-r160", Repository: "bitnami/redis"}
	stableRedis     = s2hv1beta1.StableComponent{
		ObjectMeta: metav1.ObjectMeta{Name: redisCompName, Namespace: stgNamespace},
		Spec:       stableSpecRedis,
	}

	mockQueueList = &s2hv1beta1.QueueList{
		Items: []s2hv1beta1.Queue{
			{
				ObjectMeta: metav1.ObjectMeta{Name: redisCompName, Namespace: stgNamespace},
				Spec: s2hv1beta1.QueueSpec{Name: redisCompName,
					Components: s2hv1beta1.QueueComponents{
						{Name: redisCompName, Repository: "bitnami/redis", Version: "5.0.5-debian-9-r160"},
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: wordpressCompName, Namespace: stgNamespace},
				Spec: s2hv1beta1.QueueSpec{Name: wordpressCompName,
					Components: s2hv1beta1.QueueComponents{
						{Name: wordpressCompName, Repository: "bitnami/wordpress", Version: "5.2.4-debian-9-r18"},
					},
				},
			},
		},
	}

	mockStableCompList = &s2hv1beta1.StableComponentList{
		Items: []s2hv1beta1.StableComponent{stableMariaDB, stableRedis},
	}

	mockSecret = corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      s2hobject.GetTeamSecretName(teamName),
			Namespace: samsahaiSystemNs,
		},
		Data: map[string][]byte{},
		Type: "Opaque",
	}

	mockEngine   = "mock"
	deployConfig = s2hv1beta1.ConfigDeploy{
		Timeout: metav1.Duration{Duration: 5 * time.Minute},
		Engine:  &mockEngine,
		TestRunner: &s2hv1beta1.ConfigTestRunner{
			TestMock: &s2hv1beta1.ConfigTestMock{
				Result: true,
			},
		},
	}
	configStg = &s2hv1beta1.ConfigStaging{
		Deployment: &deployConfig,
	}
	configAtp = &s2hv1beta1.ConfigActivePromotion{
		Timeout:          metav1.Duration{Duration: 10 * time.Minute},
		MaxHistories:     2,
		TearDownDuration: metav1.Duration{Duration: 10 * time.Second},
		OutdatedNotification: &s2hv1beta1.OutdatedNotification{
			ExceedDuration:            metav1.Duration{Duration: 24 * time.Hour},
			ExcludeWeekendCalculation: true,
		},
		Deployment: &deployConfig,
	}
	configReporter = &s2hv1beta1.ConfigReporter{
		ReportMock: true,
	}
	compSource      = s2hv1beta1.UpdatingSource("public-registry")
	configCompRedis = s2hv1beta1.Component{
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
	configCompWordpress = s2hv1beta1.Component{
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
			mariaDBCompName: map[string]interface{}{
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
			Staging:         configStg,
			ActivePromotion: configAtp,
			Reporter:        configReporter,
			Components: []*s2hv1beta1.Component{
				&configCompRedis,
				&configCompWordpress,
			},
		},
		Status: s2hv1beta1.ConfigStatus{
			Used: s2hv1beta1.ConfigSpec{
				Staging:         configStg,
				ActivePromotion: configAtp,
				Reporter:        configReporter,
				Components: []*s2hv1beta1.Component{
					&configCompRedis,
					&configCompWordpress,
				},
			},
		},
	}

	mockConfigUsingTemplate = s2hv1beta1.Config{
		ObjectMeta: metav1.ObjectMeta{
			Name:   teamName2,
			Labels: testLabels,
		},
		Spec: s2hv1beta1.ConfigSpec{
			Template: teamName,
		},
	}

	mockConfigOnlyRedis = s2hv1beta1.Config{
		ObjectMeta: metav1.ObjectMeta{
			Name:   teamName,
			Labels: testLabels,
		},
		Spec: s2hv1beta1.ConfigSpec{
			Staging:         configStg,
			ActivePromotion: configAtp,
			Reporter:        configReporter,
			Components: []*s2hv1beta1.Component{
				&configCompRedis,
			},
		},
		Status: s2hv1beta1.ConfigStatus{
			Used: s2hv1beta1.ConfigSpec{
				Staging:         configStg,
				ActivePromotion: configAtp,
				Reporter:        configReporter,
				Components: []*s2hv1beta1.Component{
					&configCompRedis,
				},
			},
		},
	}
)
