package slack

import (
	"bytes"
	"log"
	"strings"
	"text/template"
)

// getStatusText gets the readable active promotion status
func getStatusText(status string) string {
	switch strings.ToLower(status) {
	case "success":
		return "SUCCESS"
	default:
		return "FAILURE"
	}
}

// render creates output string from the template
func render(name, tmpl string, data interface{}) string {
	var engine *template.Template
	var err error

	defer func() {
		if err != nil {
			log.Fatalf("cannot render template: %s , %v", name, err)
		}
	}()

	if engine, err = template.New(name).Parse(tmpl); err != nil {
		return ""
	}
	var output bytes.Buffer
	if err = engine.Execute(&output, data); err != nil {
		return ""
	}

	return output.String()
}
