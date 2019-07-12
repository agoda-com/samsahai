package template

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
	"time"

	s2hlog "github.com/agoda-com/samsahai/internal/log"
)

var logger = s2hlog.Log.WithName("reporter-utils")

// TextRender creates output string from the template
func TextRender(name, tmpl string, data interface{}) string {
	var engine *template.Template
	var err error

	funcMap := template.FuncMap{
		"ToLower":             strings.ToLower,
		"ToUpper":             strings.ToUpper,
		"FmtDurationToStr":    fmtDurationToStr,
		"ConcatHTTPStr":       concatHTTPStr,
		"JoinStringWithComma": joinStringWithComma,
	}

	defer func() {
		if err != nil {
			logger.Error(err, fmt.Sprintf("cannot render template: %s", name))
		}
	}()

	if engine, err = template.New(name).Funcs(funcMap).Parse(tmpl); err != nil {
		return ""
	}
	var output bytes.Buffer
	if err = engine.Execute(&output, data); err != nil {
		return ""
	}

	return output.String()
}

func fmtDurationToStr(duration time.Duration) string {
	d := duration / (24 * time.Hour)
	duration -= d * (24 * time.Hour)
	h := duration / time.Hour
	duration -= h * time.Hour
	m := duration / time.Minute

	return fmt.Sprintf("%dd %dh %dm", d, h, m)
}

func concatHTTPStr(repository string) string {
	return fmt.Sprintf("http://%s", repository)
}

func joinStringWithComma(str []string) string {
	out := ""
	for _, s := range str {
		out += fmt.Sprintf(`"%s",`, s)
	}

	return strings.TrimSuffix(out, ",")
}
