package email_test

import (
	"testing"

	"github.com/agoda-com/samsahai/internal/samsahai/reporter/email"

	"github.com/agoda-com/samsahai/internal/samsahai/reporter"

	"github.com/agoda-com/samsahai/internal/samsahai/component"
	emailCli "github.com/agoda-com/samsahai/internal/samsahai/util/email"

	. "github.com/onsi/gomega"
)

func TestEmail_MakeComponentUpgradeFailReport(t *testing.T) {
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
			out: `<!DOCTYPE html>
<html lang="en">
<head>
    <link rel="stylesheet" href="https://www.w3schools.com/w3css/4/w3.css">
    <style>
        img {
            display: block;
            margin-left: auto;
            margin-right: auto;
        }
        * {
            margin:5px;
            padding:5px;
        }
    </style>
</head>
<body>
    <div><table class="w3-table-all w3-hoverable">
        <thead>
        <tr colspan="2" class="w3-light-grey">
            <th colspan="2">Component Upgrade Fail</th>
        </tr>
        </thead>
        <tr>
            <th>Component Name: </th>
            <td><font color="black">comp1</font></td>
        </tr>
        <tr>
            <th>Component Version: </th>
            <td><font color="black">1.1.0</font></td>
        </tr>
        <tr>
            <th>Component Repository: </th>
            <td><font color="black">image-1</font></td>
        </tr>
        <tr>
            <th>Issue Type</th>
            <td><font color="red">issue1</font></td>
        </tr>
        <tr>
            <th>Value file Url: </th>
            <td><a href="values-url">Click here</a></td>
        </tr>
        <tr>
            <th>CI Url: </th>
            <td><a href="ci-url">Click here</a></td>
        </tr>
        <tr>
            <th>Logs: </th>
            <td><a href="logs-url">Click here</a></td>
        </tr>
        <tr>
            <th>Error </th>
            <td><a href="error-url">Click here</a></td>
        </tr>
        <tr>
            <th>Owner: </th>
            <td><font color="black">owner</font></td>
        </tr>
    </table>
    </div>
</body>
</html>`,
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
			out: `<!DOCTYPE html>
<html lang="en">
<head>
    <link rel="stylesheet" href="https://www.w3schools.com/w3css/4/w3.css">
    <style>
        img {
            display: block;
            margin-left: auto;
            margin-right: auto;
        }
        * {
            margin:5px;
            padding:5px;
        }
    </style>
</head>
<body>
    <div><table class="w3-table-all w3-hoverable">
        <thead>
        <tr colspan="2" class="w3-light-grey">
            <th colspan="2">Component Upgrade Fail</th>
        </tr>
        </thead>
        <tr>
            <th>Component Name: </th>
            <td><font color="black">comp1</font></td>
        </tr>
        <tr>
            <th>Component Version: </th>
            <td><font color="black">1.1.0</font></td>
        </tr>
        <tr>
            <th>Component Repository: </th>
            <td><font color="black">image-1</font></td>
        </tr>
        <tr>
            <th>Issue Type</th>
            <td><font color="red">issue1</font></td>
        </tr>
        <tr>
            <th>Value file Url: </th>
            <td></td>
        </tr>
        <tr>
            <th>CI Url: </th>
            <td></td>
        </tr>
        <tr>
            <th>Logs: </th>
            <td></td>
        </tr>
        <tr>
            <th>Error </th>
            <td></td>
        </tr>
        <tr>
            <th>Owner: </th>
            <td><font color="black">owner</font></td>
        </tr>
    </table>
    </div>
</body>
</html>`,
		},
	}

	e := email.NewEmail("some-server", 111, []string{"notification@samsahai.com"}, []string{"a@some.com", "b@dome.com"})
	for desc, test := range tests {
		message := e.MakeComponentUpgradeFailReport(test.in)
		g.Expect(message).Should(Equal(test.out), desc)
	}
}

