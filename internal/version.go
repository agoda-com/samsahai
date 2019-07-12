package internal

import (
	"regexp"
	"strconv"
	"strings"
)

const AppName = "samsahai"
const AbbreviationName = "s2h"
const AppPrefix = AbbreviationName + "-"

// GitCommit defines commit hash of git during built
var GitCommit string

// Version defines the version of application
var Version string

type SortableVersion []string

func (v SortableVersion) Len() int { return len(v) }
func (v SortableVersion) Less(i, j int) bool {
	is := v.regexpSplit(v[i])
	js := v.regexpSplit(v[j])

	reNum := regexp.MustCompile(`\d+`)

	for i := 0; i < len(is) && i < len(js); i++ {
		c := strings.Compare(is[i], js[i])
		if c == 0 {
			continue
		}

		if reNum.MatchString(is[i]) && reNum.MatchString(js[i]) {
			iv, _ := strconv.Atoi(is[i])
			jv, _ := strconv.Atoi(js[i])
			if iv == jv {
				continue
			}
			return iv < jv
		}

		return c < 0
	}

	return len(is) < len(js)
}
func (v SortableVersion) Swap(i, j int) { v[i], v[j] = v[j], v[i] }

func (v SortableVersion) regexpSplit(input string) []string {
	reg := regexp.MustCompile("[._-]")
	split := reg.Split(input, -1)
	var output []string
	for i := range split {
		if split[i] == "" {
			continue
		}
		output = append(output, split[i])
	}
	return output
}
