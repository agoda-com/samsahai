package internal

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	s2hv1beta1 "github.com/agoda-com/samsahai/pkg/apis/env/v1beta1"
)

const (
	EnvBase      = "base"
	EnvStaging   = "staging"
	EnvPreActive = "pre-active"
	EnvActive    = "active"
	EnvDeActive  = "de-active"
	//EnvTmp       = "tmp"
)

type Configuration struct {
	// Components defines all components that are managed
	Components []*Component `json:"components" yaml:"components"`

	// Staging represents configuration about staging
	Staging *ConfigStaging `json:"staging"  yaml:"staging"`

	// ActivePromotion represents configuration about active promotion
	ActivePromotion *ConfigActivePromotion `json:"activePromotion"  yaml:"activePromotion"`

	// Envs represents environment specific configuration
	Envs map[string]ComponentsValues `json:"envs,omitempty" yaml:"envs,omitempty"`

	// Reporter represents configuration about reporter
	Reporter *ConfigReporter `json:"report" yaml:"report"`
}

// ConfigActivePromotion represents configuration about active promotion
type ConfigActivePromotion struct {
	// Timeout defines maximum duration for doing active promotion
	Timeout metav1.Duration `json:"timeout,omitempty" yaml:"timeout"`

	// DemotionTimeout defines maximum duration for doing active demotion
	DemotionTimeout metav1.Duration `json:"demotionTimeout,omitempty" yaml:"demotionTimeout"`

	// RollbackTimeout defines maximum duration for rolling back active promotion
	RollbackTimeout metav1.Duration `json:"rollbackTimeout,omitempty" yaml:"rollbackTimeout"`

	// MaxHistories defines maximum length of ActivePromotionHistory stored per team
	MaxHistories int `json:"maxHistories,omitempty" yaml:"maxHistories"`

	// TearDownDuration defines duration before teardown the previous active namespace
	TearDownDuration metav1.Duration `json:"tearDownDuration,omitempty" yaml:"tearDownDuration"`

	OutdatedNotification *OutdatedNotification `json:"outdatedNotification,omitempty" yaml:"outdatedNotification"`

	// Deployment represents configuration about deploy
	Deployment *ConfigDeploy `json:"deployment" yaml:"deployment"`
}

type ConfigDeploy struct {
	// Timeout defines maximum duration for deploying environment
	Timeout metav1.Duration `json:"timeout,omitempty" yaml:"timeout"`

	// ComponentCleanupTimeout defines timeout duration of component cleaning up
	ComponentCleanupTimeout metav1.Duration `json:"componentCleanupTimeout"`

	// Engine defines method of deploying
	//
	// mock - for test only, always return success
	//
	// flux-helm - create HelmRelease for Helm Operator from Flux
	Engine *string `json:"engine,omitempty" yaml:"engine"`

	// TestRunner represents configuration about test
	TestRunner *ConfigTestRunner `json:"testRunner" yaml:"testRunner"`
}

// ConfigStaging represents configuration about staging
type ConfigStaging struct {
	// Deployment represents configuration about deploy
	Deployment *ConfigDeploy `json:"deployment" yaml:"deployment"`

	// MaxRetry defines max retry counts of component upgrade
	MaxRetry int `json:"maxRetry,omitempty" yaml:"maxRetry"`
}

// ConfigTestRunner represents configuration about how to test the environment
type ConfigTestRunner struct {
	Timeout     metav1.Duration `json:"timeout,omitempty" yaml:"timeout"`
	PollingTime metav1.Duration `json:"pollingTime,omitempty" yaml:"pollingTime"`
	Teamcity    *Teamcity       `json:"teamcity,omitempty" yaml:"teamcity"`
	TestMock    *TestMock       `json:"testMock,omitempty" yaml:"testMock"`
}

