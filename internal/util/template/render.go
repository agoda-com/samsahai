package template

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"text/template"
	"time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	s2herrors "github.com/agoda-com/samsahai/internal/errors"
	s2hlog "github.com/agoda-com/samsahai/internal/log"
)

var logger = s2hlog.Log.WithName("template-utils")

const (
	startReplacedSign = "<<s2h<<"
	endReplacedSign   = ">>s2h>>"

	startValTemplateSign = "{{"
	endValTemplateSign   = "}}"
)

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
		"TimeFormat":          timeFormat,
	}

	defer func() {
		if err != nil {
			logger.Warnf("cannot render template: %s, %v", name, err)
		}
	}()

	outCh := make(chan string, 1)

	timeout := 5 * time.Second
	var output bytes.Buffer

	// ignore rendering value if it is missing
	go func() {
		for {
			output.Reset()
			if engine, err = template.New(name).Option("missingkey=error").Funcs(funcMap).Parse(tmpl); err != nil {
				tmpl, ok := replaceMissingValuesFromError(tmpl, err)
				if !ok {
					outCh <- tmpl
					break
				}
			}

			if err = engine.Execute(&output, data); err != nil {
				var ok bool
				tmpl, ok = replaceMissingValuesFromError(tmpl, err)
				if !ok {
					outCh <- tmpl
					break
				}

				continue
			}

			outCh <- output.String()
			break
		}
	}()

	select {
	case <-time.After(timeout):
		logger.Error(s2herrors.ErrRequestTimeout, fmt.Sprintf("template rendering took more than %v", timeout))
		return tmpl
	case out := <-outCh:
		// replace value back
		out = strings.ReplaceAll(out, startReplacedSign, "{{")
		out = strings.ReplaceAll(out, endReplacedSign, "}}")
		return out
	}
}

func replaceMissingValuesFromError(tmpl string, err error) (retTmpl string, ok bool) {
	value := extractValues(err.Error(), "<", ">", false)
	if value == "" {
		return tmpl, false
	}

	valueRegex := fmt.Sprintf(`%s\s*%s\s*%s`, startValTemplateSign, regexp.QuoteMeta(value), endValTemplateSign)
	re, err := regexp.Compile(valueRegex)
	if err != nil {
		return tmpl, false
	}

	// temporary replace missing values
	replacedValue := fmt.Sprintf("%s%s%s", startReplacedSign, value, endReplacedSign)
	tmpl = re.ReplaceAllString(tmpl, replacedValue)

	return tmpl, true
}

func extractValues(str, startSign, endSign string, includeSign bool) string {
	startIdx := strings.Index(str, startSign)
	endIdx := strings.Index(str, endSign)

	if startIdx < 0 || endIdx < 0 {
		return ""
	}

	if includeSign {
		return str[startIdx : endIdx+len(endSign)]
	}

	return str[startIdx+len(startSign) : endIdx]
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

func timeFormat(t *v1.Time) string {
	return t.Format("2006-01-02 15:04:05 MST")
}
