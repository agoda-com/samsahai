/*
Copyright 2019 Agoda DevOps Container.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	"sort"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// QueueType defines how to process this item
type QueueType string

// QueueState defines state of the queue
type QueueState string

const (
	// QueueType
	//
	// QueueTypeUpgrade
	QueueTypeUpgrade QueueType = "upgrade"

	// QueueTypeReverify we will deploy last stable to check is there any environment issue
	QueueTypeReverify QueueType = "reverify"

	// QueueTypePreActive
	QueueTypePreActive QueueType = "pre-active"

	// QueueTypePromoteToActive
	QueueTypePromoteToActive QueueType = "promote-to-active"

	// QueueTypeDemoteFromActive components will deploy with latest stable + `tmp` env config
	QueueTypeDemoteFromActive QueueType = "demote-from-active"

	// QueueTypePullRequest
	QueueTypePullRequest QueueType = "pull-request"

	// QueueState
	//
	// Waiting waiting in queues
	Waiting QueueState = "waiting"

	// CleaningBefore cleans before running
	CleaningBefore QueueState = "cleaning_before"

	// DetectingImageMissing detects image missing before running
	DetectingImageMissing QueueState = "detecting_image_missing"

	// Creating the environment is creating for test this queue
	Creating QueueState = "creating"

	// Testing the test is running for testing this queue
	Testing QueueState = "testing"

	// CleaningBefore cleans after running
	CleaningAfter QueueState = "cleaning_after"

	// Collecting collecting the result from testing
	Collecting QueueState = "collecting"

	// Deleting queue is being removed
	Deleting QueueState = "deleting"

	// Cancelling queue is being canceled (deleted by user)
	Cancelling QueueState = "cancelling"

	// Finished queue is in finished state, waiting for next process (for preActive, promoteToActive)
	Finished QueueState = "finished"
)

// QueueSpec defines the desired state of Queue
type QueueSpec struct {
	// Name represents a Component name or bundle name if exist
	Name string `json:"name"`

	// Bundle represents a bundle name of component
	// +optional
	Bundle string `json:"bundle,omitempty"`

	// Components represents a list of components which are deployed
	// +optional
	Components QueueComponents `json:"components,omitempty"`

	// Type represents how we will process this queue
	Type QueueType `json:"type"`

	// NoOfRetry defines how many times this component has been tested
	// +optional
	NoOfRetry int `json:"noOfRetry"`

	// NoOfOrder defines the position in queue
	// lower is will be picked first
	NoOfOrder int `json:"noOfOrder"`

	// NextProcessAt represents time to wait for process this queue
	NextProcessAt *metav1.Time `json:"nextProcessAt,omitempty"`

	// TeamName represents team owner of the queue
	TeamName string `json:"teamName"`

	// PRNumber represents a pull request number
	// +optional
	PRNumber string `json:"prNumber,omitempty"`

	// SkipTestRunner represents a flag for skipping running test
	// +optional
	SkipTestRunner bool `json:"skipTestRunner,omitempty"`

	// QueueExtraParameters override default behavior of how to process this queue according to QueueType
	// +optional
	*QueueExtraParameters `json:"queueExtraParameters,omitempty"`
}

// QueueExtraParameters override default behavior of how to process this queue according to QueueType
type QueueExtraParameters struct {
	// TestRunner represents configuration about how to test the environment
	// +optional
	TestRunner *ConfigTestRunnerOverrider `json:"testRunner"`
}

func (q *Queue) GetTestRunnerExtraParameter() *ConfigTestRunnerOverrider {
	if q.Spec.QueueExtraParameters == nil {
		return nil
	}
	return q.Spec.QueueExtraParameters.TestRunner
}

type Image struct {
	Repository string `json:"repository"`
	Tag        string `json:"tag"`
}

type QueueComponents []*QueueComponent

type QueueComponent struct {
	// Name represents Component name
	Name string `json:"name"`

	// Repository represents Docker image repository
	Repository string `json:"repository"`

	// Version represents Docker image tag version
	Version string `json:"version"`
}

type QueueCondition struct {
	Type   QueueConditionType     `json:"type"`
	Status corev1.ConditionStatus `json:"status"`
	// +optional
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
	// +optional
	Reason string `json:"reason,omitempty"`
	// +optional
	Message string `json:"message,omitempty"`
}

type TestRunner struct {
	Teamcity Teamcity `json:"teamcity,omitempty"`
	Gitlab   Gitlab   `json:"gitlab,omitempty"`
}

type Teamcity struct {
	Branch      string `json:"branch,omitempty"`
	BuildID     string `json:"buildID,omitempty"`
	BuildNumber string `json:"buildNumber,omitempty"`
	BuildTypeID string `json:"buildTypeID,omitempty"`
	BuildURL    string `json:"buildURL,omitempty"`
}

func (t *Teamcity) SetTeamcity(branch, buildID, buildTypeID, buildURL string) {
	t.Branch = branch
	t.BuildID = buildID
	t.BuildTypeID = buildTypeID
	t.BuildURL = buildURL
}

type Gitlab struct {
	Branch         string `json:"branch,omitempty"`
	PipelineID     string `json:"pipelineID,omitempty"`
	PipelineURL    string `json:"pipelineURL,omitempty"`
	PipelineNumber string `json:"pipelineNumber,omitempty"`
}

func (t *Gitlab) SetGitlab(branch, pipelineID, pipelineURL, pipelineNumber string) {
	t.Branch = branch
	t.PipelineID = pipelineID
	t.PipelineURL = pipelineURL
	t.PipelineNumber = pipelineNumber
}

type FailureComponent struct {
	// ComponentName defines a name of component
	ComponentName string `json:"componentName"`
	// FirstFailureContainerName defines a first found failure container name
	FirstFailureContainerName string `json:"firstFailureContainerName"`
	// RestartCount defines the number of times the container has been restarted
	RestartCount int32 `json:"restartCount"`
	// NodeName defines the node name of pod
	NodeName string `json:"nodeName"`
}

type DeploymentIssue struct {
	// IssueType defines a deployment issue type
	IssueType DeploymentIssueType `json:"issueType"`
	// FailureComponents defines a list of failure components
	FailureComponents []FailureComponent `json:"failureComponents"`
}

// DeploymentIssueType defines a deployment issue type
type DeploymentIssueType string

const (
	// DeploymentIssueImagePullBackOff means the pod can not be started due to image not found
	DeploymentIssueImagePullBackOff DeploymentIssueType = "ImagePullBackOff"
	// DeploymentIssueCrashLoopBackOff means the pod failed to start container
	DeploymentIssueCrashLoopBackOff DeploymentIssueType = "CrashLoopBackOff"
	// DeploymentIssueReadinessProbeFailed means the pod cannot be run due to readiness probe failed (zero restart count)
	DeploymentIssueReadinessProbeFailed DeploymentIssueType = "ReadinessProbeFailed"
	// DeploymentIssueContainerCreating means the pod is being creating
	DeploymentIssueContainerCreating DeploymentIssueType = "ContainerCreating"
	// DeploymentIssuePending means the pod is waiting for assigning to node
	DeploymentIssuePending DeploymentIssueType = "Pending"
	// DeploymentIssueWaitForInitContainer means the container can not be start due to wait for finishing init container
	DeploymentIssueWaitForInitContainer DeploymentIssueType = "WaitForInitContainer"
	// DeploymentIssueJobNotComplete means the job is not completed
	DeploymentIssueJobNotComplete DeploymentIssueType = "JobNotComplete"
	// DeploymentIssueUndefined represents other issues
	DeploymentIssueUndefined DeploymentIssueType = "Undefined"
)

type QueueConditionType string

const (
	// QueueDeployStarted means the queue has been started
	QueueDeployStarted QueueConditionType = "QueueDeployStarted"
	// QueueDeployed means the queue has been finished deploying
	QueueDeployed QueueConditionType = "QueueDeployed"
	// QueueTestTriggered means the queue has been triggered testing
	QueueTestTriggered QueueConditionType = "QueueTestTriggered"
	// QueueTested means the queue has been finished testing
	QueueTested QueueConditionType = "QueueTested"
	// QueueTeamcityTestResult means the test result of Teamcity
	QueueTeamcityTestResult QueueConditionType = "QueueTeamcityTestResult"
	// QueueGitlabTestResult means the test result of Gitlab
	QueueGitlabTestResult QueueConditionType = "QueueGitlabTestResult"
	// QueueCleaningBeforeStarted means cleaning namespace before running task has been started
	QueueCleaningBeforeStarted QueueConditionType = "QueueCleaningBeforeStarted"
	// QueueCleanedBefore means the namespace has been cleaned before running task
	QueueCleanedBefore QueueConditionType = "QueueCleanedBefore"
	// QueueCleaningAfterStarted means cleaning namespace after running task has been started
	QueueCleaningAfterStarted QueueConditionType = "QueueCleaningAfterStarted"
	// QueueCleanedAfter means the namespace has been cleaned after running task
	QueueCleanedAfter QueueConditionType = "QueueCleanedAfter"

	// QueueCollected means the queue has been successfully collected
	// the deploying and testing result
	QueueCollected QueueConditionType = "QueueCollected"
	// QueueFinished means the queue has been finished process
	QueueFinished QueueConditionType = "QueueFinished"
)

// QueueStatus defines the observed state of Queue
type QueueStatus struct {
	// CreatedAt represents time when the component has been added to queue
	CreatedAt *metav1.Time `json:"createdAt,omitempty"`

	// UpdatedAt represents time when the component was processed
	UpdatedAt *metav1.Time `json:"updatedAt,omitempty"`

	// NextProcessAt represents time to wait for process this queue
	NextProcessAt *metav1.Time `json:"nextProcessAt,omitempty"`

	// StartDeployTime represents the time when this queue start deploying
	StartDeployTime *metav1.Time `json:"startDeployTime,omitempty"`

	// StartTestingTime represents the time when this queue start testing
	StartTestingTime *metav1.Time `json:"startTestingTime,omitempty"`

	// State represents current status of this queue
	State QueueState `json:"state"`

	// NoOfProcessed represents how many time that this queue had been processed
	NoOfProcessed int `json:"noOfProcessed,omitempty"`

	// Conditions contains observations of the resource's state e.g.,
	// Queue deployed, being tested
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []QueueCondition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// TestRunner defines the test runner
	TestRunner TestRunner `json:"testRunners,omitempty"`

	// QueueHistoryName defines name of history of this queue
	QueueHistoryName string `json:"queueHistoryName"`

	// KubeZipLog defines log of k8s resources during deployment in base64 zip format
	KubeZipLog string `json:"kubeZipLog"`

	// DeploymentIssues defines a list of deployment issue types
	// +optional
	DeploymentIssues []DeploymentIssue `json:"deploymentIssues,omitempty"`

	// ImageMissingList defines image missing lists
	ImageMissingList []Image `json:"imageMissingList,omitempty"`

	// DeployEngine represents engine using during installation
	DeployEngine string `json:"deployEngine,omitempty"`
}

func (qs *QueueStatus) SetDeploymentIssues(deploymentIssues []DeploymentIssue) {
	qs.DeploymentIssues = deploymentIssues
}

func (qs *QueueStatus) SetImageMissingList(images []Image) {
	qs.ImageMissingList = images
}

func (qs *QueueStatus) IsConditionTrue(cond QueueConditionType) bool {
	for i, c := range qs.Conditions {
		if c.Type == cond {
			return qs.Conditions[i].Status == corev1.ConditionTrue
		}
	}
	return false
}

func (qs *QueueStatus) IsContains(cond QueueConditionType) bool {
	for _, c := range qs.Conditions {
		if c.Type == cond {
			return true
		}
	}
	return false
}

func (qs *QueueStatus) SetCondition(cond QueueConditionType, status corev1.ConditionStatus, message string) {
	for i, c := range qs.Conditions {
		if c.Type == cond {
			qs.Conditions[i].Status = status
			qs.Conditions[i].LastTransitionTime = metav1.Now()
			qs.Conditions[i].Message = message
			return
		}
	}

	qs.Conditions = append(qs.Conditions, QueueCondition{
		Type:               cond,
		Status:             status,
		LastTransitionTime: metav1.Now(),
		Message:            message,
	})
}

func (qs *QueueStatus) GetConditionLatestTime(cond QueueConditionType) *metav1.Time {
	for _, c := range qs.Conditions {
		if c.Type == cond {
			return &c.LastTransitionTime
		}
	}

	return nil
}

// +kubebuilder:object:root=true

// Queue is the Schema for the queues API
type Queue struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   QueueSpec   `json:"spec,omitempty"`
	Status QueueStatus `json:"status,omitempty"`
}

func (q *Queue) ContainSameComponent(dName string, dComp *QueueComponent) bool {
	if dName != q.Spec.Name {
		return false
	}

	for _, qComp := range q.Spec.Components {
		if qComp.Name == dComp.Name &&
			qComp.Repository == dComp.Repository &&
			qComp.Version == dComp.Version {
			return true
		}
	}

	return false
}

func (q *Queue) SetState(state QueueState) {
	now := metav1.Now()
	q.Status.UpdatedAt = &now
	q.Status.State = state
}

func (q *Queue) IsDeploySuccess() bool {
	return q.Status.IsConditionTrue(QueueDeployed)
}

func (q *Queue) IsTestSuccess() bool {
	return q.Status.IsConditionTrue(QueueTested)
}

func (q *Queue) IsTeamcityTestSuccess() bool {
	return q.Status.IsConditionTrue(QueueTeamcityTestResult)
}

func (q *Queue) IsGitlabTestSuccess() bool {
	return q.Status.IsConditionTrue(QueueGitlabTestResult)
}

func (q *Queue) IsReverify() bool {
	return q.Spec.Type == QueueTypeReverify
}

func (q *Queue) IsActivePromotionQueue() bool {
	return q.Spec.Type == QueueTypePreActive ||
		q.Spec.Type == QueueTypePromoteToActive ||
		q.Spec.Type == QueueTypeDemoteFromActive
}

func (q *Queue) IsComponentUpgradeQueue() bool {
	return q.Spec.Type == QueueTypeUpgrade
}

func (q *Queue) IsPullRequestQueue() bool {
	return q.Spec.Type == QueueTypePullRequest
}

// GetEnvType returns environment type for connection based on Queue.Spec.Type
func (q *Queue) GetEnvType() string {
	switch q.Spec.Type {
	case QueueTypePreActive:
		return "pre-active"
	case QueueTypePromoteToActive:
		return "active"
	case QueueTypePullRequest:
		return "pull-request"
	default:
		return "staging"
	}
}

// GetQueueType returns queue type based on Queue.Spec.Type
func (q *Queue) GetQueueType() string {
	switch q.Spec.Type {
	case QueueTypeUpgrade:
		return "component-upgrade"
	case QueueTypeReverify:
		return "reverification"
	case QueueTypePullRequest:
		return "pull-request"
	default:
		return "active-promotion"
	}
}

// +kubebuilder:object:root=true

// QueueList contains a list of Queue
type QueueList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Queue `json:"items"`
}

// TopQueueOrder returns no of order to be first on the queue
func (ql *QueueList) TopQueueOrder() int {
	if len(ql.Items) == 0 {
		return 1
	}
	sort.Sort(QueueByNoOfOrder(ql.Items))
	return ql.Items[0].Spec.NoOfOrder - 1
}

// LastQueueOrder returns no of order to be last on the queue
func (ql *QueueList) LastQueueOrder() int {
	if len(ql.Items) == 0 {
		return 1
	}
	sort.Sort(QueueByNoOfOrder(ql.Items))
	return ql.Items[len(ql.Items)-1].Spec.NoOfOrder + 1
}

// First returns the first order of queues
func (ql *QueueList) First() *Queue {
	if len(ql.Items) == 0 {
		return nil
	}

	ql.Sort()

	// return non-waiting Queue, if any
	for i, q := range ql.Items {
		if q.Status.State != Waiting {
			return &ql.Items[i]
		}
	}

	// return the first Queue
	now := metav1.Now()
	for i, q := range ql.Items {
		if q.Spec.NextProcessAt != nil && q.Spec.NextProcessAt.Before(&now) {
			return &ql.Items[i]
		}
		if q.Spec.NextProcessAt == nil {
			return &ql.Items[i]
		}
	}
	return &ql.Items[0]
}

// Sort sorts queue items
func (ql *QueueList) Sort() {
	sort.Sort(QueueByNoOfOrder(ql.Items))
}

type QueueByNoOfOrder []Queue

func (q QueueByNoOfOrder) Len() int { return len(q) }
func (q QueueByNoOfOrder) Less(i, j int) bool {
	now := metav1.Now()

	if q[i].Spec.NoOfOrder == q[j].Spec.NoOfOrder {
		if q[i].Spec.NextProcessAt == nil {
			return true
		} else if q[j].Spec.NextProcessAt == nil {
			return false
		}
		return q[i].Spec.NextProcessAt.Time.Before(q[j].Spec.NextProcessAt.Time)
	}

	// if next process at is after now, means that the reverify process has been finished
	// moves to the last of queue
	if q[i].Spec.NextProcessAt != nil && q[i].Spec.NextProcessAt.After(now.Time) &&
		q[j].Spec.NextProcessAt != nil && q[j].Spec.NextProcessAt.After(now.Time) {
		return q[i].Spec.NextProcessAt.Time.Before(q[j].Spec.NextProcessAt.Time)
	}

	if q[i].Spec.NextProcessAt != nil && q[i].Spec.NextProcessAt.After(now.Time) {
		return false
	}

	if q[j].Spec.NextProcessAt != nil && q[j].Spec.NextProcessAt.After(now.Time) {
		return true
	}

	// sort by order
	return q[i].Spec.NoOfOrder < q[j].Spec.NoOfOrder
}

func (q QueueByNoOfOrder) Swap(i, j int) { q[i], q[j] = q[j], q[i] }

// Sort sorts component items
func (qc QueueComponents) Sort() {
	sort.Sort(ComponentByName(qc))
}

type ComponentByName []*QueueComponent

func (q ComponentByName) Len() int { return len(q) }
func (q ComponentByName) Less(i, j int) bool {
	return q[i].Name < q[j].Name
}

func (q ComponentByName) Swap(i, j int) { q[i], q[j] = q[j], q[i] }

func init() {
	SchemeBuilder.Register(&Queue{}, &QueueList{})
}
