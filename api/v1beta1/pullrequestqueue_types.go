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

// PullRequestQueueSpec defines the desired state of PullRequestQueue
type PullRequestQueueSpec struct {
	// ComponentName represents a pull request Component name
	ComponentName string `json:"componentName"`

	// PullRequestNumber represents a pull request number
	PullRequestNumber string `json:"pullRequestNumber"`

	// Components represents a list of components which are deployed
	// +optional
	Components QueueComponents `json:"components,omitempty"`

	// NoOfRetry defines how many times this pull request component has been tested
	// +optional
	NoOfRetry int `json:"noOfRetry"`

	// NoOfOrder defines the position in queue
	// lower is will be picked first
	NoOfOrder int `json:"noOfOrder"`

	// TeamName represents team owner of the queue
	TeamName string `json:"teamName"`
}

// PullRequestQueueConditionType represents a condition type of pull request queue
type PullRequestQueueConditionType string

const (
	// PullRequestQueueCondStarted means the pull request queue has been started
	PullRequestQueueCondStarted PullRequestQueueConditionType = "PullRequestQueueStarted"
	// PullRequestQueueCondEnvCreated means the pull request environment was created
	PullRequestQueueCondEnvCreated PullRequestQueueConditionType = "PullRequestEnvCreated"
)

// PullRequestQueueState defines state of the queue
type PullRequestQueueState string

const (
	// PullRequestQueueState
	//
	// Waiting waiting in queues
	PullRequestQueueWaiting PullRequestQueueState = "waiting"

	// Running the environment is creating for test this queue
	PullRequestQueueRunning PullRequestQueueState = "running"

	// Collecting collecting the result from testing
	PullRequestQueueCollecting PullRequestQueueState = "collecting"

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

	// Result represents a result of the pull request queue
	// +optional
	Result PullRequestQueueResult `json:"result,omitempty"`

	// Conditions contains observations of the resource's state e.g.,
	// Queue deployed, being tested
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []PullRequestQueueCondition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// PullRequestQueueHistoryName defines name of history of this queue
	PullRequestQueueHistoryName string `json:"pullRequestQueueHistoryName"`

	// QueueHistory defines a deployed pull request queue history
	// +optional
	QueueHistory *QueueHistory `json:"queueHistory,omitempty"`
}

func (pr *PullRequestQueue) SetState(state PullRequestQueueState) {
	now := metav1.Now()
	pr.Status.UpdatedAt = &now
	pr.Status.State = state
}

func (pr *PullRequestQueue) SetPullRequestNamespace(namespace string) {
	pr.Status.PullRequestNamespace = namespace
}

func (pr *PullRequestQueue) SetResult(res PullRequestQueueResult) {
	pr.Status.Result = res
}

// +kubebuilder:object:root=true

// PullRequestQueue is the Schema for the queues API
type PullRequestQueue struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PullRequestQueueSpec   `json:"spec,omitempty"`
	Status PullRequestQueueStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// PullRequestQueueList contains a list of PullRequestQueue
type PullRequestQueueList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PullRequestQueue `json:"items"`
}

// sort PullRequestQueue by timestamp ASC
func (prl *PullRequestQueueList) SortASC() {
	sort.Sort(PullRequestQueueByCreatedTimeASC(prl.Items))
}

// +k8s:deepcopy-gen=false

type PullRequestQueueByCreatedTimeASC []PullRequestQueue

func (a PullRequestQueueByCreatedTimeASC) Len() int { return len(a) }
func (a PullRequestQueueByCreatedTimeASC) Less(i, j int) bool {
	return a[i].CreationTimestamp.Time.Before(a[j].CreationTimestamp.Time)
}

func (a PullRequestQueueByCreatedTimeASC) Swap(i, j int) { a[i], a[j] = a[j], a[i] }

func init() {
	SchemeBuilder.Register(&PullRequestQueue{}, &PullRequestQueueList{})
}
