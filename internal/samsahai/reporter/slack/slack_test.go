package slack_test

import (
	"testing"

	"github.com/pkg/errors"

	"github.com/agoda-com/samsahai/internal/samsahai/reporter"

	"github.com/agoda-com/samsahai/internal/samsahai/component"
	"github.com/agoda-com/samsahai/internal/samsahai/reporter/slack"
	slackCli "github.com/agoda-com/samsahai/internal/samsahai/util/slack"
	. "github.com/onsi/gomega"
)

func TestSlack_MakeComponentUpgradeFailReport(t *testing.T) {
	g := NewGomegaWithT(t)
	tests := map[string]struct {
		in  *reporter.ComponentUpgradeFail
		out string
	}{
		"should get component upgrade fail message correctly with full params": {
			in: reporter.NewComponentUpgradeFail(
				&component.Component{
					Name: "comp1", Version: "1.1.0",
					Image: &component.Image{Repository: "image-1", Tag: "1.1.0"},
				},
				"owner",
				"issue1",
				"values-url",
				"ci-url",
				"logs-url",
				"error-url",
			),
			out: `*Component Upgrade Failed*
"Stable version of component failed" - Please check your test
>*Component:* comp1
>*Version:* 1.1.0
>*Repository:* image-1
>*Issue type:* issue1
>*Values file url:* <values-url|Click here>
>*CI url:* <ci-url|Click here>
>*Logs:* <logs-url|Click here>
>*Error:* <error-url|Click here>
>*Owner:* owner`,
		},
		"should get component upgrade fail message correctly with some params": {
			in: reporter.NewComponentUpgradeFail(
				&component.Component{
					Name: "comp1", Version: "1.1.0",
					Image: &component.Image{Repository: "image-1", Tag: "1.1.0"},
				},
				"owner",
				"issue1",
				"",
				"",
				"",
				"",
			),
			out: `*Component Upgrade Failed*
"Stable version of component failed" - Please check your test
>*Component:* comp1
>*Version:* 1.1.0
>*Repository:* image-1
>*Issue type:* issue1
>*Values file url:* -
>*CI url:* -
>*Logs:* -
>*Error:* -
>*Owner:* owner`,
		},
	}

	s := slack.NewSlack("some-token", "samsahai", []string{"#chan1"})
	for desc, test := range tests {
		message := s.MakeComponentUpgradeFailReport(test.in)
		g.Expect(message).Should(Equal(test.out), desc)
	}
}

func TestSlack_MakeActivePromotionStatusReport(t *testing.T) {
	g := NewGomegaWithT(t)
	atv := reporter.NewActivePromotion("success", "owner", "namespace-1", []component.OutdatedComponent{
		{
			CurrentComponent: &component.Component{Name: "comp1", Version: "1.1.0"},
			NewComponent:     &component.Component{Name: "comp1", Version: "1.1.2"},
			OutdatedDays:     1,
		},
	})

	s := slack.NewSlack("some-token", "samsahai", []string{"#chan1"})
	message := s.MakeActivePromotionStatusReport(atv, reporter.NewOptionShowedDetails(false))
	g.Expect(message).Should(Equal(`*Active Promotion:* SUCCESS
*Owner:* owner
*Current Active Namespace:* namespace-1
*Outdated Components*
*comp1*
>Not update for 1 day(s)
>Current version: 1.1.0
>New Version: 1.1.2`))
}

func TestSlack_MakeOutdatedComponentsReport(t *testing.T) {
	g := NewGomegaWithT(t)
	oc := reporter.NewOutdatedComponents(
		[]component.OutdatedComponent{
			{
				CurrentComponent: &component.Component{Name: "comp1", Version: "1.1.0"},
				NewComponent:     &component.Component{Name: "comp1", Version: "1.1.2"},
				OutdatedDays:     1,
			},
		}, true)

	s := slack.NewSlack("some-token", "samsahai", []string{"#chan1"})
	message := s.MakeOutdatedComponentsReport(oc)
	g.Expect(message).Should(Equal(`*Outdated Components*
*comp1*
>Not update for 1 day(s)
>Current version: 1.1.0
>New Version: 1.1.2`))
}

