package msteams_test

import (
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	s2hv1beta1 "github.com/agoda-com/samsahai/api/v1beta1"
	"github.com/agoda-com/samsahai/internal"
	s2hmsteams "github.com/agoda-com/samsahai/internal/reporter/msteams"
	"github.com/agoda-com/samsahai/internal/util/msteams"
	"github.com/agoda-com/samsahai/internal/util/unittest"
	"github.com/agoda-com/samsahai/pkg/samsahai/rpc"
)

func TestUnit(t *testing.T) {
	unittest.InitGinkgo(t, "MS Teams Reporter")
}

var _ = Describe("send ms teams message", func() {
	g := NewGomegaWithT(GinkgoT())

	Describe("send component upgrade", func() {
		It("should correctly send component upgrade failure with everytime interval", func() {
			configCtrl := newMockConfigCtrl("", s2hv1beta1.IntervalEveryTime, "")
			g.Expect(configCtrl).ShouldNot(BeNil())

			rpcComp := &rpc.ComponentUpgrade{
				Name:   "comp1",
				Status: rpc.ComponentUpgrade_UpgradeStatus_FAILURE,
				Components: []*rpc.Component{
					{
						Name:  "comp1",
						Image: &rpc.Image{Repository: "image-1", Tag: "1.1.0"},
					},
				},
				TeamName:         "owner",
				IssueType:        rpc.ComponentUpgrade_IssueType_DESIRED_VERSION_FAILED,
				Namespace:        "owner-staging",
				QueueHistoryName: "comp1-1234",
				IsReverify:       false,
				Runs:             2,
				DeploymentIssues: []*rpc.DeploymentIssue{
					{
						IssueType: string(s2hv1beta1.DeploymentIssueCrashLoopBackOff),
						FailureComponents: []*rpc.FailureComponent{
							{ComponentName: "comp1"},
						},
					},
				},
			}
			mockMSTeamsCli := &mockMSTeams{}
			r := s2hmsteams.New("tenantID", "clientID", "clientSecret", "user",
				"pass", s2hmsteams.WithMSTeamsClient(mockMSTeamsCli))
			testRunner := s2hv1beta1.TestRunner{Teamcity: s2hv1beta1.Teamcity{BuildURL: "teamcity-url", BuildNumber: "teamcity-build-number"}}
			comp := internal.NewComponentUpgradeReporter(
				rpcComp,
				internal.SamsahaiConfig{SamsahaiExternalURL: "http://localhost:8080"},
				internal.WithTestRunner(testRunner),
				internal.WithQueueHistoryName("comp1-5678"),
			)
			err := r.SendComponentUpgrade(configCtrl, comp)
			g.Expect(err).Should(BeNil())
			g.Expect(mockMSTeamsCli.accessTokenCalls).Should(Equal(1))
			g.Expect(mockMSTeamsCli.getGroupIDCalls).Should(Equal(2))
			g.Expect(mockMSTeamsCli.getChannelIDCalls).Should(Equal(3))
			g.Expect(mockMSTeamsCli.postMessageCalls).Should(Equal(3))
			g.Expect(mockMSTeamsCli.channels).Should(Equal([]string{"chan1-1", "chan1-2", "chan2-1"}))
			g.Expect(mockMSTeamsCli.message).Should(ContainSubstring("Component Upgrade"))
			g.Expect(mockMSTeamsCli.message).Should(ContainSubstring("Failure"))
			// Should contain information
			g.Expect(mockMSTeamsCli.message).Should(ContainSubstring("#2"))
			g.Expect(mockMSTeamsCli.message).Should(ContainSubstring("comp1"))
			g.Expect(mockMSTeamsCli.message).Should(ContainSubstring("1.1.0"))
			g.Expect(mockMSTeamsCli.message).Should(ContainSubstring("image-1"))
			g.Expect(mockMSTeamsCli.message).Should(ContainSubstring("Desired component failed"))
			g.Expect(mockMSTeamsCli.message).Should(ContainSubstring(`<a href="teamcity-url">#teamcity-build-number</a>`))
			g.Expect(mockMSTeamsCli.message).Should(ContainSubstring(`<a href="http://localhost:8080/teams/owner/queue/histories/comp1-5678/log">Download here</a>`))
			g.Expect(mockMSTeamsCli.message).Should(ContainSubstring("owner"))
			g.Expect(mockMSTeamsCli.message).Should(ContainSubstring("owner-staging"))
			g.Expect(mockMSTeamsCli.message).Should(ContainSubstring(`<a href="http://localhost:8080/teams/owner/queue/histories/comp1-5678">Click here</a>`))
			g.Expect(mockMSTeamsCli.message).Should(ContainSubstring("<b>- Issue type:</b> CrashLoopBackOff"))
			g.Expect(mockMSTeamsCli.message).Should(ContainSubstring("<b>&nbsp;&nbsp;Components:</b> comp1"))
			g.Expect(mockMSTeamsCli.message).ShouldNot(ContainSubstring("Image Missing List"))
		})

		It("should not send component upgrade failure with retry interval", func() {
			configCtrl := newMockConfigCtrl("", "", "")
			g.Expect(configCtrl).ShouldNot(BeNil())

			rpcComp := &rpc.ComponentUpgrade{
				Name:       "comp1",
				Status:     rpc.ComponentUpgrade_UpgradeStatus_FAILURE,
				IsReverify: false,
			}
			mockMSTeamsCli := &mockMSTeams{}
			r := s2hmsteams.New("tenantID", "clientID", "clientSecret", "user",
				"pass", s2hmsteams.WithMSTeamsClient(mockMSTeamsCli))
			comp := internal.NewComponentUpgradeReporter(rpcComp, internal.SamsahaiConfig{})
			err := r.SendComponentUpgrade(configCtrl, comp)
			g.Expect(err).Should(BeNil())
			g.Expect(mockMSTeamsCli.postMessageCalls).Should(Equal(0))
		})

		It("should not send component upgrade failure with success criteria", func() {
			configCtrl := newMockConfigCtrl("", s2hv1beta1.IntervalEveryTime, s2hv1beta1.CriteriaSuccess)
			g.Expect(configCtrl).ShouldNot(BeNil())

			rpcComp := &rpc.ComponentUpgrade{
				Name:   "comp1",
				Status: rpc.ComponentUpgrade_UpgradeStatus_FAILURE,
			}
			mockMSTeamsCli := &mockMSTeams{}
			r := s2hmsteams.New("tenantID", "clientID", "clientSecret", "user",
				"pass", s2hmsteams.WithMSTeamsClient(mockMSTeamsCli))
			comp := internal.NewComponentUpgradeReporter(rpcComp, internal.SamsahaiConfig{})
			err := r.SendComponentUpgrade(configCtrl, comp)
			g.Expect(err).Should(BeNil())
			g.Expect(mockMSTeamsCli.accessTokenCalls).Should(Equal(0))
			g.Expect(mockMSTeamsCli.postMessageCalls).Should(Equal(0))
		})

		It("should correctly send component upgrade failure with image missing list message", func() {
			configCtrl := newMockConfigCtrl("", "", "")
			g.Expect(configCtrl).ShouldNot(BeNil())

			rpcComp := &rpc.ComponentUpgrade{
				Name:   "comp1",
				Status: rpc.ComponentUpgrade_UpgradeStatus_FAILURE,
				Components: []*rpc.Component{
					{
						Name:  "comp1",
						Image: &rpc.Image{Repository: "image-1", Tag: "1.1.0"},
					},
				},
				TeamName:  "owner",
				IssueType: rpc.ComponentUpgrade_IssueType_IMAGE_MISSING,
				Namespace: "owner-staging",
				ImageMissingList: []*rpc.Image{
					{Repository: "image-2", Tag: "1.1.0"},
					{Repository: "image-3", Tag: "1.2.0"},
				},
				IsReverify: true,
			}
			mockMSTeamsCli := &mockMSTeams{}
			r := s2hmsteams.New("tenantID", "clientID", "clientSecret", "user",
				"pass", s2hmsteams.WithMSTeamsClient(mockMSTeamsCli))
			comp := internal.NewComponentUpgradeReporter(rpcComp, internal.SamsahaiConfig{})
			err := r.SendComponentUpgrade(configCtrl, comp)
			g.Expect(err).Should(BeNil())
			g.Expect(mockMSTeamsCli.accessTokenCalls).Should(Equal(1))
			g.Expect(mockMSTeamsCli.getGroupIDCalls).Should(Equal(2))
			g.Expect(mockMSTeamsCli.getChannelIDCalls).Should(Equal(3))
			g.Expect(mockMSTeamsCli.postMessageCalls).Should(Equal(3))
			g.Expect(mockMSTeamsCli.message).Should(ContainSubstring("Failure"))
			// Should contain information
			g.Expect(mockMSTeamsCli.message).Should(ContainSubstring("Reverify"))
			g.Expect(mockMSTeamsCli.message).Should(ContainSubstring("Image Missing List"))
			g.Expect(mockMSTeamsCli.message).Should(ContainSubstring("image-2:1.1.0"))
			g.Expect(mockMSTeamsCli.message).Should(ContainSubstring("image-3:1.2.0"))
		})

		It("should send component upgrade failure of multiple components", func() {
			configCtrl := newMockConfigCtrl("", s2hv1beta1.IntervalEveryTime, "")
			g.Expect(configCtrl).ShouldNot(BeNil())

			rpcComp := &rpc.ComponentUpgrade{
				Name:   "group",
				Status: rpc.ComponentUpgrade_UpgradeStatus_FAILURE,
				Components: []*rpc.Component{
					{
						Name:  "comp1",
						Image: &rpc.Image{Repository: "image-1", Tag: "1.1.0"},
					},
					{
						Name:  "comp2",
						Image: &rpc.Image{Repository: "image-2", Tag: "1.1.2"},
					},
				},
			}
			mockMSTeamsCli := &mockMSTeams{}
			r := s2hmsteams.New("tenantID", "clientID", "clientSecret", "user",
				"pass", s2hmsteams.WithMSTeamsClient(mockMSTeamsCli))
			comp := internal.NewComponentUpgradeReporter(rpcComp, internal.SamsahaiConfig{})
			err := r.SendComponentUpgrade(configCtrl, comp)
			g.Expect(err).Should(BeNil())
			g.Expect(mockMSTeamsCli.message).Should(ContainSubstring("Failure"))
			// Should contain information
			g.Expect(mockMSTeamsCli.message).Should(ContainSubstring("comp1"))
			g.Expect(mockMSTeamsCli.message).Should(ContainSubstring("1.1.0"))
			g.Expect(mockMSTeamsCli.message).Should(ContainSubstring("image-1"))
			g.Expect(mockMSTeamsCli.message).Should(ContainSubstring("comp2"))
			g.Expect(mockMSTeamsCli.message).Should(ContainSubstring("1.1.2"))
			g.Expect(mockMSTeamsCli.message).Should(ContainSubstring("image-2"))
		})
	})

	Describe("send pull request queue", func() {
		It("should correctly send pull request queue failure", func() {
			configCtrl := newMockConfigCtrl("", s2hv1beta1.IntervalEveryTime, "")
			g.Expect(configCtrl).ShouldNot(BeNil())

			rpcComp := &rpc.ComponentUpgrade{
				Name:   "comp1",
				Status: rpc.ComponentUpgrade_UpgradeStatus_FAILURE,
				Components: []*rpc.Component{
					{
						Name:  "comp1",
						Image: &rpc.Image{Repository: "image-1", Tag: "1.1.0"},
					},
				},
				TeamName:         "owner",
				IssueType:        rpc.ComponentUpgrade_IssueType_DESIRED_VERSION_FAILED,
				Namespace:        "owner-staging",
				QueueHistoryName: "comp1-1234",
				IsReverify:       false,
				Runs:             2,
				DeploymentIssues: []*rpc.DeploymentIssue{
					{
						IssueType: string(s2hv1beta1.DeploymentIssueCrashLoopBackOff),
						FailureComponents: []*rpc.FailureComponent{
							{ComponentName: "comp1"},
						},
					},
				},
				PullRequestComponent: &rpc.TeamWithPullRequest{
					ComponentName: "pr-comp1",
					PRNumber:      "pr1234",
				},
			}
			mockMSTeamsCli := &mockMSTeams{}
			r := s2hmsteams.New("tenantID", "clientID", "clientSecret", "user",
				"pass", s2hmsteams.WithMSTeamsClient(mockMSTeamsCli))
			testRunner := s2hv1beta1.TestRunner{
				Teamcity: s2hv1beta1.Teamcity{BuildURL: "teamcity-url", BuildNumber: "teamcity-build-number"},
			}
			comp := internal.NewComponentUpgradeReporter(
				rpcComp,
				internal.SamsahaiConfig{SamsahaiExternalURL: "http://localhost:8080"},
				internal.WithTestRunner(testRunner),
				internal.WithQueueHistoryName("comp1-5678"),
				internal.WithNamespace("pr-namespace"),
			)
			err := r.SendPullRequestQueue(configCtrl, comp)
			g.Expect(err).Should(BeNil())
			g.Expect(mockMSTeamsCli.accessTokenCalls).Should(Equal(1))
			g.Expect(mockMSTeamsCli.getGroupIDCalls).Should(Equal(2))
			g.Expect(mockMSTeamsCli.getChannelIDCalls).Should(Equal(3))
			g.Expect(mockMSTeamsCli.postMessageCalls).Should(Equal(3))
			g.Expect(mockMSTeamsCli.channels).Should(Equal([]string{"chan1-1", "chan1-2", "chan2-1"}))
			g.Expect(mockMSTeamsCli.message).Should(ContainSubstring("Pull Request Queue"))
			g.Expect(mockMSTeamsCli.message).Should(ContainSubstring("Failure"))
			// Should contain information
			g.Expect(mockMSTeamsCli.message).Should(ContainSubstring("pr-comp1"))
			g.Expect(mockMSTeamsCli.message).Should(ContainSubstring("pr1234"))
			g.Expect(mockMSTeamsCli.message).Should(ContainSubstring("#2"))
			g.Expect(mockMSTeamsCli.message).Should(ContainSubstring("pr-namespace"))
			g.Expect(mockMSTeamsCli.message).Should(ContainSubstring(
				`<a href="http://localhost:8080/teams/owner/pullrequest/queue/histories/comp1-5678/log">Download here</a>`))
			g.Expect(mockMSTeamsCli.message).Should(ContainSubstring(
				`<a href="http://localhost:8080/teams/owner/pullrequest/queue/histories/comp1-5678">Click here</a>`))
		})
	})

	Describe("send active promotion status", func() {
		It("should correctly send active promotion success with outdated components message", func() {
			configCtrl := newMockConfigCtrl("", "", "")
			g.Expect(configCtrl).ShouldNot(BeNil())

			var comp1, repoComp1, comp2, repoComp2 = "comp1", "repo/comp1", "comp2", "repo/comp2"
			var v110, v112 = "1.1.0", "1.1.2"

			status := s2hv1beta1.ActivePromotionStatus{
				Result:               s2hv1beta1.ActivePromotionSuccess,
				HasOutdatedComponent: true,
				OutdatedComponents: map[string]s2hv1beta1.OutdatedComponent{
					comp1: {
						CurrentImage:     &s2hv1beta1.Image{Repository: repoComp1, Tag: v110},
						DesiredImage:     &s2hv1beta1.Image{Repository: repoComp1, Tag: v112},
						OutdatedDuration: time.Duration(86400000000000), // 1d0h0m
					},
					comp2: {
						CurrentImage:     &s2hv1beta1.Image{Repository: repoComp2, Tag: v110},
						DesiredImage:     &s2hv1beta1.Image{Repository: repoComp2, Tag: v110},
						OutdatedDuration: time.Duration(0),
					},
				},
				PreActiveQueue: s2hv1beta1.QueueStatus{
					TestRunner: s2hv1beta1.TestRunner{
						Teamcity: s2hv1beta1.Teamcity{BuildURL: "teamcity-url", BuildNumber: "teamcity-build-number"},
					},
				},
			}
			atpRpt := internal.NewActivePromotionReporter(status, internal.SamsahaiConfig{}, "owner",
				"owner-123456", 2)

			mockMSTeamsCli := &mockMSTeams{}
			r := s2hmsteams.New("tenantID", "clientID", "clientSecret", "user",
				"pass", s2hmsteams.WithMSTeamsClient(mockMSTeamsCli))
			err := r.SendActivePromotionStatus(configCtrl, atpRpt)
			g.Expect(mockMSTeamsCli.accessTokenCalls).Should(Equal(1))
			g.Expect(mockMSTeamsCli.getGroupIDCalls).Should(Equal(2))
			g.Expect(mockMSTeamsCli.getChannelIDCalls).Should(Equal(3))
			g.Expect(mockMSTeamsCli.postMessageCalls).Should(Equal(3))
			g.Expect(mockMSTeamsCli.channels).Should(Equal([]string{"chan1-1", "chan1-2", "chan2-1"}))
			g.Expect(mockMSTeamsCli.message).Should(ContainSubstring("Success"))
			g.Expect(mockMSTeamsCli.message).Should(ContainSubstring("#2"))
			g.Expect(mockMSTeamsCli.message).Should(ContainSubstring("owner"))
			g.Expect(mockMSTeamsCli.message).Should(ContainSubstring("owner-123456"))
			g.Expect(mockMSTeamsCli.message).Should(ContainSubstring(`<a href="teamcity-url">#teamcity-build-number</a>`))
			g.Expect(mockMSTeamsCli.message).Should(ContainSubstring("Outdated Components"))
			g.Expect(mockMSTeamsCli.message).Should(ContainSubstring("comp1"))
			g.Expect(mockMSTeamsCli.message).Should(ContainSubstring("Not update for 1d 0h 0m"))
			g.Expect(mockMSTeamsCli.message).Should(ContainSubstring(`Current Version: <a href="http://repo/comp1">1.1.0</a>`))
			g.Expect(mockMSTeamsCli.message).Should(ContainSubstring(`Latest Version: <a href="http://repo/comp1">1.1.2</a>`))
			g.Expect(mockMSTeamsCli.message).ShouldNot(ContainSubstring("comp2"))
			g.Expect(err).Should(BeNil())
		})

		It("should correctly send active promotion success without outdated components message", func() {
			configCtrl := newMockConfigCtrl("", "", "")
			g.Expect(configCtrl).ShouldNot(BeNil())

			timeNow := metav1.Now()
			status := s2hv1beta1.ActivePromotionStatus{
				Result:                     s2hv1beta1.ActivePromotionSuccess,
				HasOutdatedComponent:       false,
				ActivePromotionHistoryName: "owner-12345",
				PreviousActiveNamespace:    "owner-prevns",
				DestroyedTime:              &timeNow,
			}
			atpRpt := internal.NewActivePromotionReporter(status,
				internal.SamsahaiConfig{SamsahaiExternalURL: "http://localhost:8080"},
				"owner", "owner-123456", 1)

			mockMSTeamsCli := &mockMSTeams{}
			r := s2hmsteams.New("tenantID", "clientID", "clientSecret", "user",
				"pass", s2hmsteams.WithMSTeamsClient(mockMSTeamsCli))
			err := r.SendActivePromotionStatus(configCtrl, atpRpt)
			g.Expect(mockMSTeamsCli.accessTokenCalls).Should(Equal(1))
			g.Expect(mockMSTeamsCli.getGroupIDCalls).Should(Equal(2))
			g.Expect(mockMSTeamsCli.getChannelIDCalls).Should(Equal(3))
			g.Expect(mockMSTeamsCli.postMessageCalls).Should(Equal(3))
			g.Expect(mockMSTeamsCli.message).Should(ContainSubstring("Success"))
			g.Expect(mockMSTeamsCli.message).Should(ContainSubstring(`<a href="http://localhost:8080/teams/owner/activepromotions/histories/owner-12345">Click here</a>`))
			g.Expect(mockMSTeamsCli.message).Should(ContainSubstring("All components are up to date!"))
			g.Expect(mockMSTeamsCli.message).Should(ContainSubstring("previous active namespace <code>owner-prevns</code> will be destroyed at <code>" + timeNow.Format("2006-01-02 15:04:05 MST</code>")))
			g.Expect(err).Should(BeNil())
		})

		It("should correctly send active promotion failure with outdated components/image missing/deployment issues message",
			func() {
				configCtrl := newMockConfigCtrl("", "", "")
				g.Expect(configCtrl).ShouldNot(BeNil())

				var comp1, repoComp1, comp2, repoComp2 = "comp1", "repo/comp1", "comp2", "repo/comp2"
				var v110, v112 = "1.1.0", "1.1.2"

				status := s2hv1beta1.ActivePromotionStatus{
					Result:               s2hv1beta1.ActivePromotionFailure,
					HasOutdatedComponent: true,
					PreActiveQueue: s2hv1beta1.QueueStatus{
						ImageMissingList: []s2hv1beta1.Image{
							{Repository: "repo1", Tag: "1.xx"},
							{Repository: "repo2", Tag: "2.xx"},
						},
						DeploymentIssues: []s2hv1beta1.DeploymentIssue{
							{
								IssueType: s2hv1beta1.DeploymentIssueWaitForInitContainer,
								FailureComponents: []s2hv1beta1.FailureComponent{
									{
										ComponentName:             "comp1",
										FirstFailureContainerName: "dep1",
									},
								},
							},
						},
					},
					OutdatedComponents: map[string]s2hv1beta1.OutdatedComponent{
						comp1: {
							CurrentImage:     &s2hv1beta1.Image{Repository: repoComp1, Tag: v110},
							DesiredImage:     &s2hv1beta1.Image{Repository: repoComp1, Tag: v112},
							OutdatedDuration: time.Duration(86400000000000), // 1d0h0m
						},
						comp2: {
							CurrentImage:     &s2hv1beta1.Image{Repository: repoComp2, Tag: v110},
							DesiredImage:     &s2hv1beta1.Image{Repository: repoComp2, Tag: v110},
							OutdatedDuration: time.Duration(0),
						},
					},
					ActivePromotionHistoryName: "owner-12345",
				}
				atpRpt := internal.NewActivePromotionReporter(status,
					internal.SamsahaiConfig{SamsahaiExternalURL: "http://localhost:8080"},
					"owner", "owner-123456", 2)

				mockMSTeamsCli := &mockMSTeams{}
				r := s2hmsteams.New("tenantID", "clientID", "clientSecret", "user",
					"pass", s2hmsteams.WithMSTeamsClient(mockMSTeamsCli))
				err := r.SendActivePromotionStatus(configCtrl, atpRpt)
				g.Expect(mockMSTeamsCli.accessTokenCalls).Should(Equal(1))
				g.Expect(mockMSTeamsCli.getGroupIDCalls).Should(Equal(2))
				g.Expect(mockMSTeamsCli.getChannelIDCalls).Should(Equal(3))
				g.Expect(mockMSTeamsCli.postMessageCalls).Should(Equal(3))
				g.Expect(mockMSTeamsCli.channels).Should(Equal([]string{"chan1-1", "chan1-2", "chan2-1"}))
				g.Expect(mockMSTeamsCli.message).Should(ContainSubstring("Failure"))
				g.Expect(mockMSTeamsCli.message).Should(ContainSubstring("#2"))
				g.Expect(mockMSTeamsCli.message).Should(ContainSubstring("owner"))
				g.Expect(mockMSTeamsCli.message).Should(ContainSubstring("owner-123456"))
				g.Expect(mockMSTeamsCli.message).Should(ContainSubstring(`<a href="http://localhost:8080/teams/owner/activepromotions/histories/owner-12345/log">Download here</a>`))
				g.Expect(mockMSTeamsCli.message).Should(ContainSubstring(`<a href="http://localhost:8080/teams/owner/activepromotions/histories/owner-12345">Click here</a>`))
				g.Expect(mockMSTeamsCli.message).Should(ContainSubstring("Image Missing List"))
				g.Expect(mockMSTeamsCli.message).Should(ContainSubstring("repo1:1.xx"))
				g.Expect(mockMSTeamsCli.message).Should(ContainSubstring("repo2:2.xx"))
				g.Expect(mockMSTeamsCli.message).Should(ContainSubstring("Outdated Components"))
				g.Expect(mockMSTeamsCli.message).Should(ContainSubstring("comp1"))
				g.Expect(mockMSTeamsCli.message).Should(ContainSubstring("Not update for 1d 0h 0m"))
				g.Expect(mockMSTeamsCli.message).Should(ContainSubstring(`Current Version: <a href="http://repo/comp1">1.1.0</a>`))
				g.Expect(mockMSTeamsCli.message).Should(ContainSubstring(`Latest Version: <a href="http://repo/comp1">1.1.2</a>`))
				g.Expect(mockMSTeamsCli.message).ShouldNot(ContainSubstring("comp2"))
				g.Expect(mockMSTeamsCli.message).Should(ContainSubstring("<b>- Issue type:</b> WaitForInitContainer"))
				g.Expect(mockMSTeamsCli.message).Should(ContainSubstring("<b>&nbsp;&nbsp;Components:</b> comp1"))
				g.Expect(mockMSTeamsCli.message).Should(ContainSubstring("<b>&nbsp;&nbsp;Wait for:</b> dep1"))
				g.Expect(err).Should(BeNil())
			})

		It("should correctly send active promotion failure without outdated components message", func() {
			configCtrl := newMockConfigCtrl("", "", "")
			g.Expect(configCtrl).ShouldNot(BeNil())

			status := s2hv1beta1.ActivePromotionStatus{
				Result:               s2hv1beta1.ActivePromotionFailure,
				HasOutdatedComponent: false,
			}
			atpRpt := internal.NewActivePromotionReporter(status, internal.SamsahaiConfig{}, "owner",
				"owner-123456", 1)

			mockMSTeamsCli := &mockMSTeams{}
			r := s2hmsteams.New("tenantID", "clientID", "clientSecret", "user",
				"pass", s2hmsteams.WithMSTeamsClient(mockMSTeamsCli))
			err := r.SendActivePromotionStatus(configCtrl, atpRpt)
			g.Expect(mockMSTeamsCli.accessTokenCalls).Should(Equal(1))
			g.Expect(mockMSTeamsCli.getGroupIDCalls).Should(Equal(2))
			g.Expect(mockMSTeamsCli.getChannelIDCalls).Should(Equal(3))
			g.Expect(mockMSTeamsCli.postMessageCalls).Should(Equal(3))
			g.Expect(mockMSTeamsCli.message).Should(ContainSubstring("Failure"))
			g.Expect(mockMSTeamsCli.message).Should(ContainSubstring("All components are up to date!"))
			g.Expect(err).Should(BeNil())
		})

		It("should correctly send active promotion/demotion failure with rollback timeout message", func() {
			configCtrl := newMockConfigCtrl("", "", "")
			g.Expect(configCtrl).ShouldNot(BeNil())

			status := s2hv1beta1.ActivePromotionStatus{
				Result:         s2hv1beta1.ActivePromotionFailure,
				RollbackStatus: s2hv1beta1.ActivePromotionRollbackFailure,
				DemotionStatus: s2hv1beta1.ActivePromotionDemotionFailure,
			}
			atpRpt := internal.NewActivePromotionReporter(status, internal.SamsahaiConfig{}, "owner",
				"owner-123456", 1)

			mockMSTeamsCli := &mockMSTeams{}
			r := s2hmsteams.New("tenantID", "clientID", "clientSecret", "user",
				"pass", s2hmsteams.WithMSTeamsClient(mockMSTeamsCli))
			err := r.SendActivePromotionStatus(configCtrl, atpRpt)
			g.Expect(mockMSTeamsCli.accessTokenCalls).Should(Equal(1))
			g.Expect(mockMSTeamsCli.getGroupIDCalls).Should(Equal(2))
			g.Expect(mockMSTeamsCli.getChannelIDCalls).Should(Equal(3))
			g.Expect(mockMSTeamsCli.postMessageCalls).Should(Equal(3))
			g.Expect(mockMSTeamsCli.message).Should(ContainSubstring("Failure"))
			g.Expect(mockMSTeamsCli.message).Should(ContainSubstring(
				"cannot rollback an active promotion"))
			g.Expect(mockMSTeamsCli.message).Should(ContainSubstring(
				"cannot demote a previous active environment, previous active namespace has been destroyed immediately"))
			g.Expect(err).Should(BeNil())
		})
	})

	Describe("send image missing", func() {
		It("should correctly send image missing message", func() {
			configCtrl := newMockConfigCtrl("", "", "")
			g.Expect(configCtrl).ShouldNot(BeNil())

			mockMSTeamsCli := &mockMSTeams{}
			r := s2hmsteams.New("tenantID", "clientID", "clientSecret", "user",
				"pass", s2hmsteams.WithMSTeamsClient(mockMSTeamsCli))

			img := s2hv1beta1.Image{Repository: "registry/comp-1", Tag: "1.0.0"}
			imageMissingRpt := internal.NewImageMissingReporter(img, internal.SamsahaiConfig{}, "owner", "comp1")
			err := r.SendImageMissing(configCtrl, imageMissingRpt)
			g.Expect(mockMSTeamsCli.accessTokenCalls).Should(Equal(1))
			g.Expect(mockMSTeamsCli.getGroupIDCalls).Should(Equal(2))
			g.Expect(mockMSTeamsCli.getChannelIDCalls).Should(Equal(3))
			g.Expect(mockMSTeamsCli.postMessageCalls).Should(Equal(3))
			g.Expect(mockMSTeamsCli.channels).Should(Equal([]string{"chan1-1", "chan1-2", "chan2-1"}))
			g.Expect(mockMSTeamsCli.message).Should(ContainSubstring("registry/comp-1:1.0.0"))
			g.Expect(err).Should(BeNil())
		})
	})

	Describe("send pull request trigger result", func() {
		It("should correctly send pull request trigger failure message", func() {
			configCtrl := newMockConfigCtrl("", "", "")
			g.Expect(configCtrl).ShouldNot(BeNil())

			mockMSTeamsCli := &mockMSTeams{}
			r := s2hmsteams.New("tenantID", "clientID", "clientSecret", "user",
				"pass", s2hmsteams.WithMSTeamsClient(mockMSTeamsCli))

			timeNow := metav1.Now()
			noOfRetry := 2
			img := &s2hv1beta1.Image{Repository: "registry/comp-1", Tag: "1.0.0"}
			status := s2hv1beta1.PullRequestTriggerStatus{
				CreatedAt: &timeNow,
				NoOfRetry: &noOfRetry,
			}
			prTriggerRpt := internal.NewPullRequestTriggerResultReporter(status, internal.SamsahaiConfig{},
				"owner", "comp1", "1234", "Failure", img)
			err := r.SendPullRequestTriggerResult(configCtrl, prTriggerRpt)
			g.Expect(mockMSTeamsCli.accessTokenCalls).Should(Equal(1))
			g.Expect(mockMSTeamsCli.getGroupIDCalls).Should(Equal(2))
			g.Expect(mockMSTeamsCli.getChannelIDCalls).Should(Equal(3))
			g.Expect(mockMSTeamsCli.postMessageCalls).Should(Equal(3))
			g.Expect(mockMSTeamsCli.channels).Should(Equal([]string{"chan1-1", "chan1-2", "chan2-1"}))
			g.Expect(mockMSTeamsCli.message).Should(ContainSubstring("Failure"))
			g.Expect(mockMSTeamsCli.message).Should(ContainSubstring("comp1"))
			g.Expect(mockMSTeamsCli.message).Should(ContainSubstring("1234"))
			g.Expect(mockMSTeamsCli.message).Should(ContainSubstring("owner"))
			g.Expect(mockMSTeamsCli.message).Should(ContainSubstring("registry/comp-1:1.0.0"))
			g.Expect(mockMSTeamsCli.message).Should(ContainSubstring("<b>NO of Retry:</b> 2"))
			g.Expect(mockMSTeamsCli.message).Should(ContainSubstring(timeNow.Format("2006-01-02 15:04:05 MST")))
			g.Expect(err).Should(BeNil())
		})

		It("should correctly send pull request trigger success message", func() {
			configCtrl := newMockConfigCtrl("", "", s2hv1beta1.CriteriaBoth)
			g.Expect(configCtrl).ShouldNot(BeNil())

			mockMSTeamsCli := &mockMSTeams{}
			r := s2hmsteams.New("tenantID", "clientID", "clientSecret", "user",
				"pass", s2hmsteams.WithMSTeamsClient(mockMSTeamsCli))

			timeNow := metav1.Now()
			img := &s2hv1beta1.Image{Repository: "registry/comp-1", Tag: "1.0.0"}
			status := s2hv1beta1.PullRequestTriggerStatus{
				CreatedAt: &timeNow,
				NoOfRetry: nil,
			}
			prTriggerRpt := internal.NewPullRequestTriggerResultReporter(status, internal.SamsahaiConfig{},
				"owner", "comp1", "1234", "Success", img)
			err := r.SendPullRequestTriggerResult(configCtrl, prTriggerRpt)
			g.Expect(mockMSTeamsCli.accessTokenCalls).Should(Equal(1))
			g.Expect(mockMSTeamsCli.getGroupIDCalls).Should(Equal(2))
			g.Expect(mockMSTeamsCli.getChannelIDCalls).Should(Equal(3))
			g.Expect(mockMSTeamsCli.postMessageCalls).Should(Equal(3))
			g.Expect(mockMSTeamsCli.channels).Should(Equal([]string{"chan1-1", "chan1-2", "chan2-1"}))
			g.Expect(mockMSTeamsCli.message).Should(ContainSubstring("Success"))
			g.Expect(mockMSTeamsCli.message).Should(ContainSubstring("<b>NO of Retry:</b> 0"))
			g.Expect(err).Should(BeNil())
		})
	})

	Describe("failure path", func() {
		It("should not send message if not define ms teams reporter configuration", func() {
			configCtrl := newMockConfigCtrl("empty", "", "")
			g.Expect(configCtrl).ShouldNot(BeNil())

			rpcComp := &rpc.ComponentUpgrade{}
			mockMSTeamsCli := &mockMSTeams{}
			r := s2hmsteams.New("tenantID", "clientID", "clientSecret", "user",
				"pass", s2hmsteams.WithMSTeamsClient(mockMSTeamsCli))
			comp := internal.NewComponentUpgradeReporter(rpcComp, internal.SamsahaiConfig{})
			err := r.SendComponentUpgrade(configCtrl, comp)
			g.Expect(err).Should(BeNil())
			g.Expect(mockMSTeamsCli.postMessageCalls).Should(Equal(0))
		})

		It("should fail to send message", func() {
			configCtrl := newMockConfigCtrl("failure", "", "")
			g.Expect(configCtrl).ShouldNot(BeNil())

			rpcComp := &rpc.ComponentUpgrade{
				IsReverify: true,
			}
			mockMSTeamsCli := &mockMSTeams{}
			r := s2hmsteams.New("tenantID", "clientID", "clientSecret", "user",
				"pass", s2hmsteams.WithMSTeamsClient(mockMSTeamsCli))
			comp := internal.NewComponentUpgradeReporter(rpcComp, internal.SamsahaiConfig{})
			err := r.SendComponentUpgrade(configCtrl, comp)
			g.Expect(mockMSTeamsCli.getGroupIDCalls).Should(Equal(1))
			g.Expect(mockMSTeamsCli.getChannelIDCalls).Should(Equal(1))
			g.Expect(err).To(HaveOccurred())
		})
	})
})

