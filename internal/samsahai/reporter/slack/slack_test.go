package slack_test

import (
	"testing"

	"github.com/agoda-com/samsahai/internal/samsahai/component"
	reporter "github.com/agoda-com/samsahai/internal/samsahai/reporter/slack"
	"github.com/agoda-com/samsahai/internal/samsahai/util/slack"
	. "github.com/onsi/gomega"
)

func TestSendMessage(t *testing.T) {
	g := NewGomegaWithT(t)
	mockSlackCli := &mockSlack{}
	s := reporter.NewSlackWithClient("some-token", "samsahai", []string{"#chan1", "#chan2"}, mockSlackCli)
	err := s.SendMessage("some-message")

	g.Expect(mockSlackCli.postMessageCalls).Should(Equal(2))
	g.Expect(mockSlackCli.channels).Should(Equal([]string{"#chan1", "#chan2"}))
	g.Expect(mockSlackCli.message).Should(Equal("some-message"))
	g.Expect(mockSlackCli.username).Should(Equal("samsahai"))
	g.Expect(err).Should(BeNil())
}

func TestSendComponentUpgradeFail(t *testing.T) {
	g := NewGomegaWithT(t)
	mockSlackCli := &mockSlack{}
	s := reporter.NewSlackWithClient("some-token", "", []string{"#chan1", "#chan2"}, mockSlackCli)
	comp := &component.Component{
		Name:    "comp1",
		Version: "1.1.0",
		Image:   &component.Image{Repository: "image-1", Tag: "1.1.0"},
	}
	err := s.SendComponentUpgradeFail(comp, "owner")

	g.Expect(mockSlackCli.postMessageCalls).Should(Equal(2))
	g.Expect(mockSlackCli.channels).Should(Equal([]string{"#chan1", "#chan2"}))
	g.Expect(mockSlackCli.message).Should(Equal("*Component Upgrade Failed* \n\"Stable version of component failed\" - Please check your test\n>*Component:* comp1 \n>*Version:* 1.1.0 \n>*Repository:* image-1 \n>*Issue type:*  \n>*Values file url:*  \n>*CI url:*  \n>*Logs:*  \n>*Error:*  \n>*Owner:* owner"))
	g.Expect(mockSlackCli.username).Should(Equal("Samsahai Notification"))
	g.Expect(err).Should(BeNil())
}

func TestSendActivePromotionStatus(t *testing.T) {
	g := NewGomegaWithT(t)
	mockSlackCli := &mockSlack{}
	s := reporter.NewSlackWithClient("some-token", "", []string{"#chan1", "#chan2"}, mockSlackCli)
	err := s.SendActivePromotionStatus(
		"success",
		"namespace-1",
		"owner",
		[]component.OutdatedComponent{
			{
				CurrentComponent: &component.Component{Name: "comp1", Version: "1.1.0"},
				NewComponent:     &component.Component{Name: "comp1", Version: "1.1.2"},
				OutdatedDays:     1,
			},
			{
				CurrentComponent: &component.Component{Name: "comp2", Version: "1.1.0"},
				NewComponent:     &component.Component{Name: "comp2", Version: "1.1.0"},
				OutdatedDays:     0,
			},
		},
		true,
	)

	g.Expect(mockSlackCli.postMessageCalls).Should(Equal(2))
	g.Expect(mockSlackCli.channels).Should(Equal([]string{"#chan1", "#chan2"}))
	g.Expect(mockSlackCli.message).Should(Equal("*Active Promotion:* SUCCESS \n*Owner:* owner \n*Current Active Namespace:* namespace-1\n*Outdated Components* \n*comp1* \n>Not update for 1 day(s) \n>Current version: 1.1.0 \n>New Version: 1.1.2\n*comp2* \n>Current version: 1.1.0\n"))
	g.Expect(mockSlackCli.username).Should(Equal("Samsahai Notification"))
	g.Expect(err).Should(BeNil())
}

func TestSendOutdatedComponents(t *testing.T) {
	g := NewGomegaWithT(t)
	mockSlackCli := &mockSlack{}
	s := reporter.NewSlackWithClient("some-token", "", []string{"#chan1", "#chan2"}, mockSlackCli)
	err := s.SendOutdatedComponents(
		[]component.OutdatedComponent{
			{
				CurrentComponent: &component.Component{Name: "comp1", Version: "1.1.0"},
				NewComponent:     &component.Component{Name: "comp1", Version: "1.1.2"},
				OutdatedDays:     1,
			},
		},
	)

	g.Expect(mockSlackCli.postMessageCalls).Should(Equal(2))
	g.Expect(mockSlackCli.channels).Should(Equal([]string{"#chan1", "#chan2"}))
	g.Expect(mockSlackCli.message).Should(Equal("*Outdated Components* \n*comp1* \n>Not update for 1 day(s) \n>Current version: 1.1.0 \n>New Version: 1.1.2\n"))
	g.Expect(mockSlackCli.username).Should(Equal("Components Outdated Summary"))
	g.Expect(err).Should(BeNil())
}

func TestSendImageMissingList(t *testing.T) {
	g := NewGomegaWithT(t)
	mockSlackCli := &mockSlack{}
	s := reporter.NewSlackWithClient("some-token", "", []string{"#chan1", "#chan2"}, mockSlackCli)
	err := s.SendImageMissingList(
		[]component.Image{
			{Repository: "registry/comp-1", Tag: "1.0.0"},
		},
	)

	g.Expect(mockSlackCli.postMessageCalls).Should(Equal(2))
	g.Expect(mockSlackCli.channels).Should(Equal([]string{"#chan1", "#chan2"}))
	g.Expect(mockSlackCli.message).Should(Equal("registry/comp-1:1.0.0 (image missing)\n"))
	g.Expect(mockSlackCli.username).Should(Equal("Image Missing Alert"))
	g.Expect(err).Should(BeNil())
}

// Ensure mockSlack implements Slack
var _ slack.Slack = &mockSlack{}

// mockReporter mocks Reporter interface
type mockSlack struct {
	postMessageCalls int
	channels         []string
	message          string
	username         string
}

// PostMessage mocks PostMessage function
func (s *mockSlack) PostMessage(channelNameOrID, message, username string) (channelID, timestamp string, err error) {
	s.postMessageCalls++
	s.channels = append(s.channels, channelNameOrID)
	s.message = message
	s.username = username

	return channelNameOrID, "", nil
}