// ConfigReporter represents configuration about sending notification
type ConfigReporter struct {
	Optional   []ReportOption `json:"optionals,omitempty" yaml:"optionals"`
	Slack      *Slack         `json:"slack,omitempty" yaml:"slack"`
	Email      *Email         `json:"email,omitempty" yaml:"email"`
	Rest       *Rest          `json:"rest,omitempty" yaml:"rest"`
	Shell      *Shell         `json:"cmd,omitempty" yaml:"cmd"`
	ReportMock bool           `json:"reportMock,omitempty" yaml:"reportMock"`
}

type ConfigManager interface {
	// Sync keeps config synchronized with storage layer
	Sync() error

	// Get returns configuration from memory
	Get() *Configuration

	// GetComponents returns all components from `Configuration` that has valid `Source`
	GetComponents() map[string]*Component

	// GetParentComponents returns components that doesn't have parent (nil Parent)
	GetParentComponents() map[string]*Component

	// GetGitLatestRevision returns a git revision of HEAD commit
	GetGitLatestRevision() string

	// GetGitInfo returns git information of current config
	GetGitInfo() GitInfo

	// HasGitChanges verifies changes on git storage configuration in team
	HasGitChanges(gitStorage s2hv1beta1.GitStorage) bool

	// GetGitConfigPath return git config path
	GetGitConfigPath() string

	// Load loads configuration into ConfigManager
	Load(config *Configuration, gitRev string)

	// Clean cleans up config manager e.g. delete a directory
	Clean() error
}

// Teamcity defines a http rest configuration of teamcity
type Teamcity struct {
	BuildTypeID string `json:"buildTypeID" yaml:"buildTypeID"`
	Branch      string `json:"branch" yaml:"branch"`
}

// TestMock defines a result of testmock
type TestMock struct {
	Result bool `json:"result" yaml:"result"`
}

// ReportOption defines an optional configuration of slack
type ReportOption struct {
	Key   string `json:"key" yaml:"key"`
	Value string `json:"value" yaml:"value"`
}

// Slack defines a configuration of slack
type Slack struct {
	Channels []string `json:"channels" yaml:"channels"`
}

// Email defines a configuration of email
type Email struct {
	Server string   `json:"server" yaml:"server"`
	Port   int      `json:"port" yaml:"port"`
	From   string   `json:"from" yaml:"from"`
	To     []string `json:"to" yaml:"to"`
}

// Rest defines a configuration of http rest
type Rest struct {
	ComponentUpgrade *struct {
		Endpoints []*Endpoint `json:"endpoints" yaml:"endpoints"`
	} `json:"componentUpgrade" yaml:"componentUpgrade"`
	ActivePromotion *struct {
		Endpoints []*Endpoint `json:"endpoints" yaml:"endpoints"`
	} `json:"activePromotion" yaml:"activePromotion"`
	ImageMissing *struct {
		Endpoints []*Endpoint `json:"endpoints" yaml:"endpoints"`
	} `json:"imageMissing" yaml:"imageMissing"`
}

// Shell defines a configuration of shell command
type Shell struct {
	ComponentUpgrade *CommandAndArgs `json:"componentUpgrade" yaml:"componentUpgrade"`
	ActivePromotion  *CommandAndArgs `json:"activePromotion" yaml:"activePromotion"`
	ImageMissing     *CommandAndArgs `json:"imageMissing" yaml:"imageMissing"`
}

// CommandAndArgs defines commands and args
type CommandAndArgs struct {
	Command []string `json:"command" yaml:"command"`
	Args    []string `json:"args" yaml:"args"`
}

// Endpoint defines a configuration of rest endpoint
type Endpoint struct {
	URL      string `json:"url" yaml:"url"`
	Template string `json:"template" yaml:"template"`
	// TODO: auth
}

// OutdatedNotification defines a configuration of outdated notification
type OutdatedNotification struct {
	ExceedDuration            metav1.Duration `json:"exceedDuration" yaml:"exceedDuration"`
	ExcludeWeekendCalculation bool            `json:"excludeWeekendCalculation" yaml:"excludeWeekendCalculation"`
}
