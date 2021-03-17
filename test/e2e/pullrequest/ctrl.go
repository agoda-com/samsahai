package pullrequest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
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
	prqueuectrl "github.com/agoda-com/samsahai/internal/pullrequest/queue"
	prtriggerctrl "github.com/agoda-com/samsahai/internal/pullrequest/trigger"
	"github.com/agoda-com/samsahai/internal/queue"
	"github.com/agoda-com/samsahai/internal/samsahai"
	s2hobject "github.com/agoda-com/samsahai/internal/samsahai/k8sobject"
	s2hhttp "github.com/agoda-com/samsahai/internal/samsahai/webhook"
	"github.com/agoda-com/samsahai/internal/staging"
	utilhttp "github.com/agoda-com/samsahai/internal/util/http"
	"github.com/agoda-com/samsahai/internal/util/stringutils"
	samsahairpc "github.com/agoda-com/samsahai/pkg/samsahai/rpc"
)

const (
	verifyTime1s           = 1 * time.Second
	verifyTime10s          = 10 * time.Second
	verifyTime15s          = 15 * time.Second
	verifyTime30s          = 30 * time.Second
	verifyTime45s          = 45 * time.Second
	verifyTime60s          = 60 * time.Second
	verifyNSCreatedTimeout = verifyTime15s
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

func setupSamsahai() {
	s2hConfig := samsahaiConfig

	samsahaiCtrl = samsahai.New(mgr, "samsahai-system", s2hConfig)
	Expect(samsahaiCtrl).ToNot(BeNil())

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

func setupStaging(namespace string) (internal.StagingController, internal.QueueController) {
	// create mgr from config
	stagingCfg := rest.CopyConfig(restCfg)
	stagingMgr, err := manager.New(stagingCfg, manager.Options{
		Namespace:          namespace,
		MetricsBindAddress: "0",
	})
	Expect(err).NotTo(HaveOccurred())

	stagingCfgCtrl := configctrl.New(stagingMgr)
	qctrl := queue.New(namespace, client)
	stagingCtrl := staging.NewController(teamName, namespace, samsahaiAuthToken, samsahaiClient,
		stagingMgr, qctrl, stagingCfgCtrl, "", "", "",
		internal.StagingConfig{})

	prQueueCtrl := prqueuectrl.New(teamName, namespace, stagingMgr, samsahaiAuthToken, samsahaiClient,
		prqueuectrl.WithClient(client))
	_ = prtriggerctrl.New(teamName, stagingMgr, prQueueCtrl, samsahaiAuthToken, samsahaiClient)

	go func() {
		defer GinkgoRecover()
		Expect(stagingMgr.Start(chStop)).NotTo(HaveOccurred())
	}()

	return stagingCtrl, prQueueCtrl
}

var _ = Describe("[e2e] Pull request controller", func() {
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

		By("Creating Secret")
		secret := mockSecret
		_ = client.Create(ctx, &secret)
	}, 60)

	AfterEach(func(done Done) {
		defer close(done)

		By("Deleting all Teams")
		err = client.DeleteAllOf(ctx, &s2hv1.Team{}, rclient.MatchingLabels(testLabels))
		Expect(err).NotTo(HaveOccurred())
		err = wait.PollImmediate(verifyTime1s, verifyTime30s, func() (ok bool, err error) {
			teamList := s2hv1.TeamList{}
			listOpt := &rclient.ListOptions{LabelSelector: labels.SelectorFromSet(testLabels)}
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

		By("Deleting all Configs")
		err = client.DeleteAllOf(ctx, &s2hv1.Config{}, rclient.MatchingLabels(testLabels))
		Expect(err).NotTo(HaveOccurred())
		err = wait.PollImmediate(verifyTime1s, verifyTime10s, func() (ok bool, err error) {
			configList := s2hv1.ConfigList{}
			listOpt := &rclient.ListOptions{LabelSelector: labels.SelectorFromSet(testLabels)}
			err = client.List(ctx, &configList, listOpt)
			if err != nil && k8serrors.IsNotFound(err) {
				return true, nil
			}
			if len(configList.Items) == 0 {
				return true, nil
			}

			return false, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Deleting all Configs error")

		By("Deleting pull request namespaces")
		prNs := corev1.Namespace{}
		err = client.Get(ctx, types.NamespacedName{Name: singlePRNamespace}, &prNs)
		if err != nil && k8serrors.IsNotFound(err) {
			_ = client.Delete(ctx, &prNs)
			err = wait.PollImmediate(verifyTime1s, verifyTime10s, func() (ok bool, err error) {
				namespace := corev1.Namespace{}
				err = client.Get(ctx, types.NamespacedName{Name: singlePRNamespace}, &namespace)
				if err != nil && k8serrors.IsNotFound(err) {
					return true, nil
				}
				return false, nil
			})
		}
		prNs = corev1.Namespace{}
		err = client.Get(ctx, types.NamespacedName{Name: bundledPRNamespace}, &prNs)
		if err != nil && k8serrors.IsNotFound(err) {
			_ = client.Delete(ctx, &prNs)
			err = wait.PollImmediate(verifyTime1s, verifyTime10s, func() (ok bool, err error) {
				namespace := corev1.Namespace{}
				err = client.Get(ctx, types.NamespacedName{Name: bundledPRNamespace}, &namespace)
				if err != nil && k8serrors.IsNotFound(err) {
					return true, nil
				}
				return false, nil
			})
		}

		By("Deleting all PullRequestQueues")
		err = client.DeleteAllOf(ctx, &s2hv1.PullRequestQueue{}, rclient.InNamespace(stgNamespace))
		Expect(err).NotTo(HaveOccurred())
		err = wait.PollImmediate(verifyTime1s, verifyTime10s, func() (ok bool, err error) {
			prQueueList := s2hv1.PullRequestQueueList{}
			err = client.List(ctx, &prQueueList, &rclient.ListOptions{Namespace: stgNamespace})
			if err != nil && k8serrors.IsNotFound(err) {
				return true, nil
			}
			if len(prQueueList.Items) == 0 {
				return true, nil
			}

			return false, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Deleting all PullRequestQueues error")

		By("Deleting all PullRequestTriggers")
		err = client.DeleteAllOf(ctx, &s2hv1.PullRequestTrigger{}, rclient.InNamespace(stgNamespace))
		Expect(err).NotTo(HaveOccurred())
		err = wait.PollImmediate(verifyTime1s, verifyTime10s, func() (ok bool, err error) {
			prTriggerList := s2hv1.PullRequestTriggerList{}
			err = client.List(ctx, &prTriggerList, &rclient.ListOptions{Namespace: stgNamespace})
			if err != nil && k8serrors.IsNotFound(err) {
				return true, nil
			}
			if len(prTriggerList.Items) == 0 {
				return true, nil
			}

			return false, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Deleting all PullRequestQueues error")

		By("Deleting Secret")
		secret := mockSecret
		Expect(client.Delete(ctx, &secret)).NotTo(HaveOccurred())

		By("Deleting Config")
		Expect(samsahaiCtrl.GetConfigController().Delete(teamName)).NotTo(HaveOccurred())

		close(chStop)
		samsahaiServer.Close()
		wgStop.Wait()
	}, 60)

	It("should successfully deploy pull request queue with 2 components in a bundle", func(done Done) {
		defer close(done)

		By("Starting Samsahai internal process")
		setupSamsahai()
		go samsahaiCtrl.Start(chStop)

		By("Starting Staging internal process")
		stagingCtrl, _ := setupStaging(stgNamespace)
		go stagingCtrl.Start(chStop)

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

			config := s2hv1.Config{}
			err = client.Get(ctx, types.NamespacedName{Name: teamComp.Name}, &config)
			if err != nil {
				return false, nil
			}

			return true, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Verify namespace and config error")

		By("Starting http server")
		mux := http.NewServeMux()
		mux.Handle(samsahaiCtrl.PathPrefix(), samsahaiCtrl)
		mux.Handle("/", s2hhttp.New(samsahaiCtrl))
		server := httptest.NewServer(mux)
		defer server.Close()

		By("Send webhook")
		jsonPRData, _ := json.Marshal(map[string]interface{}{
			"bundleName": bundledCompPRBundleName,
			"prNumber":   prNumber,
			"components": []map[string]interface{}{
				{
					"name": mariaDBCompName,
					"tag":  mariaDBImageTag,
				},
			},
		})
		apiURL := fmt.Sprintf("%s/teams/%s/pullrequest/trigger", server.URL, teamName)
		_, _, err = utilhttp.Post(apiURL, jsonPRData)
		Expect(err).NotTo(HaveOccurred(), "Pull request webhook sent error")

		By("Verifying PullRequestTrigger has been created")
		err = wait.PollImmediate(verifyTime1s, verifyTime10s, func() (ok bool, err error) {
			prTrigger := s2hv1.PullRequestTrigger{}
			err = client.Get(ctx, types.NamespacedName{Name: bundledPRTriggerName, Namespace: stgNamespace}, &prTrigger)
			if err != nil {
				return false, nil
			}

			return true, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Verify PullRequestTrigger error")

		By("Verifying PullRequestQueue has been created and PullRequestTrigger has been deleted")
		err = wait.PollImmediate(verifyTime1s, verifyTime60s, func() (ok bool, err error) {
			prQueue := s2hv1.PullRequestQueue{}
			err = client.Get(ctx, types.NamespacedName{Name: bundledPRTriggerName, Namespace: stgNamespace}, &prQueue)
			if err != nil {
				return false, nil
			}

			prTrigger := s2hv1.PullRequestTrigger{}
			err = client.Get(ctx, types.NamespacedName{Name: bundledPRTriggerName, Namespace: stgNamespace}, &prTrigger)
			if err != nil && k8serrors.IsNotFound(err) {
				return true, nil
			}

			return false, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Verify PullRequestQueue created error")

		By("Verifying PullRequestQueue has been running and Team status has been updated")
		err = wait.PollImmediate(verifyTime1s, verifyNSCreatedTimeout, func() (ok bool, err error) {
			prQueue := s2hv1.PullRequestQueue{}
			err = client.Get(ctx, types.NamespacedName{Name: bundledPRTriggerName, Namespace: stgNamespace}, &prQueue)
			if err != nil {
				return false, nil
			}

			if prQueue.Status.State == s2hv1.PullRequestQueueWaiting ||
				prQueue.Status.State == s2hv1.PullRequestQueueEnvDestroying {
				return false, nil
			}

			teamComp := s2hv1.Team{}
			err = client.Get(ctx, types.NamespacedName{Name: teamName}, &teamComp)
			if err != nil {
				return false, nil
			}

			if len(teamComp.Status.Namespace.PullRequests) != 0 {
				for _, ns := range teamComp.Status.Namespace.PullRequests {
					if ns == bundledPRNamespace {
						return true, nil
					}
				}
			}

			return false, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Verify PullRequestQueue running error")

		By("Verifying Queue bundled components have been updated")
		err = wait.PollImmediate(verifyTime1s, verifyTime10s, func() (ok bool, err error) {
			queue := s2hv1.Queue{}
			err = client.Get(ctx, types.NamespacedName{Name: bundledPRTriggerName, Namespace: bundledPRNamespace}, &queue)
			if err != nil {
				return false, nil
			}

			return true, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Get pull-request Queue type error")

		queue := s2hv1.Queue{}
		err = client.Get(ctx, types.NamespacedName{Name: bundledPRTriggerName, Namespace: bundledPRNamespace}, &queue)
		Expect(err).NotTo(HaveOccurred(), "Queue bundled components should have been updated")
		Expect(queue.Spec.Components).To(HaveLen(2))
		for _, comp := range queue.Spec.Components {
			if comp.Name == prComps[0].Name {
				Expect(comp.Repository).To(Equal(prComps[0].Repository))
				Expect(comp.Version).To(Equal(prComps[0].Version))
			} else {
				Expect(comp.Repository).To(Equal(prComps[1].Repository))
				Expect(comp.Version).To(Equal(mariaDBImageTag))
			}
		}

		By("Updating mock pull-request Queue type")
		queue.Status.State = s2hv1.Finished
		queue.Status.SetCondition(s2hv1.QueueDeployed, corev1.ConditionTrue, "")
		queue.Status.SetCondition(s2hv1.QueueTested, corev1.ConditionTrue, "")
		Expect(client.Update(ctx, &queue)).NotTo(HaveOccurred(),
			"pull-request Queue type updated error")

		By("Verifying PullRequestQueue has been deleted and PullRequestQueueHistory has been created")
		err = wait.PollImmediate(verifyTime1s, verifyTime30s, func() (ok bool, err error) {
			prQueueHistList := s2hv1.PullRequestQueueHistoryList{}
			err = client.List(ctx, &prQueueHistList, &rclient.ListOptions{Namespace: stgNamespace})
			if err != nil {
				return false, nil
			}

			if len(prQueueHistList.Items) == 0 {
				return false, nil
			}

			if len(prQueueHistList.Items) != 1 {
				return false, fmt.Errorf("should create PullRequestQueueHistory once")
			}

			prQueue := s2hv1.PullRequestQueue{}
			err = client.Get(ctx, types.NamespacedName{Name: bundledPRTriggerName, Namespace: bundledPRNamespace}, &prQueue)
			if err != nil && k8serrors.IsNotFound(err) {
				return true, nil
			}

			return false, nil
		})
		Expect(err).NotTo(HaveOccurred(),
			"Verify PullRequestQueue deleted and PullRequestQueueHistory created error")

		By("Verifying PullRequestQueueHistory result")
		prQueueHistList := s2hv1.PullRequestQueueHistoryList{}
		Expect(client.List(ctx, &prQueueHistList, &rclient.ListOptions{Namespace: stgNamespace})).NotTo(HaveOccurred())
		Expect(prQueueHistList.Items).To(HaveLen(1))
		Expect(strings.Contains(prQueueHistList.Items[0].Name, bundledPRTriggerName)).To(BeTrue())
		Expect(prQueueHistList.Items[0].Spec.PullRequestQueue).NotTo(BeNil())
		Expect(prQueueHistList.Items[0].Spec.PullRequestQueue.Status.Result).To(Equal(s2hv1.PullRequestQueueSuccess))
	}, 120)

	It("should successfully deploy pull request queue with 1 component and dependencies", func(done Done) {
		defer close(done)

		By("Starting Samsahai internal process")
		setupSamsahai()
		go samsahaiCtrl.Start(chStop)

		By("Starting Staging internal process")
		stagingCtrl, _ := setupStaging(stgNamespace)
		go stagingCtrl.Start(chStop)

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

			config := s2hv1.Config{}
			err = client.Get(ctx, types.NamespacedName{Name: teamComp.Name}, &config)
			if err != nil {
				return false, nil
			}

			return true, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Verify namespace and config error")

		By("Starting http server")
		mux := http.NewServeMux()
		mux.Handle(samsahaiCtrl.PathPrefix(), samsahaiCtrl)
		mux.Handle("/", s2hhttp.New(samsahaiCtrl))
		server := httptest.NewServer(mux)
		defer server.Close()

		By("Send webhook")
		jsonPRData, _ := json.Marshal(map[string]interface{}{
			"bundleName": singleCompPRBundleName,
			"prNumber":   prNumber,
		})
		apiURL := fmt.Sprintf("%s/teams/%s/pullrequest/trigger", server.URL, teamName)
		_, _, err = utilhttp.Post(apiURL, jsonPRData)
		Expect(err).NotTo(HaveOccurred(), "Pull request webhook sent error")

		By("Verifying PullRequestTrigger has been created")
		err = wait.PollImmediate(verifyTime1s, verifyTime10s, func() (ok bool, err error) {
			prTrigger := s2hv1.PullRequestTrigger{}
			err = client.Get(ctx, types.NamespacedName{Name: singlePRTriggerName, Namespace: stgNamespace}, &prTrigger)
			if err != nil {
				return false, nil
			}

			return true, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Verify PullRequestTrigger error")

		By("Verifying PullRequestQueue has been created and PullRequestTrigger has been deleted")
		err = wait.PollImmediate(verifyTime1s, verifyTime45s, func() (ok bool, err error) {
			prQueue := s2hv1.PullRequestQueue{}
			err = client.Get(ctx, types.NamespacedName{Name: singlePRTriggerName, Namespace: stgNamespace}, &prQueue)
			if err != nil {
				return false, nil
			}

			prTrigger := s2hv1.PullRequestTrigger{}
			err = client.Get(ctx, types.NamespacedName{Name: singlePRTriggerName, Namespace: stgNamespace}, &prTrigger)
			if err != nil && k8serrors.IsNotFound(err) {
				return true, nil
			}

			return false, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Verify PullRequestQueue created error")

		By("Updating Team mock active components")
		teamComp = s2hv1.Team{}
		Expect(client.Get(ctx, types.NamespacedName{Name: teamName}, &teamComp)).NotTo(HaveOccurred())
		teamComp.Status.ActiveComponents = map[string]s2hv1.StableComponent{
			prDepCompName: {
				Spec: s2hv1.StableComponentSpec{
					Name:       prDepCompName,
					Repository: prDepImage.Repository,
					Version:    prDepImage.Tag,
				},
			},
		}
		Expect(client.Update(ctx, &teamComp)).NotTo(HaveOccurred(),
			"Team active components updated error")

		By("Verifying PullRequestQueue has been running and Team status has been updated")
		err = wait.PollImmediate(verifyTime1s, verifyNSCreatedTimeout, func() (ok bool, err error) {
			prQueue := s2hv1.PullRequestQueue{}
			err = client.Get(ctx, types.NamespacedName{Name: singlePRTriggerName, Namespace: stgNamespace}, &prQueue)
			if err != nil {
				return false, nil
			}

			if prQueue.Status.State == s2hv1.PullRequestQueueWaiting ||
				prQueue.Status.State == s2hv1.PullRequestQueueEnvDestroying {
				return false, nil
			}

			if !prQueue.Status.IsConditionTrue(s2hv1.PullRequestQueueCondDependenciesUpdated) {
				return false, nil
			}

			teamComp := s2hv1.Team{}
			err = client.Get(ctx, types.NamespacedName{Name: teamName}, &teamComp)
			if err != nil {
				return false, nil
			}

			if len(teamComp.Status.Namespace.PullRequests) != 0 {
				for _, ns := range teamComp.Status.Namespace.PullRequests {
					if ns == singlePRNamespace {
						return true, nil
					}
				}
			}

			return false, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Verify PullRequestQueue running error")

		By("Verifying Queue component dependencies have been updated")
		err = wait.PollImmediate(verifyTime1s, verifyTime10s, func() (ok bool, err error) {
			queue := s2hv1.Queue{}
			err = client.Get(ctx, types.NamespacedName{Name: singlePRTriggerName, Namespace: singlePRNamespace}, &queue)
			if err != nil {
				return false, nil
			}

			return true, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Get pull-request Queue type error")

		queue := s2hv1.Queue{}
		err = client.Get(ctx, types.NamespacedName{Name: singlePRTriggerName, Namespace: singlePRNamespace}, &queue)
		Expect(err).NotTo(HaveOccurred(), "Queue component dependencies should have been updated")
		Expect(queue.Spec.Components).To(HaveLen(2))
		Expect(queue.Spec.Components[0].Name).To(Equal(prComps[0].Name))
		Expect(queue.Spec.Components[0].Repository).To(Equal(prComps[0].Repository))
		Expect(queue.Spec.Components[0].Version).To(Equal(prComps[0].Version))
		Expect(queue.Spec.Components[1].Name).To(Equal(prComps[1].Name))
		Expect(queue.Spec.Components[1].Repository).To(Equal(prComps[1].Repository))
		Expect(queue.Spec.Components[1].Version).To(Equal(prComps[1].Version),
			"dependency version should equal active version")
	}, 90)

	It("should successfully add/remove/run pull request from queue", func(done Done) {
		defer close(done)

		By("Starting Samsahai internal process")
		setupSamsahai()
		go samsahaiCtrl.Start(chStop)

		By("Starting Staging internal process")
		stagingCtrl, prQueueCtrl := setupStaging(stgNamespace)
		go stagingCtrl.Start(chStop)

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

			if namespace.Status.Phase == corev1.NamespaceTerminating {
				return false, nil
			}

			config := s2hv1.Config{}
			err = client.Get(ctx, types.NamespacedName{Name: teamComp.Name}, &config)
			if err != nil {
				return false, nil
			}

			return true, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Verify namespace and config error")

		Expect(prQueueCtrl.Size(stgNamespace)).To(Equal(0),
			"should start with empty queue")

		By("Creating 2 mock PullRequestQueues")
		prQueue := s2hv1.PullRequestQueue{
			ObjectMeta: metav1.ObjectMeta{
				Name:      singlePRTriggerName,
				Namespace: stgNamespace,
			},
			Spec: s2hv1.PullRequestQueueSpec{
				BundleName: singleCompPRBundleName,
				PRNumber:   prNumber,
				Components: prComps,
			},
		}
		Expect(prQueueCtrl.Add(&prQueue, nil)).NotTo(HaveOccurred(),
			"add pull request queue #1")
		prQueueName2 := singlePRTriggerName + "-2"
		prQueue2 := s2hv1.PullRequestQueue{
			ObjectMeta: metav1.ObjectMeta{
				Name:      prQueueName2,
				Namespace: stgNamespace,
			},
			Spec: s2hv1.PullRequestQueueSpec{
				BundleName: singleCompPRBundleName,
				PRNumber:   prNumber,
				Components: prComps,
			},
		}
		Expect(prQueueCtrl.Add(&prQueue2, nil)).NotTo(HaveOccurred(),
			"add pull request queue #2")

		By("Verifying one PullRequestQueue has been running and another has been waiting")
		err = wait.PollImmediate(verifyTime1s, verifyTime10s, func() (ok bool, err error) {
			prQueue := s2hv1.PullRequestQueue{}
			err = client.Get(ctx, types.NamespacedName{Name: singlePRTriggerName, Namespace: stgNamespace}, &prQueue)
			if err != nil {
				return false, nil
			}

			if prQueue.Status.State != s2hv1.PullRequestQueueDeploying {
				return false, nil
			}

			prQueue2 := s2hv1.PullRequestQueue{}
			err = client.Get(ctx, types.NamespacedName{Name: prQueueName2, Namespace: stgNamespace}, &prQueue2)
			if err != nil {
				return false, nil
			}

			if prQueue2.Status.State != s2hv1.PullRequestQueueWaiting {
				return false, nil
			}

			return true, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Verify 2 PullRequestQueues running and waiting error")

		By("Deleting running PullRequestQueue")
		prQueue = s2hv1.PullRequestQueue{}
		err = client.Get(ctx, types.NamespacedName{Name: singlePRTriggerName, Namespace: stgNamespace}, &prQueue)
		Expect(err).NotTo(HaveOccurred(), "Get running PullRequestQueue error")
		Expect(client.Delete(ctx, &prQueue)).NotTo(HaveOccurred(),
			"Delete running PullRequestQueue error")

		By("Verify running PullRequestQueue has been deleted and waiting PullRequestQueue has been being run")
		err = wait.PollImmediate(verifyTime1s, verifyNSCreatedTimeout, func() (ok bool, err error) {
			prQueue2 = s2hv1.PullRequestQueue{}
			err = client.Get(ctx, types.NamespacedName{Name: prQueueName2, Namespace: stgNamespace}, &prQueue2)
			if err != nil && k8serrors.IsNotFound(err) {
				return true, nil
			}

			if prQueue2.Status.State == s2hv1.PullRequestQueueWaiting {
				return false, nil
			}

			prQueue = s2hv1.PullRequestQueue{}
			err = client.Get(ctx, types.NamespacedName{Name: singlePRTriggerName, Namespace: stgNamespace}, &prQueue)
			if err != nil && k8serrors.IsNotFound(err) {
				return true, nil
			}

			return false, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Verify running PullRequestQueue deleted error")
	}, 45)

	It("should successfully reset pull request queue if commit SHA changed", func(done Done) {
		defer close(done)

		By("Starting Samsahai internal process")
		setupSamsahai()
		go samsahaiCtrl.Start(chStop)

		By("Starting Staging internal process")
		stagingCtrl, prQueueCtrl := setupStaging(stgNamespace)
		go stagingCtrl.Start(chStop)

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

			if namespace.Status.Phase == corev1.NamespaceTerminating {
				return false, nil
			}

			config := s2hv1.Config{}
			err = client.Get(ctx, types.NamespacedName{Name: teamComp.Name}, &config)
			if err != nil {
				return false, nil
			}

			return true, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Verify namespace and config error")

		Expect(prQueueCtrl.Size(stgNamespace)).To(Equal(0),
			"should start with empty queue")

		By("Creating mock success PullRequestQueue")
		prQueue := s2hv1.PullRequestQueue{
			ObjectMeta: metav1.ObjectMeta{
				Name:      singlePRTriggerName,
				Namespace: stgNamespace,
			},
			Spec: s2hv1.PullRequestQueueSpec{
				BundleName:        singleCompPRBundleName,
				PRNumber:          prNumber,
				Components:        prComps,
				CommitSHA:         commitSHA,
				UpcomingCommitSHA: upComingCommitSHA,
				NoOfRetry:         2,
			},
			Status: s2hv1.PullRequestQueueStatus{
				Result:               s2hv1.PullRequestQueueSuccess,
				State:                s2hv1.PullRequestQueueEnvDestroying,
				PullRequestNamespace: singlePRNamespace,
			},
		}
		Expect(prQueueCtrl.Add(&prQueue, nil)).NotTo(HaveOccurred(),
			"add pull request queue")

		By("Verifying one PullRequestQueue has been updated")
		err = wait.PollImmediate(verifyTime1s, verifyTime15s, func() (ok bool, err error) {
			prQueue := s2hv1.PullRequestQueue{}
			err = client.Get(ctx, types.NamespacedName{Name: singlePRTriggerName, Namespace: stgNamespace}, &prQueue)
			if err != nil {
				return false, nil
			}

			if prQueue.Status.State == s2hv1.PullRequestQueueEnvDestroying {
				return false, nil
			}

			return true, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Verify PullRequestQueue updated error")

		prQueue = s2hv1.PullRequestQueue{}
		err = client.Get(ctx, types.NamespacedName{Name: singlePRTriggerName, Namespace: stgNamespace}, &prQueue)
		Expect(err).NotTo(HaveOccurred())
		Expect(prQueue.Spec.CommitSHA).To(Equal(upComingCommitSHA))
		Expect(prQueue.Spec.NoOfRetry).To(Equal(0))
	}, 45)

	It("should update pull request retry queue if deployment fail", func(done Done) {
		defer close(done)

		By("Starting Samsahai internal process")
		setupSamsahai()
		go samsahaiCtrl.Start(chStop)

		By("Starting Staging internal process")
		stagingCtrl, _ := setupStaging(stgNamespace)
		go stagingCtrl.Start(chStop)

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

			if namespace.Status.Phase == corev1.NamespaceTerminating {
				return false, nil
			}

			config := s2hv1.Config{}
			err = client.Get(ctx, types.NamespacedName{Name: teamComp.Name}, &config)
			if err != nil {
				return false, nil
			}

			return true, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Verify namespace and config error")

		By("Creating mock PullRequestQueue")
		prQueue := s2hv1.PullRequestQueue{
			ObjectMeta: metav1.ObjectMeta{
				Name:      singlePRTriggerName,
				Namespace: stgNamespace,
			},
			Spec: s2hv1.PullRequestQueueSpec{
				BundleName: singleCompPRBundleName,
				PRNumber:   prNumber,
				Components: prComps,
				NoOfRetry:  1,
			},
			Status: s2hv1.PullRequestQueueStatus{
				State:                s2hv1.PullRequestQueueEnvDestroying,
				Result:               s2hv1.PullRequestQueueFailure,
				PullRequestNamespace: singlePRNamespace,
				Conditions:           []s2hv1.PullRequestQueueCondition{mockPrQueueCondition},
			},
		}
		Expect(client.Create(ctx, &prQueue)).NotTo(HaveOccurred(), "Mock queue created error")

		By("Verifying PullRequestQueue has been updated")
		err = wait.PollImmediate(verifyTime1s, verifyNSCreatedTimeout, func() (ok bool, err error) {
			prQueue := s2hv1.PullRequestQueue{}
			err = client.Get(ctx, types.NamespacedName{Name: singlePRTriggerName, Namespace: stgNamespace}, &prQueue)
			if err != nil {
				return false, nil
			}

			if prQueue.Status.State != s2hv1.PullRequestQueueDeploying {
				return false, nil
			}

			if prQueue.Spec.NoOfRetry != 2 {
				return false, nil
			}

			return true, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Verify PullRequestQueue updated error")
	}, 45)

	It("should re-create pull request trigger if re-send webhook and do retry if image not found", func(done Done) {
		defer close(done)

		By("Starting Samsahai internal process")
		setupSamsahai()
		go samsahaiCtrl.Start(chStop)

		By("Starting Staging internal process")
		stagingCtrl, _ := setupStaging(stgNamespace)
		go stagingCtrl.Start(chStop)

		By("Creating Config")
		config := mockConfig
		config.Status.Used.PullRequest.Bundles[0].Components[0].Image.Repository = "missing"
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

			config := s2hv1.Config{}
			err = client.Get(ctx, types.NamespacedName{Name: teamComp.Name}, &config)
			if err != nil {
				return false, nil
			}

			return true, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Verify namespace and config error")

		By("Creating mock PullRequestTrigger")
		prTrigger := s2hv1.PullRequestTrigger{
			ObjectMeta: metav1.ObjectMeta{
				Name:      singlePRTriggerName,
				Namespace: stgNamespace,
			},
			Spec: s2hv1.PullRequestTriggerSpec{
				BundleName: singleCompPRBundleName,
				PRNumber:   prNumber,
				Components: []*s2hv1.PullRequestTriggerComponent{
					{
						ComponentName: wordpressCompName,
						Image:         &s2hv1.Image{Repository: "mock-repo", Tag: "mock-tag"},
					},
				},
				NextProcessAt: &metav1.Time{Time: time.Now().Add(10 * time.Minute)},
			},
		}
		Expect(client.Create(ctx, &prTrigger)).To(BeNil(), "Create mock PullRequestTrigger error")

		By("Starting http server")
		mux := http.NewServeMux()
		mux.Handle(samsahaiCtrl.PathPrefix(), samsahaiCtrl)
		mux.Handle("/", s2hhttp.New(samsahaiCtrl))
		server := httptest.NewServer(mux)
		defer server.Close()

		By("Re-send webhook")
		jsonPRData, _ := json.Marshal(map[string]interface{}{
			"bundleName": singleCompPRBundleName,
			"prNumber":   prNumber,
		})
		apiURL := fmt.Sprintf("%s/teams/%s/pullrequest/trigger", server.URL, teamName)
		_, _, err = utilhttp.Post(apiURL, jsonPRData)
		Expect(err).NotTo(HaveOccurred(), "Pull request webhook sent error")

		By("Verifying PullRequestTrigger has been created with retry")
		err = wait.PollImmediate(verifyTime1s, verifyTime10s, func() (ok bool, err error) {
			prTrigger := s2hv1.PullRequestTrigger{}
			err = client.Get(ctx, types.NamespacedName{Name: singlePRTriggerName, Namespace: stgNamespace}, &prTrigger)
			if err != nil {
				return false, nil
			}

			if prTrigger.Spec.NoOfRetry != nil && *prTrigger.Spec.NoOfRetry < maxPRTriggerRetry {
				return false, nil
			}

			return true, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Verify PullRequestTrigger created error")

		By("Verifying PullRequestTrigger has been deleted")
		err = wait.PollImmediate(verifyTime1s, verifyTime30s, func() (ok bool, err error) {
			prTrigger := s2hv1.PullRequestTrigger{}
			err = client.Get(ctx, types.NamespacedName{Name: singlePRTriggerName, Namespace: stgNamespace}, &prTrigger)
			if err != nil && k8serrors.IsNotFound(err) {
				return true, nil
			}

			return false, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Verify PullRequestTrigger deleted error")
	}, 45)

	It("should return error on trigger if there is no pull request bundle name in configuration or invalid input", func(done Done) {
		defer close(done)

		By("Starting Samsahai internal process")
		setupSamsahai()
		go samsahaiCtrl.Start(chStop)

		By("Starting Staging internal process")
		stagingCtrl, _ := setupStaging(stgNamespace)
		go stagingCtrl.Start(chStop)

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

			config := s2hv1.Config{}
			err = client.Get(ctx, types.NamespacedName{Name: teamComp.Name}, &config)
			if err != nil {
				return false, nil
			}

			return true, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Verify namespace and config error")

		By("Starting http server")
		mux := http.NewServeMux()
		mux.Handle(samsahaiCtrl.PathPrefix(), samsahaiCtrl)
		mux.Handle("/", s2hhttp.New(samsahaiCtrl))
		server := httptest.NewServer(mux)
		defer server.Close()

		By("Send webhook with missing pull request bundle name")
		jsonPRData, _ := json.Marshal(map[string]interface{}{
			"bundleName": "missing-comp",
			"prNumber":   prNumber,
		})
		apiURL := fmt.Sprintf("%s/teams/%s/pullrequest/trigger", server.URL, teamName)
		_, _, err = utilhttp.Post(apiURL, jsonPRData)
		Expect(err).To(HaveOccurred(),
			"Should get status code error due to pull request bundle name not found")

		By("Send webhook with invalid input")
		jsonPRData, _ = json.Marshal(map[string]interface{}{
			"bundleName": singlePRTriggerName,
			"prNumber":   "Invalid/123",
		})
		apiURL = fmt.Sprintf("%s/teams/%s/pullrequest/trigger", server.URL, teamName)
		_, _, err = utilhttp.Post(apiURL, jsonPRData)
		Expect(err).To(HaveOccurred(),
			"Should get status code error due to invalid prNumber")
	}, 20)

	It("should create pull request queue even pull request trigger failed", func(done Done) {
		defer close(done)

		By("Starting Samsahai internal process")
		setupSamsahai()
		go samsahaiCtrl.Start(chStop)

		By("Starting Staging internal process")
		stagingCtrl, _ := setupStaging(stgNamespace)
		go stagingCtrl.Start(chStop)

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

			config := s2hv1.Config{}
			err = client.Get(ctx, types.NamespacedName{Name: teamComp.Name}, &config)
			if err != nil {
				return false, nil
			}

			return true, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Verify namespace and config error")

		By("Starting http server")
		mux := http.NewServeMux()
		mux.Handle(samsahaiCtrl.PathPrefix(), samsahaiCtrl)
		mux.Handle("/", s2hhttp.New(samsahaiCtrl))
		server := httptest.NewServer(mux)
		defer server.Close()

		By("Send webhook with missing image")
		jsonPRData, _ := json.Marshal(map[string]interface{}{
			"bundleName": invalidCompPRBundleName,
			"prNumber":   prNumber,
			"components": []map[string]interface{}{
				{
					"name": wordpressCompName,
				},
			},
		})
		apiURL := fmt.Sprintf("%s/teams/%s/pullrequest/trigger", server.URL, teamName)
		_, _, err = utilhttp.Post(apiURL, jsonPRData)
		Expect(err).NotTo(HaveOccurred(), "Pull request webhook sent error")

		By("Verifying PullRequestQueue has been created and PullRequestTrigger has been deleted")
		err = wait.PollImmediate(verifyTime1s, verifyTime60s, func() (ok bool, err error) {
			prQueueHistList := s2hv1.PullRequestQueueHistoryList{}
			err = client.List(ctx, &prQueueHistList, &rclient.ListOptions{Namespace: stgNamespace})
			//err = client.Get(ctx, types.NamespacedName{Name: invalidPRTriggerName, Namespace: stgNamespace}, &prQueueHistList)
			if err != nil || k8serrors.IsNotFound(err) {
				return false, nil
			}

			if len(prQueueHistList.Items) == 0 {
				return false, nil
			}

			if len(prQueueHistList.Items) != 1 && !strings.Contains(prQueueHistList.Items[0].Name, invalidPRTriggerName) {
				return false, nil
			}

			prTrigger := s2hv1.PullRequestTrigger{}
			err = client.Get(ctx, types.NamespacedName{Name: bundledPRTriggerName, Namespace: stgNamespace}, &prTrigger)
			if err != nil && k8serrors.IsNotFound(err) {
				return true, nil
			}

			return false, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Verify PullRequestQueue created error")

	}, 90)
})

var (
	ctx = context.TODO()

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
			PromoteOnTeamCreation: false,
		},

		SamsahaiCredential: internal.SamsahaiCredential{
			InternalAuthToken: samsahaiAuthToken,
		},
	}

	teamName = "teamtest-pr"

	stgNamespace       = fmt.Sprintf("%s%s", internal.AppPrefix, teamName)
	singlePRNamespace  = fmt.Sprintf("%s-%s-%s", stgNamespace, singleCompPRBundleName, prNumber)
	bundledPRNamespace = fmt.Sprintf("%s-%s-%s", stgNamespace, bundledCompPRBundleName, prNumber)

	testLabels = map[string]string{
		"created-for": "s2h-testing",
	}

	prNumber = "32"

	wordpressMariadbBundleName = "wp-mariadb"
	wordpressBundleName        = "wordpress-bd"
	mariaDBCompName            = "mariadb"
	wordpressCompName          = "wordpress"
	invalidCompPRBundleName    = "invalid-wordpress"
	bundledCompPRBundleName    = wordpressMariadbBundleName
	singleCompPRBundleName     = wordpressBundleName
	prDepCompName              = mariaDBCompName

	mockTeam = s2hv1.Team{
		ObjectMeta: metav1.ObjectMeta{
			Name:   teamName,
			Labels: testLabels,
		},
		Spec: s2hv1.TeamSpec{
			Description: "team for testing",
			Owners:      []string{"samsahai@samsahai.io"},
			Credential: s2hv1.Credential{
				SecretName: s2hobject.GetTeamSecretName(teamName),
			},
			StagingCtrl: &s2hv1.StagingCtrl{
				IsDeploy: false,
			},
		},
		Status: s2hv1.TeamStatus{
			Namespace: s2hv1.TeamNamespace{},
			DesiredComponentImageCreatedTime: map[string]map[string]s2hv1.DesiredImageTime{
				mariaDBCompName: {
					stringutils.ConcatImageString(mariaDBImage.Repository, mariaDBImageTag): s2hv1.DesiredImageTime{
						Image: &s2hv1.Image{Repository: mariaDBImage.Repository, Tag: mariaDBImageTag},
						CreatedTime: metav1.Time{Time: time.Date(2019, 10, 1, 9, 0, 0,
							0, time.UTC)},
					},
				},
				wordpressCompName: {
					stringutils.ConcatImageString(wordpressImage.Repository, wordpressImageTag): s2hv1.DesiredImageTime{
						Image: &s2hv1.Image{Repository: wordpressImage.Repository, Tag: wordpressImageTag},
						CreatedTime: metav1.Time{Time: time.Date(2019, 10, 1, 9, 0, 0,
							0, time.UTC)},
					},
				},
			},
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

	mockPrQueueCondition = s2hv1.PullRequestQueueCondition{
		Type:               s2hv1.PullRequestQueueCondTriggerImagesVerified,
		Status:             corev1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
	}

	compSource          = s2hv1.UpdatingSource("public-registry")
	configCompWordpress = s2hv1.Component{
		Name: wordpressCompName,
		Chart: s2hv1.ComponentChart{
			Repository: "https://charts.helm.sh/stable",
			Name:       wordpressCompName,
		},
		Dependencies: []*s2hv1.Dependency{
			{
				Name: mariaDBCompName,
			},
		},
	}

	configReporter = &s2hv1.ConfigReporter{
		ReportMock: true,
	}

	wordpressImage = s2hv1.ComponentImage{
		Repository: "bitnami/wordpress",
		Pattern:    "5.3.2-debian-10-r{{ .PRNumber }}",
	}
	wordpressImageTag = "5.3.2-debian-10-r32"

	wordpressInvalidImage = s2hv1.ComponentImage{
		Repository: "invalid/wordpress",
		Pattern:    "invalid-{{ .PRNumber }}",
	}

	mariaDBImage = s2hv1.ComponentImage{
		Repository: "bitnami/mariadb",
		Pattern:    "10.5.8-debian-10-r{{ .PRNumber }}",
	}
	mariaDBImageTag = "10.5.8-debian-10-r32"

	prDepImage = s2hv1.ComponentImage{
		Repository: mariaDBImage.Repository,
		Tag:        "latest",
	}

	prComps = []*s2hv1.QueueComponent{
		{
			Name:       wordpressCompName,
			Repository: wordpressImage.Repository,
			Version:    wordpressImageTag,
		},
		{
			Name:       prDepCompName,
			Repository: prDepImage.Repository,
			Version:    prDepImage.Tag,
		},
	}

	commitSHA         = "12345"
	upComingCommitSHA = "67890"

	maxPRTriggerRetry    = 2
	singlePRTriggerName  = internal.GenPullRequestBundleName(singleCompPRBundleName, prNumber)
	bundledPRTriggerName = internal.GenPullRequestBundleName(bundledCompPRBundleName, prNumber)
	invalidPRTriggerName = internal.GenPullRequestBundleName(invalidCompPRBundleName, prNumber)

	configSpec = s2hv1.ConfigSpec{
		Envs: map[s2hv1.EnvType]s2hv1.ChartValuesURLs{
			"pull-request": map[string][]string{
				bundledPRTriggerName: {"https://raw.githubusercontent.com/agoda-com/samsahai-example/master/envs/pull-request/wordpress-missing-mariadb-image.yaml"},
			},
		},
		Staging: &s2hv1.ConfigStaging{
			Deployment: &s2hv1.ConfigDeploy{},
		},
		Components: []*s2hv1.Component{
			&configCompWordpress,
		},
		PullRequest: &s2hv1.ConfigPullRequest{
			Trigger: s2hv1.PullRequestTriggerConfig{
				PollingTime: metav1.Duration{Duration: 1 * time.Second},
				MaxRetry:    &maxPRTriggerRetry,
			},
			Bundles: []*s2hv1.PullRequestBundle{
				{
					Name: singleCompPRBundleName,
					Components: []*s2hv1.PullRequestComponent{
						{
							Name:   wordpressCompName,
							Image:  wordpressImage,
							Source: &compSource,
						},
					},
					Deployment:   &s2hv1.ConfigDeploy{},
					Dependencies: []string{prDepCompName},
				},
				{
					Name: bundledCompPRBundleName,
					Components: []*s2hv1.PullRequestComponent{
						{
							Name:   wordpressCompName,
							Image:  wordpressImage,
							Source: &compSource,
						},
						{
							Name:   mariaDBCompName,
							Image:  mariaDBImage,
							Source: &compSource,
						},
					},
					Deployment: &s2hv1.ConfigDeploy{},
				},
				{
					Name: invalidCompPRBundleName,
					Components: []*s2hv1.PullRequestComponent{
						{
							Name:   wordpressCompName,
							Image:  wordpressInvalidImage,
							Source: &compSource,
						},
					},
					Deployment: &s2hv1.ConfigDeploy{},
				},
			},
			Concurrences: 1,
			PullRequestExtraConfig: s2hv1.PullRequestExtraConfig{
				MaxRetry: &maxPRTriggerRetry,
			},
		},
		Reporter: configReporter,
	}

	mockConfig = s2hv1.Config{
		ObjectMeta: metav1.ObjectMeta{
			Name:   teamName,
			Labels: testLabels,
		},
		Spec: configSpec,
		Status: s2hv1.ConfigStatus{
			Used: configSpec,
		},
	}
)
