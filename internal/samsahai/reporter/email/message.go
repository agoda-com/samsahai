package email

import (
	"bytes"
	"fmt"
	"html/template"
	"strings"

	"github.com/agoda-com/samsahai/internal/samsahai/reporter"
)

func getActivePromotionWithTemplate(atv *reporter.ActivePromotion) (string, error) {
	mainTemplate, err := getMainTemplate()
	if err != nil {
		return "", err
	}

	apTemplate, err := mainTemplate.Clone()
	if err != nil {
		return "", err
	}

	apTemplateStr := baseTmpl + activePromotionTmpl
	apTemplate = template.Must(apTemplate.Parse(apTemplateStr))
	buf := new(bytes.Buffer)
	if err := apTemplate.Execute(buf, atv); err != nil {
		return "", err
	}

	text := buf.String()
	return text, nil
}

func getOutdatedComponentWithTemplate(oc *reporter.OutdatedComponents) (string, error) {
	mainTemplate, err := getMainTemplate()
	if err != nil {
		return "", err
	}

	ocTemplate, err := mainTemplate.Clone()
	if err != nil {
		return "", err
	}

	ocTemplateStr := baseTmpl + outdatedComponentsTmpl
	ocTemplate = template.Must(ocTemplate.Parse(ocTemplateStr))
	buf := new(bytes.Buffer)
	if err := ocTemplate.Execute(buf, oc); err != nil {
		return "", err
	}

	text := buf.String()
	return text, nil
}

func getComponentUpgradeFailWithTemplate(cuf *reporter.ComponentUpgradeFail) (string, error) {
	mainTemplate, err := getMainTemplate()
	if err != nil {
		return "", err
	}

	cufTemplate, err := mainTemplate.Clone()
	if err != nil {
		return "", err
	}

	cufTemplateStr := baseTmpl + componentUpgradeFailTmpl
	cufTemplate = template.Must(cufTemplate.Parse(cufTemplateStr))
	buf := new(bytes.Buffer)
	if err := cufTemplate.Execute(buf, cuf); err != nil {
		return "", err
	}

	text := buf.String()
	return text, nil
}

func getImageMissingWithTemplate(im *reporter.ImageMissing) (string, error) {
	mainTemplate, err := getMainTemplate()
	if err != nil {
		return "", err
	}

	imTemplate, err := mainTemplate.Clone()
	if err != nil {
		return "", err
	}

	imTemplateStr := baseTmpl + imageMissingTmpl
	imTemplate = template.Must(imTemplate.Parse(imTemplateStr))
	buf := new(bytes.Buffer)
	if err := imTemplate.Execute(buf, im); err != nil {
		return "", err
	}

	text := buf.String()
	return text, nil
}

func getMainTemplate() (*template.Template, error) {
	mainTemplateStr := `{{ define "main" }} {{- template "baseTmpl" . -}} {{ end }}`
	mainTemplate, err := template.New("main").Parse(mainTemplateStr)
	if err != nil {
		return nil, err
	}

	return mainTemplate, nil
}

// getSubject gets default email subject if not defined
func getSubject(notiType, content, subject string) string {
	if subject != "" {
		return subject
	}

	switch notiType {
	case reporter.TypeImageMissing:
		return "Image Missing Alert"
	case reporter.TypeOutdatedComponent:
		return "Components Outdated Summary"
	case reporter.TypeActivePromotion:
		return fmt.Sprintf("Active Promotion: %s", content)
	case reporter.TypeComponentUpgradeFail:
		return "Component Upgrade Failed"
	default:
		return "Samsahai Notification"
	}
}

// getStatusText gets the readable active promotion status
func getStatusText(status string) string {
	switch strings.ToLower(status) {
	case "success":
		return "SUCCESS"
	default:
		return "FAIL"
	}
}
