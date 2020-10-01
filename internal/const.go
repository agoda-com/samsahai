package internal

import (
	s2hv1beta1 "github.com/agoda-com/samsahai/api/v1beta1"
)

const (
	// URIHealthz represents URI for health check
	URIHealthz = "/healthz"
	URIVersion = "/version"
	//URIGetTeamConfiguration = "/team/%s/config"
	//URIConfig               = "/config"

	//DefaultHTTPRequestTimeout = 10 * time.Second

	//SamsahaiCtrlName           = "samsahai-ctrl"
	SamsahaiAuthHeader  = "x-samsahai-auth"
	SamsahaiDefaultPort = "8080"
	//SamsahaiDefaultServiceName = "samsahai"

	StagingCtrlName    = "s2h-staging-ctrl"
	StagingDefaultPort = 8090

	// Viper keys
	VKDebug                           = "debug"
	VKServerHTTPPort                  = "port"
	VKMetricHTTPPort                  = "metric-port"
	VKPodNamespace                    = "pod-namespace"
	VKS2HConfigPath                   = "s2h-config-path"
	VKClusterDomain                   = "cluster-domain"
	VKS2HTeamName                     = "s2h-team-name"
	VKS2HAuthToken                    = "s2h-auth-token"
	VKS2HServerURL                    = "s2h-server-url"
	VKS2HServiceName                  = "s2h-service-name"
	VKS2HServiceScheme                = "s2h-service-scheme"
	VKS2HImage                        = "s2h-image"
	VKS2HExternalURL                  = "s2h-external-url"
	VKTeamcityURL                     = "teamcity-url"
	VKTeamcityUsername                = "teamcity-username"
	VKTeamcityPassword                = "teamcity-password"
	VKSlackToken                      = "slack-token"
	VKMSTeamsTenantID                 = "ms-teams-tenant-id"
	VKMSTeamsClientID                 = "ms-teams-client-id"
	VKMSTeamsClientSecret             = "ms-teams-client-secret"
	VKMSTeamsUsername                 = "ms-teams-username"
	VKMSTeamsPassword                 = "ms-teams-password"
	VKActivePromotionConcurrences     = "active-promotion-concurrences"
	VKActivePromotionTimeout          = "active-promotion-timeout"
	VKActivePromotionDemotionTimeout  = "active-demotion-timeout"
	VKActivePromotionRollbackTimeout  = "active-promotion-rollback-timeout"
	VKActivePromotionTearDownDuration = "active-promotion-teardown-duration"
	VKActivePromotionMaxHistories     = "active-promotion-max-histories"
	VKActivePromotionOnTeamCreation   = "active-promotion-on-team-creation"
	VKQueueMaxHistoryDays             = "queue-max-history-days"
	VKPRQueueConcurrences             = "pr-queue-concurrences"
	VKPRVerificationMaxRetryCounts    = "pr-verification-max-retry-counts"
	VKPRTriggerMaxRetryCounts         = "pr-trigger-max-retry-counts"
	VKPRTriggerPollingTime            = "pr-trigger-polling-time"
	VKPullRequestQueueMaxHistoryDays  = "pr-queue-max-history-days"
)

type ConfigurationJSON struct {
	GitRevision   string                 `json:"gitRevision"`
	Configuration *s2hv1beta1.ConfigSpec `json:"config"`
}
