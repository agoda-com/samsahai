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
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ActivePromotionState holds a possible state of an active promotion
// Only one of its members may be specified
type ActivePromotionState string

const (
	ActivePromotionWaiting                   ActivePromotionState = "Waiting"
	ActivePromotionCreatingPreActive         ActivePromotionState = "CreatingPreActiveEnvironment"
	ActivePromotionDeployingComponents       ActivePromotionState = "DeployingStableComponents"
	ActivePromotionTestingPreActive          ActivePromotionState = "TestingPreActiveEnvironment"
	ActivePromotionCollectingPreActiveResult ActivePromotionState = "CollectingPreActiveResult"
	ActivePromotionDemoting                  ActivePromotionState = "DemotingActiveEnvironment"
	ActivePromotionActiveEnvironment         ActivePromotionState = "PromotingActiveEnvironment"
	ActivePromotionDestroyingPreviousActive  ActivePromotionState = "DestroyingPreviousActiveEnvironment"
	ActivePromotionDestroyingPreActive       ActivePromotionState = "DestroyingPreActiveEnvironment"
	ActivePromotionFinished                  ActivePromotionState = "Finished"
	ActivePromotionRollback                  ActivePromotionState = "Rollback"
)

// ActivePromotionResult represents the result status of an active promotion
type ActivePromotionResult string

const (
	ActivePromotionCanceled ActivePromotionResult = "Canceled"
	ActivePromotionSuccess  ActivePromotionResult = "Success"
	ActivePromotionFailure  ActivePromotionResult = "Failure"
)

// ActivePromotionRollbackStatus represents the rollback status of an active promotion
type ActivePromotionRollbackStatus string

const (
	ActivePromotionRollbackSuccess ActivePromotionRollbackStatus = "Success"
	ActivePromotionRollbackFailure ActivePromotionRollbackStatus = "Failure"
)

// ActivePromotionDemotionStatus represents the active demotion status
type ActivePromotionDemotionStatus string

const (
	ActivePromotionDemotionSuccess ActivePromotionDemotionStatus = "Success"
	ActivePromotionDemotionFailure ActivePromotionDemotionStatus = "Failure"
)

type ActivePromotionCondition struct {
	Type   ActivePromotionConditionType `json:"type"`
	Status v1.ConditionStatus           `json:"status"`
	// +optional
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
	// +optional
	Reason string `json:"reason,omitempty"`
	// +optional
	Message string `json:"message,omitempty"`
}

type ActivePromotionConditionType string

const (
	// ActivePromotionCondStarted means the active promotion process has been started
	ActivePromotionCondStarted ActivePromotionConditionType = "ActivePromotionStarted"
	// ActivePromotionPreActiveCreated means the pre-active environment was created
	ActivePromotionCondPreActiveCreated ActivePromotionConditionType = "PreActiveCreated"
	// ActivePromotionCondVerificationStarted means start verifying pre-active environment
	ActivePromotionCondVerificationStarted ActivePromotionConditionType = "PreActiveVerificationStarted"
	// ActivePromotionCondVerified means the pre-active environment has been verified
	ActivePromotionCondVerified ActivePromotionConditionType = "PreActiveVerified"
	// ActivePromotionCondResultCollected means the result of active promotion has been collected
	ActivePromotionCondResultCollected ActivePromotionConditionType = "ResultCollected"
	// ActivePromotionCondActiveDemotionStarted means start demoting a previous active namespace
	ActivePromotionCondActiveDemotionStarted ActivePromotionConditionType = "ActiveDemotionStarted"
	// ActivePromotionCondActiveDemotionFinished means a previous active environment has been demoted
	ActivePromotionCondActiveDemoted ActivePromotionConditionType = "ActiveDemoted"

	// ActivePromotionCondActivePromoted means the pre-active namespace has been promoted to be a new active
	// In case of successful promoting
	ActivePromotionCondActivePromoted ActivePromotionConditionType = "ActivePromoted"
	// ActivePromotionCondPreviousActiveDestroyed means previous active namespace has been destroyed
	// In case of successful promoting
	ActivePromotionCondPreviousActiveDestroyed ActivePromotionConditionType = "PreviousActiveDestroyed"
	// ActivePromotionCondPreActiveDestroyed means the pre-active namespace has been destroyed
	// In case of failed promoting
	ActivePromotionCondPreActiveDestroyed ActivePromotionConditionType = "PreActiveDestroyed"

	// ActivePromotionCondFinished means the active promotion process has been finished
	ActivePromotionCondFinished ActivePromotionConditionType = "Finished"

	// ActivePromotionCondRollbackStarted means the rollback process has been started
	ActivePromotionCondRollbackStarted ActivePromotionConditionType = "Rollback"
)

