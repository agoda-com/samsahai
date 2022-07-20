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
	"encoding/json"
	"fmt"

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
	// +kubebuilder:pruning:PreserveUnknownFields
	Values ComponentValues `json:"values,omitempty"`
	// +optional
	Source *UpdatingSource `json:"source,omitempty"`
	// +optional
	Schedules []string `json:"schedules,omitempty"`
	// +optional
	Dependencies []*Dependency `json:"dependencies,omitempty"`
}

// Dependency represents a chart of dependency
type Dependency struct {
	// +optional
	Parent string `json:"parent,omitempty"`
	Name   string `json:"name"`
	// +optional
	Chart ComponentChart `json:"chart"`
	Image ComponentImage `json:"image,omitempty"`
	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	Values ComponentValues `json:"values,omitempty"`
	// +optional
	Source *UpdatingSource `json:"source,omitempty"`
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
	// +optional
	Deployment *ConfigDeploy `json:"deployment,omitempty"`

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
	// TODO: make Timeout and PollingTime pointers to reduce duplicate code in ConfigTestRunnerOverrider

	// +optional
	Timeout metav1.Duration `json:"timeout,omitempty"`
	// +optional
	PollingTime metav1.Duration `json:"pollingTime,omitempty"`
	// +optional
	Gitlab *ConfigGitlab `json:"gitlab,omitempty"`
	// +optional
	Teamcity *ConfigTeamcity `json:"teamcity,omitempty"`
	// +optional
	TestMock *ConfigTestMock `json:"testMock,omitempty"`
}

// ConfigTestRunnerOverrider is data that overrides ConfigTestRunner field by field
type ConfigTestRunnerOverrider struct {
	// +optional
	Timeout *metav1.Duration `json:"timeout,omitempty"`
	// +optional
	PollingTime *metav1.Duration `json:"pollingTime,omitempty"`
	// +optional
	Gitlab *ConfigGitlabOverrider `json:"gitlab,omitempty"`
	// +optional
	Teamcity *ConfigTeamcityOverrider `json:"teamcity,omitempty"`
	// +optional
	TestMock *ConfigTestMock `json:"testMock,omitempty"`
}

// Override overrides ConfigTestRunner and return a reference to the overridden instance.
// The operation will try to override an instance in-place if possible.
func (c ConfigTestRunnerOverrider) Override(confTestRunner *ConfigTestRunner) *ConfigTestRunner {
	ensureConfTestRunner := func() {
		if confTestRunner == nil {
			confTestRunner = &ConfigTestRunner{}
		}
	}
	if c.Timeout != nil {
		ensureConfTestRunner()
		confTestRunner.Timeout = *c.Timeout.DeepCopy()
	}
	if c.PollingTime != nil {
		ensureConfTestRunner()
		confTestRunner.PollingTime = *c.PollingTime.DeepCopy()
	}
	if c.Gitlab != nil {
		ensureConfTestRunner()
		confTestRunner.Gitlab = c.Gitlab.Override(confTestRunner.Gitlab)
	}
	if c.Teamcity != nil {
		ensureConfTestRunner()
		confTestRunner.Teamcity = c.Teamcity.Override(confTestRunner.Teamcity)
	}
	if c.TestMock != nil {
		ensureConfTestRunner()
		confTestRunner.TestMock = c.TestMock.DeepCopy()
	}
	return confTestRunner
}

// ConfigTeamcity defines a http rest configuration of teamcity
type ConfigTeamcity struct {
	// TODO: make every fields optional to reduce duplicate code in ConfigTeamcityOverrider

	BuildTypeID string `json:"buildTypeID" yaml:"buildTypeID"`
	Branch      string `json:"branch" yaml:"branch"`
}

// ConfigTeamcityOverrider is data that overrides ConfigTeamcity field by field
type ConfigTeamcityOverrider struct {
	// +optional
	BuildTypeID *string `json:"buildTypeID,omitempty"`
	// +optional
	Branch *string `json:"branch,omitempty"`
}

// Override overrides ConfigTeamcity and return a reference to the overridden instance.
// The operation will try to override an instance in-place if possible.
func (c ConfigTeamcityOverrider) Override(confTeamcity *ConfigTeamcity) *ConfigTeamcity {
	ensureConfTeamcity := func() {
		if confTeamcity == nil {
			confTeamcity = &ConfigTeamcity{}
		}
	}
	if c.BuildTypeID != nil {
		ensureConfTeamcity()
		confTeamcity.BuildTypeID = *c.BuildTypeID
	}
	if c.Branch != nil {
		ensureConfTeamcity()
		confTeamcity.Branch = *c.Branch
	}
	return confTeamcity
}

