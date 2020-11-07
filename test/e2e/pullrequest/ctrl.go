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

var _ = FDescribe("[e2e] Pull request controller", func() {
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
		_ = client.Create(context.TODO(), &secret)
	}, 60)

	AfterEach(func(done Done) {
		defer close(done)
		ctx := context.TODO()

		By("Deleting all Teams")
		err = client.DeleteAllOf(ctx, &s2hv1.Team{}, rclient.MatchingLabels(testLabels))
		Expect(err).NotTo(HaveOccurred())
		err = wait.PollImmediate(verifyTime1s, verifyTime10s, func() (ok bool, err error) {
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

		By("Deleting pull request namespace")
		prNs := corev1.Namespace{}
		err = client.Get(ctx, types.NamespacedName{Name: prNamespace}, &prNs)
		if err != nil && k8serrors.IsNotFound(err) {
			_ = client.Delete(context.TODO(), &prNs)
			err = wait.PollImmediate(verifyTime1s, verifyTime10s, func() (ok bool, err error) {
				namespace := corev1.Namespace{}
				err = client.Get(ctx, types.NamespacedName{Name: prNamespace}, &namespace)
				if err != nil && k8serrors.IsNotFound(err) {
					return true, nil
				}
				return false, nil
			})
		}

		By("Deleting all PullRequestQueues")
		err = client.DeleteAllOf(context.TODO(), &s2hv1.PullRequestQueue{}, rclient.InNamespace(stgNamespace))
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

		By("Deleting Secret")
		secret := mockSecret
		Expect(client.Delete(context.TODO(), &secret)).NotTo(HaveOccurred())

		By("Deleting Config")
		Expect(samsahaiCtrl.GetConfigController().Delete(teamName)).NotTo(HaveOccurred())

		close(chStop)
		samsahaiServer.Close()
		wgStop.Wait()
	}, 60)

	Describe("Pull request", func() {
		It("should successfully deploy pull request queue", func(done Done) {
			defer close(done)

			By("Starting Samsahai internal process")
			setupSamsahai()
			go samsahaiCtrl.Start(chStop)

			By("Starting Staging internal process")
			stagingCtrl, _ := setupStaging(stgNamespace)
			go stagingCtrl.Start(chStop)

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
			jsonPRData, err := json.Marshal(map[string]interface{}{
				"component": prCompName,
				"prNumber":  prNumber,
			})
			Expect(err).NotTo(HaveOccurred(), "Pull request webhook sent error")

			By("Verifying PullRequestTrigger has been created")
			err = wait.PollImmediate(verifyTime1s, verifyTime10s, func() (ok bool, err error) {
				apiURL := fmt.Sprintf("%s/teams/%s/pullrequest/trigger", server.URL, teamName)
				_, _, _ = utilhttp.Post(apiURL, jsonPRData)
				prTrigger := s2hv1.PullRequestTrigger{}
				err = client.Get(ctx, types.NamespacedName{Name: prTriggerName, Namespace: stgNamespace}, &prTrigger)
				if err != nil {
					return false, nil
				}

				return true, nil
			})
			Expect(err).NotTo(HaveOccurred(), "Verify PullRequestTrigger error")

			By("Verifying PullRequestQueue has been created and PullRequestTrigger has been deleted")
			err = wait.PollImmediate(verifyTime1s, verifyTime45s, func() (ok bool, err error) {
				prQueue := s2hv1.PullRequestQueue{}
				err = client.Get(ctx, types.NamespacedName{Name: prTriggerName, Namespace: stgNamespace}, &prQueue)
				if err != nil {
					return false, nil
				}

				prTrigger := s2hv1.PullRequestTrigger{}
				err = client.Get(ctx, types.NamespacedName{Name: prTriggerName, Namespace: stgNamespace}, &prTrigger)
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
						Repository: prComps[1].Repository,
						Version:    prComps[1].Version,
					},
				},
			}
			Expect(client.Update(ctx, &teamComp)).NotTo(HaveOccurred(),
				"Team active components updated error")

			By("Verifying PullRequestQueue has been running and Team status has been updated")
			err = wait.PollImmediate(verifyTime1s, verifyNSCreatedTimeout, func() (ok bool, err error) {
				prQueue := s2hv1.PullRequestQueue{}
				err = client.Get(ctx, types.NamespacedName{Name: prTriggerName, Namespace: stgNamespace}, &prQueue)
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
						if ns == prNamespace {
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
				err = client.Get(ctx, types.NamespacedName{Name: prTriggerName, Namespace: prNamespace}, &queue)
				if err != nil {
					return false, nil
				}

				return true, nil
			})
			Expect(err).NotTo(HaveOccurred(), "Get pull-request Queue type error")

			queue := s2hv1.Queue{}
			err = client.Get(ctx, types.NamespacedName{Name: prTriggerName, Namespace: prNamespace}, &queue)
			Expect(err).NotTo(HaveOccurred(), "Queue component dependencies should have been updated")
			Expect(queue.Spec.Components).To(HaveLen(2))
			Expect(queue.Spec.Components[0].Name).To(Equal(prComps[0].Name))
			Expect(queue.Spec.Components[0].Repository).To(Equal(prComps[0].Repository))
			Expect(queue.Spec.Components[0].Version).To(Equal(prComps[0].Version))
			Expect(queue.Spec.Components[1].Name).To(Equal(prComps[1].Name))
			Expect(queue.Spec.Components[1].Repository).To(Equal(prComps[1].Repository))
			Expect(queue.Spec.Components[1].Version).To(Equal(prComps[1].Version))

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
				err = client.Get(ctx, types.NamespacedName{Name: prTriggerName, Namespace: prNamespace}, &prQueue)
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
			Expect(strings.Contains(prQueueHistList.Items[0].Name, prTriggerName)).To(BeTrue())
			Expect(prQueueHistList.Items[0].Spec.PullRequestQueue).NotTo(BeNil())
			Expect(prQueueHistList.Items[0].Spec.PullRequestQueue.Status.Result).To(Equal(s2hv1.PullRequestQueueSuccess))
		}, 120)

		It("should successfully add/remove/run pull request from queue", func(done Done) {
			defer close(done)

			By("Starting Samsahai internal process")
			setupSamsahai()
			go samsahaiCtrl.Start(chStop)

			By("Starting Staging internal process")
			stagingCtrl, prQueueCtrl := setupStaging(stgNamespace)
			go stagingCtrl.Start(chStop)

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
					Name:      prTriggerName,
					Namespace: stgNamespace,
				},
				Spec: s2hv1.PullRequestQueueSpec{
					ComponentName: prCompName,
					PRNumber:      prNumber,
					Components:    prComps,
				},
			}
			Expect(prQueueCtrl.Add(&prQueue, nil)).NotTo(HaveOccurred(),
				"add pull request queue #1")
			prQueueName2 := prTriggerName + "-2"
			prQueue2 := s2hv1.PullRequestQueue{
				ObjectMeta: metav1.ObjectMeta{
					Name:      prQueueName2,
					Namespace: stgNamespace,
				},
				Spec: s2hv1.PullRequestQueueSpec{
					ComponentName: prCompName,
					PRNumber:      prNumber,
					Components:    prComps,
				},
			}
			Expect(prQueueCtrl.Add(&prQueue2, nil)).NotTo(HaveOccurred(),
				"add pull request queue #2")

			By("Verifying one PullRequestQueue has been running and another has been waiting")
			err = wait.PollImmediate(verifyTime1s, verifyTime10s, func() (ok bool, err error) {
				prQueue := s2hv1.PullRequestQueue{}
				err = client.Get(ctx, types.NamespacedName{Name: prTriggerName, Namespace: stgNamespace}, &prQueue)
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
			err = client.Get(ctx, types.NamespacedName{Name: prTriggerName, Namespace: stgNamespace}, &prQueue)
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
				err = client.Get(ctx, types.NamespacedName{Name: prTriggerName, Namespace: stgNamespace}, &prQueue)
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
					Name:      prTriggerName,
					Namespace: stgNamespace,
				},
				Spec: s2hv1.PullRequestQueueSpec{
					ComponentName:     prCompName,
					PRNumber:          prNumber,
					Components:        prComps,
					CommitSHA:         commitSHA,
					UpcomingCommitSHA: upComingCommitSHA,
					NoOfRetry:         2,
				},
				Status: s2hv1.PullRequestQueueStatus{
					Result:               s2hv1.PullRequestQueueSuccess,
					State:                s2hv1.PullRequestQueueEnvDestroying,
					PullRequestNamespace: prNamespace,
				},
			}
			Expect(prQueueCtrl.Add(&prQueue, nil)).NotTo(HaveOccurred(),
				"add pull request queue")

			By("Verifying one PullRequestQueue has been updated")
			err = wait.PollImmediate(verifyTime1s, verifyTime15s, func() (ok bool, err error) {
				prQueue := s2hv1.PullRequestQueue{}
				err = client.Get(ctx, types.NamespacedName{Name: prTriggerName, Namespace: stgNamespace}, &prQueue)
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
			err = client.Get(ctx, types.NamespacedName{Name: prTriggerName, Namespace: stgNamespace}, &prQueue)
			Expect(err).NotTo(HaveOccurred())
			Expect(prQueue.Spec.CommitSHA).To(Equal(upComingCommitSHA))
			Expect(prQueue.Spec.NoOfRetry).To(Equal(0))
		}, 45)

		It("should do pull request retry trigger if image not found", func(done Done) {
			defer close(done)

			By("Starting Samsahai internal process")
			setupSamsahai()
			go samsahaiCtrl.Start(chStop)

			By("Starting Staging internal process")
			stagingCtrl, _ := setupStaging(stgNamespace)
			go stagingCtrl.Start(chStop)

			ctx := context.TODO()

			By("Creating Config")
			config := mockConfig
			config.Status.Used.PullRequest.Components[0].Image.Repository = "missing"
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
			jsonPRData, err := json.Marshal(map[string]interface{}{
				"component": prCompName,
				"prNumber":  prNumber,
			})
			Expect(err).NotTo(HaveOccurred(), "Pull request webhook sent error")

			By("Verifying PullRequestTrigger has been created with retry")
			err = wait.PollImmediate(verifyTime1s, verifyTime10s, func() (ok bool, err error) {
				apiURL := fmt.Sprintf("%s/teams/%s/pullrequest/trigger", server.URL, teamName)
				_, _, _ = utilhttp.Post(apiURL, jsonPRData)
				prTrigger := s2hv1.PullRequestTrigger{}
				err = client.Get(ctx, types.NamespacedName{Name: prTriggerName, Namespace: stgNamespace}, &prTrigger)
				if err != nil {
					return false, nil
				}

				if prTrigger.Status.NoOfRetry != nil && *prTrigger.Status.NoOfRetry < prMaxRetry {
					return false, nil
				}

				return true, nil
			})
			Expect(err).NotTo(HaveOccurred(), "Verify PullRequestTrigger created error")

			By("Verifying PullRequestTrigger has been deleted")
			err = wait.PollImmediate(verifyTime1s, verifyTime30s, func() (ok bool, err error) {
				prTrigger := s2hv1.PullRequestTrigger{}
				err = client.Get(ctx, types.NamespacedName{Name: prTriggerName, Namespace: stgNamespace}, &prTrigger)
				if err != nil && k8serrors.IsNotFound(err) {
					return true, nil
				}

				return false, nil
			})
			Expect(err).NotTo(HaveOccurred(), "Verify PullRequestTrigger deleted error")
		}, 60)

		It("should update pull request retry queue if deployment fail", func(done Done) {
			defer close(done)

			By("Starting Samsahai internal process")
			setupSamsahai()
			go samsahaiCtrl.Start(chStop)

			By("Starting Staging internal process")
			stagingCtrl, _ := setupStaging(stgNamespace)
			go stagingCtrl.Start(chStop)

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
					Name:      prTriggerName,
					Namespace: stgNamespace,
				},
				Spec: s2hv1.PullRequestQueueSpec{
					ComponentName: prCompName,
					PRNumber:      prNumber,
					Components:    prComps,
					NoOfRetry:     1,
				},
				Status: s2hv1.PullRequestQueueStatus{
					State:                s2hv1.PullRequestQueueEnvDestroying,
					Result:               s2hv1.PullRequestQueueFailure,
					PullRequestNamespace: prNamespace,
				},
			}
			Expect(client.Create(ctx, &prQueue)).NotTo(HaveOccurred(), "Mock queue created error")

			By("Verifying PullRequestQueue has been updated")
			err = wait.PollImmediate(verifyTime1s, verifyNSCreatedTimeout, func() (ok bool, err error) {
				prQueue := s2hv1.PullRequestQueue{}
				err = client.Get(ctx, types.NamespacedName{Name: prTriggerName, Namespace: stgNamespace}, &prQueue)
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
	})
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
			PromoteOnTeamCreation: false,
		},

		SamsahaiCredential: internal.SamsahaiCredential{
			InternalAuthToken: samsahaiAuthToken,
		},
	}

	teamName = "teamtest-pr"

	stgNamespace = fmt.Sprintf("%s%s", internal.AppPrefix, teamName)
	prNamespace  = fmt.Sprintf("%s-%s-%s", stgNamespace, prCompName, prNumber)

	testLabels = map[string]string{
		"created-for": "s2h-testing",
	}

	prNumber = "18"

	redisCompName     = "redis"
	mariaDBCompName   = "mariadb"
	wordpressCompName = "wordpress"
	prCompName        = wordpressCompName
	prDepCompName     = mariaDBCompName

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
					stringutils.ConcatImageString("bitnami/mariadb", "10.3.18-debian-9-r32"): s2hv1.DesiredImageTime{
						Image:       &s2hv1.Image{Repository: "bitnami/mariadb", Tag: "10.3.18-debian-9-r32"},
						CreatedTime: metav1.Time{Time: time.Date(2019, 10, 1, 9, 0, 0, 0, time.UTC)},
					},
				},
				redisCompName: {
					stringutils.ConcatImageString("bitnami/redis", "5.0.5-debian-9-r160"): s2hv1.DesiredImageTime{
						Image:       &s2hv1.Image{Repository: "bitnami/redis", Tag: "5.0.5-debian-9-r160"},
						CreatedTime: metav1.Time{Time: time.Date(2019, 10, 1, 9, 0, 0, 0, time.UTC)},
					},
				},
				wordpressCompName: {
					stringutils.ConcatImageString("bitnami/wordpress", "5.2.4-debian-9-r18"): s2hv1.DesiredImageTime{
						Image:       &s2hv1.Image{Repository: "bitnami/wordpress", Tag: "5.2.4-debian-9-r18"},
						CreatedTime: metav1.Time{Time: time.Date(2019, 10, 1, 9, 0, 0, 0, time.UTC)},
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

	compSource      = s2hv1.UpdatingSource("public-registry")
	configCompRedis = s2hv1.Component{
		Name: redisCompName,
		Chart: s2hv1.ComponentChart{
			Repository: "https://kubernetes-charts.storage.googleapis.com",
			Name:       redisCompName,
		},
		Image: s2hv1.ComponentImage{
			Repository: "bitnami/redis",
			Pattern:    "5.*debian-9.*",
		},
		Source: &compSource,
		Values: s2hv1.ComponentValues{
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

	configReporter = &s2hv1.ConfigReporter{
		ReportMock: true,
	}

	prImage = s2hv1.ComponentImage{
		Repository: "bitnami/wordpress",
		Pattern:    "5.2.4-debian-9-r{{ .PRNumber }}",
	}

	prComps = []*s2hv1.QueueComponent{
		{
			Name:       prCompName,
			Repository: "bitnami/wordpress",
			Version:    "5.2.4-debian-9-r18",
		},
		{
			Name:       prDepCompName,
			Repository: "bitnami/mariadb",
			Version:    "latest",
		},
	}

	commitSHA         = "12345"
	upComingCommitSHA = "67890"

	prMaxRetry    = 2
	prTriggerName = internal.GenPullRequestComponentName(prCompName, prNumber)

	configSpec = s2hv1.ConfigSpec{
		Staging: &s2hv1.ConfigStaging{
			Deployment: &s2hv1.ConfigDeploy{},
		},
		Components: []*s2hv1.Component{
			&configCompRedis,
		},
		PullRequest: &s2hv1.ConfigPullRequest{
			Trigger: s2hv1.PullRequestTriggerConfig{
				PollingTime: metav1.Duration{Duration: 1 * time.Second},
				MaxRetry:    &prMaxRetry,
			},
			Components: []*s2hv1.PullRequestComponent{
				{
					Name:         prCompName,
					Image:        prImage,
					Source:       &compSource,
					Deployment:   &s2hv1.ConfigDeploy{},
					Dependencies: []string{prDepCompName},
				},
			},
			Concurrences: 1,
			PullRequestExtraConfig: s2hv1.PullRequestExtraConfig{
				MaxRetry: &prMaxRetry,
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
