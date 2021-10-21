package trigger

import (
	"context"
	"net/http"
	"time"

	"github.com/twitchtv/twirp"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crctrl "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	s2hv1 "github.com/agoda-com/samsahai/api/v1"
	"github.com/agoda-com/samsahai/internal"
	"github.com/agoda-com/samsahai/internal/errors"
	s2herrors "github.com/agoda-com/samsahai/internal/errors"
	s2hlog "github.com/agoda-com/samsahai/internal/log"
	prqueuectrl "github.com/agoda-com/samsahai/internal/pullrequest/queue"
	samsahairpc "github.com/agoda-com/samsahai/pkg/samsahai/rpc"
)

const (
	CtrlName = "pull-request-trigger-ctrl"
)

var logger = s2hlog.Log.WithName(CtrlName)

type controller struct {
	teamName    string
	client      client.Client
	prQueueCtrl internal.QueueController
	authToken   string
	s2hClient   samsahairpc.RPC
}

func New(
	teamName string,
	mgr manager.Manager,
	prQueueCtrl internal.QueueController,
	authToken string,
	s2hClient samsahairpc.RPC,
) internal.PullRequestTriggerController {
	c := &controller{
		teamName:    teamName,
		client:      mgr.GetClient(),
		prQueueCtrl: prQueueCtrl,
		authToken:   authToken,
		s2hClient:   s2hClient,
	}

	if err := add(mgr, c); err != nil {
		logger.Error(err, "cannot add new controller to manager")
	}

	return c
}

