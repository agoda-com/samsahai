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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PullRequestQueueHistorySpec defines the desired state of PullRequestQueueHistory
type PullRequestQueueHistorySpec struct {
	PullRequestQueue *PullRequestQueue `json:"pullRequestQueue,omitempty"`
}

// PullRequestQueueHistoryStatus defines the observed state of PullRequestQueueHistory
type PullRequestQueueHistoryStatus struct {
}

// +kubebuilder:object:root=true

// PullRequestQueueHistory is the Schema for the PullRequestQueueHistories API
type PullRequestQueueHistory struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PullRequestQueueHistorySpec   `json:"spec,omitempty"`
	Status PullRequestQueueHistoryStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// PullRequestQueueHistoryList contains a list of PullRequestQueueHistory
type PullRequestQueueHistoryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PullRequestQueueHistory `json:"items"`
}

// sort PullRequestQueueHistory by timestamp DESC
func (prql *PullRequestQueueHistoryList) SortDESC() {
	sort.Sort(PullRequestQueueHistoryByCreatedTimeDESC(prql.Items))
}

type PullRequestQueueHistoryByCreatedTimeDESC []PullRequestQueueHistory

func (prq PullRequestQueueHistoryByCreatedTimeDESC) Len() int { return len(prq) }
func (prq PullRequestQueueHistoryByCreatedTimeDESC) Less(i, j int) bool {
	return prq[i].CreationTimestamp.Time.After(prq[j].CreationTimestamp.Time)
}

func (prq PullRequestQueueHistoryByCreatedTimeDESC) Swap(i, j int) { prq[i], prq[j] = prq[j], prq[i] }

func init() {
	SchemeBuilder.Register(&PullRequestQueueHistory{}, &PullRequestQueueHistoryList{})
}
