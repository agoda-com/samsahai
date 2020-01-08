package staging

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"runtime"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/twitchtv/twirp"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/agoda-com/samsahai/internal"
	s2herrors "github.com/agoda-com/samsahai/internal/errors"
	"github.com/agoda-com/samsahai/internal/k8s/helmrelease"
	s2hlog "github.com/agoda-com/samsahai/internal/log"
	"github.com/agoda-com/samsahai/internal/staging/deploy/fluxhelm"
	"github.com/agoda-com/samsahai/internal/staging/deploy/mock"
	"github.com/agoda-com/samsahai/internal/staging/testrunner/teamcity"
	"github.com/agoda-com/samsahai/internal/staging/testrunner/testmock"
	"github.com/agoda-com/samsahai/internal/util/random"
	s2hv1beta1 "github.com/agoda-com/samsahai/pkg/apis/env/v1beta1"
	samsahairpc "github.com/agoda-com/samsahai/pkg/samsahai/rpc"
	stagingrpc "github.com/agoda-com/samsahai/pkg/staging/rpc"
)

var logger = s2hlog.Log.WithName(internal.StagingCtrlName)

const componentCleanupTimeout = 15 * time.Minute

type controller struct {
	deployEngines map[string]internal.DeployEngine
	testRunners   map[string]internal.StagingTestRunner

	teamName   string
	namespace  string
	authToken  string
	configMgr  internal.ConfigManager
	queueCtrl  internal.QueueController
	clientset  *kubernetes.Clientset
	client     client.Client
	helmClient internal.HelmReleaseClient
	scheme     *apiruntime.Scheme
	//recoder    record.EventRecorder
	//wg         *sync.WaitGroup
	//shutdown   chan struct{}
	internalStop    <-chan struct{}
	internalStopper chan<- struct{}
	rpcHandler      stagingrpc.TwirpServer

	currentQueue *s2hv1beta1.Queue
	mtQueue      sync.Mutex
	mtConfig     sync.Mutex
	s2hClient    samsahairpc.RPC

	lastAppliedValues       map[string]interface{}
	lastStableComponentList s2hv1beta1.StableComponentList

	teamcityBaseURL  string
	teamcityUsername string
	teamcityPassword string
}

func NewController(
	teamName string,
	namespace string,
	authToken string,
	s2hClient samsahairpc.RPC,
	mgr manager.Manager,
	queueCtrl internal.QueueController,
	configMgr internal.ConfigManager,
	teamcityBaseURL string,
	teamcityUsername string,
	teamcityPassword string,
) internal.StagingController {
	if queueCtrl == nil {
		logger.Error(s2herrors.ErrInternalError, "queue ctrl cannot be nil")
		panic(s2herrors.ErrInternalError)
	}

	// creates clientset
	clientset, err := kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		logger.Error(s2herrors.ErrInternalError, "cannot create clientset")
		panic(s2herrors.ErrInternalError)
	}

	stopper := make(chan struct{})
	c := &controller{
		deployEngines:           map[string]internal.DeployEngine{},
		testRunners:             map[string]internal.StagingTestRunner{},
		teamName:                teamName,
		namespace:               namespace,
		authToken:               authToken,
		s2hClient:               s2hClient,
		configMgr:               configMgr,
		queueCtrl:               queueCtrl,
		client:                  mgr.GetClient(),
		clientset:               clientset,
		helmClient:              helmrelease.New(namespace, mgr.GetConfig()),
		scheme:                  mgr.GetScheme(),
		internalStop:            stopper,
		internalStopper:         stopper,
		lastAppliedValues:       nil,
		lastStableComponentList: s2hv1beta1.StableComponentList{},
		teamcityBaseURL:         teamcityBaseURL,
		teamcityUsername:        teamcityUsername,
		teamcityPassword:        teamcityPassword,
	}

	c.rpcHandler = stagingrpc.NewRPCServer(c, nil)

	c.loadDeployEngines()
	c.loadTestRunners()

	return c
}

func (c *controller) Start(stop <-chan struct{}) {
	defer close(c.internalStopper)

	concurrentProcess := 1
	jitterPeriod := time.Millisecond * 1000
	for i := 0; i < concurrentProcess; i++ {
		go wait.Until(func() {
			for c.process() {
			}
		}, jitterPeriod, c.internalStop)
	}

	logger.Debug(fmt.Sprintf("%s is running", internal.StagingCtrlName))

	<-stop

	logger.Info(fmt.Sprintf("%s is shutting down", internal.StagingCtrlName))
}

