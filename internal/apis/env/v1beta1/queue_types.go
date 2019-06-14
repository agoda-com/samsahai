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
)

// QueueType defines how to process this item
type QueueType string

const (
	UPGRADE QueueType = "upgrade"

	// REVERIFY we will deploy last stable to check is there any environment issue
	REVERIFY QueueType = "reverify"
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
	NoOfRetry int `json:"noOfRetry"`

	// NoOfOrder defines the position in queue
	// lower is will be picked first
	NoOfOrder int `json:"noOfOrder"`
}

// QueueStatus defines the observed state of Queue
type QueueStatus struct {
	// CreatedAt represents time when the component has been added to queue
	CreatedAt *metav1.Time `json:"createdAt,omitempty"`

	// UpdatedAt represents time when the component was processed
	UpdatedAt *metav1.Time `json:"updatedAt,omitempty"`

	// NextProcessAt represents time to wait for process this queue
	NextProcessAt *metav1.Time `json:"nextProcessAt,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Queue is the Schema for the queues API
// +k8s:openapi-gen=true
type Queue struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   QueueSpec   `json:"spec,omitempty"`
	Status QueueStatus `json:"status,omitempty"`
}

func (s *Queue) IsSame(d *Queue) bool {
	return s.Spec.Name == d.Spec.Name &&
		s.Spec.Repository == d.Spec.Repository &&
		s.Spec.Version == d.Spec.Version
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// QueueList contains a list of Queue
type QueueList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Queue `json:"items"`
}

// TopQueueOrder returns no of order to be first on the queue
func (l *QueueList) TopQueueOrder() int {
	if len(l.Items) == 0 {
		return 1
	}
	sort.Sort(ByNoOfOrder(l.Items))
	return l.Items[0].Spec.NoOfOrder - 1
}

// LastQueueOrder returns no of order to be last on the queue
func (l *QueueList) LastQueueOrder() int {
	if len(l.Items) == 0 {
		return 1
	}
	sort.Sort(ByNoOfOrder(l.Items))
	return l.Items[len(l.Items)-1].Spec.NoOfOrder + 1
}

// Sort sorts items
func (l *QueueList) First() *Queue {
	if len(l.Items) == 0 {
		return nil
	}
	l.Sort()
	return &l.Items[0]
}

// Sort sorts items
func (l *QueueList) Sort() {
	sort.Sort(ByNoOfOrder(l.Items))
}

type ByNoOfOrder []Queue

func (q ByNoOfOrder) Len() int           { return len(q) }
func (q ByNoOfOrder) Less(i, j int) bool { return q[i].Spec.NoOfOrder < q[j].Spec.NoOfOrder }
func (q ByNoOfOrder) Swap(i, j int)      { q[i], q[j] = q[j], q[i] }

func init() {
	SchemeBuilder.Register(&Queue{}, &QueueList{})
}
