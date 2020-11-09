package queue

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	s2hv1 "github.com/agoda-com/samsahai/api/v1"
	"github.com/agoda-com/samsahai/internal"
	s2herrors "github.com/agoda-com/samsahai/internal/errors"
	s2hlog "github.com/agoda-com/samsahai/internal/log"
	samsahairpc "github.com/agoda-com/samsahai/pkg/samsahai/rpc"
)

var logger = s2hlog.Log.WithName(CtrlName)

const CtrlName = "queue-ctrl"

type controller struct {
	client    client.Client
	namespace string
}

var _ internal.QueueController = &controller{}

func NewQueue(teamName, namespace, name, bundle string, comps []*s2hv1.QueueComponent, queueType s2hv1.QueueType) *s2hv1.Queue {
	queueName := name
	if bundle != "" {
		queueName = bundle
	}

	qLabels := getQueueLabels(teamName, queueName)
	return &s2hv1.Queue{
		ObjectMeta: metav1.ObjectMeta{
			Name:      queueName,
			Namespace: namespace,
			Labels:    qLabels,
		},
		Spec: s2hv1.QueueSpec{
			Name:       queueName,
			TeamName:   teamName,
			Bundle:     bundle,
			Components: comps,
			Type:       queueType,
		},
		Status: s2hv1.QueueStatus{},
	}
}

// New returns QueueController
func New(ns string, runtimeClient client.Client) internal.QueueController {
	c := &controller{
		namespace: ns,
		client:    runtimeClient,
	}
	return c
}

func (c *controller) Add(obj runtime.Object, priorityQueues []string) error {
	q, ok := obj.(*s2hv1.Queue)
	if !ok {
		return s2herrors.ErrParsingRuntimeObject
	}

	return c.add(context.TODO(), q, false, priorityQueues)
}

func (c *controller) AddTop(obj runtime.Object) error {
	q, ok := obj.(*s2hv1.Queue)
	if !ok {
		return s2herrors.ErrParsingRuntimeObject
	}

	return c.add(context.TODO(), q, true, nil)
}

func (c *controller) Size(namespace string) int {
	listOpts := &client.ListOptions{Namespace: namespace}
	list, err := c.list(listOpts)
	if err != nil {
		logger.Error(err, "cannot list queue")
		return 0
	}
	return len(list.Items)
}

func (c *controller) First(namespace string) (runtime.Object, error) {
	listOpts := &client.ListOptions{Namespace: namespace}
	list, err := c.list(listOpts)
	if err != nil {
		logger.Error(err, "cannot list queue")
		return nil, err
	}

	q := list.First()
	c.resetQueueOrderWithCurrentQueue(list, q)
	if err := c.updateQueueList(list); err != nil {
		return nil, err
	}

	if q == nil {
		return nil, nil
	}

	if q.Spec.NextProcessAt == nil || time.Now().After(q.Spec.NextProcessAt.Time) {
		return q, nil
	}

	return nil, nil
}

func (c *controller) Remove(obj client.Object) error {
	return c.client.Delete(context.TODO(), obj)
}

func (c *controller) RemoveAllQueues(namespace string) error {
	return c.client.DeleteAllOf(context.TODO(), &s2hv1.Queue{}, client.InNamespace(namespace))
}

