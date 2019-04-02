package msg

import (
	"fmt"

	"github.com/agoda-com/samsahai/internal/samsahai/component"
)

// ComponentUpgradeFail manages components upgrade fail
type ComponentUpgradeFail struct {
	Component     *component.Component `required:"true"`
	ServiceOwner  string               `required:"true"`
	IssueType     string
	ValuesFileURL string
	CIURL         string
	LogsURL       string
	ErrorURL      string
}

// NewComponentUpgradeFail creates a new component upgrade fail
func NewComponentUpgradeFail(component *component.Component, serviceOwner, issueType, valuesFileURL, ciURL, logsURL, errorURL string) *ComponentUpgradeFail {
	upgradeFail := &ComponentUpgradeFail{
		Component:     component,
		ServiceOwner:  serviceOwner,
		IssueType:     issueType,
		ValuesFileURL: valuesFileURL,
		CIURL:         ciURL,
		LogsURL:       logsURL,
		ErrorURL:      errorURL,
	}

	return upgradeFail
}

// NewComponentUpgradeFailMessage creates a component upgrade fail message
func (upgradeFail *ComponentUpgradeFail) NewComponentUpgradeFailMessage() string {
	message := getComponentUpgradeFailTitle()
	message += getComponentUpgradeFailMessage(upgradeFail)
	return message
}

func getComponentUpgradeFailTitle() string {
	return fmt.Sprintln("*Component Upgrade Failed* \n\"Stable version of component failed\" - Please check your test")
}

func getComponentUpgradeFailMessage(upgradeFail *ComponentUpgradeFail) string {
	var valuesFile, ciURL, logsURL, errorURL string
	if valuesFile = upgradeFail.ValuesFileURL; valuesFile != "" {
		valuesFile = fmt.Sprintf("<%s|Click here>", upgradeFail.ValuesFileURL)
	}
	if ciURL = upgradeFail.CIURL; ciURL != "" {
		ciURL = fmt.Sprintf("<%s|Click here>", upgradeFail.CIURL)
	}
	if logsURL = upgradeFail.LogsURL; logsURL != "" {
		logsURL = fmt.Sprintf("<%s|Click here>", upgradeFail.LogsURL)
	}
	if errorURL = upgradeFail.ErrorURL; logsURL != "" {
		errorURL = fmt.Sprintf("<%s|Click here>", upgradeFail.ErrorURL)
	}
	return fmt.Sprintf(">*Component:* %s \n>*Version:* %s \n>*Repository:* %s \n>*Issue type:* %s \n>*Values file url:* %s \n>*CI url:* %s \n>*Logs:* %s \n>*Error:* %s \n>*Owner:* %s",
		upgradeFail.Component.Name, upgradeFail.Component.Version, upgradeFail.Component.Image.Repository,
		upgradeFail.IssueType, valuesFile, ciURL, logsURL, errorURL, upgradeFail.ServiceOwner,
	)
}