// mockMSTeams mocks MS Teams interface
type mockMSTeams struct {
	postMessageCalls  int
	accessTokenCalls  int
	getGroupIDCalls   int
	getChannelIDCalls int
	channels          []string
	message           string
}

// GetAccessToken mocks GetAccessToken function
func (s *mockMSTeams) GetAccessToken() (string, error) {
	s.accessTokenCalls++
	return "12345", nil
}

// PostMessage mocks PostMessage function
func (s *mockMSTeams) PostMessage(groupID, channelID, message, accessTokens string, opts ...msteams.PostMsgOption) error {
	if channelID == "msg-error" {
		return errors.New("error")
	}

	s.postMessageCalls++
	s.channels = append(s.channels, channelID)
	s.message = message

	return nil
}

// GetGroupID mocks GetGroupID function
func (s *mockMSTeams) GetGroupID(groupNameOrID, accessToken string) (string, error) {
	if groupNameOrID == "group-error" {
		return "", errors.New("error")
	}

	s.getGroupIDCalls++
	return groupNameOrID, nil
}

// GetChannelID mocks GetChannelID function
func (s *mockMSTeams) GetChannelID(groupID, channelNameOrID, accessToken string) (string, error) {
	if channelNameOrID == "chan-error" {
		return "", errors.New("error")
	}

	s.getChannelIDCalls++
	return channelNameOrID, nil
}

