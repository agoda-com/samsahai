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

package v1beta1

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

	// Cancelling queue is being cancelled (deleted by user)
	Cancelling QueueState = "cancelling"

	// Finished queue is in finished state, waiting for next process (for preActive, promoteToActive)
	Finished QueueState = "finished"
)

// QueueSpec defines the desired state of Queue
type QueueSpec struct {
	// Name represents Component name
	Name string `json:"name"`

	// Repository represents Docker image repository
	Repository string `json:"repository"`

	// Version represents Docker image tag version
	Version string `json:"version"`

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
}

type Image struct {
	Repository string `json:"repository"`
	Tag        string `json:"tag"`
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
}

type Teamcity struct {
	BuildID     string `json:"buildID,omitempty"`
	BuildTypeID string `json:"buildTypeID,omitempty"`
	BuildURL    string `json:"buildURL,omitempty"`
}

func (t *Teamcity) SetTeamcity(buildID, buildTypeID, buildURL string) {
	t.BuildID = buildID
	t.BuildTypeID = buildTypeID
	t.BuildURL = buildURL
}

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

	// ReleaseName defines name of helmrelease
	ReleaseName string `json:"releaseName"`

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

	// ImageMissingList defines image missing list
	ImageMissingList []Image `json:"imageMissingList,omitempty"`

	// DeployEngine represents engine using during installation
	DeployEngine string `json:"deployEngine,omitempty"`
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

func (q *Queue) IsSame(d *Queue) bool {
	return q.Spec.Name == d.Spec.Name &&
		q.Spec.Repository == d.Spec.Repository &&
		q.Spec.Version == d.Spec.Version
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

func (q *Queue) IsReverify() bool {
	return q.Spec.Type == QueueTypeReverify
}

func (q *Queue) IsActivePromotionQueue() bool {
	return q.Spec.Type == QueueTypePreActive ||
		q.Spec.Type == QueueTypePromoteToActive ||
		q.Spec.Type == QueueTypeDemoteFromActive
}

// GetEnvType returns environment type for connection based on Queue.Spec.Type
func (q *Queue) GetEnvType() string {
	switch q.Spec.Type {
	case QueueTypePreActive:
		return "pre-active"
	case QueueTypePromoteToActive:
		return "active"
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
	sort.Sort(ByNoOfOrder(ql.Items))
	return ql.Items[0].Spec.NoOfOrder - 1
}

// LastQueueOrder returns no of order to be last on the queue
func (ql *QueueList) LastQueueOrder() int {
	if len(ql.Items) == 0 {
		return 1
	}
	sort.Sort(ByNoOfOrder(ql.Items))
	return ql.Items[len(ql.Items)-1].Spec.NoOfOrder + 1
}

// Sort sorts items
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
	return &ql.Items[0]
}

// Sort sorts items
func (ql *QueueList) Sort() {
	sort.Sort(ByNoOfOrder(ql.Items))
}

type ByNoOfOrder []Queue

func (q ByNoOfOrder) Len() int { return len(q) }
func (q ByNoOfOrder) Less(i, j int) bool {
	if q[i].Spec.NextProcessAt == nil {
		if q[j].Spec.NextProcessAt == nil {
			return q[i].Spec.NoOfOrder < q[j].Spec.NoOfOrder
		}
		// i always less
		return true
	} else if q[j].Spec.NextProcessAt == nil {
		// j always less if i not nil
		return false
	} else if q[i].Spec.NextProcessAt.Time.Equal(q[j].Spec.NextProcessAt.Time) {
		return q[i].Spec.NoOfOrder < q[j].Spec.NoOfOrder
	}
	return q[i].Spec.NextProcessAt.Time.Before(q[j].Spec.NextProcessAt.Time)
}

func (q ByNoOfOrder) Swap(i, j int) { q[i], q[j] = q[j], q[i] }

func init() {
	SchemeBuilder.Register(&Queue{}, &QueueList{})
}
