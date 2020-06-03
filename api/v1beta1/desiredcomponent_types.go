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

// DesiredComponentSpec defines the desired state of DesiredComponent
type DesiredComponentSpec struct {
	Name       string `json:"name"`
	Version    string `json:"version"`
	Repository string `json:"repository"`
}

// DesiredComponentStatus defines the observed state of DesiredComponent
type DesiredComponentStatus struct {
	CreatedAt *metav1.Time `json:"createdAt,omitempty"`
	UpdatedAt *metav1.Time `json:"updatedAt,omitempty"`
}

// +kubebuilder:object:root=true

// DesiredComponent is the Schema for the desiredcomponents API
type DesiredComponent struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DesiredComponentSpec   `json:"spec,omitempty"`
	Status DesiredComponentStatus `json:"status,omitempty"`
}

// TODO: pohfy, unused?
func (c *DesiredComponent) IsSame(d *DesiredComponent) bool {
	return c.Spec.Name == d.Spec.Name &&
		c.Spec.Repository == d.Spec.Repository &&
		c.Spec.Version == d.Spec.Version
}

// +kubebuilder:object:root=true

// DesiredComponentList contains a list of DesiredComponent
type DesiredComponentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DesiredComponent `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DesiredComponent{}, &DesiredComponentList{})
}