// ActivePromotionSpec defines the desired state of ActivePromotion
type ActivePromotionSpec struct {
	// TearDownDuration represents duration before tear down the previous active namespace
	// +optional
	TearDownDuration *metav1.Duration `json:"tearDownDuration,omitempty"`

	// SkipTestRunner represents a flag for skipping running pre-active test
	// +optional
	SkipTestRunner bool `json:"skipTestRunner,omitempty"`

	// ActivePromotedBy represents a person who promoted the ActivePromotion
	// +optional
	PromotedBy string `json:"promotedBy,omitempty"`
}

func (s *ActivePromotionSpec) SetTearDownDuration(d metav1.Duration) {
	s.TearDownDuration = &d
}

// ActivePromotionStatus defines the observed state of ActivePromotion
type ActivePromotionStatus struct {
	// ActivePromotionState represents a current state of the active promotion
	// +optional
	State ActivePromotionState `json:"state,omitempty"`
	// Message defines details about why the active promotion is in this condition
	// +optional
	Message string `json:"message,omitempty"`
	// StartedAt represents time at which the active promotion started
	// +optional
	StartedAt *metav1.Time `json:"startedAt,omitempty"`
	// UpdatedAt represents time at which the active promotion finished
	// +optional
	UpdatedAt *metav1.Time `json:"updatedAt,omitempty"`
	// TargetNamespace represents a pre-active namespace
	// +optional
	TargetNamespace string `json:"targetNamespace,omitempty"`
	// PreviousActiveNamespace represents an active namespace before promoting
	// +optional
	PreviousActiveNamespace string `json:"previousActiveNamespace,omitempty"`
	// Result represents a result of the active promotion
	// +optional
	Result ActivePromotionResult `json:"result,omitempty"`
	// DestroyedTime represents time at which the previous active namespace will be destroyed
	// +optional
	DestroyedTime *metav1.Time `json:"destroyedTime,omitempty"`
	// ActivePromotionHistoryName represents created ActivePromotionHistoryName name
	// +optional
	ActivePromotionHistoryName string `json:"activePromotionHistoryName,omitempty"`
	// HasOutdatedComponent defines whether current active promotion has outdated component or not
	// +optional
	HasOutdatedComponent bool `json:"hasOutdatedComponent,omitempty"`
	// IsTimeout defines whether the active promotion has been timeout or not
	// +optional
	IsTimeout bool `json:"isTimeout,omitempty"`
	// ActiveComponents represents a list of promoted active components
	// +optional
	ActiveComponents map[string]StableComponent `json:"activeComponents,omitempty"`
	// OutdatedComponents represents map of outdated components
	// +optional
	OutdatedComponents map[string]OutdatedComponent `json:"outdatedComponents,omitempty"`
	// RollbackStatus represents a status of the rollback process
	// +optional
	RollbackStatus ActivePromotionRollbackStatus `json:"rollbackStatus,omitempty"`
	// DemotionStatus represents a status of the active demotion
	// +optional
	DemotionStatus ActivePromotionDemotionStatus `json:"demotionStatus,omitempty"`
	// PreActiveQueue represents a pre-active queue status
	// +optional
	PreActiveQueue QueueStatus `json:"preActiveQueue,omitempty"`

	// Conditions contains observations of the resource's state e.g.,
	// Queue deployed, being tested
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []ActivePromotionCondition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

func (s *ActivePromotionStatus) SetNamespace(targetNs, currentActiveNs string) {
	s.TargetNamespace = targetNs
	s.PreviousActiveNamespace = currentActiveNs
}

func (s *ActivePromotionStatus) SetResult(res ActivePromotionResult) {
	s.Result = res
}

func (s *ActivePromotionStatus) SetRollbackStatus(status ActivePromotionRollbackStatus) {
	s.RollbackStatus = status
}

func (s *ActivePromotionStatus) SetDemotionStatus(status ActivePromotionDemotionStatus) {
	s.DemotionStatus = status
}

func (s *ActivePromotionStatus) SetIsTimeout() {
	s.IsTimeout = true
}

func (s *ActivePromotionStatus) SetDestroyedTime(destroyedTime metav1.Time) {
	s.DestroyedTime = &destroyedTime
}

func (s *ActivePromotionStatus) SetActivePromotionHistoryName(atpHistName string) {
	s.ActivePromotionHistoryName = atpHistName
}

func (s *ActivePromotionStatus) SetPreActiveQueue(qs QueueStatus) {
	s.PreActiveQueue = qs
}

func (s *ActivePromotionStatus) SetActiveComponents(comps []StableComponent) {
	s.ActiveComponents = make(map[string]StableComponent)
	for _, currentComp := range comps {
		s.ActiveComponents[currentComp.Spec.Name] = StableComponent{
			Spec:   currentComp.Spec,
			Status: currentComp.Status,
		}
	}
}

func (s *ActivePromotionStatus) GetConditionLatestTime(cond ActivePromotionConditionType) *metav1.Time {
	for _, c := range s.Conditions {
		if c.Type == cond {
			return &c.LastTransitionTime
		}
	}

	return nil
}

func (s *ActivePromotionStatus) IsConditionTrue(cond ActivePromotionConditionType) bool {
	for i, c := range s.Conditions {
		if c.Type == cond {
			return s.Conditions[i].Status == v1.ConditionTrue
		}
	}
	return false
}

func (s *ActivePromotionStatus) SetCondition(cond ActivePromotionConditionType, status v1.ConditionStatus, message string) {
	for i, c := range s.Conditions {
		if c.Type == cond {
			s.Conditions[i].Status = status
			s.Conditions[i].LastTransitionTime = metav1.Now()
			s.Conditions[i].Message = message
			return
		}
	}

	s.Conditions = append(s.Conditions, ActivePromotionCondition{
		Type:               cond,
		Status:             status,
		LastTransitionTime: metav1.Now(),
		Message:            message,
	})
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster

// ActivePromotion is the Schema for the activepromotions API
type ActivePromotion struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ActivePromotionSpec   `json:"spec,omitempty"`
	Status ActivePromotionStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ActivePromotionList contains a list of ActivePromotion
type ActivePromotionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ActivePromotion `json:"items"`
}

func (a *ActivePromotion) SetState(state ActivePromotionState, message string) {
	now := metav1.Now()
	a.Status.State = state
	a.Status.Message = message
	a.Status.UpdatedAt = &now

	if a.Status.State == ActivePromotionCreatingPreActive && a.Status.StartedAt == nil {
		a.Status.StartedAt = &now
	}
}

func (a *ActivePromotion) IsActivePromotionSuccess() bool {
	return a.Status.Result == ActivePromotionSuccess
}

func (a *ActivePromotion) IsActivePromotionFailure() bool {
	return a.Status.Result == ActivePromotionFailure
}

func (a *ActivePromotion) IsActivePromotionCanceled() bool {
	return a.Status.Result == ActivePromotionCanceled
}

// sort ActivePromotion by timestamp ASC
func (al *ActivePromotionList) SortASC() {
	sort.Sort(ActivePromotionByCreatedTimeASC(al.Items))
}

// +k8s:deepcopy-gen=false

type ActivePromotionByCreatedTimeASC []ActivePromotion

func (a ActivePromotionByCreatedTimeASC) Len() int { return len(a) }
func (a ActivePromotionByCreatedTimeASC) Less(i, j int) bool {
	return a[i].CreationTimestamp.Time.Before(a[j].CreationTimestamp.Time)
}

func (a ActivePromotionByCreatedTimeASC) Swap(i, j int) { a[i], a[j] = a[j], a[i] }

// OutdatedComponent defines properties of outdated component
type OutdatedComponent struct {
	CurrentImage     *Image        `json:"currentImage"`
	DesiredImage     *Image        `json:"desiredImage"`
	OutdatedDuration time.Duration `json:"outdatedDuration"`
}

// SortComponentsByOutdatedDuration sorts components by outdated days descending order
func SortComponentsByOutdatedDuration(components []OutdatedComponent) {
	sort.Slice(components, func(i, j int) bool { return components[i].OutdatedDuration > components[j].OutdatedDuration })
}

func init() {
	SchemeBuilder.Register(&ActivePromotion{}, &ActivePromotionList{})
}
