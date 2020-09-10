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

// PullRequestTriggerSpec defines the desired state of PullRequestTrigger
type PullRequestTriggerSpec struct {
	Name     string `json:"name"`
	PRNumber int    `json:"prNumber"`
	Image    Image  `json:"image"`
}

// PullRequestTriggerStatus defines the observed state of PullRequestTrigger
type PullRequestTriggerStatus struct {
	// CreatedAt represents time when pull request has been triggered firstly
	CreatedAt *metav1.Time `json:"createdAt,omitempty"`

	// UpdatedAt represents time when pull request has been re-triggered
	UpdatedAt *metav1.Time `json:"updatedAt,omitempty"`

	// NextProcessAt represents time to re-check the image in the target registry
	NextProcessAt *metav1.Time `json:"nextProcessAt,omitempty"`
}

// +kubebuilder:object:root=true

// PullRequestTrigger is the Schema for the pullrequesttriggers API
type PullRequestTrigger struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PullRequestTriggerSpec   `json:"spec,omitempty"`
	Status PullRequestTriggerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// PullRequestTriggerList contains a list of PullRequestTrigger
type PullRequestTriggerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PullRequestTrigger `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PullRequestTrigger{}, &PullRequestTriggerList{})
}
