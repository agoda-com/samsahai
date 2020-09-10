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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// QueueHistorySpec defines the desired state of QueueHistory
type PullRequestQueueHistorySpec struct {
	PullRequestQueue      *PullRequestQueue `json:"pullRequestQueue,omitempty"`
	QueueHistoryExtraSpec `json:",inline"`
}

// +kubebuilder:object:root=true

// PullRequestQueueHistory is the Schema for the PullRequestQueueHistories API
type PullRequestQueueHistory struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   QueueHistorySpec   `json:"spec,omitempty"`
	Status QueueHistoryStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// PullRequestQueueHistoryList contains a list of PullRequestQueueHistory
type PullRequestQueueHistoryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PullRequestQueueHistory `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PullRequestQueueHistory{}, &PullRequestQueueHistoryList{})
}
