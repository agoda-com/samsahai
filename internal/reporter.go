package internal

import (
	"os"
	"strings"

	s2hv1beta1 "github.com/agoda-com/samsahai/api/v1beta1"
	"github.com/agoda-com/samsahai/pkg/samsahai/rpc"
)

// EventType represents an event type of reporter
type EventType string

const (
	ComponentUpgradeType   EventType = "ComponentUpgrade"
	ActivePromotionType    EventType = "ActivePromotion"
	ImageMissingType       EventType = "ImageMissing"
	PullRequestTriggerType EventType = "PullRequestTrigger"
	PullRequestQueueType   EventType = "PullRequestQueue"
)

// ComponentUpgradeOption allows specifying various configuration
type ComponentUpgradeOption func(*ComponentUpgradeReporter)

// WithTestRunner specifies test runner to override when creating component upgrade reporter object
func WithTestRunner(tr s2hv1beta1.TestRunner) ComponentUpgradeOption {
	return func(c *ComponentUpgradeReporter) {
		c.TestRunner = tr
	}
}

// WithQueueHistoryName specifies queuehistory name to override when creating component upgrade reporter object
// QueueHistoryName will be the latest failure of component upgrade
// if reverification is success, QueueHistoryName will be the history of queue before running reverification
func WithQueueHistoryName(qHist string) ComponentUpgradeOption {
	return func(c *ComponentUpgradeReporter) {
		c.QueueHistoryName = qHist
	}
}

// WithNamespace specifies namespace to override when creating component upgrade reporter object
func WithNamespace(ns string) ComponentUpgradeOption {
	return func(c *ComponentUpgradeReporter) {
		if ns != "" {
			c.Namespace = ns
		}
	}
}

// WithComponentUpgradeOptCredential specifies credential to override when create component upgrade reporter object
func WithComponentUpgradeOptCredential(creds s2hv1beta1.Credential) ComponentUpgradeOption {
	return func(c *ComponentUpgradeReporter) {
		c.Credential = creds
	}
}

// ComponentUpgradeReporter manages component upgrade report
type ComponentUpgradeReporter struct {
	IssueTypeStr IssueType             `json:"issueTypeStr,omitempty"`
	StatusStr    StatusType            `json:"statusStr,omitempty"`
	StatusInt    int32                 `json:"statusInt,omitempty"`
	TestRunner   s2hv1beta1.TestRunner `json:"testRunner,omitempty"`
	Credential   s2hv1beta1.Credential `json:"credential,omitempty"`
	Envs         map[string]string

	*rpc.ComponentUpgrade
	SamsahaiConfig
}

// NewComponentUpgradeReporter creates component upgrade reporter from rpc object
func NewComponentUpgradeReporter(comp *rpc.ComponentUpgrade, s2hConfig SamsahaiConfig, opts ...ComponentUpgradeOption) *ComponentUpgradeReporter {
	c := &ComponentUpgradeReporter{
		ComponentUpgrade: comp,
		SamsahaiConfig:   s2hConfig,
		IssueTypeStr:     convertIssueType(comp.IssueType),
		StatusStr:        convertStatusType(comp.Status),
		StatusInt:        int32(comp.Status),
		Envs:             listEnv(),
	}

	// apply the new options
	for _, opt := range opts {
		opt(c)
	}

	return c
}

// StatusType represents an active promotion type
type StatusType string

const (
	StatusSuccess StatusType = "Success"
	StatusFailure StatusType = "Failure"
)

// IssueType represents an issue type of component upgrade failure
type IssueType string

const (
	IssueUnknown              IssueType = "Unknown issue"
	IssueDesiredVersionFailed IssueType = "Desired component failed"
	IssueImageMissing         IssueType = "Image missing"
	IssueEnvironment          IssueType = "Environment issue - Verification failed"
)

// ActivePromotionOption allows specifying various configuration
type ActivePromotionOption func(*ActivePromotionReporter)

// TODO: should override tc credential per team
// WithActivePromotionOptCredential specifies credential to override when create active promotion reporter object
func WithActivePromotionOptCredential(creds s2hv1beta1.Credential) ActivePromotionOption {
	return func(c *ActivePromotionReporter) {
		c.Credential = creds
	}
}

// ActivePromotionReporter manages active promotion report
type ActivePromotionReporter struct {
	TeamName               string                `json:"teamName,omitempty"`
	CurrentActiveNamespace string                `json:"currentActiveNamespace,omitempty"`
	Runs                   int                   `json:"runs,omitempty"`
	Credential             s2hv1beta1.Credential `json:"credential,omitempty"`
	Envs                   map[string]string
	s2hv1beta1.ActivePromotionStatus
	SamsahaiConfig
}

