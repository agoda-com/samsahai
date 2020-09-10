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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// UpdatingSource represents source for checking desired version of components
type UpdatingSource string

// Component represents a chart of component and it's dependencies
type Component struct {
	// +optional
	Parent string         `json:"parent,omitempty"`
	Name   string         `json:"name"`
	Chart  ComponentChart `json:"chart"`
	Image  ComponentImage `json:"image,omitempty"`
	// +optional
	Values ComponentValues `json:"values,omitempty"`
	// +optional
	Source *UpdatingSource `json:"source,omitempty"`
	// +optional
	Dependencies []*Component `json:"dependencies,omitempty"`
	// +optional
	Schedules []string `json:"schedules,omitempty"`
}

// ComponentImage represents an image repository, tag and pattern which is a regex of tag
type ComponentImage struct {
	Repository string `json:"repository"`
	// +optional
	Tag string `json:"tag,omitempty"`
	// +optional
	Pattern string `json:"pattern,omitempty"`
}

// ComponentChart represents a chart repository, name and version
type ComponentChart struct {
	Repository string `json:"repository"`
	Name       string `json:"name"`
	// +optional
	Version string `json:"version,omitempty"`
}

// ConfigBundles represents a group of component for each bundle
// to verify a group of components of a same bundle together in staging environment
type ConfigBundles map[string][]string

// ConfigStaging represents configuration about staging
type ConfigStaging struct {
	// Deployment represents configuration about deploy
	Deployment *ConfigDeploy `json:"deployment"`

	// MaxRetry defines max retry counts of component upgrade
	// +optional
	MaxRetry int `json:"maxRetry,omitempty"`

	// MaxHistoryDays defines maximum days of QueueHistory stored
	// +optional
	MaxHistoryDays int `json:"maxHistoryDays,omitempty"`
}

type ConfigDeploy struct {
	// Timeout defines maximum duration for deploying environment
	// +optional
	Timeout metav1.Duration `json:"timeout,omitempty"`

	// ComponentCleanupTimeout defines timeout duration of component cleaning up
	// +optional
	ComponentCleanupTimeout metav1.Duration `json:"componentCleanupTimeout,omitempty"`

	// Engine defines method of deploying
	//
	// mock - for test only, always return success
	//
	// helm3 - deploy chart with helm3
	// +optional
	Engine *string `json:"engine,omitempty"`

	// TestRunner represents configuration about test
	// +optional
	TestRunner *ConfigTestRunner `json:"testRunner,omitempty"`
}

// ConfigTestRunner represents configuration about how to test the environment
type ConfigTestRunner struct {
	// +optional
	Timeout metav1.Duration `json:"timeout,omitempty"`
	// +optional
	PollingTime metav1.Duration `json:"pollingTime,omitempty"`
	// +optional
	Teamcity *ConfigTeamcity `json:"teamcity,omitempty"`
	// +optional
	TestMock *ConfigTestMock `json:"testMock,omitempty"`
}

// ConfigTeamcity defines a http rest configuration of teamcity
type ConfigTeamcity struct {
	BuildTypeID string `json:"buildTypeID" yaml:"buildTypeID"`
	Branch      string `json:"branch" yaml:"branch"`
}

// ConfigTestMock defines a result of testmock
type ConfigTestMock struct {
	Result bool `json:"result" yaml:"result"`
}

// ConfigActivePromotion represents configuration about active promotion
type ConfigActivePromotion struct {
	// Timeout defines maximum duration for doing active promotion
	// +optional
	Timeout metav1.Duration `json:"timeout,omitempty"`

	// DemotionTimeout defines maximum duration for doing active demotion
	// +optional
	DemotionTimeout metav1.Duration `json:"demotionTimeout,omitempty"`

	// RollbackTimeout defines maximum duration for rolling back active promotion
	// +optional
	RollbackTimeout metav1.Duration `json:"rollbackTimeout,omitempty"`

	// MaxHistories defines maximum length of ActivePromotionHistory stored per team
	// +optional
	MaxHistories int `json:"maxHistories,omitempty"`

	// TearDownDuration defines duration before teardown the previous active namespace
	// +optional
	TearDownDuration metav1.Duration `json:"tearDownDuration,omitempty"`

	// OutdatedNotification defines a configuration of outdated notification
	// +optional
	OutdatedNotification *OutdatedNotification `json:"outdatedNotification,omitempty"`

	// Deployment represents configuration about deploy
	Deployment *ConfigDeploy `json:"deployment"`
}