type mockConfigCtrl struct {
	configType string
	interval   s2hv1beta1.ReporterInterval
	criteria   s2hv1beta1.ReporterCriteria
}

func newMockConfigCtrl(configType string, interval s2hv1beta1.ReporterInterval, criteria s2hv1beta1.ReporterCriteria) internal.ConfigController {
	return &mockConfigCtrl{
		configType: configType,
		interval:   interval,
		criteria:   criteria,
	}
}

func (c *mockConfigCtrl) Get(configName string) (*s2hv1beta1.Config, error) {
	switch c.configType {
	case "empty":
		return &s2hv1beta1.Config{}, nil
	case "failure":
		return &s2hv1beta1.Config{
			Spec: s2hv1beta1.ConfigSpec{
				Reporter: &s2hv1beta1.ConfigReporter{
					MSTeams: &s2hv1beta1.MSTeams{
						Groups: []s2hv1beta1.MSTeamsGroup{
							{
								GroupNameOrID:    "group-1",
								ChannelNameOrIDs: []string{"msg-error", "chan-error"},
							},
							{
								GroupNameOrID:    "group-error",
								ChannelNameOrIDs: []string{"chan-1"},
							},
						},
					},
				},
			},
			Status: s2hv1beta1.ConfigStatus{
				Used: s2hv1beta1.ConfigSpec{
					Reporter: &s2hv1beta1.ConfigReporter{
						MSTeams: &s2hv1beta1.MSTeams{
							Groups: []s2hv1beta1.MSTeamsGroup{
								{
									GroupNameOrID:    "group-1",
									ChannelNameOrIDs: []string{"msg-error", "chan-error"},
								},
								{
									GroupNameOrID:    "group-error",
									ChannelNameOrIDs: []string{"chan-1"},
								},
							},
						},
					},
				},
			},
		}, nil
	default:
		return &s2hv1beta1.Config{
			Spec: s2hv1beta1.ConfigSpec{
				Reporter: &s2hv1beta1.ConfigReporter{
					MSTeams: &s2hv1beta1.MSTeams{
						Groups: []s2hv1beta1.MSTeamsGroup{
							{
								GroupNameOrID:    "group1",
								ChannelNameOrIDs: []string{"chan1-1", "chan1-2"},
							},
							{
								GroupNameOrID:    "group2",
								ChannelNameOrIDs: []string{"chan2-1"},
							},
						},
						ComponentUpgrade: &s2hv1beta1.ConfigComponentUpgradeReport{
							Interval: c.interval,
							Criteria: c.criteria,
						},
					},
				},
			},
			Status: s2hv1beta1.ConfigStatus{
				Used: s2hv1beta1.ConfigSpec{
					Reporter: &s2hv1beta1.ConfigReporter{
						MSTeams: &s2hv1beta1.MSTeams{
							Groups: []s2hv1beta1.MSTeamsGroup{
								{
									GroupNameOrID:    "group1",
									ChannelNameOrIDs: []string{"chan1-1", "chan1-2"},
								},
								{
									GroupNameOrID:    "group2",
									ChannelNameOrIDs: []string{"chan2-1"},
								},
							},
							ComponentUpgrade: &s2hv1beta1.ConfigComponentUpgradeReport{
								Interval: c.interval,
								Criteria: c.criteria,
							},
						},
					},
				},
			},
		}, nil
	}
}