var _ reconcile.Reconciler = &controller{}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := crctrl.New(CtrlName, mgr, crctrl.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to PullRequestTrigger
	err = c.Watch(&source.Kind{Type: &s2hv1.PullRequestTrigger{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

// Reconcile reads that state of the cluster for a PullRequestTrigger object and makes changes based on the state read
// and what is in the PullRequestTrigger.Spec
// +kubebuilder:rbac:groups=env.samsahai.io,resources=pullrequesttriggers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=env.samsahai.io,resources=pullrequesttriggers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=env.samsahai.io,resources=pullrequestqueues,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=env.samsahai.io,resources=pullrequestqueues/status,verbs=get;update;patch
func (c *controller) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	ctx := context.TODO()

	now := metav1.Now()
	prTrigger := &s2hv1.PullRequestTrigger{}
	err := c.client.Get(ctx, req.NamespacedName, prTrigger)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			// Object not found, return. Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// wait until next run
	nextProcessAt := prTrigger.Spec.NextProcessAt
	if nextProcessAt != nil && now.Before(nextProcessAt) {
		return reconcile.Result{
			Requeue:      true,
			RequeueAfter: 2 * time.Second,
		}, nil
	}

	// make request with samsahai controller
	headers := make(http.Header)
	headers.Set(internal.SamsahaiAuthHeader, c.authToken)
	ctx, err = twirp.WithHTTPRequestHeaders(ctx, headers)
	if err != nil {
		logger.Error(err, "cannot set request header")
	}

	prConfig, err := c.s2hClient.GetPullRequestConfig(ctx, &samsahairpc.TeamWithBundleName{
		TeamName:   c.teamName,
		BundleName: prTrigger.Spec.BundleName})
	if err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "cannot get pull request config of team: %s", c.teamName)
	}

	pollingTime, err := time.ParseDuration(prConfig.Trigger.PollingTime)
	if err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "cannot parse trigger polling time string: %s",
			prConfig.Trigger.PollingTime)
	}

	name := prTrigger.Spec.BundleName
	prNumber := prTrigger.Spec.PRNumber
	commitSHA := prTrigger.Spec.CommitSHA
	noOfRetry := prTrigger.Spec.NoOfRetry
	maxRetry := prConfig.Trigger.MaxRetry
	gitRepo := prConfig.GitRepository

	var tearDownDuration s2hv1.PullRequestTearDownDuration
	// overwrite tearDownDuration
	if prTrigger.Spec.TearDownDuration != nil {
		tearDownDuration = *prTrigger.Spec.TearDownDuration
	} else {
		configTearDownDuration := prConfig.GetTearDownDuration()

		duration := metav1.Duration{
			Duration: time.Duration(configTearDownDuration.Duration),
		}
		criteria, err := configTearDownDuration.Criteria.ToCrdCriteria()
		if err != nil {
			if !errors.IsErrPullRequestRPCTearDownDurationCriteriaUnknown(err) {
				return reconcile.Result{}, errors.Wrapf(err, "cannot parse tearDownDuration criteria from rpc")
			}
			// if criteria unknown (tearDownDuration not being set), always destroys the environment
			duration.Duration = time.Duration(0)
			criteria = s2hv1.PullRequestTearDownDurationCriteriaBoth
		}

		tearDownDuration = s2hv1.PullRequestTearDownDuration{
			Duration: duration,
			Criteria: criteria,
		}
	}

	if maxRetry >= 0 && noOfRetry != nil && *noOfRetry >= int(maxRetry) {
		isPRTriggerFailed := true
		imageMissingList := prTrigger.Status.ImageMissingList
		prTriggerCreateAt := prTrigger.Status.CreatedAt
		prTriggerFinishedAt := prTrigger.Status.UpdatedAt
		err = c.createPullRequestQueue(req.Namespace, name, prNumber, commitSHA, gitRepo,
			s2hv1.QueueComponents{}, imageMissingList, isPRTriggerFailed, prTriggerCreateAt, prTriggerFinishedAt, tearDownDuration)
		if err != nil {
			return reconcile.Result{}, err
		}

		if err := c.deleteAndSendPullRequestTriggerResult(ctx, prTrigger); err != nil {
			return reconcile.Result{}, err
		}

		return reconcile.Result{}, nil
	}

	prCompSources, err := c.getOverridingComponentSource(ctx, prTrigger)
	if err != nil {
		return reconcile.Result{}, err
	}

	changed := c.fillEmptyData(prTrigger, prCompSources)
	if changed {
		if err := c.client.Update(context.TODO(), prTrigger); err != nil {
			return reconcile.Result{}, err
		}

		return reconcile.Result{}, nil
	}

	prQueueComponents, err := c.getPRQueueComponentsIfImageExisted(ctx, prTrigger, prCompSources)
	if err != nil {
		// cannot get component version from image registry
		initRetry := 0
		if prTrigger.Spec.NoOfRetry == nil {
			prTrigger.Spec.NoOfRetry = &initRetry
		} else {
			*prTrigger.Spec.NoOfRetry++
		}

		nextProcessAt = &metav1.Time{Time: now.Add(pollingTime)}

		noOfRetry := prTrigger.Spec.NoOfRetry
		maxRetry := prConfig.Trigger.MaxRetry
		if maxRetry >= 0 && noOfRetry != nil && *noOfRetry >= int(maxRetry) {
			prTrigger.Spec.NextProcessAt = &now
		} else {
			prTrigger.Spec.NextProcessAt = nextProcessAt
		}

		prTrigger.Status.UpdatedAt = &now

		prTrigger.Status.SetCondition(s2hv1.PullRequestTriggerCondFailed, corev1.ConditionTrue, err.Error())
		prTrigger.Status.SetResult(s2hv1.PullRequestTriggerFailure)

		if err := c.client.Update(context.TODO(), prTrigger); err != nil {
			if k8serrors.IsConflict(err) {
				logger.Debug("this PullRequestTrigger has been updated/deleted",
					"team", c.teamName, "bundle", prTrigger.Spec.BundleName,
					"prNumber", prTrigger.Spec.PRNumber)
				return reconcile.Result{}, nil
			}
			return reconcile.Result{}, err
		}

		return reconcile.Result{}, nil
	}

	// successfully get component version from image registry
	prTrigger.Status.SetResult(s2hv1.PullRequestTriggerSuccess)
	isPRTriggerFailed := false
	imageMissingList := prTrigger.Status.ImageMissingList
	prTriggerCreateAt := prTrigger.Status.CreatedAt
	prTriggerFinishedAt := prTrigger.Status.UpdatedAt
	err = c.createPullRequestQueue(req.Namespace, name, prNumber, commitSHA, gitRepo,
		prQueueComponents, imageMissingList, isPRTriggerFailed, prTriggerCreateAt, prTriggerFinishedAt, tearDownDuration)
	if err != nil {
		return reconcile.Result{}, err
	}

	if err := c.deleteAndSendPullRequestTriggerResult(ctx, prTrigger); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (c *controller) fillEmptyData(prTrigger *s2hv1.PullRequestTrigger,
	prCompSources []*samsahairpc.ComponentSource) (changed bool) {
	now := metav1.Now()

	if prTrigger.Status.CreatedAt == nil {
		prTrigger.Status.CreatedAt = &now
		changed = true
	}
	if prTrigger.Status.UpdatedAt == nil {
		prTrigger.Status.UpdatedAt = &now
		changed = true
	}

	// remove/update components which invalid
	newPRTriggerComponents := make([]*s2hv1.PullRequestTriggerComponent, 0)
	for _, prTriggerComp := range prTrigger.Spec.Components {
		if targetCompSource := c.getSameComponentInCompSources(prCompSources, prTriggerComp); targetCompSource != nil {
			// override empty field of pull request trigger component
			if prTriggerComp.Image == nil {
				prTriggerComp.Image = &s2hv1.Image{}
			}
			if prTriggerComp.Image.Repository == "" {
				prTriggerComp.Image.Repository = targetCompSource.Image.Repository
				changed = true
			}
			if prTriggerComp.Image.Tag == "" {
				prTriggerComp.Image.Tag = targetCompSource.Image.Tag
				changed = true
			}
			if prTriggerComp.Pattern == "" {
				prTriggerComp.Pattern = targetCompSource.Pattern
				changed = true
			}
			if prTriggerComp.Source == "" {
				prTriggerComp.Source = s2hv1.UpdatingSource(targetCompSource.Source)
				changed = true
			}

			// if image tag is still empty, override with pattern
			if prTriggerComp.Image.Tag == "" {
				prTriggerComp.Image.Tag = prTriggerComp.Pattern
			}
			newPRTriggerComponents = append(newPRTriggerComponents, prTriggerComp)
		}
	}
	prTrigger.Spec.Components = newPRTriggerComponents

	// add components which not exist in pull request trigger components
	for _, prCompSource := range prCompSources {
		if !c.containComponentInPRTriggerComps(prTrigger.Spec.Components, prCompSource) {
			newPRTriggerComponents = append(newPRTriggerComponents, &s2hv1.PullRequestTriggerComponent{
				ComponentName: prCompSource.ComponentName,
				Image: &s2hv1.Image{
					Repository: prCompSource.Image.Repository,
					Tag:        prCompSource.Image.Tag,
				},
				Pattern: prCompSource.Pattern,
				Source:  s2hv1.UpdatingSource(prCompSource.Source),
			})
		}
	}
	prTrigger.Spec.Components = newPRTriggerComponents

	return
}