// incoming s`queue` always contains 1 component
// will add component into queue ordering by priority
func (c *controller) add(ctx context.Context, queue *s2hv1.Queue, atTop bool, priorityQueues []string) error {
	if len(queue.Spec.Components) == 0 {
		return fmt.Errorf("components should not be empty, queueName: %s", queue.Name)
	}

	listOpts := &client.ListOptions{Namespace: queue.Namespace}
	queueList, err := c.list(listOpts)
	if err != nil {
		logger.Error(err, "cannot list queue")
		return err
	}

	if isMatch, err := c.isMatchWithStableComponent(ctx, queue); err != nil {
		return err
	} else if isMatch {
		return nil
	}

	pQueue := &s2hv1.Queue{}
	isAlreadyInQueue := false
	isAlreadyInBundle := false
	for i, q := range queueList.Items {
		if q.ContainSameComponent(queue.Spec.Name, queue.Spec.Components[0]) {
			isAlreadyInQueue = true
			pQueue = &queueList.Items[i]
			break
		}
	}

	// remove/update duplicate component
	removingList, updatingList := c.removeAndUpdateSimilarQueue(queue, queueList)
	for i := range removingList {
		if err := c.client.Delete(ctx, &removingList[i]); err != nil {
			return err
		}
	}
	for i := range updatingList {
		if updatingList[i].Name != "" {
			updatingList[i].Spec.Components.Sort()
			updatingList[i].Spec.NoOfRetry = 0
			updatingList[i].Spec.NextProcessAt = nil
			updatingList[i].Status.State = s2hv1.Waiting
			updatingList[i].Spec.Components.Sort()
			if err := c.client.Update(ctx, &updatingList[i]); err != nil {
				return err
			}
			queueList.Items[i] = updatingList[i]
		}
	}

	updatingList = c.addExistingBundleQueue(queue, queueList)
	for i := range updatingList {
		if updatingList[i].Name != "" {
			isAlreadyInBundle = true
			updatingList[i].Spec.NoOfRetry = 0
			updatingList[i].Spec.NextProcessAt = nil
			updatingList[i].Status.State = s2hv1.Waiting
			updatingList[i].Spec.Components.Sort()
			if err := c.client.Update(ctx, &updatingList[i]); err != nil {
				return err
			}
			queueList.Items[i] = updatingList[i]
		}
	}

	queueList.Sort()

	now := metav1.Now()
	if isAlreadyInQueue {
		isQueueOnTop := queueList.Items[0].ContainSameComponent(pQueue.Spec.Name, pQueue.Spec.Components[0])

		// move queue to the top
		if atTop && !isQueueOnTop {
			pQueue.Spec.NoOfOrder = queueList.TopQueueOrder()
		}

		pQueue.Spec.Components.Sort()
		if err = c.client.Update(ctx, pQueue); err != nil {
			return err
		}
	} else {
		updatingList = c.setQueueOrderFollowingPriorityQueues(queue, queueList, priorityQueues)
		for i := range updatingList {
			if err := c.client.Update(ctx, &updatingList[i]); err != nil {
				return err
			}
		}

		if !isAlreadyInBundle {
			// queue not exist
			if atTop {
				queue.Spec.NoOfOrder = queueList.TopQueueOrder()
			}

			queue.Status.State = s2hv1.Waiting
			queue.Status.CreatedAt = &now
			queue.Status.UpdatedAt = &now

			queue.Spec.Components.Sort()
			if err = c.client.Create(ctx, queue); err != nil {
				return err
			}
		}
	}

	return nil
}

// queue always contains 1 component
func (c *controller) isMatchWithStableComponent(ctx context.Context, q *s2hv1.Queue) (isMatch bool, err error) {
	if len(q.Spec.Components) == 0 {
		isMatch = true
		return
	}

	qComp := q.Spec.Components[0]
	stableComp := &s2hv1.StableComponent{}
	err = c.client.Get(ctx, types.NamespacedName{Namespace: c.namespace, Name: qComp.Name}, stableComp)
	if err != nil && k8serrors.IsNotFound(err) {
		return false, nil
	} else if err != nil {
		return
	}

	isMatch = stableComp.Spec.Repository == qComp.Repository &&
		stableComp.Spec.Version == qComp.Version

	return
}

