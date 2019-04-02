package msg

import (
	"testing"

	"github.com/agoda-com/samsahai/internal/samsahai/component"
	. "github.com/onsi/gomega"
)

func TestNewComponentUpgradeFail(t *testing.T) {
	g := NewGomegaWithT(t)
	upgradeFail := NewComponentUpgradeFail(
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
	)
	g.Expect(upgradeFail).Should(Equal(&ComponentUpgradeFail{
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
	}))
}

func TestNewComponentUpgradeFailMessage(t *testing.T) {
	g := NewGomegaWithT(t)
	tests := map[string]struct {
		in  *ComponentUpgradeFail
		out string
	}{
		"should get component upgrade fail message correctly with full params": {
			in: &ComponentUpgradeFail{
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
			},
			out: "*Component Upgrade Failed* \n\"Stable version of component failed\" - Please check your test\n>*Component:* comp1 \n>*Version:* 1.1.0 \n>*Repository:* image-1 \n>*Issue type:* issue1 \n>*Values file url:* <values-url|Click here> \n>*CI url:* <ci-url|Click here> \n>*Logs:* <logs-url|Click here> \n>*Error:* <error-url|Click here> \n>*Owner:* owner",
		},
		"should get component upgrade fail message correctly with some params": {
			in: &ComponentUpgradeFail{
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
			},
			out: "*Component Upgrade Failed* \n\"Stable version of component failed\" - Please check your test\n>*Component:* comp1 \n>*Version:* 1.1.0 \n>*Repository:* image-1 \n>*Issue type:* issue1 \n>*Values file url:*  \n>*CI url:*  \n>*Logs:*  \n>*Error:*  \n>*Owner:* owner",
		},
	}

	for desc, test := range tests {
		message := test.in.NewComponentUpgradeFailMessage()
		g.Expect(message).Should(Equal(test.out), desc)
	}
}
