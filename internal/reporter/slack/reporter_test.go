package slack_test

import (
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/agoda-com/samsahai/internal"
	"github.com/agoda-com/samsahai/internal/config"
	"github.com/agoda-com/samsahai/internal/reporter/slack"
	"github.com/agoda-com/samsahai/internal/util/unittest"
	s2hv1beta1 "github.com/agoda-com/samsahai/pkg/apis/env/v1beta1"
	"github.com/agoda-com/samsahai/pkg/samsahai/rpc"
)

func TestUnit(t *testing.T) {
	unittest.InitGinkgo(t, "Slack Reporter")
}

var _ = Describe("send slack message", func() {
	g := NewGomegaWithT(GinkgoT())

	Describe("send component upgrade", func() {
		It("should correctly send component upgrade failure without image missing message", func() {
			configMgr := newConfigMock()
			g.Expect(configMgr).ShouldNot(BeNil())

			rpcComp := &rpc.ComponentUpgrade{
				Name:             "comp1",
				Status:           rpc.ComponentUpgrade_FAILURE,
				Image:            &rpc.Image{Repository: "image-1", Tag: "1.1.0"},
				TeamName:         "owner",
				IssueType:        rpc.ComponentUpgrade_DESIRED_VERSION_FAILED,
				Namespace:        "owner-staging",
				QueueHistoryName: "comp1-1234",
			}
			mockSlackCli := &mockSlack{}
			r := slack.New("mock-token", slack.WithSlackClient(mockSlackCli))
			testRunner := s2hv1beta1.TestRunner{Teamcity: s2hv1beta1.Teamcity{BuildURL: "teamcity-url"}}
			comp := internal.NewComponentUpgradeReporter(
				rpcComp,
				internal.SamsahaiConfig{SamsahaiExternalURL: "http://localhost:8080"},
				internal.WithTestRunner(testRunner),
				internal.WithQueueHistoryName("comp1-5678"),
			)
			err := r.SendComponentUpgrade(configMgr, comp)
			g.Expect(err).Should(BeNil())
			g.Expect(mockSlackCli.postMessageCalls).Should(Equal(2))
			g.Expect(mockSlackCli.channels).Should(Equal([]string{"chan1", "chan2"}))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("Component Upgrade Failed"))
			// Should contain information
			g.Expect(mockSlackCli.message).Should(ContainSubstring("comp1"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("1.1.0"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("image-1"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("Desired component failed"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("<teamcity-url|Click here>"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("<http://localhost:8080/teams/owner/queue/histories/comp1-5678/log|Download here>"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("owner"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("owner-staging"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("<http://localhost:8080/teams/owner/queue/histories/comp1-5678|Click here>"))
			g.Expect(mockSlackCli.message).ShouldNot(ContainSubstring("*Image Missing List*"))
		})

		It("should correctly send component upgrade failure with image missing list message", func() {
			configMgr := newConfigMock()
			g.Expect(configMgr).ShouldNot(BeNil())

			rpcComp := &rpc.ComponentUpgrade{
				Name:      "comp1",
				Status:    rpc.ComponentUpgrade_FAILURE,
				Image:     &rpc.Image{Repository: "image-1", Tag: "1.1.0"},
				TeamName:  "owner",
				IssueType: rpc.ComponentUpgrade_IMAGE_MISSING,
				Namespace: "owner-staging",
				ImageMissingList: []*rpc.Image{
					{Repository: "image-2", Tag: "1.1.0"},
					{Repository: "image-3", Tag: "1.2.0"},
				},
			}
			mockSlackCli := &mockSlack{}
			r := slack.New("mock-token", slack.WithSlackClient(mockSlackCli))
			comp := internal.NewComponentUpgradeReporter(rpcComp, internal.SamsahaiConfig{})
			err := r.SendComponentUpgrade(configMgr, comp)
			g.Expect(err).Should(BeNil())
			g.Expect(mockSlackCli.postMessageCalls).Should(Equal(2))
			g.Expect(mockSlackCli.channels).Should(Equal([]string{"chan1", "chan2"}))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("Component Upgrade Failed"))
			// Should contain information
			g.Expect(mockSlackCli.message).Should(ContainSubstring("comp1"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("Image missing"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("*Image Missing List*"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("- image-2:1.1.0"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("- image-3:1.2.0"))
		})
	})

	Describe("send active promotion", func() {
		It("should correctly send active promotion success with outdated components message", func() {
			configMgr := newConfigMock()
			g.Expect(configMgr).ShouldNot(BeNil())

			var comp1, repoComp1, comp2, repoComp2 = "comp1", "repo/comp1", "comp2", "repo/comp2"
			var v110, v112 = "1.1.0", "1.1.2"

			status := &s2hv1beta1.ActivePromotionStatus{
				Result:               s2hv1beta1.ActivePromotionSuccess,
				HasOutdatedComponent: true,
				OutdatedComponents: []*s2hv1beta1.OutdatedComponent{
					{
						Name:             comp1,
						CurrentImage:     &s2hv1beta1.Image{Repository: repoComp1, Tag: v110},
						LatestImage:      &s2hv1beta1.Image{Repository: repoComp1, Tag: v112},
						OutdatedDuration: time.Duration(86400000000000), // 1d0h0m
					},
					{
						Name:             comp2,
						CurrentImage:     &s2hv1beta1.Image{Repository: repoComp2, Tag: v110},
						LatestImage:      &s2hv1beta1.Image{Repository: repoComp2, Tag: v110},
						OutdatedDuration: time.Duration(0),
					},
				},
				PreActiveQueue: s2hv1beta1.QueueStatus{
					TestRunner: s2hv1beta1.TestRunner{
						Teamcity: s2hv1beta1.Teamcity{BuildURL: "teamcity-url"},
					},
				},
			}
			atpRpt := internal.NewActivePromotionReporter(status, internal.SamsahaiConfig{}, "owner", "owner-123456")

			mockSlackCli := &mockSlack{}
			r := slack.New("mock-token", slack.WithSlackClient(mockSlackCli))
			err := r.SendActivePromotionStatus(configMgr, atpRpt)
			g.Expect(mockSlackCli.postMessageCalls).Should(Equal(2))
			g.Expect(mockSlackCli.channels).Should(Equal([]string{"chan1", "chan2"}))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("Success"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("owner"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("owner-123456"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("<teamcity-url|Click here>"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring(`*Outdated Components:*
*comp1*
>Not update for 1d 0h 0m
>Current Version: <http://repo/comp1|1.1.0>
>Latest Version: <http://repo/comp1|1.1.2>`))
			g.Expect(err).Should(BeNil())
		})

		It("should correctly send active promotion success without outdated components message", func() {
			configMgr := newConfigMock()
			g.Expect(configMgr).ShouldNot(BeNil())

			timeNow := metav1.Now()
			status := &s2hv1beta1.ActivePromotionStatus{
				Result:                     s2hv1beta1.ActivePromotionSuccess,
				HasOutdatedComponent:       false,
				ActivePromotionHistoryName: "owner-12345",
				PreviousActiveNamespace:    "owner-prevns",
				DestroyTime:                &timeNow,
			}
			atpRpt := internal.NewActivePromotionReporter(status, internal.SamsahaiConfig{SamsahaiExternalURL: "http://localhost:8080"}, "owner", "owner-123456")

			mockSlackCli := &mockSlack{}
			r := slack.New("mock-token", slack.WithSlackClient(mockSlackCli))
			err := r.SendActivePromotionStatus(configMgr, atpRpt)
			g.Expect(mockSlackCli.postMessageCalls).Should(Equal(2))
			g.Expect(mockSlackCli.channels).Should(Equal([]string{"chan1", "chan2"}))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("Success"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("owner"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("owner-123456"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("<http://localhost:8080/teams/owner/activepromotions/histories/owner-12345|Click here>"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring(">*All components are up to date!*"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("previous active namespace `owner-prevns` will be destroyed at `" + timeNow.Format("2006-01-02 15:04:05")))
			g.Expect(err).Should(BeNil())
		})

		It("should correctly send active promotion failure with outdated components message", func() {
			configMgr := newConfigMock()
			g.Expect(configMgr).ShouldNot(BeNil())

			var comp1, repoComp1, comp2, repoComp2 = "comp1", "repo/comp1", "comp2", "repo/comp2"
			var v110, v112 = "1.1.0", "1.1.2"

			status := &s2hv1beta1.ActivePromotionStatus{
				Result:               s2hv1beta1.ActivePromotionFailure,
				HasOutdatedComponent: true,
				OutdatedComponents: []*s2hv1beta1.OutdatedComponent{
					{
						Name:             comp1,
						CurrentImage:     &s2hv1beta1.Image{Repository: repoComp1, Tag: v110},
						LatestImage:      &s2hv1beta1.Image{Repository: repoComp1, Tag: v112},
						OutdatedDuration: time.Duration(86400000000000), // 1d0h0m
					},
					{
						Name:             comp2,
						CurrentImage:     &s2hv1beta1.Image{Repository: repoComp2, Tag: v110},
						LatestImage:      &s2hv1beta1.Image{Repository: repoComp2, Tag: v110},
						OutdatedDuration: time.Duration(0),
					},
				},
				ActivePromotionHistoryName: "owner-12345",
			}
			atpRpt := internal.NewActivePromotionReporter(status, internal.SamsahaiConfig{SamsahaiExternalURL: "http://localhost:8080"}, "owner", "owner-123456")

			mockSlackCli := &mockSlack{}
			r := slack.New("mock-token", slack.WithSlackClient(mockSlackCli))
			err := r.SendActivePromotionStatus(configMgr, atpRpt)
			g.Expect(mockSlackCli.postMessageCalls).Should(Equal(2))
			g.Expect(mockSlackCli.channels).Should(Equal([]string{"chan1", "chan2"}))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("Failure"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("owner"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("owner-123456"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("<http://localhost:8080/teams/owner/activepromotions/histories/owner-12345/log|Download here>"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("<http://localhost:8080/teams/owner/activepromotions/histories/owner-12345|Click here>"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring(`*Outdated Components:*
*comp1*
>Not update for 1d 0h 0m
>Current Version: <http://repo/comp1|1.1.0>
>Latest Version: <http://repo/comp1|1.1.2>`))
			g.Expect(err).Should(BeNil())
		})

		It("should correctly send active promotion failure without outdated components message", func() {
			configMgr := newConfigMock()
			g.Expect(configMgr).ShouldNot(BeNil())

			status := &s2hv1beta1.ActivePromotionStatus{
				Result:               s2hv1beta1.ActivePromotionFailure,
				HasOutdatedComponent: false,
			}
			atpRpt := internal.NewActivePromotionReporter(status, internal.SamsahaiConfig{}, "owner", "owner-123456")

			mockSlackCli := &mockSlack{}
			r := slack.New("mock-token", slack.WithSlackClient(mockSlackCli))
			err := r.SendActivePromotionStatus(configMgr, atpRpt)
			g.Expect(mockSlackCli.postMessageCalls).Should(Equal(2))
			g.Expect(mockSlackCli.channels).Should(Equal([]string{"chan1", "chan2"}))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("Failure"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("owner"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("owner-123456"))
			g.Expect(mockSlackCli.message).Should(ContainSubstring(">*All components are up to date!*"))
			g.Expect(err).Should(BeNil())
		})

		It("should correctly send active promotion/demotion failure with rollback timeout message", func() {
			configMgr := newConfigMock()
			g.Expect(configMgr).ShouldNot(BeNil())

			status := &s2hv1beta1.ActivePromotionStatus{
				Result:         s2hv1beta1.ActivePromotionFailure,
				RollbackStatus: s2hv1beta1.ActivePromotionRollbackFailure,
				DemotionStatus: s2hv1beta1.ActivePromotionDemotionFailure,
			}
			atpRpt := internal.NewActivePromotionReporter(status, internal.SamsahaiConfig{}, "owner", "owner-123456")

			mockSlackCli := &mockSlack{}
			r := slack.New("mock-token", slack.WithSlackClient(mockSlackCli))
			err := r.SendActivePromotionStatus(configMgr, atpRpt)
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
			configMgr := newConfigMock()
			g.Expect(configMgr).ShouldNot(BeNil())

			mockSlackCli := &mockSlack{}
			r := slack.New("mock-token", slack.WithSlackClient(mockSlackCli))
			err := r.SendImageMissing(configMgr, &rpc.Image{Repository: "registry/comp-1", Tag: "1.0.0"})
			g.Expect(mockSlackCli.postMessageCalls).Should(Equal(2))
			g.Expect(mockSlackCli.channels).Should(Equal([]string{"chan1", "chan2"}))
			g.Expect(mockSlackCli.message).Should(ContainSubstring("registry/comp-1:1.0.0"))
			g.Expect(err).Should(BeNil())
		})
	})

	It("should not send message if not define slack reporter configuration", func() {
		configMgr := newNoSlackConfig()
		g.Expect(configMgr).ShouldNot(BeNil())

		rpcComp := &rpc.ComponentUpgrade{}
		mockSlackCli := &mockSlack{}
		r := slack.New("mock-token", slack.WithSlackClient(mockSlackCli))
		comp := internal.NewComponentUpgradeReporter(rpcComp, internal.SamsahaiConfig{})
		err := r.SendComponentUpgrade(configMgr, comp)
		g.Expect(err).Should(BeNil())
		g.Expect(mockSlackCli.postMessageCalls).Should(Equal(0))
	})

	It("should fail to send message", func() {
		configMgr := newFailureConfig()
		g.Expect(configMgr).ShouldNot(BeNil())

		rpcComp := &rpc.ComponentUpgrade{}
		mockSlackCli := &mockSlack{}
		r := slack.New("mock-token", slack.WithSlackClient(mockSlackCli))
		comp := internal.NewComponentUpgradeReporter(rpcComp, internal.SamsahaiConfig{})
		err := r.SendComponentUpgrade(configMgr, comp)
		g.Expect(err).To(HaveOccurred())
	})
})

// mockSlack mocks Slack interface
type mockSlack struct {
	postMessageCalls int
	channels         []string
	message          string
	username         string
}

// PostMessage mocks PostMessage function
func (s *mockSlack) PostMessage(channelNameOrID, message, username string) (channelID, timestamp string, err error) {
	if channelNameOrID == "error" {
		return channelNameOrID, "", errors.New("error")
	}

	s.postMessageCalls++
	s.channels = append(s.channels, channelNameOrID)
	s.message = message
	s.username = username

	return channelNameOrID, "", nil
}

func newConfigMock() internal.ConfigManager {
	configMgr := config.NewWithBytes([]byte(`
report:
 slack:
   channels:
     - chan1
     - chan2
`))

	return configMgr
}

func newNoSlackConfig() internal.ConfigManager {
	configMgr := config.NewWithBytes([]byte(`
report:
`))

	return configMgr
}

func newFailureConfig() internal.ConfigManager {
	configMgr := config.NewWithBytes([]byte(`
report:
 slack:
   channels:
     - error
`))

	return configMgr
}
