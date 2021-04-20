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
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PullRequestTriggerComponent represents a pull request component in bundle
type PullRequestTriggerComponent struct {
	// ComponentName defines a name of bundle component
	ComponentName string `json:"componentName"`
	// Image defines an image repository and tag
	Image *Image `json:"image"`
	// Pattern defines a pattern of bundle component which is a regex of tag
	// +optional
	Pattern string `json:"pattern,omitempty"`
	// +optional
	Source UpdatingSource `json:"source,omitempty"`
}

// PullRequestTriggerSpec defines the desired state of PullRequestTrigger
type PullRequestTriggerSpec struct {
	BundleName string `json:"bundleName"`
	PRNumber   string `json:"prNumber"`
	// +optional
	CommitSHA string `json:"commitSHA,omitempty"`
	// +optional
	Components []*PullRequestTriggerComponent `json:"components,omitempty"`
	// +optional
	NextProcessAt *metav1.Time `json:"nextProcessAt,omitempty"`
	// +optional
	NoOfRetry *int `json:"noOfRetry,omitempty"`
	// GitRepository represents a github repository of the pull request
	GitRepository string `json:"gitRepository,omitempty"`
}

// PullRequestTriggerResult represents the result status of a pull request trigger
type PullRequestTriggerResult string

const (
	PullRequestTriggerSuccess PullRequestTriggerResult = "Success"
	PullRequestTriggerFailure PullRequestTriggerResult = "Failure"
)

// PullRequestTriggerStatus defines the observed state of PullRequestTrigger
type PullRequestTriggerStatus struct {
	// CreatedAt represents time when pull request has been triggered firstly
	CreatedAt *metav1.Time `json:"createdAt,omitempty"`

	// UpdatedAt represents time when pull request has been re-triggered
	UpdatedAt *metav1.Time `json:"updatedAt,omitempty"`

	// Result represents a result of the pull request trigger
	// +optional
	Result PullRequestTriggerResult `json:"result,omitempty"`

	// ImageMissingList defines image missing lists
	// +optional
	ImageMissingList []Image `json:"imageMissingList,omitempty"`

	// Conditions contains observations of the resource's state e.g.,
	// Queue deployed, being tested
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []PullRequestTriggerCondition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

func (pr *PullRequestTriggerStatus) SetResult(res PullRequestTriggerResult) {
	pr.Result = res
}

type PullRequestTriggerCondition struct {
	Type   PullRequestTriggerConditionType `json:"type"`
	Status v1.ConditionStatus              `json:"status"`
	// +optional
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
	// +optional
	Reason string `json:"reason,omitempty"`
	// +optional
	Message string `json:"message,omitempty"`
}

type PullRequestTriggerConditionType string

const (
	// PullRequestTriggerCondFailed means the pull request trigger failed to retrieve the image from the registry
	PullRequestTriggerCondFailed PullRequestTriggerConditionType = "Failed"
)

func (pr *PullRequestTriggerStatus) SetCondition(cond PullRequestTriggerConditionType, status v1.ConditionStatus, message string) {
	for i, c := range pr.Conditions {
		if c.Type == cond {
			pr.Conditions[i].Status = status
			pr.Conditions[i].LastTransitionTime = metav1.Now()
			pr.Conditions[i].Message = message
			return
		}
	}

	pr.Conditions = append(pr.Conditions, PullRequestTriggerCondition{
		Type:               cond,
		Status:             status,
		LastTransitionTime: metav1.Now(),
		Message:            message,
	})
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
