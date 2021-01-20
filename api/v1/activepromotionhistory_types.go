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

// ActivePromotionHistorySpec defines the desired state of ActivePromotionHistory
type ActivePromotionHistorySpec struct {
	TeamName        string           `json:"teamName,omitempty"`
	ActivePromotion *ActivePromotion `json:"activePromotion,omitempty"`
	IsSuccess       bool             `json:"isSuccess,omitempty"`

	// TODO: store values file of all components
	CreatedAt *metav1.Time `json:"createdAt,omitempty"`
}

// ActivePromotionHistoryStatus defines the observed state of ActivePromotionHistory
type ActivePromotionHistoryStatus struct {
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster

// ActivePromotionHistory is the Schema for the activepromotionhistories API
type ActivePromotionHistory struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ActivePromotionHistorySpec   `json:"spec,omitempty"`
	Status ActivePromotionHistoryStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ActivePromotionHistoryList contains a list of ActivePromotionHistory
type ActivePromotionHistoryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ActivePromotionHistory `json:"items"`
}

// sort ActivePromotion by timestamp DESC
func (al *ActivePromotionHistoryList) SortDESC() {
	sort.Sort(ActivePromotionHistoryByCreatedTimeDESC(al.Items))
}

type ActivePromotionHistoryByCreatedTimeDESC []ActivePromotionHistory

func (a ActivePromotionHistoryByCreatedTimeDESC) Len() int { return len(a) }
func (a ActivePromotionHistoryByCreatedTimeDESC) Less(i, j int) bool {
	return a[i].CreationTimestamp.Time.After(a[j].CreationTimestamp.Time)
}

func (a ActivePromotionHistoryByCreatedTimeDESC) Swap(i, j int) { a[i], a[j] = a[j], a[i] }

func init() {
	SchemeBuilder.Register(&ActivePromotionHistory{}, &ActivePromotionHistoryList{})
}