func (c *controller) getSameComponentInCompSources(prCompSources []*samsahairpc.ComponentSource,
	prTriggerComp *s2hv1.PullRequestTriggerComponent) *samsahairpc.ComponentSource {

	for _, prCompSource := range prCompSources {
		if prCompSource.ComponentName == prTriggerComp.ComponentName {
			return prCompSource
		}
	}

	return nil
}

func (c *controller) containComponentInPRTriggerComps(prTriggerComps []*s2hv1.PullRequestTriggerComponent,
	prCompSource *samsahairpc.ComponentSource) bool {

	for _, prTriggerComp := range prTriggerComps {
		if prTriggerComp.ComponentName == prCompSource.ComponentName {
			return true
		}
	}

	return false
}

func (c *controller) getOverridingComponentSource(ctx context.Context, prTrigger *s2hv1.PullRequestTrigger) (
	[]*samsahairpc.ComponentSource, error) {

	bundleName := prTrigger.Spec.BundleName
	prCompSourceList, err := c.s2hClient.GetPullRequestComponentSources(ctx, &samsahairpc.TeamWithPullRequest{
		TeamName:   c.teamName,
		BundleName: bundleName,
		PRNumber:   prTrigger.Spec.PRNumber,
	})
	if err != nil {
		return []*samsahairpc.ComponentSource{}, err
	}

	for i, prCompSource := range prCompSourceList.ComponentSources {
		for _, prBundleComponent := range prTrigger.Spec.Components {
			if prBundleComponent.ComponentName == prCompSource.ComponentName {
				overridingPRImage := prBundleComponent.Image
				if overridingPRImage != nil {
					if overridingPRImage.Repository != "" {
						prCompSourceList.ComponentSources[i].Image.Repository = overridingPRImage.Repository
					}

					if overridingPRImage.Tag != "" {
						prCompSourceList.ComponentSources[i].Image.Tag = overridingPRImage.Tag
					}
				}

				overridingPattern := prBundleComponent.Pattern
				if overridingPattern != "" {
					prCompSourceList.ComponentSources[i].Pattern = overridingPattern
				}

				overridingSource := prBundleComponent.Source
				if overridingSource != "" {
					prCompSourceList.ComponentSources[i].Source = string(overridingSource)
				}
			}
		}
	}

	return prCompSourceList.ComponentSources, nil
}

