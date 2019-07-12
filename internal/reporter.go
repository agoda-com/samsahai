package internal

import (
	s2hv1beta1 "github.com/agoda-com/samsahai/pkg/apis/env/v1beta1"
	"github.com/agoda-com/samsahai/pkg/samsahai/rpc"
)

// EventType represents an event type of reporter
type EventType string

const (
	ComponentUpgradeType EventType = "ComponentUpgrade"
	ActivePromotionType  EventType = "ActivePromotion"
	ImageMissingType     EventType = "ImageMissing"
)

// ComponentUpgradeOption allows specifying various configuration
type ComponentUpgradeOption func(*ComponentUpgradeReporter)

// WithTestRunner specifies test runner to override when create component upgrade reporter object
func WithTestRunner(tr s2hv1beta1.TestRunner) ComponentUpgradeOption {
	return func(c *ComponentUpgradeReporter) {
		c.TestRunner = tr
	}
}

// WithIsReverify specifies isReverify to override when create component upgrade reporter object
func WithIsReverify(isReverify bool) ComponentUpgradeOption {
	return func(c *ComponentUpgradeReporter) {
		c.IsReverify = isReverify
	}
}

// WithIsBuildSuccess specifies isBuildSuccess to override when create component upgrade reporter object
func WithIsBuildSuccess(isBuildSuccess bool) ComponentUpgradeOption {
	return func(c *ComponentUpgradeReporter) {
		c.IsBuildSuccess = isBuildSuccess
	}
}

// WithQueueHistoryName specifies queuehistory name to override when create component upgrade reporter object
func WithQueueHistoryName(qHist string) ComponentUpgradeOption {
	return func(c *ComponentUpgradeReporter) {
		c.QueueHistoryName = qHist
	}
}

// ComponentUpgradeReporter manages component upgrade report
type ComponentUpgradeReporter struct {
	IssueTypeStr   IssueType             `json:"issueTypeStr,omitempty"`
	StatusStr      StatusType            `json:"statusStr,omitempty"`
	StatusInt      int32                 `json:"statusInt,omitempty"`
	IsReverify     bool                  `json:"isReverify,omitempty"`
	IsBuildSuccess bool                  `json:"isBuildSuccess,omitempty"`
	TestRunner     s2hv1beta1.TestRunner `json:"testRunner,omitempty"`
	Credential     s2hv1beta1.Credential `json:"credential,omitempty"`
	rpc.ComponentUpgrade
	SamsahaiConfig
}

// NewComponentUpgradeReporter creates component upgrade reporter from rpc object
func NewComponentUpgradeReporter(comp *rpc.ComponentUpgrade, s2hConfig SamsahaiConfig, opts ...ComponentUpgradeOption) *ComponentUpgradeReporter {
	c := &ComponentUpgradeReporter{
		ComponentUpgrade: *comp,
		SamsahaiConfig:   s2hConfig,
		IssueTypeStr:     convertIssuetype(comp.IssueType),
		StatusStr:        convertStatusType(comp.Status),
		StatusInt:        int32(comp.Status),
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
	IssueDesiredVersionFailed IssueType = "Desired component failed - Please check your test"
	IssueImageMissing         IssueType = "Image missing"
	IssueEnvironment          IssueType = "Environment issue - Verification failed"
)

// ActivePromotionOption allows specifying various configuration
type ActivePromotionOption func(*ActivePromotionReporter)

// WithCredential specifies credential to override when create active promotion reporter object
func WithCredential(creds s2hv1beta1.Credential) ActivePromotionOption {
	return func(c *ActivePromotionReporter) {
		c.Credential = creds
	}
}

// ActivePromotionReporter manages active promotion report
type ActivePromotionReporter struct {
	TeamName               string                `json:"teamName,omitempty"`
	CurrentActiveNamespace string                `json:"currentActiveNamespace,omitempty"`
	Credential             s2hv1beta1.Credential `json:"credential,omitempty"`
	s2hv1beta1.ActivePromotionStatus
	SamsahaiConfig
}

// NewActivePromotionReporter creates active promotion reporter object
func NewActivePromotionReporter(status *s2hv1beta1.ActivePromotionStatus, s2hConfig SamsahaiConfig, teamName, currentNs string, opts ...ActivePromotionOption) *ActivePromotionReporter {
	c := &ActivePromotionReporter{
		SamsahaiConfig:         s2hConfig,
		TeamName:               teamName,
		CurrentActiveNamespace: currentNs,
		ActivePromotionStatus:  *status,
	}

	// apply the new options
	for _, opt := range opts {
		opt(c)
	}

	return c
}

// Reporter is the interface of reporter
type Reporter interface {
	// GetName returns type of reporter
	GetName() string

	// SendComponentUpgrade sends details of component upgrade failure
	SendComponentUpgrade(configMgr ConfigManager, comp *ComponentUpgradeReporter) error

	// SendActivePromotionStatus sends active promotion status
	SendActivePromotionStatus(configMgr ConfigManager, atpRpt *ActivePromotionReporter) error

	// SendImageMissing sends image missing
	SendImageMissing(configMgr ConfigManager, images *rpc.Image) error
}

func convertIssuetype(issueType rpc.ComponentUpgrade_IssueType) IssueType {
	switch issueType {
	case rpc.ComponentUpgrade_DESIRED_VERSION_FAILED:
		return IssueDesiredVersionFailed
	case rpc.ComponentUpgrade_ENVIRONMENT_ISSUE:
		return IssueEnvironment
	case rpc.ComponentUpgrade_IMAGE_MISSING:
		return IssueImageMissing
	default:
		return IssueUnknown
	}
}

func convertStatusType(statusType rpc.ComponentUpgrade_UpgradeStatus) StatusType {
	switch statusType {
	case rpc.ComponentUpgrade_SUCCESS:
		return StatusSuccess
	default:
		return StatusFailure
	}
}