// ConfigGitlab defines a http rest configuration of gitlab
type ConfigGitlab struct {
	// TODO: make every fields optional to reduce duplicate code in ConfigGitlabOverrider

	ProjectID            string `json:"projectID" yaml:"projectID"`
	PipelineTriggerToken string `json:"pipelineTriggerToken" yaml:"pipelineTriggerToken"`
	// +optional
	Branch string `json:"branch,omitempty" yaml:"branch,omitempty"`
	// InferBranch is for Pull Request's testRunner on gitlab.
	// If true, samsahai will try to infer the testRunner branch name
	// from the gitlab MR associated with the PR flow if branch is empty [default: true].
	// +optional
	InferBranch *bool `json:"inferBranch,omitempty" yaml:"inferBranch,omitempty"`
}

func (c ConfigGitlab) GetInferBranch() bool {
	// default is true
	if c.InferBranch == nil {
		return true
	}
	return *c.InferBranch
}

func (c *ConfigGitlab) SetInferBranch(inferBranch bool) {
	c.InferBranch = &inferBranch
}

// ConfigGitlabOverrider is data that overrides ConfigGitlab field by field
type ConfigGitlabOverrider struct {
	// +optional
	ProjectID *string `json:"projectID,omitempty"`
	// +optional
	PipelineTriggerToken *string `json:"pipelineTriggerToken,omitempty"`
	// +optional
	Branch *string `json:"branch,omitempty"`
	// +optional
	InferBranch *bool `json:"inferBranch,omitempty"`
}

