package internal

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"net/http"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	s2hv1 "github.com/agoda-com/samsahai/api/v1"
	s2hrpc "github.com/agoda-com/samsahai/pkg/samsahai/rpc"
)

type HTTPHeader string

// GetDefaultLabels returns default labels for kubernetes resources
func GetDefaultLabels(teamName string) map[string]string {
	teamKey := GetTeamLabelKey()
	return map[string]string{
		teamKey:                        teamName,
		"app.kubernetes.io/managed-by": AppName,
	}
}

// GetTeamLabelKey returns team label key
func GetTeamLabelKey() string {
	return "samsahai.io/teamname"
}

type SamsahaiCredential struct {
	InternalAuthToken string
	SlackToken        string
	GithubToken       string
	MSTeams           MSTeamsCredential
	TeamcityUsername  string
	TeamcityPassword  string
}

type MSTeamsCredential struct {
	TenantID     string
	ClientID     string
	ClientSecret string
	Username     string
	Password     string
}

// TODO: add tc credential

// SamsahaiConfig represents configuration of Samsahai itself
type SamsahaiConfig struct {
	// ConfigDirPath defines a directory path of Samsahai configuration
	ConfigDirPath string `json:"-" yaml:"-"`

	// PluginsDir defines a plugins directory path
	PluginsDir string `json:"pluginsDir" yaml:"pluginsDir"`

	// SamsahaiImage defines a Samsahai image name and tag
	SamsahaiImage string `json:"s2hImage" yaml:"s2hImage"`

	// SamsahaiExternalURL defines a Samsahai external url
	SamsahaiExternalURL string `json:"s2hExternalURL" yaml:"s2hExternalURL"`

	// GithubURL defines a Github url
	GithubURL string `json:"githubURL" yaml:"githubURL"`

	// TeamcityURL defines a Teamcity url
	TeamcityURL string `json:"teamcityURL" yaml:"teamcityURL"`

	// ClusterDomain defines a cluster domain name
	ClusterDomain string `json:"clusterDomain" yaml:"clusterDomain"`

	// ActivePromotion defines an active promotion configuration
	ActivePromotion ActivePromotionConfig `json:"activePromotion,omitempty" yaml:"activePromotion,omitempty"`

	// PullRequest represents configuration of pull request
	PullRequest PullRequestConfig `json:"pullRequest,omitempty" yaml:"pullRequest,omitempty"`

	// PostNamespaceCreation defines commands executing after creating s2h namespace
	PostNamespaceCreation *struct {
		s2hv1.CommandAndArgs
	} `json:"postNamespaceCreation,omitempty" yaml:"postNamespaceCreation,omitempty"`

	// StagingEnvs defines environment variables of staging controller
	StagingEnvs map[string]string `json:"stagingEnvs,omitempty" yaml:"stagingEnvs,omitempty"`

	SamsahaiURL        string             `json:"-" yaml:"-"`
	SamsahaiCredential SamsahaiCredential `json:"-" yaml:"-"`
}

// PullRequestConfig represents configuration of pull request
type PullRequestConfig struct {
	// QueueConcurrences defines number of pull request queue concurrences
	QueueConcurrences int `json:"queueConcurrences" yaml:"queueConcurrences"`

	// MaxVerificationRetryCounts defines the maximum times of pull request has been verified
	MaxVerificationRetryCounts int `json:"maxVerificationRetryCounts" yaml:"maxVerificationRetryCounts"`

	// MaxPRTriggerRetryCounts defines the maximum times of pull request has been triggered
	MaxTriggerRetryCounts int `json:"maxTriggerRetryCounts" yaml:"maxTriggerRetryCounts"`

	// TriggerPollingTime defines a waiting duration time to re-check the pull request image in the registry
	TriggerPollingTime metav1.Duration `json:"triggerPollingTime" yaml:"triggerPollingTime"`

	// MaxHistoryDays defines maximum days of PullRequestQueueHistory stored
	MaxHistoryDays int `json:"maxHistoryDays" yaml:"maxHistoryDays"`
}