func TestSlack_MakeImageMissingListReport(t *testing.T) {
	g := NewGomegaWithT(t)
	im := reporter.NewImageMissing([]component.Image{
		{Repository: "registry/comp-1", Tag: "1.0.0"},
		{Repository: "registry/comp-2", Tag: "1.0.1-rc"},
	})

	s := slack.NewSlack("some-token", "samsahai", []string{"#chan1"})
	message := s.MakeImageMissingListReport(im)
	g.Expect(message).Should(Equal("registry/comp-1:1.0.0 (image missing)\nregistry/comp-2:1.0.1-rc (image missing)"))
}

func TestSlack_SendMessage(t *testing.T) {
	g := NewGomegaWithT(t)
	mockSlackCli := &mockSlack{}
	s := slack.NewSlackWithClient("some-token", "", []string{"#chan1", "#chan2"}, mockSlackCli)
	err := s.SendMessage("some-message")

	g.Expect(mockSlackCli.postMessageCalls).Should(Equal(2))
	g.Expect(mockSlackCli.channels).Should(Equal([]string{"#chan1", "#chan2"}))
	g.Expect(mockSlackCli.message).Should(Equal("some-message"))
	g.Expect(mockSlackCli.username).Should(Equal("Samsahai Notification"))
	g.Expect(err).Should(BeNil())
}

func TestSlack_SendMessageFailed(t *testing.T) {
	g := NewGomegaWithT(t)
	mockSlackCli := &mockSlack{}
	s := slack.NewSlackWithClient("some-token", "error", []string{"#chan1", "#chan2"}, mockSlackCli)
	err := s.SendMessage("some-message")
	g.Expect(err).ShouldNot(BeNil())
}

func TestSlack_SendComponentUpgradeFail(t *testing.T) {
	g := NewGomegaWithT(t)
	mockSlackCli := &mockSlack{}
	s := slack.NewSlackWithClient("some-token", "", []string{"#chan1", "#chan2"}, mockSlackCli)
	comp := &component.Component{
		Name:    "comp1",
		Version: "1.1.0",
		Image:   &component.Image{Repository: "image-1", Tag: "1.1.0"},
	}
	opts := []reporter.Option{
		reporter.NewOptionValuesFileURL("url-to-values"),
		reporter.NewOptionIssueType("some-issue-type"),
		reporter.NewOptionCIURL("url-to-ci"),
		reporter.NewOptionLogsURL("url-to-logs"),
		reporter.NewOptionErrorURL("url-to-error"),
	}
	err := s.SendComponentUpgradeFail(comp, "owner1", opts...)

	g.Expect(err).Should(BeNil())

	g.Expect(mockSlackCli.postMessageCalls).Should(Equal(2))
	g.Expect(mockSlackCli.channels).Should(Equal([]string{"#chan1", "#chan2"}))
	g.Expect(mockSlackCli.username).Should(Equal("Samsahai Notification"))
	g.Expect(mockSlackCli.message).Should(ContainSubstring("Component Upgrade Failed"))
	// Should contain information
	g.Expect(mockSlackCli.message).Should(ContainSubstring("comp1"))
	g.Expect(mockSlackCli.message).Should(ContainSubstring("1.1.0"))
	g.Expect(mockSlackCli.message).Should(ContainSubstring("image-1"))
	g.Expect(mockSlackCli.message).Should(ContainSubstring("some-issue-type"))
	g.Expect(mockSlackCli.message).Should(ContainSubstring("<url-to-values|Click here>"))
	g.Expect(mockSlackCli.message).Should(ContainSubstring("<url-to-ci|Click here>"))
	g.Expect(mockSlackCli.message).Should(ContainSubstring("<url-to-logs|Click here>"))
	g.Expect(mockSlackCli.message).Should(ContainSubstring("<url-to-error|Click here>"))
	g.Expect(mockSlackCli.message).Should(ContainSubstring("owner1"))
}

