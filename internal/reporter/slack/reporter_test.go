package slack_test

import (
	"testing"
	"time"

	"github.com/nlopes/slack"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	s2hv1beta1 "github.com/agoda-com/samsahai/api/v1beta1"
	"github.com/agoda-com/samsahai/internal"
	s2hslack "github.com/agoda-com/samsahai/internal/reporter/slack"
	"github.com/agoda-com/samsahai/internal/util/unittest"
	"github.com/agoda-com/samsahai/pkg/samsahai/rpc"
)

func TestUnit(t *testing.T) {
	unittest.InitGinkgo(t, "Slack Reporter")
}

var _ = Describe("send slack message", func() {
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
			mockSlackCli := &mockSlack{}
			r := s2hslack.New("mock-token", s2hslack.WithSlackClient(mockSlackCli))
			testRunner := s2hv1beta1.TestRunner{Teamcity: s2hv1beta1.Teamcity{BuildURL: "teamcity-url", BuildNumber: "teamcity-build-number"}}
			comp := internal.NewComponentUpgradeReporter(
				rpcComp,
				internal.SamsahaiConfig{SamsahaiExternalURL: "http://localhost:8080"},
				internal.WithTestRunner(testRunner),
				internal.WithQueueHistoryName("comp1-5678"),
			)
			err := r.SendComponentUpgrade(configCtrl, comp)
			g.Expect(err).Should(BeNil())
			g.Expect(mockSlackCli.postMessageCalls).Should(Equal(2))
			g.Expect(mockSlackCli.channels).Should(Equal([]string{"chan1", "chan2"}))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("Component Upgrade"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("Failure"))
			// Should contain information
			g.Expect(mockSlackCli.message).Should(ContainSubstring("#2"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("*Name:* comp1"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("1.1.0"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("image-1"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("Desired component failed"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("<teamcity-url|teamcity-build-number>"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("<http://localhost:8080/teams/owner/queue/histories/comp1-5678/log|Download here>"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("owner"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("owner-staging"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("<http://localhost:8080/teams/owner/queue/histories/comp1-5678|Click here>"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("*Issue type:* CrashLoopBackOff"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("*Components:* comp1"))
			g.Expect(mockSlackCli.message).ShouldNot(ContainSubstring("Image Missing List"))
		})

		It("should not send component upgrade failure with retry interval", func() {
			configCtrl := newMockConfigCtrl("", "", "")
			g.Expect(configCtrl).ShouldNot(BeNil())

			rpcComp := &rpc.ComponentUpgrade{
				Name:       "comp1",
				Status:     rpc.ComponentUpgrade_UpgradeStatus_FAILURE,
				IsReverify: false,
			}
			mockSlackCli := &mockSlack{}
			r := s2hslack.New("mock-token", s2hslack.WithSlackClient(mockSlackCli))
			comp := internal.NewComponentUpgradeReporter(rpcComp, internal.SamsahaiConfig{})
			err := r.SendComponentUpgrade(configCtrl, comp)
			g.Expect(err).Should(BeNil())
			g.Expect(mockSlackCli.postMessageCalls).Should(Equal(0))
		})

		It("should not send component upgrade failure with success criteria", func() {
			configCtrl := newMockConfigCtrl("", s2hv1beta1.IntervalEveryTime, s2hv1beta1.CriteriaSuccess)
			g.Expect(configCtrl).ShouldNot(BeNil())

			rpcComp := &rpc.ComponentUpgrade{
				Name:   "comp1",
				Status: rpc.ComponentUpgrade_UpgradeStatus_FAILURE,
			}
			mockSlackCli := &mockSlack{}
			r := s2hslack.New("mock-token", s2hslack.WithSlackClient(mockSlackCli))
			comp := internal.NewComponentUpgradeReporter(rpcComp, internal.SamsahaiConfig{})
			err := r.SendComponentUpgrade(configCtrl, comp)
			g.Expect(err).Should(BeNil())
			g.Expect(mockSlackCli.postMessageCalls).Should(Equal(0))
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
			mockSlackCli := &mockSlack{}
			r := s2hslack.New("mock-token", s2hslack.WithSlackClient(mockSlackCli))
			comp := internal.NewComponentUpgradeReporter(rpcComp, internal.SamsahaiConfig{})
			err := r.SendComponentUpgrade(configCtrl, comp)
			g.Expect(err).Should(BeNil())
			g.Expect(mockSlackCli.postMessageCalls).Should(Equal(2))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("Failure"))
			// Should contain information
			g.Expect(mockSlackCli.message).Should(ContainSubstring("Reverify"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("Image Missing List"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("- image-2:1.1.0"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("- image-3:1.2.0"))
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
			mockSlackCli := &mockSlack{}
			r := s2hslack.New("mock-token", s2hslack.WithSlackClient(mockSlackCli))
			comp := internal.NewComponentUpgradeReporter(rpcComp, internal.SamsahaiConfig{})
			err := r.SendComponentUpgrade(configCtrl, comp)
			g.Expect(err).Should(BeNil())
			g.Expect(mockSlackCli.message).Should(ContainSubstring("Failure"))
			// Should contain information
			g.Expect(mockSlackCli.message).Should(ContainSubstring("comp1"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("1.1.0"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("image-1"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("comp2"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("1.1.2"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("image-2"))
		})
	})

	Describe("send pull request queue", func() {
		It("should correctly send pull request queue failure", func() {
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
				TeamName:         "owner",
				IssueType:        rpc.ComponentUpgrade_IssueType_DESIRED_VERSION_FAILED,
				Namespace:        "owner-staging",
				QueueHistoryName: "comp1-1234",
				IsReverify:       true,
				Runs:             3,
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
			mockSlackCli := &mockSlack{}
			r := s2hslack.New("mock-token", s2hslack.WithSlackClient(mockSlackCli))
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
			g.Expect(mockSlackCli.postMessageCalls).Should(Equal(2))
			g.Expect(mockSlackCli.channels).Should(Equal([]string{"chan1", "chan2"}))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("Pull Request Queue"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("Failure"))
			// Should contain information
			g.Expect(mockSlackCli.message).Should(ContainSubstring("pr1234"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("pr-comp1"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("#3"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("pr-namespace"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring(
				"<http://localhost:8080/teams/owner/pullrequest/queue/histories/comp1-5678/log|Download here>"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring(
				"<http://localhost:8080/teams/owner/pullrequest/queue/histories/comp1-5678|Click here>"))

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

			mockSlackCli := &mockSlack{}
			r := s2hslack.New("mock-token", s2hslack.WithSlackClient(mockSlackCli))
			err := r.SendActivePromotionStatus(configCtrl, atpRpt)
			g.Expect(mockSlackCli.postMessageCalls).Should(Equal(2))
			g.Expect(mockSlackCli.channels).Should(Equal([]string{"chan1", "chan2"}))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("Success"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("#2"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("owner"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("owner-123456"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("<teamcity-url|teamcity-build-number"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("Outdated Components"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("comp1"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("Not update for 1d 0h 0m"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("Current Version: <http://repo/comp1|1.1.0>"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("Latest Version: <http://repo/comp1|1.1.2>"))
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
			atpRpt := internal.NewActivePromotionReporter(status, internal.SamsahaiConfig{SamsahaiExternalURL: "http://localhost:8080"}, "owner", "owner-123456", 1)

			mockSlackCli := &mockSlack{}
			r := s2hslack.New("mock-token", s2hslack.WithSlackClient(mockSlackCli))
			err := r.SendActivePromotionStatus(configCtrl, atpRpt)
			g.Expect(mockSlackCli.postMessageCalls).Should(Equal(2))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("Success"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("<http://localhost:8080/teams/owner/activepromotions/histories/owner-12345|Click here>"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("All components are up to date!"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("previous active namespace `owner-prevns` will be destroyed at `" + timeNow.Format("2006-01-02 15:04:05 MST")))
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
					internal.SamsahaiConfig{SamsahaiExternalURL: "http://localhost:8080"}, "owner",
					"owner-123456", 2)

				mockSlackCli := &mockSlack{}
				r := s2hslack.New("mock-token", s2hslack.WithSlackClient(mockSlackCli))
				err := r.SendActivePromotionStatus(configCtrl, atpRpt)
				g.Expect(mockSlackCli.postMessageCalls).Should(Equal(2))
				g.Expect(mockSlackCli.channels).Should(Equal([]string{"chan1", "chan2"}))
				g.Expect(mockSlackCli.message).Should(ContainSubstring("Failure"))
				g.Expect(mockSlackCli.message).Should(ContainSubstring("#2"))
				g.Expect(mockSlackCli.message).Should(ContainSubstring("owner"))
				g.Expect(mockSlackCli.message).Should(ContainSubstring("owner-123456"))
				g.Expect(mockSlackCli.message).Should(ContainSubstring("<http://localhost:8080/teams/owner/activepromotions/histories/owner-12345/log|Download here>"))
				g.Expect(mockSlackCli.message).Should(ContainSubstring("<http://localhost:8080/teams/owner/activepromotions/histories/owner-12345|Click here>"))
				g.Expect(mockSlackCli.message).Should(ContainSubstring("Image Missing List"))
				g.Expect(mockSlackCli.message).Should(ContainSubstring("- repo1:1.xx"))
				g.Expect(mockSlackCli.message).Should(ContainSubstring("- repo2:2.xx"))
				g.Expect(mockSlackCli.message).Should(ContainSubstring("Outdated Components"))
				g.Expect(mockSlackCli.message).Should(ContainSubstring("comp1"))
				g.Expect(mockSlackCli.message).Should(ContainSubstring("Not update for 1d 0h 0m"))
				g.Expect(mockSlackCli.message).Should(ContainSubstring("Current Version: <http://repo/comp1|1.1.0>"))
				g.Expect(mockSlackCli.message).Should(ContainSubstring("Latest Version: <http://repo/comp1|1.1.2>"))
				g.Expect(mockSlackCli.message).Should(ContainSubstring("*Issue type:* WaitForInitContainer"))
				g.Expect(mockSlackCli.message).Should(ContainSubstring("*Components:* comp1"))
				g.Expect(mockSlackCli.message).Should(ContainSubstring("*Wait for:* dep1"))
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

			mockSlackCli := &mockSlack{}
			r := s2hslack.New("mock-token", s2hslack.WithSlackClient(mockSlackCli))
			err := r.SendActivePromotionStatus(configCtrl, atpRpt)
			g.Expect(mockSlackCli.postMessageCalls).Should(Equal(2))
			g.Expect(mockSlackCli.channels).Should(Equal([]string{"chan1", "chan2"}))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("Failure"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("owner"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("owner-123456"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("All components are up to date!"))
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

			mockSlackCli := &mockSlack{}
			r := s2hslack.New("mock-token", s2hslack.WithSlackClient(mockSlackCli))
			err := r.SendActivePromotionStatus(configCtrl, atpRpt)
			g.Expect(mockSlackCli.postMessageCalls).Should(Equal(2))
			g.Expect(mockSlackCli.channels).Should(Equal([]string{"chan1", "chan2"}))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("Failure"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("owner"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("owner-123456"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring(
				"cannot rollback an active promotion"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring(
				"cannot demote a previous active environment, previous active namespace has been destroyed immediately"))
			g.Expect(err).Should(BeNil())
		})
	})

	Describe("send image missing", func() {
		It("should correctly send image missing message", func() {
			configCtrl := newMockConfigCtrl("", "", "")
			g.Expect(configCtrl).ShouldNot(BeNil())

			mockSlackCli := &mockSlack{}
			r := s2hslack.New("mock-token", s2hslack.WithSlackClient(mockSlackCli))
			img := s2hv1beta1.Image{Repository: "registry/comp-1", Tag: "1.0.0"}
			imageMissingRpt := internal.NewImageMissingReporter(img, internal.SamsahaiConfig{}, "owner", "comp1")
			err := r.SendImageMissing(configCtrl, imageMissingRpt)
			g.Expect(mockSlackCli.postMessageCalls).Should(Equal(2))
			g.Expect(mockSlackCli.channels).Should(Equal([]string{"chan1", "chan2"}))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("registry/comp-1:1.0.0"))
			g.Expect(err).Should(BeNil())
		})
	})

	Describe("send pull request trigger result", func() {
		It("should correctly send pull request trigger failure message", func() {
			configCtrl := newMockConfigCtrl("", "", "")
			g.Expect(configCtrl).ShouldNot(BeNil())

			mockSlackCli := &mockSlack{}
			r := s2hslack.New("mock-token", s2hslack.WithSlackClient(mockSlackCli))
			timeNow := metav1.Now()
			noOfRetry := 2
			img := &s2hv1beta1.Image{Repository: "registry/comp-1", Tag: "1.0.0"}
			status := s2hv1beta1.PullRequestTriggerStatus{
				CreatedAt: &timeNow,
				NoOfRetry: &noOfRetry,
			}
			prTriggerRpt := internal.NewPullRequestTriggerResultReporter(status, internal.SamsahaiConfig{},
				"owner", "comp1", "pr1234", "Failure", img)
			err := r.SendPullRequestTriggerResult(configCtrl, prTriggerRpt)
			g.Expect(mockSlackCli.postMessageCalls).Should(Equal(2))
			g.Expect(mockSlackCli.channels).Should(Equal([]string{"chan1", "chan2"}))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("Failure"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("comp1"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("pr1234"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("owner"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("registry/comp-1:1.0.0"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("*NO of Retry:* 2"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring(timeNow.Format("2006-01-02 15:04:05 MST")))
			g.Expect(err).Should(BeNil())
		})

		It("should correctly send pull request trigger success message", func() {
			configCtrl := newMockConfigCtrl("", "", s2hv1beta1.CriteriaBoth)
			g.Expect(configCtrl).ShouldNot(BeNil())

			mockSlackCli := &mockSlack{}
			r := s2hslack.New("mock-token", s2hslack.WithSlackClient(mockSlackCli))
			timeNow := metav1.Now()
			img := &s2hv1beta1.Image{Repository: "registry/comp-1", Tag: "1.0.0"}
			status := s2hv1beta1.PullRequestTriggerStatus{
				CreatedAt: &timeNow,
				NoOfRetry: nil,
			}
			prTriggerRpt := internal.NewPullRequestTriggerResultReporter(status, internal.SamsahaiConfig{},
				"owner", "comp1", "pr1234", "Success", img)
			err := r.SendPullRequestTriggerResult(configCtrl, prTriggerRpt)
			g.Expect(mockSlackCli.postMessageCalls).Should(Equal(2))
			g.Expect(mockSlackCli.channels).Should(Equal([]string{"chan1", "chan2"}))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("Success"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("*NO of Retry:* 0"))
			g.Expect(err).Should(BeNil())
		})
	})

	Describe("failure path", func() {
		It("should not send message if not define slack reporter configuration", func() {
			configCtrl := newMockConfigCtrl("empty", "", "")
			g.Expect(configCtrl).ShouldNot(BeNil())

			rpcComp := &rpc.ComponentUpgrade{}
			mockSlackCli := &mockSlack{}
			r := s2hslack.New("mock-token", s2hslack.WithSlackClient(mockSlackCli))
			comp := internal.NewComponentUpgradeReporter(rpcComp, internal.SamsahaiConfig{})
			err := r.SendComponentUpgrade(configCtrl, comp)
			g.Expect(err).Should(BeNil())
			g.Expect(mockSlackCli.postMessageCalls).Should(Equal(0))
		})

		It("should fail to send message", func() {
			configCtrl := newMockConfigCtrl("failure", "", "")
			g.Expect(configCtrl).ShouldNot(BeNil())

			rpcComp := &rpc.ComponentUpgrade{
				IsReverify: true,
			}
			mockSlackCli := &mockSlack{}
			r := s2hslack.New("mock-token", s2hslack.WithSlackClient(mockSlackCli))
			comp := internal.NewComponentUpgradeReporter(rpcComp, internal.SamsahaiConfig{})
			err := r.SendComponentUpgrade(configCtrl, comp)
			g.Expect(err).To(HaveOccurred())
		})
	})
})

// mockSlack mocks Slack interface
type mockSlack struct {
	postMessageCalls int
	channels         []string
	message          string
}

// PostMessage mocks PostMessage function
func (s *mockSlack) PostMessage(channelNameOrID, message string, opts ...slack.MsgOption) error {
	if channelNameOrID == "error" {
		return errors.New("error")
	}

	s.postMessageCalls++
	s.channels = append(s.channels, channelNameOrID)
	s.message = message

	return nil
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
					Slack: &s2hv1beta1.Slack{
						Channels: []string{"error"},
					},
				},
			},
			Status: s2hv1beta1.ConfigStatus{
				Used: s2hv1beta1.ConfigSpec{
					Reporter: &s2hv1beta1.ConfigReporter{
						Slack: &s2hv1beta1.Slack{
							Channels: []string{"error"},
						},
					},
				},
			},
		}, nil
	default:
		return &s2hv1beta1.Config{
			Spec: s2hv1beta1.ConfigSpec{
				Reporter: &s2hv1beta1.ConfigReporter{
					Slack: &s2hv1beta1.Slack{
						Channels: []string{"chan1", "chan2"},
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
						Slack: &s2hv1beta1.Slack{
							Channels: []string{"chan1", "chan2"},
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