func TestEmail_MakeActivePromotionStatusReport(t *testing.T) {
	g := NewGomegaWithT(t)
	atv := reporter.NewActivePromotion("success", "owner", "namespace-1", []component.OutdatedComponent{
		{
			CurrentComponent: &component.Component{Name: "comp1", Version: "1.1.0"},
			NewComponent:     &component.Component{Name: "comp1", Version: "1.1.2"},
			OutdatedDays:     1,
		},
	})

	e := email.NewEmail("some-server", 111, []string{"notification@samsahai.com"}, []string{"a@some.com", "b@dome.com"})
	message := e.MakeActivePromotionStatusReport(atv, reporter.NewOptionShowedDetails(false))
	g.Expect(message).Should(Equal(
		`<!DOCTYPE html>
<html lang="en">
<head>
    <link rel="stylesheet" href="https://www.w3schools.com/w3css/4/w3.css">
    <style>
        img {
            display: block;
            margin-left: auto;
            margin-right: auto;
        }
        * {
            margin:5px;
            padding:5px;
        }
    </style>
</head>
<body>
    <div><table class="w3-table-all w3-hoverable">
        <thead>
        <tr colspan="2" class="w3-light-grey">
            <th colspan="2">Active Promotion Status</th>
        </tr>
        </thead>
        <tr>
            <th>Active promotion:</th>
            <td><font color="black">success</font></td>
        </tr>
        <tr>
            <th>Owner:</th>
            <td><font color="black">owner</font></td>
        </tr>
        <tr>
            <th>Current active namespace:</th>
            <td><font color="black">namespace-1</font></td>
        </tr><tr colspan="2">
            <th colspan="2">comp1</th>
        </tr>
        <tr>
            <th>&emsp;&emsp;&emsp;&emsp;Not update for</th>
            <td><font color="red">1 day(s)</font></td>
        </tr>
        <tr>
            <th>&emsp;&emsp;&emsp;&emsp;Current version:</th>
            <td><font color="black">1.1.0</font></td>
        </tr>
        <tr>
            <th>&emsp;&emsp;&emsp;&emsp;Latest Version:</th>
            <td><font color="black">1.1.2</font></td>
        </tr></table>
    </div>
</body>
</html>`,
	))
}

func TestEmail_MakeOutdatedComponentsReport(t *testing.T) {
	g := NewGomegaWithT(t)
	oc := reporter.NewOutdatedComponents(
		[]component.OutdatedComponent{
			{
				CurrentComponent: &component.Component{Name: "comp1", Version: "1.1.0"},
				NewComponent:     &component.Component{Name: "comp1", Version: "1.1.2"},
				OutdatedDays:     1,
			},
		}, true)

	e := email.NewEmail("some-server", 111, []string{"notification@samsahai.com"}, []string{"a@some.com", "b@dome.com"})
	message := e.MakeOutdatedComponentsReport(oc)
	g.Expect(message).Should(Equal(
		`<!DOCTYPE html>
<html lang="en">
<head>
    <link rel="stylesheet" href="https://www.w3schools.com/w3css/4/w3.css">
    <style>
        img {
            display: block;
            margin-left: auto;
            margin-right: auto;
        }
        * {
            margin:5px;
            padding:5px;
        }
    </style>
</head>
<body>
    <div><table class="w3-table-all w3-hoverable">
        <thead>
        <tr colspan="2" class="w3-light-grey">
            <th colspan="2">Component Outdated Summary</th>
        </tr>
        </thead>
        <tr colspan="2">
            <th colspan="2">comp1</th>
        </tr>
        <tr>
            <th>&emsp;&emsp;&emsp;&emsp;Not update for</th>
            <td><font color="red">1 day(s)</font></td>
        </tr>
        <tr>
            <th>&emsp;&emsp;&emsp;&emsp;Current version:</th>
            <td><font color="black">1.1.0</font></td>
        </tr>
        <tr>
            <th>&emsp;&emsp;&emsp;&emsp;Latest Version:</th>
            <td><font color="black">1.1.2</font></td>
        </tr>
    </table>
    </div>
</body>
</html>`,
	))
}

