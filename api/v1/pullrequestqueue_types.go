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

// PullRequestQueueSpec defines the desired state of PullRequestQueue
type PullRequestQueueSpec struct {
	// BundleName represents a pull request bundle name
	BundleName string `json:"bundleName"`

	// PRNumber represents a pull request number
	PRNumber string `json:"prNumber"`

	// CommitSHA represents a commit SHA
	// +optional
	CommitSHA string `json:"commitSHA,omitempty"`

	// Components represents a list of components which are deployed
	// +optional
	Components QueueComponents `json:"components,omitempty"`

	// UpcomingCommitSHA represents an upcoming commit SHA in case queue is running
	// +optional
	UpcomingCommitSHA string `json:"upcomingCommitSHA,omitempty"`

	// UpcomingComponents represents an upcoming components which are deployed in case queue is running
	// +optional
	UpcomingComponents QueueComponents `json:"upcomingComponents,omitempty"`

	// NoOfRetry defines how many times this pull request component has been tested
	// +optional
	NoOfRetry int `json:"noOfRetry"`

	// NoOfOrder defines the position in queue
	// lower is will be picked first
	NoOfOrder int `json:"noOfOrder"`

	// TeamName represents team owner of the pull request queue
	TeamName string `json:"teamName"`

	// GitRepository represents a github repository of the pull request
	GitRepository string `json:"gitRepository,omitempty"`

	// ImageMissingList represents image missing lists
	// +optional
	ImageMissingList []Image `json:"imageMissingList,omitempty"`

	// IsPRTriggerFailed represents the result of pull request trigger
	// +optional
	IsPRTriggerFailed *bool `json:"isPrTriggerFailed,omitempty"`

	// PRTriggerCreatedAt represents time when pull request trigger has been start
	// +optional
	PRTriggerCreatedAt *metav1.Time `json:"prTriggerCreatedAt,omitempty"`

	// PRTriggerFinishedAt represents time when pull request trigger has been finish
	// +optional
	PRTriggerFinishedAt *metav1.Time `json:"prTriggerFinishedAt,omitempty"`
}

// PullRequestQueueConditionType represents a condition type of pull request queue
type PullRequestQueueConditionType string

const (
	// PullRequestQueueCondStarted means the pull request queue has been started
	PullRequestQueueCondStarted PullRequestQueueConditionType = "PullRequestQueueStarted"
	// PullRequestQueueCondEnvCreated means the pull request queue environment has been created
	PullRequestQueueCondEnvCreated PullRequestQueueConditionType = "PullRequestQueueEnvCreated"
	// PullRequestQueueCondDependenciesUpdated means the pull request component dependencies have been updated
	PullRequestQueueCondDependenciesUpdated PullRequestQueueConditionType = "PullRequestQueueDependenciesUpdated"
	// PullRequestQueueCondDeployed means the pull request components have been deployed into pull request namespace
	PullRequestQueueCondDeployed PullRequestQueueConditionType = "PullRequestQueueComponentsDeployed"
	// PullRequestQueueCondTested means the pull request components have been tested
	PullRequestQueueCondTested PullRequestQueueConditionType = "PullRequestQueueComponentsTested"
	// PullRequestQueueCondResultCollected means the result of pull request queue has been collected
	PullRequestQueueCondResultCollected PullRequestQueueConditionType = "PullRequestQueueResultCollected"
	// PullRequestQueueCondEnvCreated means the pull request queue environment has been destroyed
	PullRequestQueueCondEnvDestroyed PullRequestQueueConditionType = "PullRequestQueueEnvDestroyed"
	// PullRequestQueueCondPromptedDeleted means the pull request queue was prompted deleted
	PullRequestQueueCondPromptedDeleted PullRequestQueueConditionType = "PullRequestQueueCondPromptedDeleted"
)

// PullRequestQueueState defines state of the queue
type PullRequestQueueState string

const (
	// Waiting waiting in queues
	PullRequestQueueWaiting PullRequestQueueState = "waiting"

	// Creating the environment is creating for deploying components
	PullRequestQueueEnvCreating PullRequestQueueState = "creating"

	// Deploying the components are being deployed into pull request namespace
	PullRequestQueueDeploying PullRequestQueueState = "deploying"

	// Testing the components are being tested
	PullRequestQueueTesting PullRequestQueueState = "testing"

	// Collecting collecting the result from testing
	PullRequestQueueCollecting PullRequestQueueState = "collecting"

	// Destroying destroying the pull request namespace
	PullRequestQueueEnvDestroying PullRequestQueueState = "destroying"

	// Finished queue is in finished state, waiting for next process
	PullRequestQueueFinished PullRequestQueueState = "finished"
)

type PullRequestQueueCondition struct {
	Type   PullRequestQueueConditionType `json:"type"`
	Status corev1.ConditionStatus        `json:"status"`
	// +optional
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
	// +optional
	Reason string `json:"reason,omitempty"`
	// +optional
	Message string `json:"message,omitempty"`
}

// PullRequestQueueResult represents the result status of a pull request queue
type PullRequestQueueResult string