// ActivePromotionConfig represents configuration of active promotion
type ActivePromotionConfig struct {
	// Concurrences defines number of active promotion concurrences
	Concurrences int `json:"concurrences" yaml:"concurrences"`

	// Timeout defines timeout duration of active promotion process
	Timeout metav1.Duration `json:"timeout" yaml:"timeout"`

	// DemotionTimeout defines timeout duration of active demotion process
	DemotionTimeout metav1.Duration `json:"demotionTimeout" yaml:"demotionTimeout"`

	// RollbackTimeout defines timeout duration of rollback process
	RollbackTimeout metav1.Duration `json:"rollbackTimeout" yaml:"rollbackTimeout"`

	// TearDownDuration defines tear down duration of previous active environment
	TearDownDuration metav1.Duration `json:"teardownDuration" yaml:"teardownDuration"`

	// MaxRetry defines max retry counts of active promotion process in case failure
	MaxRetry *int `json:"maxRetry"`

	// MaxHistories defines max stored histories of active promotion
	MaxHistories int `json:"maxHistories" yaml:"maxHistories"`

	// PromoteOnTeamCreation defines whether auto-promote active environment or not when team creation?
	PromoteOnTeamCreation bool `json:"promoteOnTeamCreation" yaml:"promoteOnTeamCreation"`
}

// SamsahaiController
type SamsahaiController interface {
	s2hrpc.RPC

	http.Handler

	PathPrefix() string

	// Start runs internal worker
	Start(stop <-chan struct{})

	// QueueLen returns no. of internal queue
	QueueLen() int

	// GetTeam returns Team CRD
	GetTeam(teamName string, teamComp *s2hv1.Team) error

	// GetConfigController returns samsahai configuration from config crd
	GetConfigController() ConfigController

	// GetPlugins returns samsahai plugins
	GetPlugins() map[string]Plugin

	// GetActivePromotionDeployEngine returns samsahai deploy engine
	GetActivePromotionDeployEngine(teamName, ns string) DeployEngine

	// EnsureTeamTemplateChanged  updates team if template changed
	EnsureTeamTemplateChanged(teamComp *s2hv1.Team) error

	// LoadTeamSecret loads team secret from main namespace
	LoadTeamSecret(teamComp *s2hv1.Team) error

	// CreateStagingEnvironment creates staging environment
	CreateStagingEnvironment(teamName, namespace string) error

	// CreatePreActiveEnvironment creates pre-active environment
	CreatePreActiveEnvironment(teamName, namespace string) error

	// PromoteActiveEnvironment switches environment from pre-active to active and stores current active components
	PromoteActiveEnvironment(teamComp *s2hv1.Team, namespace, promotedBy string, comps map[string]s2hv1.StableComponent) error

	// DestroyActiveEnvironment destroys active environment when active demotion is failure.
	DestroyActiveEnvironment(teamName, namespace string) error

	// DestroyPreActiveEnvironment destroys pre-active environment when active promotion is failure.
	DestroyPreActiveEnvironment(teamName, namespace string) error

	// DestroyPreviousActiveEnvironment destroys previous active environment when active promotion is success.
	DestroyPreviousActiveEnvironment(teamName, namespace string) error

	// SetPreviousActiveNamespace updates previous active namespace to team status
	SetPreviousActiveNamespace(teamComp *s2hv1.Team, namespace string) error

	// SetPreActiveNamespace updates pre-active namespace to team status
	SetPreActiveNamespace(teamComp *s2hv1.Team, namespace string) error

	// SetActiveNamespace updates active namespace to team status
	SetActiveNamespace(teamComp *s2hv1.Team, namespace string) error

	// NotifyComponentChanged adds Component to queue for checking new version
	NotifyComponentChanged(name, repository, teamName string)

	// NotifyActivePromotionReport sends active promotion status report
	NotifyActivePromotionReport(atpRpt *ActivePromotionReporter)

	// TriggerPullRequestDeployment creates PullRequestTrigger crd object
	TriggerPullRequestDeployment(teamName, component, tag, prNumber, commitSHA string) error

	// API

	// GetConnections returns Services in NodePort type and Ingresses that exist in the namespace
	GetConnections(namespace string) (map[string][]Connection, error)

	// GetTeams returns list of teams in Samsahai
	GetTeams() (*s2hv1.TeamList, error)

	// GetQueueHistories returns QueueHistoryList of the namespace
	GetQueueHistories(namespace string) (*s2hv1.QueueHistoryList, error)

	// GetQueueHistory returns Queue by name and namespace
	GetQueueHistory(name, namespace string) (*s2hv1.QueueHistory, error)

	// GetQueues returns QueueList of the namespace
	GetQueues(namespace string) (*s2hv1.QueueList, error)

	// GetPullRequestQueueHistories returns PullRequestQueueHistoryList of the namespace
	GetPullRequestQueueHistories(namespace string) (*s2hv1.PullRequestQueueHistoryList, error)

	// GetQueueHistory returns PullRequestQueue by name and namespace
	GetPullRequestQueueHistory(name, namespace string) (*s2hv1.PullRequestQueueHistory, error)

	// GetQueues returns PullRequestQueueList of the namespace
	GetPullRequestQueues(namespace string) (*s2hv1.PullRequestQueueList, error)

	// GetStableValues returns Stable Values of parent component in team
	GetStableValues(team *s2hv1.Team, comp *s2hv1.Component) (s2hv1.ComponentValues, error)

	// GetActivePromotions returns ActivePromotionList by labels
	GetActivePromotions() (*s2hv1.ActivePromotionList, error)

	// GetActivePromotion returns ActivePromotion by name
	GetActivePromotion(name string) (v *s2hv1.ActivePromotion, err error)

	// GetActivePromotionHistories returns ActivePromotionList by labels
	GetActivePromotionHistories(selectors map[string]string) (*s2hv1.ActivePromotionHistoryList, error)

	// GetActivePromotionHistory returns ActivePromotion by name
	GetActivePromotionHistory(name string) (*s2hv1.ActivePromotionHistory, error)

	// DeleteTeamActiveEnvironment deletes all component in namespace and namespace object
	DeleteTeamActiveEnvironment(teamName, namespace string) error
}

