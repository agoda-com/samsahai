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

// StableComponentSpec defines the desired state of StableComponent
type StableComponentSpec struct {
	// Name represents Component name
	Name string `json:"name"`

	// Repository represents Docker image repository
	Repository string `json:"repository"`

	// Version represents Docker image tag version
	Version string `json:"version"`
}

// StableComponentStatus defines the observed state of StableComponent
type StableComponentStatus struct {
	CreatedAt *metav1.Time `json:"createdAt,omitempty"`
	UpdatedAt *metav1.Time `json:"updatedAt,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// StableComponent is the Schema for the stablecomponents API
// +k8s:openapi-gen=true
type StableComponent struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   StableComponentSpec   `json:"spec,omitempty"`
	Status StableComponentStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// StableComponentList contains a list of StableComponent
type StableComponentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []StableComponent `json:"items"`
}

func init() {
	SchemeBuilder.Register(&StableComponent{}, &StableComponentList{})
}