// OutdatedNotification defines a configuration of outdated notification
type OutdatedNotification struct {
	// +optional
	ExceedDuration metav1.Duration `json:"exceedDuration,omitempty"`
	// +optional
	ExcludeWeekendCalculation bool `json:"excludeWeekendCalculation,omitempty"`
}

// ConfigReporter represents configuration about sending notification
type ConfigReporter struct {
	// +optional
	Optional []ReportOption `json:"optionals,omitempty"`
	// +optional
	Slack *Slack `json:"slack,omitempty"`
	// +optional
	MSTeams *MSTeams `json:"msTeams,omitempty"`
	// +optional
	Rest *Rest `json:"rest,omitempty"`
	// +optional
	Shell *Shell `json:"cmd,omitempty"`
	// +optional
	ReportMock bool `json:"reportMock,omitempty"`
}

// ReportOption defines an optional configuration of slack
type ReportOption struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// ReporterInterval represents how often of sending component upgrade notification within a retry cycle
type ReporterInterval string

const (
	// IntervalEveryTime means sending slack notification in every component upgrade runs
	IntervalEveryTime ReporterInterval = "everytime"
	// IntervalRetry means sending slack notification after retry only
	IntervalRetry ReporterInterval = "retry"
)

// ReporterCriteria represents a criteria of sending component upgrade notification
type ReporterCriteria string

const (
	// CriteriaSuccess means sending slack notification when component upgrade is success only
	CriteriaSuccess ReporterCriteria = "success"
	// CriteriaFailure means sending slack notification when component upgrade is failure only
	CriteriaFailure ReporterCriteria = "failure"
	// CriteriaBoth means sending slack notification whether component upgrade is success or failure
	CriteriaBoth ReporterCriteria = "both"
)

// Slack defines a configuration of slack
type Slack struct {
	Channels []string `json:"channels"`
	// +optional
	ComponentUpgrade *ConfigComponentUpgrade `json:"componentUpgrade,omitempty"`
}

// MSTeams defines a configuration of Microsoft Teams
type MSTeams struct {
	Groups []MSTeamsGroup `json:"groups"`
	// +optional
	ComponentUpgrade *ConfigComponentUpgrade `json:"componentUpgrade,omitempty"`
}

// MSTeamsGroup defines group name/id and channel name/id of Microsoft Teams
type MSTeamsGroup struct {
	GroupNameOrID    string   `json:"groupNameOrID"`
	ChannelNameOrIDs []string `json:"channelNameOrIDs"`
}

// ConfigComponentUpgrade defines a configuration of component upgrade report
type ConfigComponentUpgrade struct {
	// +optional
	Interval ReporterInterval `json:"interval,omitempty"`
	// +optional
	Criteria ReporterCriteria `json:"criteria,omitempty"`
}

// Rest defines a configuration of http rest
type Rest struct {
	// +optional
	ComponentUpgrade *RestObject `json:"componentUpgrade,omitempty"`
	// +optional
	ActivePromotion *RestObject `json:"activePromotion,omitempty"`
	// +optional
	ImageMissing *RestObject `json:"imageMissing,omitempty"`
}

type RestObject struct {
	Endpoints []*Endpoint `json:"endpoints"`
}

// Shell defines a configuration of shell command
type Shell struct {
	// +optional
	ComponentUpgrade *CommandAndArgs `json:"componentUpgrade,omitempty"`
	// +optional
	ActivePromotion *CommandAndArgs `json:"activePromotion,omitempty"`
	// +optional
	ImageMissing *CommandAndArgs `json:"imageMissing,omitempty"`
}

// CommandAndArgs defines commands and args
type CommandAndArgs struct {
	Command []string `json:"command"`
	// +optional
	Args []string `json:"args,omitempty"`
}

// Endpoint defines a configuration of rest endpoint
type Endpoint struct {
	URL string `json:"url"`
	// TODO: auth
}

type EnvType string