// NewActivePromotionReporter creates active promotion reporter object
func NewActivePromotionReporter(status s2hv1beta1.ActivePromotionStatus, s2hConfig SamsahaiConfig,
	teamName, currentNs string, runs int, opts ...ActivePromotionOption) *ActivePromotionReporter {
	c := &ActivePromotionReporter{
		SamsahaiConfig:         s2hConfig,
		TeamName:               teamName,
		CurrentActiveNamespace: currentNs,
		Runs:                   runs,
		ActivePromotionStatus:  status,
		Envs:                   listEnv(),
	}

	// apply the new options
	for _, opt := range opts {
		opt(c)
	}

	return c
}

// ImageMissingReporter manages image missing report
type ImageMissingReporter struct {
	TeamName      string `json:"teamName,omitempty"`
	ComponentName string `json:"componentName,omitempty"`
	// Reason represents error reason
	Reason        string `json:"reason,omitempty"`
	Envs          map[string]string
	s2hv1beta1.Image
	SamsahaiConfig
}

// NewImageMissingReporter creates image missing reporter object
func NewImageMissingReporter(image s2hv1beta1.Image, s2hConfig SamsahaiConfig,
	teamName, compName, reason string) *ImageMissingReporter {

	c := &ImageMissingReporter{
		SamsahaiConfig: s2hConfig,
		TeamName:       teamName,
		ComponentName:  compName,
		Image:          image,
		Envs:           listEnv(),
		Reason:         reason,
	}

	return c
}

// PullRequestTriggerReporter manages pull request trigger report
type PullRequestTriggerReporter struct {
	TeamName      string            `json:"teamName,omitempty"`
	ComponentName string            `json:"componentName,omitempty"`
	PRNumber      string            `json:"prNumber,omitempty"`
	Result        string            `json:"result,omitempty"`
	Image         *s2hv1beta1.Image `json:"image,omitempty"`
	s2hv1beta1.PullRequestTriggerStatus
	SamsahaiConfig
}

// NewPullRequestTriggerResultReporter creates pull request trigger result reporter object
func NewPullRequestTriggerResultReporter(status s2hv1beta1.PullRequestTriggerStatus, s2hConfig SamsahaiConfig,
	teamName, compName, prNumber, result string, image *s2hv1beta1.Image) *PullRequestTriggerReporter {

	c := &PullRequestTriggerReporter{
		PullRequestTriggerStatus: status,
		TeamName:                 teamName,
		ComponentName:            compName,
		PRNumber:                 prNumber,
		Result:                   result,
		Image:                    image,
		SamsahaiConfig:           s2hConfig,
	}

	return c
}

func convertIssueType(issueType rpc.ComponentUpgrade_IssueType) IssueType {
	switch issueType {
	case rpc.ComponentUpgrade_IssueType_DESIRED_VERSION_FAILED:
		return IssueDesiredVersionFailed
	case rpc.ComponentUpgrade_IssueType_ENVIRONMENT_ISSUE:
		return IssueEnvironment
	case rpc.ComponentUpgrade_IssueType_IMAGE_MISSING:
		return IssueImageMissing
	default:
		return IssueUnknown
	}
}

func convertStatusType(statusType rpc.ComponentUpgrade_UpgradeStatus) StatusType {
	switch statusType {
	case rpc.ComponentUpgrade_UpgradeStatus_SUCCESS:
		return StatusSuccess
	default:
		return StatusFailure
	}
}

func listEnv() map[string]string {
	env := make(map[string]string)
	for _, setting := range os.Environ() {
		pair := strings.SplitN(setting, "=", 2)
		env[pair[0]] = pair[1]
	}

	return env
}

// Reporter is the interface of reporter
type Reporter interface {
	// GetName returns type of reporter
	GetName() string

	// SendComponentUpgrade sends details of component upgrade
	SendComponentUpgrade(configCtrl ConfigController, comp *ComponentUpgradeReporter) error

	// SendPullRequestQueue sends details of pull request deployment queue
	SendPullRequestQueue(configCtrl ConfigController, comp *ComponentUpgradeReporter) error

	// SendActivePromotionStatus sends active promotion status
	SendActivePromotionStatus(configCtrl ConfigController, atpRpt *ActivePromotionReporter) error

	// SendImageMissing sends image missing
	SendImageMissing(configCtrl ConfigController, imageMissingRpt *ImageMissingReporter) error

	// SendPullRequestTriggerResult sends pull request trigger result information
	SendPullRequestTriggerResult(configCtrl ConfigController, prTriggerRpt *PullRequestTriggerReporter) error
}