func (c *controller) process() bool {
	var err error
	if c.getCurrentQueue() == nil {
		c.mtQueue.Lock()
		// pick new queue
		if c.currentQueue, err = c.queueCtrl.First(); err != nil {
			c.mtQueue.Unlock()
			return false
		} else if c.currentQueue != nil {
			if err := c.loadConfiguration(); err != nil {
				logger.Error(err, "cannot load configuration from samsahai")
				c.mtQueue.Unlock()
				return false
			}
		}
		c.mtQueue.Unlock()
	}

	// no queue
	if c.getCurrentQueue() == nil {
		time.Sleep(2 * time.Second)
		return true
	}

	// try to get current queue from k8s
	// if queue is not deleting or cancelling
	if c.isQueueStateValid() {
		if err := c.syncQueueWithK8s(); err != nil {
			return false
		}
	}

	queue := c.getCurrentQueue()

	switch queue.Spec.Type {
	case s2hv1beta1.QueueTypePromoteToActive, s2hv1beta1.QueueTypeDemoteFromActive:
		switch queue.Status.State {
		case "", s2hv1beta1.Waiting:
			queue.Status.NoOfProcessed++
			err = c.updateQueueWithState(queue, s2hv1beta1.DetectingImageMissing)
		case s2hv1beta1.DetectingImageMissing:
			err = c.detectImageMissing(queue)
		case s2hv1beta1.Creating:
			err = c.deployEnvironment(queue)
		case s2hv1beta1.Testing:
			err = c.updateQueueWithState(queue, s2hv1beta1.Collecting)
		case s2hv1beta1.Collecting:
			err = c.collectResult(queue)
		case s2hv1beta1.Cancelling:
			err = c.cancelQueue(queue)
		case s2hv1beta1.Finished:
		}
	default:
		switch queue.Status.State {
		case "", s2hv1beta1.Waiting:
			err = c.initQueue(queue)
		case s2hv1beta1.CleaningBefore:
			err = c.cleanBefore(queue)
		case s2hv1beta1.DetectingImageMissing:
			err = c.detectImageMissing(queue)
		case s2hv1beta1.Creating:
			err = c.deployEnvironment(queue)
		case s2hv1beta1.Testing:
			err = c.startTesting(queue)
		case s2hv1beta1.Collecting:
			err = c.collectResult(queue)
		case s2hv1beta1.CleaningAfter:
			err = c.cleanAfter(queue)
		case s2hv1beta1.Deleting:
			err = c.deleteQueue(queue)
		case s2hv1beta1.Cancelling:
			err = c.cancelQueue(queue)
		case s2hv1beta1.Finished:
		default:
		}
	}

	return err != nil
}

func (c *controller) loadDeployEngines() {
	// init test runner
	engines := []internal.DeployEngine{
		mock.New(),
		fluxhelm.New(c.configMgr, c.helmClient),
	}

	for _, e := range engines {
		if e == nil {
			continue
		}

		c.deployEngines[e.GetName()] = e
	}
}

func (c *controller) loadTestRunners() {
	// init test runner
	testRunners := []internal.StagingTestRunner{
		testmock.New(),
	}

	// TODO: should load teamcity credentials from secret, default from samsahai
	if c.teamcityBaseURL != "" && c.teamcityUsername != "" && c.teamcityPassword != "" {
		testRunners = append(testRunners, teamcity.New(c.client, c.teamcityBaseURL, c.teamcityUsername, c.teamcityPassword))
	}

	for _, r := range testRunners {
		if r == nil {
			continue
		}

		c.testRunners[r.GetName()] = r
	}
}

func (c *controller) IsBusy() bool {
	return c.getCurrentQueue() != nil
}

func (c *controller) LoadTestRunner(runner internal.StagingTestRunner) {
	if runner == nil || runner.GetName() == "" {
		return
	}
	c.testRunners[runner.GetName()] = runner
}

func (c *controller) LoadDeployEngine(engine internal.DeployEngine) {
	if engine == nil || engine.GetName() == "" {
		return
	}
	c.deployEngines[engine.GetName()] = engine
}

// isQueueValid returns true if Queue not in Deleting and Cancelling state
func (c *controller) isQueueStateValid() bool {
	q := c.getCurrentQueue()
	return q.Status.State != s2hv1beta1.Deleting && q.Status.State != s2hv1beta1.Cancelling
}