type Connection struct {
	Name          string `json:"name"`
	URL           string `json:"url"`
	IP            string `json:"ip"`
	Port          string `json:"port"`
	ServicePort   string `json:"servicePort"`
	Type          string `json:"type"`
	ContainerPort string `json:"containerPort"`
}

type ActivePromotionController interface {
}

type StableComponentController interface {
}

// GitInfo represents git repo, branch info. for process the update
type GitInfo struct {
	Name         string
	FullName     string
	BranchName   string
	HeadRevision string
}

// PostNamespaceCreation represents a struct for running post namespace creation
type PostNamespaceCreation struct {
	// Namespace defines a creating namespace
	Namespace string     `json:"namespace"`
	Team      s2hv1.Team `json:"team"`
	SamsahaiConfig
}

// GenStagingNamespace returns the name of staging namespace by team name
func GenStagingNamespace(teamName string) string {
	return AppPrefix + teamName
}

// GenPullRequestComponentName generates PullRequest object name from component and pull request number
func GenPullRequestComponentName(component, prNumber string) string {
	return fmt.Sprintf("%s-%s", component, prNumber)
}

// PullRequestData defines a pull request data for template rendering
type PullRequestData struct {
	// PRNumber defines a pull request number
	PRNumber string
}

// GenConfigHashID generates config hash from Config status.used
func GenConfigHashID(configStatus s2hv1.ConfigStatus) string {
	configUsed := configStatus.Used
	bytesConfigComp, _ := json.Marshal(&configUsed)
	bytesHashID := md5.Sum(bytesConfigComp)
	hashID := fmt.Sprintf("%x", bytesHashID)

	return hashID
}

// GenTeamHashID generates team hash from Team status.used
func GenTeamHashID(teamStatus s2hv1.TeamStatus) string {
	teamUsed := teamStatus.Used
	bytesTeamComp, _ := json.Marshal(teamUsed)
	bytesHashID := md5.Sum(bytesTeamComp)
	hashID := fmt.Sprintf("%x", bytesHashID)

	return hashID
}