func (c *controller) getPRQueueComponentsIfImageExisted(ctx context.Context, prTrigger *s2hv1.PullRequestTrigger,
	prCompSources []*samsahairpc.ComponentSource) (s2hv1.QueueComponents, error) {

	timeout := 300 * time.Second
	ctx, cancelFunc := context.WithTimeout(ctx, timeout)
	defer cancelFunc()

	compSourceCh, compVersionCh := make(chan *samsahairpc.ComponentSource), make(chan *samsahairpc.ComponentVersion)
	errCh := make(chan error)
	for _, prCompSource := range prCompSources {
		go func(prCompSource *samsahairpc.ComponentSource) {
			version, err := c.s2hClient.GetComponentVersion(ctx, prCompSource)
			if err != nil {
				errCh <- err
				compSourceCh <- prCompSource
				return
			}
			compVersionCh <- version
			compSourceCh <- prCompSource
		}(prCompSource)
	}

	var prQueueComponents s2hv1.QueueComponents
	var globalErr error
	for i := 0; i < len(prCompSources); i++ {
		select {
		case <-ctx.Done():
			return s2hv1.QueueComponents{}, errors.Wrapf(s2herrors.ErrRequestTimeout,
				"detect missing images for pull request trigger took longer than %v",
				timeout)
		case compVersion := <-compVersionCh:
			compSource := <-compSourceCh
			prQueueComp := &s2hv1.QueueComponent{
				Name:       compSource.ComponentName,
				Repository: compSource.Image.Repository,
				Version:    compVersion.Version,
			}
			prQueueComponents = append(prQueueComponents, prQueueComp)
		case err := <-errCh:
			compSource := <-compSourceCh
			prTrigger.Status.ImageMissingList = make([]s2hv1.Image, 0)
			if err != nil {
				globalErr = err
				img := s2hv1.Image{
					Repository: compSource.Image.Repository,
					Tag:        compSource.Image.Tag,
				}
				prTrigger.Status.ImageMissingList = append(prTrigger.Status.ImageMissingList, img)
			}
		}
	}

	return prQueueComponents, globalErr
}

func (c *controller) createPullRequestQueue(namespace, name, prNumber, commitSHA, gitRepo string, comps s2hv1.QueueComponents,
	imageMissingList []s2hv1.Image, isPRTriggerFailed bool, createAt, finishedAt *metav1.Time, teardownDuration s2hv1.PullRequestTearDownDuration) error {
	prQueue := prqueuectrl.NewPullRequestQueue(c.teamName, namespace, name, prNumber, commitSHA, gitRepo, comps,
		imageMissingList, isPRTriggerFailed, createAt, finishedAt, teardownDuration)
	if err := c.prQueueCtrl.Add(prQueue, nil); err != nil {
		return err
	}

	return nil
}

func (c *controller) deleteAndSendPullRequestTriggerResult(ctx context.Context,
	prTrigger *s2hv1.PullRequestTrigger) error {
	missingImgListRPC := make([]*samsahairpc.Image, 0)
	for _, img := range prTrigger.Status.ImageMissingList {
		missingImgListRPC = append(missingImgListRPC, &samsahairpc.Image{Repository: img.Repository, Tag: img.Tag})
	}

	prTriggerRPC := &samsahairpc.PullRequestTrigger{
		Name:             prTrigger.Name,
		Namespace:        prTrigger.Namespace,
		TeamName:         c.teamName,
		Result:           string(prTrigger.Status.Result),
		ImageMissingList: missingImgListRPC,
	}
	if _, err := c.s2hClient.RunPostPullRequestTrigger(ctx, prTriggerRPC); err != nil {
		return errors.Wrapf(err,
			"cannot send pull request trigger result report, team: %s, component: %s, prNumber: %s",
			c.teamName, prTrigger.Spec.BundleName, prTrigger.Spec.PRNumber)
	}

	if err := c.client.Delete(context.TODO(), prTrigger); err != nil && !k8serrors.IsNotFound(err) {
		return err
	}

	return nil
}