const (
	PullRequestQueueCanceled PullRequestQueueResult = "Canceled"
	PullRequestQueueSuccess  PullRequestQueueResult = "Success"
	PullRequestQueueFailure  PullRequestQueueResult = "Failure"
)

// PullRequestQueueStatus defines the observed state of PullRequestQueue
type PullRequestQueueStatus struct {
	// CreatedAt represents time when the component has been added to queue
	CreatedAt *metav1.Time `json:"createdAt,omitempty"`

	// UpdatedAt represents time when the component was processed
	UpdatedAt *metav1.Time `json:"updatedAt,omitempty"`

	// State represents current status of this queue
	State PullRequestQueueState `json:"state"`

	// PullRequestNamespace represents a current pull request namespace
	PullRequestNamespace string `json:"pullRequestNamespace"`

	// PullRequestQueueHistoryName represents created PullRequestQueueHistory name
	// +optional
	PullRequestQueueHistoryName string `json:"pullRequestQueueHistoryName,omitempty"`

	// Result represents a result of the pull request queue
	// +optional
	Result PullRequestQueueResult `json:"result,omitempty"`

	// Conditions contains observations of the resource's state e.g.,
	// Queue deployed, being tested
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []PullRequestQueueCondition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// ComponentUpgrade defines a deployed pull request queue
	// +optional
	DeploymentQueue *Queue `json:"deploymentQueue,omitempty"`
}

func (prqs *PullRequestQueueStatus) SetPullRequestNamespace(namespace string) {
	prqs.PullRequestNamespace = namespace
}

func (prqs *PullRequestQueueStatus) SetResult(res PullRequestQueueResult) {
	prqs.Result = res
}

func (prqs *PullRequestQueueStatus) SetPullRequestQueueHistoryName(prQueueHistName string) {
	prqs.PullRequestQueueHistoryName = prQueueHistName
}

func (prqs *PullRequestQueueStatus) SetDeploymentQueue(q *Queue) {
	prqs.DeploymentQueue = &Queue{
		Spec:   q.Spec,
		Status: q.Status,
	}
}

func (prqs *PullRequestQueueStatus) IsConditionTrue(cond PullRequestQueueConditionType) bool {
	for i, c := range prqs.Conditions {
		if c.Type == cond {
			return prqs.Conditions[i].Status == corev1.ConditionTrue
		}
	}
	return false
}

func (prqs *PullRequestQueueStatus) SetCondition(cond PullRequestQueueConditionType, status corev1.ConditionStatus, message string) {
	for i, c := range prqs.Conditions {
		if c.Type == cond {
			prqs.Conditions[i].Status = status
			prqs.Conditions[i].LastTransitionTime = metav1.Now()
			prqs.Conditions[i].Message = message
			return
		}
	}

	prqs.Conditions = append(prqs.Conditions, PullRequestQueueCondition{
		Type:               cond,
		Status:             status,
		LastTransitionTime: metav1.Now(),
		Message:            message,
	})
}

// +kubebuilder:object:root=true

// PullRequestQueue is the Schema for the queues API
type PullRequestQueue struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PullRequestQueueSpec   `json:"spec,omitempty"`
	Status PullRequestQueueStatus `json:"status,omitempty"`
}

func (prq *PullRequestQueue) SetState(state PullRequestQueueState) {
	now := metav1.Now()
	prq.Status.UpdatedAt = &now
	prq.Status.State = state

	if prq.Status.CreatedAt == nil {
		prq.Status.CreatedAt = &now
	}
}

// +kubebuilder:object:root=true

// PullRequestQueueList contains a list of PullRequestQueue
type PullRequestQueueList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PullRequestQueue `json:"items"`
}

// LastQueueOrder returns no of order to be last on the pull request queue
func (prql *PullRequestQueueList) LastQueueOrder() int {
	if len(prql.Items) == 0 {
		return 1
	}
	sort.Sort(PullRequestQueueByNoOfOrder(prql.Items))
	return prql.Items[len(prql.Items)-1].Spec.NoOfOrder + 1
}

func (prq *PullRequestQueue) IsFailure() bool {
	return prq.Status.Result == PullRequestQueueFailure
}

func (prq *PullRequestQueue) IsCanceled() bool {
	return prq.Status.Result == PullRequestQueueCanceled
}

// Sort sorts pull request queue items
func (prql *PullRequestQueueList) Sort() {
	sort.Sort(PullRequestQueueByNoOfOrder(prql.Items))
}

// +k8s:deepcopy-gen=false

type PullRequestQueueByNoOfOrder []PullRequestQueue

func (prq PullRequestQueueByNoOfOrder) Len() int { return len(prq) }
func (prq PullRequestQueueByNoOfOrder) Less(i, j int) bool {
	if prq[i].Spec.NoOfOrder == prq[j].Spec.NoOfOrder {
		return prq[i].Name < prq[j].Name
	}

	return prq[i].Spec.NoOfOrder < prq[j].Spec.NoOfOrder
}

func (prq PullRequestQueueByNoOfOrder) Swap(i, j int) { prq[i], prq[j] = prq[j], prq[i] }

func init() {
	SchemeBuilder.Register(&PullRequestQueue{}, &PullRequestQueueList{})
}