func TestEmail_MakeImageMissingListReport(t *testing.T) {
	g := NewGomegaWithT(t)
	im := reporter.NewImageMissing([]component.Image{
		{Repository: "registry/comp-1", Tag: "1.0.0"},
		{Repository: "registry/comp-2", Tag: "1.0.1-rc"},
	})

	e := email.NewEmail("some-server", 111, []string{"notification@samsahai.com"}, []string{"a@some.com", "b@dome.com"})
	message := e.MakeImageMissingListReport(im)
	g.Expect(message).Should(Equal(
		`<!DOCTYPE html>
<html lang="en">
<head>
    <link rel="stylesheet" href="https://www.w3schools.com/w3css/4/w3.css">
    <style>
        img {
            display: block;
            margin-left: auto;
            margin-right: auto;
        }
        * {
            margin:5px;
            padding:5px;
        }
    </style>
</head>
<body>
    <div><table class="w3-table-all w3-hoverable">
        <thead>
        <tr colspan="2" class="w3-light-grey">
            <th colspan="2">Image Missing List</th>
        </tr>
        </thead>
        <tr>
            <td><font color="black">registry/comp-1:1.0.0</font></td>
        </tr>
        <tr>
            <td><font color="black">registry/comp-2:1.0.1-rc</font></td>
        </tr>
    </table>
    </div>
</body>
</html>`,
	))
}

func TestEmail_SendMessage(t *testing.T) {
	g := NewGomegaWithT(t)
	mockEmailCli := &mockEmail{}
	e := email.NewEmailWithClient("some-server", 111, []string{"notification@samsahai.com"}, []string{"a@some.com", "b@dome.com"}, mockEmailCli)
	err := e.SendMessage("some-body", reporter.NewOptionSubject("samsahai notification"))

	g.Expect(mockEmailCli.sendMessageCalls).Should(Equal(1))
	g.Expect(mockEmailCli.from).Should(Equal([]string{"notification@samsahai.com"}))
	g.Expect(mockEmailCli.to).Should(Equal([]string{"a@some.com", "b@dome.com"}))
	g.Expect(mockEmailCli.subject).Should(Equal("samsahai notification"))
	g.Expect(mockEmailCli.body).Should(Equal("some-body"))
	g.Expect(err).Should(BeNil())
}

func TestEmail_SendComponentUpgradeFail(t *testing.T) {
	g := NewGomegaWithT(t)
	mockEmailCli := &mockEmail{}
	e := email.NewEmailWithClient("some-server", 111, []string{"notification@samsahai.com"}, []string{"a@some.com", "b@dome.com"}, mockEmailCli)
	comp := &component.Component{
		Name:    "comp1",
		Version: "1.1.0",
		Image:   &component.Image{Repository: "image-1", Tag: "1.1.0"},
	}

	optIssueType := reporter.NewOptionIssueType("issue1")
	optValuesFile := reporter.NewOptionValuesFileURL("values-url")
	optCIURL := reporter.NewOptionCIURL("ci-url")
	err := e.SendComponentUpgradeFail(comp, "owner", optIssueType, optValuesFile, optCIURL)

	g.Expect(mockEmailCli.sendMessageCalls).Should(Equal(1))
	g.Expect(mockEmailCli.from).Should(Equal([]string{"notification@samsahai.com"}))
	g.Expect(mockEmailCli.to).Should(Equal([]string{"a@some.com", "b@dome.com"}))
	g.Expect(mockEmailCli.subject).Should(Equal("Component Upgrade Failed"))
	g.Expect(mockEmailCli.body).Should(Equal(
		`<!DOCTYPE html>
<html lang="en">
<head>
    <link rel="stylesheet" href="https://www.w3schools.com/w3css/4/w3.css">
    <style>
        img {
            display: block;
            margin-left: auto;
            margin-right: auto;
        }
        * {
            margin:5px;
            padding:5px;
        }
    </style>
</head>
<body>
    <div><table class="w3-table-all w3-hoverable">
        <thead>
        <tr colspan="2" class="w3-light-grey">
            <th colspan="2">Component Upgrade Fail</th>
        </tr>
        </thead>
        <tr>
            <th>Component Name: </th>
            <td><font color="black">comp1</font></td>
        </tr>
        <tr>
            <th>Component Version: </th>
            <td><font color="black">1.1.0</font></td>
        </tr>
        <tr>
            <th>Component Repository: </th>
            <td><font color="black">image-1</font></td>
        </tr>
        <tr>
            <th>Issue Type</th>
            <td><font color="red">issue1</font></td>
        </tr>
        <tr>
            <th>Value file Url: </th>
            <td><a href="values-url">Click here</a></td>
        </tr>
        <tr>
            <th>CI Url: </th>
            <td><a href="ci-url">Click here</a></td>
        </tr>
        <tr>
            <th>Logs: </th>
            <td></td>
        </tr>
        <tr>
            <th>Error </th>
            <td></td>
        </tr>
        <tr>
            <th>Owner: </th>
            <td><font color="black">owner</font></td>
        </tr>
    </table>
    </div>
</body>
</html>`,
	))
	g.Expect(err).Should(BeNil())
}

