package cmd

import (
	"context"
	"os"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	s2hv1beta1 "github.com/agoda-com/samsahai/api/v1beta1"
	"github.com/agoda-com/samsahai/internal/util/unittest"
)

func TestUnit(t *testing.T) {
	unittest.InitGinkgo(t, "Execute shell command")
}

var _ = Describe("shell command", func() {
	g := NewGomegaWithT(GinkgoT())

	Describe("success path", func() {
		It("should execute command with single argument correctly", func() {
			cmdObj := &s2hv1beta1.CommandAndArgs{
				Command: []string{"/bin/sh", "-c"},
				Args:    []string{"echo hello with newline"},
			}

			pwd, err := os.Getwd()
			Expect(err).NotTo(HaveOccurred())

			out, err := ExecuteCommand(context.TODO(), pwd, cmdObj)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(string(out)).To(Equal("hello with newline\n"))
		})

		It("should execute command with multiple arguments correctly", func() {
			cmdObj := &s2hv1beta1.CommandAndArgs{
				Command: []string{"/bin/echo"},
				Args:    []string{"-n", "hello without newline"},
			}

			pwd, err := os.Getwd()
			Expect(err).NotTo(HaveOccurred())

			out, err := ExecuteCommand(context.TODO(), pwd, cmdObj)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(string(out)).To(Equal("hello without newline"))
		})

		It("should execute command with multi-line arguments correctly", func() {
			cmdObj := &s2hv1beta1.CommandAndArgs{
				Command: []string{"/bin/sh", "-c"},
				Args:    []string{"/bin/echo hello\n/bin/echo -n world"},
			}

			pwd, err := os.Getwd()
			Expect(err).NotTo(HaveOccurred())

			out, err := ExecuteCommand(context.TODO(), pwd, cmdObj)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(string(out)).To(Equal("hello\nworld"))
		})

		It("should execute command without argument correctly", func() {
			cmdObj := &s2hv1beta1.CommandAndArgs{
				Command: []string{"/bin/sh", "-c", "/bin/echo hello\n/bin/echo -n world"},
			}

			pwd, err := os.Getwd()
			Expect(err).NotTo(HaveOccurred())

			out, err := ExecuteCommand(context.TODO(), pwd, cmdObj)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(string(out)).To(Equal("hello\nworld"))
		})

		It("should execute command from file correctly", func() {
			cmdObj := &s2hv1beta1.CommandAndArgs{
				Command: []string{"/bin/sh", "./testdata/test.sh"},
			}

			pwd, err := os.Getwd()
			Expect(err).NotTo(HaveOccurred())
			out, err := ExecuteCommand(context.TODO(), pwd, cmdObj)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(string(out)).To(Equal("hello world\n"))
		})
	})

	Describe("failure path", func() {
		It("should fail to execute command if not define command", func() {
			cmdObj := &s2hv1beta1.CommandAndArgs{
				Args: []string{"echo hello with newline"},
			}

			pwd, err := os.Getwd()
			Expect(err).NotTo(HaveOccurred())

			_, err = ExecuteCommand(context.TODO(), pwd, cmdObj)
			g.Expect(err).To(HaveOccurred())
		})
	})
})
