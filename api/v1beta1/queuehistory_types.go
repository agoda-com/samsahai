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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// QueueHistorySpec defines the desired state of QueueHistory
type QueueHistorySpec struct {
	Queue            *Queue            `json:"queue,omitempty"`
	AppliedValues    Values            `json:"appliedValues,omitempty"`
	StableComponents []StableComponent `json:"stableComponents,omitempty"`
	IsDeploySuccess  bool              `json:"isDeploySuccess"`
	IsTestSuccess    bool              `json:"isTestSuccess"`
	IsReverify       bool              `json:"isReverify,omitempty"`
	CreatedAt        *metav1.Time      `json:"createdAt,omitempty"`
}

// QueueHistoryStatus defines the observed state of QueueHistory
type QueueHistoryStatus struct {
}

type Values map[string]interface{}

func (in *Values) DeepCopyInto(out *Values) {
	if in == nil {
		*out = nil
	} else {
		*out = runtime.DeepCopyJSON(*in)
	}
}

func (in *Values) DeepCopy() *Values {
	if in == nil {
		return nil
	}
	out := new(Values)
	in.DeepCopyInto(out)
	return out
}

// +kubebuilder:object:root=true

// QueueHistory is the Schema for the queuehistories API
type QueueHistory struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   QueueHistorySpec   `json:"spec,omitempty"`
	Status QueueHistoryStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// QueueHistoryList contains a list of QueueHistory
type QueueHistoryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []QueueHistory `json:"items"`
}

// sort QueueHistory by timestamp DESC
func (ql *QueueHistoryList) SortDESC() {
	sort.Sort(QueueHistoryByCreatedTimeDESC(ql.Items))
}

type QueueHistoryByCreatedTimeDESC []QueueHistory

func (a QueueHistoryByCreatedTimeDESC) Len() int { return len(a) }
func (a QueueHistoryByCreatedTimeDESC) Less(i, j int) bool {
	return a[i].CreationTimestamp.Time.After(a[j].CreationTimestamp.Time)
}

func (a QueueHistoryByCreatedTimeDESC) Swap(i, j int) { a[i], a[j] = a[j], a[i] }

func init() {
	SchemeBuilder.Register(&QueueHistory{}, &QueueHistoryList{})
}