func TestSlack_SendActivePromotionStatus(t *testing.T) {
	g := NewGomegaWithT(t)
	mockSlackCli := &mockSlack{}
	s := slack.NewSlackWithClient("some-token", "", []string{"#chan1", "#chan2"}, mockSlackCli)
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
		reporter.NewOptionShowedDetails(true),
	)

	g.Expect(mockSlackCli.postMessageCalls).Should(Equal(2))
	g.Expect(mockSlackCli.channels).Should(Equal([]string{"#chan1", "#chan2"}))
	g.Expect(mockSlackCli.message).Should(Equal(`*Active Promotion:* SUCCESS
*Owner:* owner
*Current Active Namespace:* namespace-1
*Outdated Components*
*comp1*
>Not update for 1 day(s)
>Current version: 1.1.0
>New Version: 1.1.2
*comp2*
>Current version: 1.1.0`))
	g.Expect(mockSlackCli.username).Should(Equal("Samsahai Notification"))
	g.Expect(err).Should(BeNil())
}

func TestSlack_SendActivePromotionStatusFailure(t *testing.T) {
	g := NewGomegaWithT(t)
	mockSlackCli := &mockSlack{}
	s := slack.NewSlackWithClient("some-token", "", []string{"#chan1", "#chan2"}, mockSlackCli)
	err := s.SendActivePromotionStatus(
		"failed",
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
		reporter.NewOptionShowedDetails(true),
	)

	g.Expect(mockSlackCli.postMessageCalls).Should(Equal(2))
	g.Expect(mockSlackCli.channels).Should(Equal([]string{"#chan1", "#chan2"}))
	g.Expect(mockSlackCli.message).Should(ContainSubstring("FAILURE"))
	g.Expect(mockSlackCli.username).Should(Equal("Samsahai Notification"))
	g.Expect(err).Should(BeNil())
}

func TestSlack_SendOutdatedComponents(t *testing.T) {
	g := NewGomegaWithT(t)
	mockSlackCli := &mockSlack{}
	s := slack.NewSlackWithClient("some-token", "", []string{"#chan1", "#chan2"}, mockSlackCli)
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
	g.Expect(mockSlackCli.message).Should(Equal(`*Outdated Components*
*comp1*
>Not update for 1 day(s)
>Current version: 1.1.0
>New Version: 1.1.2`))
	g.Expect(mockSlackCli.username).Should(Equal("Components Outdated Summary"))
	g.Expect(err).Should(BeNil())
}

func TestSlack_SendImageMissingList(t *testing.T) {
	g := NewGomegaWithT(t)
	mockSlackCli := &mockSlack{}
	s := slack.NewSlackWithClient("some-token", "", []string{"#chan1", "#chan2"}, mockSlackCli)
	err := s.SendImageMissingList(
		[]component.Image{
			{Repository: "registry/comp-1", Tag: "1.0.0"},
		},
	)

	g.Expect(mockSlackCli.postMessageCalls).Should(Equal(2))
	g.Expect(mockSlackCli.channels).Should(Equal([]string{"#chan1", "#chan2"}))
	g.Expect(mockSlackCli.message).Should(Equal("registry/comp-1:1.0.0 (image missing)"))
	g.Expect(mockSlackCli.username).Should(Equal("Image Missing Alert"))
	g.Expect(err).Should(BeNil())
}

// Ensure mockSlack implements Slack
var _ slackCli.Slack = &mockSlack{}

// mockSlack mocks Slack interface
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

	if username == "error" {
		return channelNameOrID, "", errors.New("error")
	}
	return channelNameOrID, "", nil
}