// expect sorted queue list
func (c *controller) setQueueOrderFollowingPriorityQueues(queue *s2hv1.Queue, list *s2hv1.QueueList, priorityQueues []string) []s2hv1.Queue {
	targetNo := c.getPriorityNo(queue, priorityQueues)
	if targetNo == -1 || len(list.Items) == 0 {
		queue.Spec.NoOfOrder = list.LastQueueOrder()
		return []s2hv1.Queue{}
	}

	existingQueueNo := c.getExistingQueueNumberInList(queue, list)

	var items []s2hv1.Queue
	var updating []s2hv1.Queue
	var found, foundTop bool
	expectedNo := list.LastQueueOrder()
	for i, q := range list.Items {
		priorityNo := c.getPriorityNo(&list.Items[i], priorityQueues)
		isLowerPriority := priorityNo == -1 || priorityNo > targetNo
		if !found && isLowerPriority {
			if i-1 < 0 {
				expectedNo = list.TopQueueOrder()
				foundTop = true
			} else {
				expectedNo = list.Items[i-1].Spec.NoOfOrder + 1
			}

			found = true

		}

		if found && existingQueueNo != i {
			if !foundTop {
				q.Spec.NoOfOrder = q.Spec.NoOfOrder + 1
			}
			updating = append(updating, q)
		}

		items = append(items, q)
	}

	if existingQueueNo != -1 && items != nil {
		items[existingQueueNo].Spec.NoOfOrder = expectedNo
		updating = append(updating, items[existingQueueNo])
	} else {
		queue.Spec.NoOfOrder = expectedNo
	}

	list.Items = items
	return updating
}

func (c *controller) getPriorityNo(queue *s2hv1.Queue, priorityQueues []string) int {
	for i, priorComp := range priorityQueues {
		if queue.Spec.Name == priorComp {
			return i
		}

		for _, comp := range queue.Spec.Components {
			if comp.Name == priorComp {
				return i
			}
		}
	}

	return -1
}

func (c *controller) getExistingQueueNumberInList(queue *s2hv1.Queue, list *s2hv1.QueueList) int {
	for i, q := range list.Items {
		for _, qComp := range q.Spec.Components {
			// queue already existed
			if qComp.Name == queue.Spec.Components[0].Name {
				return i
			}
		}
	}

	return -1
}

// addExistingBundleQueue returns updated bundle queue list
func (c *controller) addExistingBundleQueue(queue *s2hv1.Queue, list *s2hv1.QueueList) []s2hv1.Queue {
	if queue.Spec.Bundle == "" {
		return []s2hv1.Queue{}
	}

	updating := make([]s2hv1.Queue, len(list.Items))
	var items []s2hv1.Queue
	var containComp = false
	for i, q := range list.Items {
		if !containComp && q.ContainSameComponent(queue.Spec.Name, queue.Spec.Components[0]) {
			// only add one `queue` to items
			containComp = true
			// update component of bundle
		} else if q.Spec.Bundle == queue.Spec.Bundle {
			var found bool
			for j, qComp := range q.Spec.Components {
				if qComp.Name == queue.Spec.Components[0].Name {
					q.Spec.Components[j].Repository = queue.Spec.Components[0].Repository
					q.Spec.Components[j].Version = queue.Spec.Components[0].Version
					found = true
					break
				}
			}

			// add a new component to bundle
			if !found {
				q.Spec.Components = append(q.Spec.Components, queue.Spec.Components[0])
			}

			updating[i] = q
		}

		items = append(items, q)
	}

	list.Items = items
	return updating
}

// removeAndUpdateSimilarQueue removes similar component/queue (same `component name` from queue) from QueueList
func (c *controller) removeAndUpdateSimilarQueue(queue *s2hv1.Queue, list *s2hv1.QueueList) (
	removing []s2hv1.Queue, updating []s2hv1.Queue) {

	updating = make([]s2hv1.Queue, len(list.Items))
	var items []s2hv1.Queue
	var containComp = false

	for i, q := range list.Items {
		if !containComp && q.ContainSameComponent(queue.Spec.Name, queue.Spec.Components[0]) {
			// only add one `queue` to items
			containComp = true
		} else {
			newComps := make([]*s2hv1.QueueComponent, 0)
			for _, qComp := range q.Spec.Components {
				if qComp.Name == queue.Spec.Components[0].Name {
					continue
				}
				newComps = append(newComps, qComp)
			}

			if len(newComps) == 0 {
				removing = append(removing, q)
				continue
			}

			if len(newComps) != len(q.Spec.Components) {
				q.Spec.Components = newComps
				updating[i] = q
			}
		}

		items = append(items, q)
	}

	list.Items = items
	return
}

