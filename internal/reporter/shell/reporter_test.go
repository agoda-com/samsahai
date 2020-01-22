package shell_test

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	s2hv1beta1 "github.com/agoda-com/samsahai/api/v1beta1"
	"github.com/agoda-com/samsahai/internal"
	"github.com/agoda-com/samsahai/internal/config"
	"github.com/agoda-com/samsahai/internal/reporter/shell"
	"github.com/agoda-com/samsahai/internal/util/unittest"
	"github.com/agoda-com/samsahai/pkg/samsahai/rpc"
)

func TestUnit(t *testing.T) {
	unittest.InitGinkgo(t, "Shell Reporter")
}

var _ = Describe("shell command reporter", func() {
	g := NewGomegaWithT(GinkgoT())

	Describe("success path", func() {
		It("should correctly execute component upgrade", func() {
			testCmdObj := &internal.CommandAndArgs{}
			mockExecCommand := func(ctx context.Context, configPath string, cmdObj *internal.CommandAndArgs) ([]byte, error) {
				testCmdObj = cmdObj
				return []byte{}, nil
			}

			r := shell.New(shell.WithExecCommand(mockExecCommand))
			configMgr := newConfigMock()

			comp := internal.NewComponentUpgradeReporter(&rpc.ComponentUpgrade{Status: 1}, internal.SamsahaiConfig{})
			err := r.SendComponentUpgrade(configMgr, comp)
			g.Expect(err).NotTo(HaveOccurred())

			g.Expect(testCmdObj.Command).To(Equal([]string{"/bin/sh", "-c"}))
			g.Expect(testCmdObj.Args).To(Equal([]string{"echo executing\necho upgraded component Success\n"}))
		})

		It("should correctly execute active promotion", func() {
			testCmdObj := &internal.CommandAndArgs{}
			mockExecCommand := func(ctx context.Context, configPath string, cmdObj *internal.CommandAndArgs) ([]byte, error) {
				testCmdObj = cmdObj
				return []byte{}, nil
			}

			r := shell.New(shell.WithExecCommand(mockExecCommand))
			configMgr := newConfigMock()

			status := &s2hv1beta1.ActivePromotionStatus{
				Result: s2hv1beta1.ActivePromotionSuccess,
			}
			atpRpt := internal.NewActivePromotionReporter(status, internal.SamsahaiConfig{}, "", "")

			err := r.SendActivePromotionStatus(configMgr, atpRpt)
			g.Expect(err).NotTo(HaveOccurred())

			g.Expect(testCmdObj.Command).To(Equal([]string{"echo active promotion status Success\n"}))
			g.Expect(testCmdObj.Args).To(BeNil())
		})

		It("should correctly execute image missing", func() {
			testCmdObj := &internal.CommandAndArgs{}
			mockExecCommand := func(ctx context.Context, configPath string, cmdObj *internal.CommandAndArgs) ([]byte, error) {
				testCmdObj = cmdObj
				return []byte{}, nil
			}

			r := shell.New(shell.WithExecCommand(mockExecCommand))
			configMgr := newConfigMock()

			img := &rpc.Image{Repository: "docker.io/hello-a", Tag: "2018.01.01"}
			err := r.SendImageMissing(configMgr, img)
			g.Expect(err).NotTo(HaveOccurred())

			g.Expect(testCmdObj.Command).To(Equal([]string{"/bin/sh", "-c"}))
			g.Expect(testCmdObj.Args).To(Equal([]string{"echo image missing docker.io/hello-a:2018.01.01"}))
		})
	})

	Describe("failure path", func() {
		It("should fail to execute command due to timeout", func() {
			r := shell.New(shell.WithTimeout(1 * time.Second))
			configMgr := newFailureConfig()

			comp := internal.NewComponentUpgradeReporter(&rpc.ComponentUpgrade{}, internal.SamsahaiConfig{})
			err := r.SendComponentUpgrade(configMgr, comp)
			g.Expect(err).To(HaveOccurred())
		})

		It("should not execute command if not define shell reporter configuration", func() {
			calls := 0
			mockExecCommand := func(ctx context.Context, configPath string, cmdObj *internal.CommandAndArgs) ([]byte, error) {
				calls++
				return []byte{}, nil
			}

			r := shell.New(shell.WithExecCommand(mockExecCommand))
			configMgr := newNoShellConfig()

			err := r.SendComponentUpgrade(configMgr, nil)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(calls).To(Equal(0))

			err = r.SendActivePromotionStatus(configMgr, nil)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(calls).To(Equal(0))

			err = r.SendImageMissing(configMgr, nil)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(calls).To(Equal(0))
		})
	})
})

func newConfigMock() internal.ConfigManager {
	return config.NewWithBytes([]byte(`
report:
  cmd:
    componentUpgrade:
      command: ["/bin/sh", "-c"]
      args: 
        - |
          echo executing
          echo upgraded component {{ .StatusStr }}
    activePromotion:
      command: 
        - |
          echo active promotion status {{ .Result }}
    imageMissing:
      command: ["/bin/sh", "-c"]
      args: ["echo image missing {{ .Repository }}:{{ .Tag }}"]
`))
}

func newNoShellConfig() internal.ConfigManager {
	configMgr := config.NewWithBytes([]byte(`
report:
`))

	return configMgr
}

func newFailureConfig() internal.ConfigManager {
	return config.NewWithBytes([]byte(`
report:
  cmd:
    componentUpgrade:
      command: ["/bin/sleep", "5"]
`))
}
