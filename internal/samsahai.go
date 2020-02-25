package internal

import (
	"net/http"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	s2hv1beta1 "github.com/agoda-com/samsahai/api/v1beta1"
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
	TeamcityUsername  string
	TeamcityPassword  string
}

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

	// TeamcityURL defines a Teamcity url
	TeamcityURL string `json:"teamcityURL" yaml:"teamcityURL"`

	// ClusterDomain defines a cluster domain name
	ClusterDomain string `json:"clusterDomain" yaml:"clusterDomain"`

	// ActivePromotion defines an active promotion configuration
	ActivePromotion ActivePromotionConfig `json:"activePromotion,omitempty" yaml:"activePromotion,omitempty"`

	// PostNamespaceCreation defines commands executing after creating s2h namespace
	PostNamespaceCreation *struct {
		CommandAndArgs
	} `json:"postNamespaceCreation,omitempty" yaml:"postNamespaceCreation,omitempty"`

	SamsahaiURL        string             `json:"-" yaml:"-"`
	SamsahaiCredential SamsahaiCredential `json:"-" yaml:"-"`
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

	// MaxHistories defines max stored histories of active promotion
	MaxHistories int `json:"maxHistories" yaml:"maxHistories"`
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
	GetTeam(teamName string, teamComp *s2hv1beta1.Team) error

	// GetTeamConfigManagers returns Samsahai configuration from all teams
	GetTeamConfigManagers() map[string]ConfigManager

	// GetTeamConfigManager returns samsahai configuration from team's github
	GetTeamConfigManager(teamName string) (ConfigManager, bool)

	// GetPlugins returns samsahai plugins
	GetPlugins() map[string]Plugin

	// LoadTeamSecret loads team secret from main namespace
	LoadTeamSecret(teamComp *s2hv1beta1.Team) error

	// CreateStagingEnvironment creates staging environment
	CreateStagingEnvironment(teamName, namespaceName string) error

	// CreatePreActiveEnvironment creates pre-active environment
	CreatePreActiveEnvironment(teamName, namespace string) error

	// PromoteActiveEnvironment switches environment from pre-active to active and stores current active components
	PromoteActiveEnvironment(teamComp *s2hv1beta1.Team, namespace string, comps []s2hv1beta1.StableComponent) error

	// DestroyActiveEnvironment destroys active environment when active demotion is failure.
	DestroyActiveEnvironment(teamName, namespace string) error

	// DestroyPreActiveEnvironment destroys pre-active environment when active promotion is failure.
	DestroyPreActiveEnvironment(teamName, namespace string) error

	// DestroyPreviousActiveEnvironment destroys previous active environment when active promotion is success.
	DestroyPreviousActiveEnvironment(teamName, namespace string) error

	// SetPreviousActiveNamespace updates previous active namespace to team status
	SetPreviousActiveNamespace(teamComp *s2hv1beta1.Team, namespace string) error

	// SetPreActiveNamespace updates pre-active namespace to team status
	SetPreActiveNamespace(teamComp *s2hv1beta1.Team, namespace string) error

	// SetActiveNamespace updates active namespace to team status
	SetActiveNamespace(teamComp *s2hv1beta1.Team, namespace string) error

	// NotifyGitChanged adds GitInfo to channel for process
	NotifyGitChanged(updated GitInfo)

	// NotifyComponentChanged adds Component to queue for checking new version
	NotifyComponentChanged(name, repository string)

	// NotifyActivePromotion sends active promotion status report
	NotifyActivePromotion(atpRpt *ActivePromotionReporter) error

	// API

	// GetConnections returns Services in NodePort type and Ingresses that exist in the namespace
	GetConnections(namespace string) (map[string][]Connection, error)

	// GetTeams returns list of teams in Samsahai
	GetTeams() (*s2hv1beta1.TeamList, error)

	// GetTeamNames returns map of team names in Samsahai
	GetTeamNames() map[string]struct{}

	// GetQueueHistories returns QueueHistoryList of the namespace
	GetQueueHistories(namespace string) (*s2hv1beta1.QueueHistoryList, error)

	// GetQueueHistory returns Queue by name and namespace
	GetQueueHistory(name, namespace string) (*s2hv1beta1.QueueHistory, error)

	// GetQueues returns QueueList of the namespace
	GetQueues(namespace string) (*s2hv1beta1.QueueList, error)

	// GetStableValues returns Stable Values of parent component in team
	GetStableValues(team *s2hv1beta1.Team, comp *Component) (ComponentValues, error)

	// GetActivePromotions returns ActivePromotionList by labels
	GetActivePromotions() (*s2hv1beta1.ActivePromotionList, error)

	// GetActivePromotion returns ActivePromotion by name
	GetActivePromotion(name string) (v *s2hv1beta1.ActivePromotion, err error)

	// GetActivePromotionHistories returns ActivePromotionList by labels
	GetActivePromotionHistories(selectors map[string]string) (*s2hv1beta1.ActivePromotionHistoryList, error)

	// GetActivePromotionHistory returns ActivePromotion by name
	GetActivePromotionHistory(name string) (*s2hv1beta1.ActivePromotionHistory, error)
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
	Namespace string          `json:"namespace"`
	Team      s2hv1beta1.Team `json:"team"`
	SamsahaiConfig
}

// GenStagingNamespace returns the name of staging namespace by team name
func GenStagingNamespace(teamName string) string {
	return AppPrefix + teamName
}