func (c *controller) list(opts *client.ListOptions) (list *s2hv1.QueueList, err error) {
	list = &s2hv1.QueueList{}
	if opts == nil {
		opts = &client.ListOptions{Namespace: c.namespace}
	}
	if err = c.client.List(context.Background(), list, opts); err != nil {
		return list, errors.Wrapf(err, "cannot list queue with options: %+v", opts)
	}
	return list, nil
}

func (c *controller) SetLastOrder(obj runtime.Object) error {
	q, ok := obj.(*s2hv1.Queue)
	if !ok {
		return s2herrors.ErrParsingRuntimeObject
	}

	queueList, err := c.list(nil)
	if err != nil {
		logger.Error(err, "cannot list queue")
		return err
	}

	q.Spec.NoOfOrder = queueList.LastQueueOrder()
	q.Status.Conditions = nil

	return c.client.Update(context.TODO(), q)
}

func (c *controller) SetReverifyQueueAtFirst(obj runtime.Object) error {
	q, ok := obj.(*s2hv1.Queue)
	if !ok {
		return s2herrors.ErrParsingRuntimeObject
	}

	listOpts := &client.ListOptions{Namespace: q.Namespace}
	list, err := c.list(listOpts)
	if err != nil {
		logger.Error(err, "cannot list queue")
		return err
	}

	now := metav1.Now()
	q.Status = s2hv1.QueueStatus{
		CreatedAt:     &now,
		NoOfProcessed: q.Status.NoOfProcessed,
		State:         s2hv1.Waiting,
	}
	q.Spec.Type = s2hv1.QueueTypeReverify
	q.Spec.NoOfOrder = list.TopQueueOrder()
	return c.client.Update(context.TODO(), q)
}

func (c *controller) SetRetryQueue(obj runtime.Object, noOfRetry int, nextAt time.Time) error {
	q, ok := obj.(*s2hv1.Queue)
	if !ok {
		return s2herrors.ErrParsingRuntimeObject
	}

	listOpts := &client.ListOptions{Namespace: q.Namespace}
	list, err := c.list(listOpts)
	if err != nil {
		logger.Error(err, "cannot list queue")
		return err
	}

	now := metav1.Now()
	q.Status = s2hv1.QueueStatus{
		CreatedAt:     &now,
		NoOfProcessed: q.Status.NoOfProcessed,
		State:         s2hv1.Waiting,
	}
	q.Spec.NextProcessAt = &metav1.Time{Time: nextAt}
	q.Spec.NoOfRetry = noOfRetry
	q.Spec.Type = s2hv1.QueueTypeUpgrade
	q.Spec.NoOfOrder = list.LastQueueOrder()
	return c.client.Update(context.TODO(), q)
}

func (c *controller) updateQueueList(ql *s2hv1.QueueList) error {
	for i := range ql.Items {
		if err := c.client.Update(context.TODO(), &ql.Items[i]); err != nil {
			logger.Error(err, "cannot update queue list", "queue", ql.Items[i].Name)
			return errors.Wrapf(err, "cannot update queue %s in %s", ql.Items[i].Name, ql.Items[i].Namespace)
		}
	}

	return nil
}

// resetQueueOrderWithCurrentQueue resets order of all queues to start with 1 respectively
func (c *controller) resetQueueOrderWithCurrentQueue(ql *s2hv1.QueueList, currentQueue *s2hv1.Queue) {
	ql.Sort()
	count := 2
	for i := range ql.Items {
		if ql.Items[i].Name == currentQueue.Name {
			ql.Items[i].Spec.NoOfOrder = 1
			continue
		}
		ql.Items[i].Spec.NoOfOrder = count
		count++
	}
}

// EnsurePreActiveComponents ensures that components were deployed with `pre-active` config and tested
func EnsurePreActiveComponents(c client.Client, teamName, namespace string, skipTest bool) (q *s2hv1.Queue, err error) {
	q = &s2hv1.Queue{
		ObjectMeta: metav1.ObjectMeta{
			Name:      string(s2hv1.EnvPreActive),
			Namespace: namespace,
		},
		Spec: s2hv1.QueueSpec{
			Type:           s2hv1.QueueTypePreActive,
			TeamName:       teamName,
			SkipTestRunner: skipTest,
		},
	}

	err = ensureQueue(context.TODO(), c, q)
	return
}