func TestEmail_SendActivePromotionStatus(t *testing.T) {
	g := NewGomegaWithT(t)
	mockEmailCli := &mockEmail{}
	e := email.NewEmailWithClient("some-server", 111, []string{"notification@samsahai.com"}, []string{"a@some.com", "b@dome.com"}, mockEmailCli)
	err := e.SendActivePromotionStatus(
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

	g.Expect(mockEmailCli.sendMessageCalls).Should(Equal(1))
	g.Expect(mockEmailCli.from).Should(Equal([]string{"notification@samsahai.com"}))
	g.Expect(mockEmailCli.to).Should(Equal([]string{"a@some.com", "b@dome.com"}))
	g.Expect(mockEmailCli.subject).Should(Equal("Active Promotion: SUCCESS"))
	g.Expect(mockEmailCli.body).Should(Equal(
		`<!DOCTYPE html>
<html lang="en">
<head>
    <link rel="stylesheet" href="https://www.w3schools.com/w3css/4/w3.css">
    <style>
        img {
            display: block;
            margin-left: auto;
            margin-right: auto;
        }
        * {
            margin:5px;
            padding:5px;
        }
    </style>
</head>
<body>
    <div><table class="w3-table-all w3-hoverable">
        <thead>
        <tr colspan="2" class="w3-light-grey">
            <th colspan="2">Active Promotion Status</th>
        </tr>
        </thead>
        <tr>
            <th>Active promotion:</th>
            <td><font color="black">SUCCESS</font></td>
        </tr>
        <tr>
            <th>Owner:</th>
            <td><font color="black">owner</font></td>
        </tr>
        <tr>
            <th>Current active namespace:</th>
            <td><font color="black">namespace-1</font></td>
        </tr><tr colspan="2">
            <th colspan="2">comp1</th>
        </tr>
        <tr>
            <th>&emsp;&emsp;&emsp;&emsp;Not update for</th>
            <td><font color="red">1 day(s)</font></td>
        </tr>
        <tr>
            <th>&emsp;&emsp;&emsp;&emsp;Current version:</th>
            <td><font color="black">1.1.0</font></td>
        </tr>
        <tr>
            <th>&emsp;&emsp;&emsp;&emsp;Latest Version:</th>
            <td><font color="black">1.1.2</font></td>
        </tr><tr colspan="2">
            <th colspan="2">comp2</th>
        </tr>
        <tr>
            <th>&emsp;&emsp;&emsp;&emsp;Current version:</th>
            <td><font color="black">1.1.0</font></td>
        </tr></table>
    </div>
</body>
</html>`,
	))
	g.Expect(err).Should(BeNil())
}

func TestEmail_SendOutdatedComponents(t *testing.T) {
	g := NewGomegaWithT(t)
	mockEmailCli := &mockEmail{}
	e := email.NewEmailWithClient("some-server", 111, []string{"notification@samsahai.com"}, []string{"a@some.com", "b@dome.com"}, mockEmailCli)
	err := e.SendOutdatedComponents(
		[]component.OutdatedComponent{
			{
				CurrentComponent: &component.Component{Name: "comp1", Version: "1.1.0"},
				NewComponent:     &component.Component{Name: "comp1", Version: "1.1.2"},
				OutdatedDays:     1,
			},
		},
	)

	g.Expect(mockEmailCli.sendMessageCalls).Should(Equal(1))
	g.Expect(mockEmailCli.from).Should(Equal([]string{"notification@samsahai.com"}))
	g.Expect(mockEmailCli.to).Should(Equal([]string{"a@some.com", "b@dome.com"}))
	g.Expect(mockEmailCli.subject).Should(Equal("Components Outdated Summary"))
	g.Expect(mockEmailCli.body).Should(Equal(
		`<!DOCTYPE html>
<html lang="en">
<head>
    <link rel="stylesheet" href="https://www.w3schools.com/w3css/4/w3.css">
    <style>
        img {
            display: block;
            margin-left: auto;
            margin-right: auto;
        }
        * {
            margin:5px;
            padding:5px;
        }
    </style>
</head>
<body>
    <div><table class="w3-table-all w3-hoverable">
        <thead>
        <tr colspan="2" class="w3-light-grey">
            <th colspan="2">Component Outdated Summary</th>
        </tr>
        </thead>
        <tr colspan="2">
            <th colspan="2">comp1</th>
        </tr>
        <tr>
            <th>&emsp;&emsp;&emsp;&emsp;Not update for</th>
            <td><font color="red">1 day(s)</font></td>
        </tr>
        <tr>
            <th>&emsp;&emsp;&emsp;&emsp;Current version:</th>
            <td><font color="black">1.1.0</font></td>
        </tr>
        <tr>
            <th>&emsp;&emsp;&emsp;&emsp;Latest Version:</th>
            <td><font color="black">1.1.2</font></td>
        </tr>
    </table>
    </div>
</body>
</html>`,
	))
	g.Expect(err).Should(BeNil())
}

func TestEmail_SendImageMissingList(t *testing.T) {
	g := NewGomegaWithT(t)
	mockEmailCli := &mockEmail{}
	e := email.NewEmailWithClient("some-server", 111, []string{"notification@samsahai.com"}, []string{"a@some.com", "b@dome.com"}, mockEmailCli)
	err := e.SendImageMissingList(
		[]component.Image{
			{Repository: "registry/comp-1", Tag: "1.0.0"},
		},
	)

	g.Expect(mockEmailCli.sendMessageCalls).Should(Equal(1))
	g.Expect(mockEmailCli.from).Should(Equal([]string{"notification@samsahai.com"}))
	g.Expect(mockEmailCli.to).Should(Equal([]string{"a@some.com", "b@dome.com"}))
	g.Expect(mockEmailCli.subject).Should(Equal("Image Missing Alert"))
	g.Expect(mockEmailCli.body).Should(Equal(
		`<!DOCTYPE html>
<html lang="en">
<head>
    <link rel="stylesheet" href="https://www.w3schools.com/w3css/4/w3.css">
    <style>
        img {
            display: block;
            margin-left: auto;
            margin-right: auto;
        }
        * {
            margin:5px;
            padding:5px;
        }
    </style>
</head>
<body>
    <div><table class="w3-table-all w3-hoverable">
        <thead>
        <tr colspan="2" class="w3-light-grey">
            <th colspan="2">Image Missing List</th>
        </tr>
        </thead>
        <tr>
            <td><font color="black">registry/comp-1:1.0.0</font></td>
        </tr>
    </table>
    </div>
</body>
</html>`,
	))
	g.Expect(err).Should(BeNil())
}

// Ensure mockSlack implements Slack
var _ emailCli.Email = &mockEmail{}

// mockEmail mocks Email interface
type mockEmail struct {
	sendMessageCalls int
	from             []string
	to               []string
	subject          string
	body             string
}

// SendMessage mocks SendMessage function
func (e *mockEmail) SendMessage(from, to []string, subject, body string) error {
	e.sendMessageCalls++
	e.from = from
	e.to = to
	e.subject = subject
	e.body = body

	return nil
}