func (c *mockConfigCtrl) GetComponents(configName string) (map[string]*s2hv1beta1.Component, error) {
	return map[string]*s2hv1beta1.Component{}, nil
}

func (c *mockConfigCtrl) GetParentComponents(configName string) (map[string]*s2hv1beta1.Component, error) {
	return map[string]*s2hv1beta1.Component{}, nil
}

func (c *mockConfigCtrl) GetPullRequestComponents(configName string) (map[string]*s2hv1beta1.Component, error) {
	return map[string]*s2hv1beta1.Component{}, nil
}

func (c *mockConfigCtrl) GetBundles(configName string) (s2hv1beta1.ConfigBundles, error) {
	return s2hv1beta1.ConfigBundles{}, nil
}

func (c *mockConfigCtrl) GetPriorityQueues(configName string) ([]string, error) {
	return nil, nil
}

func (c *mockConfigCtrl) GetPullRequestConfig(configName string) (*s2hv1beta1.ConfigPullRequest, error) {
	return nil, nil
}

func (c *mockConfigCtrl) GetPullRequestComponentDependencies(configName, prCompName string) ([]string, error) {
	return nil, nil
}

func (c *mockConfigCtrl) Update(config *s2hv1beta1.Config) error {
	return nil
}

func (c *mockConfigCtrl) Delete(configName string) error {
	return nil
}

func (c *mockConfigCtrl) EnsureConfigTemplateChanged(config *s2hv1beta1.Config) error {
	return nil
}