// EnsurePromoteToActiveComponents ensures that components were deployed with `active` config
func EnsurePromoteToActiveComponents(c client.Client, teamName, namespace string) (q *s2hv1.Queue, err error) {
	q = &s2hv1.Queue{
		ObjectMeta: metav1.ObjectMeta{
			Name:      string(s2hv1.EnvActive),
			Namespace: namespace,
		},
		Spec: s2hv1.QueueSpec{
			Type:     s2hv1.QueueTypePromoteToActive,
			TeamName: teamName,
		},
	}
	err = ensureQueue(context.TODO(), c, q)
	return
}

// EnsureDemoteFromActiveComponents ensures that components were deployed without `active` config
func EnsureDemoteFromActiveComponents(c client.Client, teamName, namespace string) (q *s2hv1.Queue, err error) {
	q = &s2hv1.Queue{
		ObjectMeta: metav1.ObjectMeta{
			Name:      string(s2hv1.EnvDeActive),
			Namespace: namespace,
		},
		Spec: s2hv1.QueueSpec{
			Type:     s2hv1.QueueTypeDemoteFromActive,
			TeamName: teamName,
		},
	}
	err = ensureQueue(context.TODO(), c, q)
	return
}

// EnsurePullRequestComponents ensures that pull request components were deployed with `pull-request` config and tested
func EnsurePullRequestComponents(c client.Client, teamName, namespace, queueName, prCompName, prNumber string,
	comps s2hv1.QueueComponents, noOfRetry int) (q *s2hv1.Queue, err error) {

	q = &s2hv1.Queue{
		ObjectMeta: metav1.ObjectMeta{
			Name:      queueName,
			Namespace: namespace,
		},
		Spec: s2hv1.QueueSpec{
			Name:       prCompName,
			Type:       s2hv1.QueueTypePullRequest,
			TeamName:   teamName,
			PRNumber:   prNumber,
			Components: comps,
			NoOfRetry:  noOfRetry,
		},
	}

	err = ensureQueue(context.TODO(), c, q)
	return
}

func DeletePreActiveQueue(c client.Client, ns string) error {
	return deleteQueue(c, ns, string(s2hv1.EnvPreActive))
}

func DeletePromoteToActiveQueue(c client.Client, ns string) error {
	return deleteQueue(c, ns, string(s2hv1.EnvActive))
}

func DeleteDemoteFromActiveQueue(c client.Client, ns string) error {
	return deleteQueue(c, ns, string(s2hv1.EnvDeActive))
}

func DeletePullRequestQueue(c client.Client, ns, queueName string) error {
	return deleteQueue(c, ns, queueName)
}

// deleteQueue removes Queue in target namespace by name
func deleteQueue(c client.Client, ns, name string) error {
	q := &s2hv1.Queue{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
	}
	err := c.Delete(context.TODO(), q)
	if err != nil && k8serrors.IsNotFound(err) {
		return nil
	}

	return errors.Wrap(err, "cannot delete queue")
}

func ensureQueue(ctx context.Context, c client.Client, q *s2hv1.Queue) (err error) {
	fetched := &s2hv1.Queue{}
	err = c.Get(ctx, types.NamespacedName{Namespace: q.Namespace, Name: q.Name}, fetched)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			// Create
			return c.Create(ctx, q)
		}
		return err
	}
	q.Spec = fetched.Spec
	q.Status = fetched.Status
	return nil
}

func getQueueLabels(teamName, component string) map[string]string {
	qLabels := internal.GetDefaultLabels(teamName)
	qLabels["app"] = component
	qLabels["component"] = component

	return qLabels
}