// syncQueueWithK8s fetches Queue from k8s and set it to currentQueue if mismatch
func (c *controller) syncQueueWithK8s() error {
	var err error

	q := c.getCurrentQueue()
	fetched := &s2hv1beta1.Queue{}
	err = c.client.Get(context.TODO(), types.NamespacedName{
		Namespace: q.GetNamespace(),
		Name:      q.GetName()}, fetched)
	if err != nil && k8serrors.IsNotFound(err) {
		// queue not found
		// delete by user
		logger.Debug(fmt.Sprintf("queue: %s/%s got cancel", q.GetNamespace(), q.GetName()))
		c.mtQueue.Lock()
		c.currentQueue.SetState(s2hv1beta1.Cancelling)
		c.mtQueue.Unlock()
	} else if err != nil {
		logger.Error(err, fmt.Sprintf("cannot get queue: %s/%s", q.GetNamespace(), q.GetName()))
		return err
	} else if !reflect.DeepEqual(fetched, q) {
		// update current queue
		c.mtQueue.Lock()
		c.currentQueue = fetched
		c.mtQueue.Unlock()
	}

	return nil
}

func (c *controller) initQueue(q *s2hv1beta1.Queue) error {
	q.Status.NoOfProcessed++
	q.Status.QueueHistoryName = q.Name + "-" + random.GenerateRandomString(10)
	q.Status.SetCondition(s2hv1beta1.QueueCleaningBeforeStarted, corev1.ConditionTrue,
		"starts cleaning the namespace before running task")

	return c.updateQueueWithState(q, s2hv1beta1.CleaningBefore)
}

func (c *controller) cleanBefore(queue *s2hv1beta1.Queue) error {
	deployEngine := c.getDeployEngine(c.getDeployConfiguration(queue))
	if !queue.Status.IsConditionTrue(s2hv1beta1.QueueCleanedBefore) {
		if err := deployEngine.Delete(queue); err != nil {
			return err
		}
	}

	cleanupTimeout := &metav1.Duration{Duration: componentCleanupTimeout}
	deployConfig := c.getDeployConfiguration(queue)
	if deployConfig != nil {
		cleanupTimeout = &deployConfig.ComponentCleanupTimeout
	}

	startedCleaningTime := queue.Status.GetConditionLatestTime(s2hv1beta1.QueueCleaningBeforeStarted)
	isCleaned, err := WaitForComponentsCleaned(c.clientset, deployEngine, c.configMgr.GetParentComponents(),
		c.teamName, c.namespace, startedCleaningTime, cleanupTimeout)
	if err != nil {
		return err
	} else if !isCleaned {
		time.Sleep(2 * time.Second)
		return nil
	}

	queue.Status.SetCondition(
		s2hv1beta1.QueueCleanedBefore,
		corev1.ConditionTrue,
		"namespace cleaned")

	return c.updateQueueWithState(queue, s2hv1beta1.DetectingImageMissing)
}

func (c *controller) cleanAfter(queue *s2hv1beta1.Queue) error {
	deployEngine := c.getDeployEngine(c.getDeployConfiguration(queue))
	if !queue.Status.IsConditionTrue(s2hv1beta1.QueueCleanedAfter) {
		if err := deployEngine.Delete(queue); err != nil {
			return err
		}
	}

	cleanupTimeout := &metav1.Duration{Duration: componentCleanupTimeout}
	deployConfig := c.getDeployConfiguration(queue)
	if deployConfig != nil {
		cleanupTimeout = &deployConfig.ComponentCleanupTimeout
	}

	startedCleaningTime := queue.Status.GetConditionLatestTime(s2hv1beta1.QueueCleaningAfterStarted)
	isCleaned, err := WaitForComponentsCleaned(c.clientset, deployEngine, c.configMgr.GetParentComponents(),
		c.teamName, c.namespace, startedCleaningTime, cleanupTimeout)
	if err != nil {
		return err
	} else if !isCleaned {
		time.Sleep(2 * time.Second)
		return nil
	}

	queue.Status.SetCondition(
		s2hv1beta1.QueueCleanedAfter,
		corev1.ConditionTrue,
		"namespace cleaned")

	return c.updateQueueWithState(queue, s2hv1beta1.Deleting)
}

func (c *controller) cancelQueue(q *s2hv1beta1.Queue) error {
	c.clearCurrentQueue()
	return nil
}

// loadConfiguration loads config from Samsahai Controller
func (c *controller) loadConfiguration() (err error) {
	if c.s2hClient == nil {
		return nil
	}

	headers := http.Header{}
	headers.Set(internal.SamsahaiAuthHeader, c.authToken)
	ctx := context.TODO()
	ctx, err = twirp.WithHTTPRequestHeaders(ctx, headers)
	if err != nil {
		return errors.Wrap(err, "cannot set request header")
	}
	config, err := c.s2hClient.GetConfiguration(ctx, &samsahairpc.Team{Name: c.teamName})
	if err != nil {
		return errors.Wrap(err, "cannot load configuration from server")
	}

	var cfg internal.Configuration
	err = json.Unmarshal(config.Config, &cfg)
	if err != nil {
		return errors.Wrap(err, "cannot load configuration from server")
	}

	c.mtConfig.Lock()
	c.configMgr.Load(&cfg, config.GitRevision)
	c.mtConfig.Unlock()
	return nil
}