// Override overrides ConfigGitlab and return a reference to the overridden instance.
// The operation will try to override an instance in-place if possible.
func (c ConfigGitlabOverrider) Override(confGitlab *ConfigGitlab) *ConfigGitlab {
	ensureConfGitlab := func() {
		if confGitlab == nil {
			confGitlab = &ConfigGitlab{}
		}
	}
	if c.ProjectID != nil {
		ensureConfGitlab()
		confGitlab.ProjectID = *c.ProjectID
	}
	if c.Branch != nil {
		ensureConfGitlab()
		confGitlab.Branch = *c.Branch
	}
	if c.PipelineTriggerToken != nil {
		ensureConfGitlab()
		confGitlab.PipelineTriggerToken = *c.PipelineTriggerToken
	}
	if c.InferBranch != nil {
		ensureConfGitlab()
		clone := *c.InferBranch
		confGitlab.InferBranch = &clone
	}
	return confGitlab
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

	// MaxRetry defines max retry counts of active promotion process in case failure
	// +optional
	MaxRetry *int `json:"maxRetry,omitempty"`

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
	// +optional
	Deployment *ConfigDeploy `json:"deployment,omitempty"`
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
	Slack *ReporterSlack `json:"slack,omitempty"`
	// +optional
	MSTeams *ReporterMSTeams `json:"msTeams,omitempty"`
	// +optional
	Github *ReporterGithub `json:"github,omitempty"`
	// +optional
	Gitlab *ReporterGitlab `json:"gitlab,omitempty"`
	// +optional
	Rest *ReporterRest `json:"rest,omitempty"`
	// +optional
	Shell *ReporterShell `json:"cmd,omitempty"`
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

// ReporterExtraMessage represents an extra message of sending component upgrade notification
type ReporterExtraMessage string

const (
	// CriteriaSuccess means sending slack notification when component upgrade is success only
	CriteriaSuccess ReporterCriteria = "success"
	// CriteriaFailure means sending slack notification when component upgrade is failure only
	CriteriaFailure ReporterCriteria = "failure"
	// CriteriaBoth means sending slack notification whether component upgrade is success or failure
	CriteriaBoth ReporterCriteria = "both"
	// ConfigUsedUpdated means the configuration used has been updated
	ConfigUsedUpdated ConfigConditionType = "ConfigUsedUpdated"
	// ConfigRequiredFieldsValidated means the required fields have been validated
	ConfigRequiredFieldsValidated ConfigConditionType = "ConfigRequiredFieldsValidated"
)

// ReporterSlack defines a configuration of slack
type ReporterSlack struct {
	Channels []string `json:"channels"`
	// +optional
	ExtraMessage ReporterExtraMessage `json:"extraMessage,omitempty"`
	// +optional
	ComponentUpgrade *ConfigComponentUpgradeReport `json:"componentUpgrade,omitempty"`
	// +optional
	ActivePromotion *ConfigActivePromotionReport `json:"activePromotion,omitempty"`
	// +optional
	PullRequestTrigger *ConfigPullRequestTriggerReport `json:"pullRequestTrigger,omitempty"`
	// +optional
	PullRequestQueue *ConfigPullRequestQueueReport `json:"pullRequestQueue,omitempty"`
}

// ReporterMSTeams defines a configuration of Microsoft Teams
type ReporterMSTeams struct {
	Groups []MSTeamsGroup `json:"groups"`
	// +optional
	ComponentUpgrade *ConfigComponentUpgradeReport `json:"componentUpgrade,omitempty"`
	// +optional
	PullRequestTrigger *ConfigPullRequestTriggerReport `json:"pullRequestTrigger,omitempty"`
	// +optional
	PullRequestQueue *ConfigPullRequestQueueReport `json:"pullRequestQueue,omitempty"`
}

// MSTeamsGroup defines group name/id and channel name/id of Microsoft Teams
type MSTeamsGroup struct {
	GroupNameOrID    string   `json:"groupNameOrID"`
	ChannelNameOrIDs []string `json:"channelNameOrIDs"`
}

// ConfigComponentUpgradeReport defines a configuration of component upgrade report
type ConfigComponentUpgradeReport struct {
	// +optional
	Interval ReporterInterval `json:"interval,omitempty"`
	// +optional
	Criteria ReporterCriteria `json:"criteria,omitempty"`
	// +optional
	ExtraMessage ReporterExtraMessage `json:"extraMessage,omitempty"`
}

// ConfigActivePromotionReport defines a configuration of active promotion report
type ConfigActivePromotionReport struct {
	// +optional
	ExtraMessage ReporterExtraMessage `json:"extraMessage,omitempty"`
}

// ConfigPullRequestTrigger defines a configuration of pull request trigger report
type ConfigPullRequestTriggerReport struct {
	// +optional
	Criteria ReporterCriteria `json:"criteria,omitempty"`
	// +optional
	ExtraMessage ReporterExtraMessage `json:"extraMessage,omitempty"`
}

// ConfigPullRequestQueueReport defines a configuration of pull request queues report
type ConfigPullRequestQueueReport struct {
	// +optional
	Interval ReporterInterval `json:"interval,omitempty"`
	// +optional
	Criteria ReporterCriteria `json:"criteria,omitempty"`
	// +optional
	ExtraMessage ReporterExtraMessage `json:"extraMessage,omitempty"`
}

// ReporterGithub defines a configuration of github reporter
// supports pull request queue reporter type only
type ReporterGithub struct {
	// Enabled represents an enabled flag
	// +optional
	Enabled bool `json:"enabled"`
	// BaseURL represents a github base url e.g., https://github.com
	// +optional
	BaseURL string `json:"baseURL,omitempty"`
}

// ReporterGitlab defines a configuration of gitlab reporter
// supports pull request queue reporter type only
type ReporterGitlab struct {
	// Enabled represents an enabled flag
	// +optional
	Enabled bool `json:"enabled"`
	// BaseURL represents a gitlab base url e.g., https://gitlab.com
	// +optional
	BaseURL string `json:"baseURL,omitempty"`
}

// ReporterRest defines a configuration of http rest
type ReporterRest struct {
	// +optional
	ComponentUpgrade *RestObject `json:"componentUpgrade,omitempty"`
	// +optional
	ActivePromotion *RestObject `json:"activePromotion,omitempty"`
	// +optional
	ImageMissing *RestObject `json:"imageMissing,omitempty"`
	// +optional
	PullRequestTrigger *RestObject `json:"pullRequestTrigger,omitempty"`
	// +optional
	PullRequestQueue *RestObject `json:"pullRequestQueue,omitempty"`
}

type RestObject struct {
	Endpoints []*Endpoint `json:"endpoints"`
}

// ReporterShell defines a configuration of shell command
type ReporterShell struct {
	// +optional
	ComponentUpgrade *CommandAndArgs `json:"componentUpgrade,omitempty"`
	// +optional
	ActivePromotion *CommandAndArgs `json:"activePromotion,omitempty"`
	// +optional
	ImageMissing *CommandAndArgs `json:"imageMissing,omitempty"`
	// +optional
	PullRequestTrigger *CommandAndArgs `json:"pullRequestTrigger,omitempty"`
	// +optional
	PullRequestQueue *CommandAndArgs `json:"pullRequestQueue,omitempty"`
	// +optional
	ActiveEnvironmentDeleted *CommandAndArgs `json:"activeEnvironmentDeleted,omitempty"`
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

// PullRequestBundle represents a bundle of pull request components configuration
type PullRequestBundle struct {
	// Name defines a bundle component name, can be any name
	Name string `json:"name"`
	// Components represents a list of pull request components which are deployed together as a bundle
	Components []*PullRequestComponent `json:"components"`
	// Deployment represents configuration about deploy
	// +optional
	Deployment *ConfigDeploy `json:"deployment,omitempty"`
	// Dependencies defines a list of components which are required to be deployed together with the main component
	// +optional
	Dependencies []string `json:"dependencies,omitempty"`
	// GitRepository represents a string of git "<owner>/<repository>" e.g., agoda-com/samsahai
	// used for publishing commit status
	// +optional
	GitRepository          string `json:"gitRepository,omitempty"`
	PullRequestExtraConfig `json:",inline"`
}

// PullRequestComponent represents a pull request component configuration
type PullRequestComponent struct {
	// Name defines a main component name which is deployed per pull request
	Name string `json:"name"`
	// Image defines an image repository, tag and pattern of pull request component which is a regex of tag
	// +optional
	Image ComponentImage `json:"image,omitempty"`
	// Source defines a source for image repository
	// +optional
	Source                 *UpdatingSource `json:"source,omitempty"`
	PullRequestExtraConfig `json:",inline"`
}

type PullRequestTearDownDurationCriteria string

const (
	// PullRequestTearDownDurationCriteriaBoth means the duration will apply when the tests either succeeded or failed.
	PullRequestTearDownDurationCriteriaBoth PullRequestTearDownDurationCriteria = "both"
	// PullRequestTearDownDurationCriteriaFailure means the duration will apply at only when the tests failed.
	PullRequestTearDownDurationCriteriaFailure PullRequestTearDownDurationCriteria = "failure"
	// PullRequestTearDownDurationCriteriaSuccess means the duration will apply at only when the tests succeeded.
	PullRequestTearDownDurationCriteriaSuccess PullRequestTearDownDurationCriteria = "success"
)

func (c *PullRequestTearDownDurationCriteria) UnmarshalJSON(b []byte) (err error) {
	var str string
	err = json.Unmarshal(b, &str)
	if err != nil {
		return
	}

	switch str {
	case "":
		// Default is failure
		*c = PullRequestTearDownDurationCriteriaFailure
	case string(PullRequestTearDownDurationCriteriaBoth):
		*c = PullRequestTearDownDurationCriteriaBoth
	case string(PullRequestTearDownDurationCriteriaFailure):
		*c = PullRequestTearDownDurationCriteriaFailure
	case string(PullRequestTearDownDurationCriteriaSuccess):
		*c = PullRequestTearDownDurationCriteriaSuccess
	default:
		err = fmt.Errorf("%s is not a valid tearDownDuration criteria", str)
	}
	return
}

func (c *PullRequestTearDownDurationCriteria) MarshalJSON() (b []byte, err error) {
	criteria := *c
	if string(criteria) == "" {
		// Default is failure
		criteria = PullRequestTearDownDurationCriteriaFailure
	}
	return json.Marshal(string(criteria))
}

// PullRequestTearDownDuration contains information about tearDownDuration of the pull request
type PullRequestTearDownDuration struct {
	// Duration tells how much the staging controller will wait before destroying the pull request namespace
	Duration metav1.Duration `json:"duration"`
	// Criteria tells how does the duration apply, default is `failure`.
	// +optional
	Criteria PullRequestTearDownDurationCriteria `json:"criteria"`
}

// PullRequestTriggerConfig represents a pull request trigger configuration
type PullRequestTriggerConfig struct {
	// PollingTime defines a waiting duration time to re-check the pull request image in the registry
	// +optional
	PollingTime metav1.Duration `json:"pollingTime,omitempty"`
	// MaxRetry defines max retry counts of pull request trigger if cannot find image in the registry
	// +optional
	MaxRetry *int `json:"maxRetry,omitempty"`
}

// PullRequestExtraConfig represents a pull request extra configuration
type PullRequestExtraConfig struct {
	// MaxRetry defines max retry counts of pull request component upgrade
	// +optional
	MaxRetry *int `json:"maxRetry,omitempty"`
	// Resources represents how many resources of pull request namespace
	// +optional
	Resources corev1.ResourceList `json:"resources,omitempty"`
	// TearDownDuration defines duration before teardown the pull request components
	// +optional
	TearDownDuration *PullRequestTearDownDuration `json:"tearDownDuration,omitempty"`
}

// ConfigPullRequest defines a configuration of pull request
type ConfigPullRequest struct {
	// MaxHistoryDays defines maximum days of PullRequestQueueHistory stored
	// +optional
	MaxHistoryDays int `json:"maxHistoryDays,omitempty"`
	// Trigger represents a pull request trigger configuration
	// +optional
	Trigger PullRequestTriggerConfig `json:"trigger,omitempty"`
	// Bundles represents a bundle of pull request components configuration
	Bundles []*PullRequestBundle `json:"bundles,omitempty"`
	// Concurrences defines a parallel number of pull request queue
	// +optional
	Concurrences int `json:"concurrences,omitempty"`

	PullRequestExtraConfig `json:",inline"`
}

// ConfigSpec defines the desired state of Config
type ConfigSpec struct {
	// Components represents all components that are managed
	// +optional
	Components []*Component `json:"components,omitempty"`

	// Bundles represents a group of component for each bundle
	// +optional
	Bundles ConfigBundles `json:"bundles,omitempty"`

	// PriorityQueues represents a list of bundles/components' name which needs to be prioritized
	// the first one has the highest priority and the last one has the lowest priority
	// +optional
	PriorityQueues []string `json:"priorityQueues,omitempty"`

	// Staging represents configuration about staging
	// +optional
	Staging *ConfigStaging `json:"staging,omitempty"`

	// ActivePromotion represents configuration about active promotion
	// +optional
	ActivePromotion *ConfigActivePromotion `json:"activePromotion,omitempty"`

	// PullRequest represents configuration about pull request
	// +optional
	PullRequest *ConfigPullRequest `json:"pullRequest,omitempty"`

	// Envs represents urls of values file per environments
	// ordering by less priority to high priority
	// +optional
	Envs map[EnvType]ChartValuesURLs `json:"envs,omitempty"`

	// Reporter represents configuration about reporter
	// +optional
	Reporter *ConfigReporter `json:"report,omitempty"`

	// Template represents configuration's template
	// +optional
	Template string `json:"template,omitempty"`
}

// ConfigStatus defines the observed state of Config
type ConfigStatus struct {
	// Used represents overridden configuration specification
	// +optional
	Used ConfigSpec `json:"used,omitempty"`

	// TemplateUID represents the template update ID
	// +optional
	TemplateUID string `json:"templateUID,omitempty"`

	// SyncTemplate represents whether the configuration has been synced to the template or not
	// +optional
	SyncTemplate bool `json:"syncTemplate,omitempty"`

	// Conditions contains observations of the state
	// +optional
	Conditions []ConfigCondition `json:"conditions,omitempty"`
}

type ConfigCondition struct {
	Type   ConfigConditionType    `json:"type"`
	Status corev1.ConditionStatus `json:"status"`
	// +optional
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
	// +optional
	Message string `json:"message,omitempty"`
}

type ConfigConditionType string

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

func (cs *ConfigStatus) IsConditionTrue(cond ConfigConditionType) bool {
	for i, c := range cs.Conditions {
		if c.Type == cond {
			return cs.Conditions[i].Status == corev1.ConditionTrue
		}
	}

	return false
}

func (cs *ConfigStatus) SetCondition(cond ConfigConditionType, status corev1.ConditionStatus, message string) {
	for i, c := range cs.Conditions {
		if c.Type == cond {
			cs.Conditions[i].Status = status
			cs.Conditions[i].LastTransitionTime = metav1.Now()
			cs.Conditions[i].Message = message
			return
		}
	}

	cs.Conditions = append(cs.Conditions, ConfigCondition{
		Type:               cond,
		Status:             status,
		LastTransitionTime: metav1.Now(),
		Message:            message,
	})
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