const (
	EnvBase        EnvType = "base"
	EnvStaging     EnvType = "staging"
	EnvPreActive   EnvType = "pre-active"
	EnvActive      EnvType = "active"
	EnvDeActive    EnvType = "de-active"
	EnvPullRequest EnvType = "pull-request"
)

// ChartValuesURLs represents values file URL of each chart
type ChartValuesURLs map[string][]string

// PullRequestComponent represents a pull request component configuration
type PullRequestComponent struct {
	// Name defines a main component name which is deployed per pull request
	Name string `json:"name"`
	// Chart defines a chart repository, name and version of pull request component
	Chart ComponentChart `json:"chart"`
	// Image defines an image repository, tag and pattern of pull request component which is a regex of tag
	// +optional
	Image ComponentImage `json:"image,omitempty"`
	// Dependencies defines a list of components which are required to be deployed together with the main component
	// +optional
	Dependencies           []string `json:"dependencies,omitempty"`
	PullRequestExtraConfig `json:",inline"`
}

// PullRequestTriggerConfig represents a pull request trigger configuration
type PullRequestTriggerConfig struct {
	// PollingTime defines a waiting duration time to re-check the pull request image in the registry
	// +optional
	PollingTime metav1.Duration `json:"pollingTime,omitempty"`
	// MaxRetry defines max retry counts of pull request trigger if cannot find image in the registry
	// +optional
	MaxRetry int `json:"maxRetry,omitempty"`
}

// PullRequestExtraConfig represents a pull request extra configuration
type PullRequestExtraConfig struct {
	// Parallel defines a parallel number of pull request queue
	// +optional
	Parallel int `json:"parallel,omitempty"`
	// MaxRetry defines max retry counts of pull request deployment if it fails
	// +optional
	MaxRetry int `json:"maxRetry,omitempty"`
	// Resources represents how many resources of pull request namespace
	// +optional
	Resources corev1.ResourceList `json:"resources,omitempty"`
	// +optional
	Trigger PullRequestTriggerConfig `json:"trigger,omitempty"`
}

// ConfigPullRequest defines a configuration of pull request
type ConfigPullRequest struct {
	Components             []*PullRequestComponent `json:"components"`
	PullRequestExtraConfig `json:",inline"`
}

// ConfigSpec defines the desired state of Config
type ConfigSpec struct {
	// Components represents all components that are managed
	Components []*Component `json:"components"`

	// Bundles represents a group of component for each bundle
	// +optional
	Bundles ConfigBundles `json:"bundles,omitempty"`

	// PriorityQueues represents a list of bundles/components' name which needs to be prioritized
	// the first one has the highest priority and the last one has the lowest priority
	// +optional
	PriorityQueues []string `json:"priorityQueues,omitempty"`

	// Staging represents configuration about staging
	Staging *ConfigStaging `json:"staging"`

	// ActivePromotion represents configuration about active promotion
	// +optional
	ActivePromotion *ConfigActivePromotion `json:"activePromotion,omitempty"`

	// Envs represents urls of values file per environments
	// ordering by less priority to high priority
	// +optional
	Envs map[EnvType]ChartValuesURLs `json:"envs,omitempty"`

	// Reporter represents configuration about reporter
	// +optional
	Reporter *ConfigReporter `json:"report,omitempty"`
}

// ConfigStatus defines the observed state of Config
type ConfigStatus struct {
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster

// Config is the Schema for the configs API
type Config struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ConfigSpec   `json:"spec,omitempty"`
	Status ConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster

// ConfigList contains a list of Config
type ConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Config `json:"items"`
}

// +k8s:deepcopy-gen=false
//ComponentValues represents values of a component chart
type ComponentValues map[string]interface{}

func (in *ComponentValues) DeepCopyInto(out *ComponentValues) {
	if in == nil {
		*out = nil
	} else {
		*out = runtime.DeepCopyJSON(*in)
	}
}

func (in *ComponentValues) DeepCopy() *ComponentValues {
	if in == nil {
		return nil
	}
	out := new(ComponentValues)
	in.DeepCopyInto(out)
	return out
}

func init() {
	SchemeBuilder.Register(&Config{}, &ConfigList{})
}