func (c *controller) getConfiguration() *internal.Configuration {
	c.mtConfig.Lock()
	defer c.mtConfig.Unlock()
	return c.configMgr.Get()
}

func (c *controller) getConfigManager() internal.ConfigManager {
	c.mtConfig.Lock()
	defer c.mtConfig.Unlock()
	return c.configMgr
}

func WaitForComponentsCleaned(
	client *kubernetes.Clientset,
	deployEngine internal.DeployEngine,
	parentComps map[string]*internal.Component,
	teamName string,
	namespace string,
	startedCleanupTime *metav1.Time,
	timeout *metav1.Duration,
) (bool, error) {
	if deployEngine.IsMocked() {
		return true, nil
	}

	for _, comp := range parentComps {
		selectors := deployEngine.GetLabelSelectors(genReleaseName(teamName, namespace, comp.Name))
		listOpt := metav1.ListOptions{LabelSelector: labels.SelectorFromSet(selectors).String()}

		if startedCleanupTime == nil || timeout.Duration == 0 {
			timeout = &metav1.Duration{Duration: componentCleanupTimeout}
		}
		if isTimeout := isCleanupTimeout(startedCleanupTime, timeout); isTimeout {
			forceCleanupResources(client, namespace, listOpt)
		}

		// check pods
		podList, err := client.CoreV1().Pods(namespace).List(listOpt)
		if err != nil {
			logger.Error(err, "list pods error",
				"namespace", namespace, "selector", labels.SelectorFromSet(selectors).String())

			return false, err
		}

		if len(podList.Items) > 0 {
			return false, nil
		}

		// check services
		services, err := client.CoreV1().Services(namespace).List(listOpt)
		if err != nil {
			logger.Error(err, "list services error",
				"namespace", namespace, "selector", labels.SelectorFromSet(selectors).String())
			return false, err
		}

		if len(services.Items) > 0 {
			return false, nil
		}

		// check pvcs
		pvcs, err := client.CoreV1().PersistentVolumeClaims(namespace).List(listOpt)
		if err != nil {
			logger.Error(err, "list pvcs error",
				"namespace", namespace, "selector", labels.SelectorFromSet(selectors).String())
			return false, err
		}

		if len(pvcs.Items) > 0 {
			logger.Debug("pvc found, deleting")
			deletePropagation := metav1.DeletePropagationBackground
			err = client.CoreV1().PersistentVolumeClaims(namespace).
				DeleteCollection(&metav1.DeleteOptions{PropagationPolicy: &deletePropagation}, listOpt)
			if err != nil {
				logger.Warn(fmt.Sprintf("delete all pvc error: %+v", err),
					"namespace", namespace, "selector", labels.SelectorFromSet(selectors).String())
			}
			return false, nil
		}
	}

	return true, nil
}

func isCleanupTimeout(startedTime *metav1.Time, timeout *metav1.Duration) bool {
	// if started time or timeout values are nil, no timeout
	if startedTime == nil || timeout == nil {
		return false
	}

	now := metav1.Now()
	return now.Sub(startedTime.Time) > timeout.Duration
}

func forceCleanupResources(client *kubernetes.Clientset, namespace string, listOpt metav1.ListOptions) {
	logger.Debug("force cleaning up all pods", "namespace", namespace)

	// force delete pods and all jobs without list options
	gracePeriod := int64(0)
	deletePropagation := metav1.DeletePropagationBackground
	deleteOpt := &metav1.DeleteOptions{GracePeriodSeconds: &gracePeriod, PropagationPolicy: &deletePropagation}

	if err := client.CoreV1().Pods(namespace).DeleteCollection(deleteOpt, listOpt); err != nil {
		logger.Error(err, "cannot delete pods", "namespace", namespace, "listOpt", listOpt)
	}

	// to cleanup all jobs in case of pre-delete hook
	forceCleanupJobsWithoutListOpt(client, namespace, deleteOpt)
}

func forceCleanupJobsWithoutListOpt(client *kubernetes.Clientset, namespace string, deleteOpt *metav1.DeleteOptions) {
	logger.Debug("force cleaning up all jobs", "namespace", namespace)

	if err := client.BatchV1().Jobs(namespace).DeleteCollection(deleteOpt, metav1.ListOptions{}); err != nil {
		logger.Error(err, "cannot delete jobs", "namespace", namespace)
	}
}