func GetComponentUpgradeRPCFromQueue(
	comStatus samsahairpc.ComponentUpgrade_UpgradeStatus,
	queueHistName string,
	queueHistNamespace string,
	queue *s2hv1.Queue,
	prQueueRPC *samsahairpc.TeamWithPullRequest,
) *samsahairpc.ComponentUpgrade {

	outImgList := make([]*samsahairpc.Image, 0)
	for _, img := range queue.Status.ImageMissingList {
		outImgList = append(outImgList, &samsahairpc.Image{Repository: img.Repository, Tag: img.Tag})
	}

	rpcComps := make([]*samsahairpc.Component, 0)
	for _, qComp := range queue.Spec.Components {
		rpcComps = append(rpcComps, &samsahairpc.Component{
			Name: qComp.Name,
			Image: &samsahairpc.Image{
				Repository: qComp.Repository,
				Tag:        qComp.Version,
			},
		})
	}

	isReverify := queue.IsReverify()
	if prQueueRPC != nil && prQueueRPC.PRNumber != "" {
		isReverify = int(prQueueRPC.MaxRetryQueue) >= queue.Spec.NoOfRetry
	}

	prNamespace := ""
	if prQueueRPC != nil {
		prNamespace = prQueueRPC.Namespace
	}

	comp := &samsahairpc.ComponentUpgrade{
		Status:               comStatus,
		Name:                 queue.Spec.Name,
		TeamName:             queue.Spec.TeamName,
		Components:           rpcComps,
		IssueType:            getIssueTypeRPC(outImgList, queue),
		QueueHistoryName:     queueHistName,
		Namespace:            queueHistNamespace,
		ImageMissingList:     outImgList,
		Runs:                 int32(queue.Spec.NoOfRetry + 1),
		IsReverify:           isReverify,
		ReverificationStatus: getReverificationStatusRPC(queue),
		DeploymentIssues:     getDeploymentIssuesRPC(queue),
		PullRequestComponent: prQueueRPC,
		PullRequestNamespace: prNamespace,
	}

	return comp
}

func getIssueTypeRPC(imageMissingList []*samsahairpc.Image, queue *s2hv1.Queue) samsahairpc.ComponentUpgrade_IssueType {
	switch {
	case len(imageMissingList) > 0:
		return samsahairpc.ComponentUpgrade_IssueType_IMAGE_MISSING
	case queue.IsReverify() && queue.IsDeploySuccess() && queue.IsTestSuccess():
		return samsahairpc.ComponentUpgrade_IssueType_DESIRED_VERSION_FAILED
	case queue.IsReverify() && (!queue.IsDeploySuccess() || !queue.IsTestSuccess()):
		return samsahairpc.ComponentUpgrade_IssueType_ENVIRONMENT_ISSUE
	default:
		return samsahairpc.ComponentUpgrade_IssueType_DESIRED_VERSION_FAILED
	}
}

func getReverificationStatusRPC(queue *s2hv1.Queue) samsahairpc.ComponentUpgrade_ReverificationStatus {
	if !queue.IsReverify() {
		return samsahairpc.ComponentUpgrade_ReverificationStatus_UNKNOWN
	}

	if queue.IsDeploySuccess() && queue.IsTestSuccess() {
		return samsahairpc.ComponentUpgrade_ReverificationStatus_SUCCESS
	}

	return samsahairpc.ComponentUpgrade_ReverificationStatus_FAILURE
}

func getDeploymentIssuesRPC(queue *s2hv1.Queue) []*samsahairpc.DeploymentIssue {
	deploymentIssues := make([]*samsahairpc.DeploymentIssue, 0)
	for _, deploymentIssue := range queue.Status.DeploymentIssues {
		failureComps := make([]*samsahairpc.FailureComponent, 0)
		for _, failureComp := range deploymentIssue.FailureComponents {
			failureComps = append(failureComps, &samsahairpc.FailureComponent{
				ComponentName:             failureComp.ComponentName,
				FirstFailureContainerName: failureComp.FirstFailureContainerName,
				RestartCount:              failureComp.RestartCount,
				NodeName:                  failureComp.NodeName,
			})
		}

		deploymentIssues = append(deploymentIssues, &samsahairpc.DeploymentIssue{
			IssueType:         string(deploymentIssue.IssueType),
			FailureComponents: failureComps,
		})
	}

	return deploymentIssues
}
