package email

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestGetStatusText(t *testing.T) {
	g := NewGomegaWithT(t)
	tests := map[string]struct {
		in  string
		out string
	}{
		"should get success status": {
			in:  "Success",
			out: "SUCCESS",
		},
		"should get fail status": {
			in:  "Failed",
			out: "FAIL",
		},
	}

	for desc, test := range tests {
		statusText := getStatusText(test.in)
		g.Expect(statusText).Should(Equal(test.out), desc)
	}
}
