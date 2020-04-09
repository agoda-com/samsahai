package shell_test

import (
	"context"
	"os"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	s2hv1beta1 "github.com/agoda-com/samsahai/api/v1beta1"
	"github.com/agoda-com/samsahai/internal"
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
			testCmdObj := &s2hv1beta1.CommandAndArgs{}
			mockExecCommand := func(ctx context.Context, configPath string, cmdObj *s2hv1beta1.CommandAndArgs) ([]byte, error) {
				testCmdObj = cmdObj
				return []byte{}, nil
			}

			r := shell.New(shell.WithExecCommand(mockExecCommand))
			configCtrl := newMockConfigCtrl("")

			comp := internal.NewComponentUpgradeReporter(&rpc.ComponentUpgrade{Status: 1}, internal.SamsahaiConfig{})
			err := r.SendComponentUpgrade(configCtrl, comp)
			g.Expect(err).NotTo(HaveOccurred())

			g.Expect(testCmdObj.Command).To(Equal([]string{"/bin/sh", "-c"}))
			g.Expect(testCmdObj.Args).To(Equal([]string{"echo executing\n echo upgraded component Success"}))
		})

		It("should correctly execute active promotion", func() {
			testCmdObj := &s2hv1beta1.CommandAndArgs{}
			mockExecCommand := func(ctx context.Context, configPath string, cmdObj *s2hv1beta1.CommandAndArgs) ([]byte, error) {
				testCmdObj = cmdObj
				return []byte{}, nil
			}

			r := shell.New(shell.WithExecCommand(mockExecCommand))
			configCtrl := newMockConfigCtrl("")

			status := &s2hv1beta1.ActivePromotionStatus{
				Result: s2hv1beta1.ActivePromotionSuccess,
			}
			atpRpt := internal.NewActivePromotionReporter(status, internal.SamsahaiConfig{}, "", "")

			err := r.SendActivePromotionStatus(configCtrl, atpRpt)
			g.Expect(err).NotTo(HaveOccurred())

			g.Expect(testCmdObj.Command).To(Equal([]string{"echo active promotion status Success"}))
			g.Expect(testCmdObj.Args).To(BeNil())
		})

		It("should correctly execute image missing", func() {
			testCmdObj := &s2hv1beta1.CommandAndArgs{}
			mockExecCommand := func(ctx context.Context, configPath string, cmdObj *s2hv1beta1.CommandAndArgs) ([]byte, error) {
				testCmdObj = cmdObj
				return []byte{}, nil
			}

			r := shell.New(shell.WithExecCommand(mockExecCommand))
			configCtrl := newMockConfigCtrl("")

			img := &rpc.Image{Repository: "docker.io/hello-a", Tag: "2018.01.01"}
			err := r.SendImageMissing("mock", configCtrl, img)
			g.Expect(err).NotTo(HaveOccurred())

			g.Expect(testCmdObj.Command).To(Equal([]string{"/bin/sh", "-c"}))
			g.Expect(testCmdObj.Args).To(Equal([]string{"echo image missing docker.io/hello-a:2018.01.01"}))
		})

		It("should correctly execute command with environment variables", func() {
			testCmdObj := &s2hv1beta1.CommandAndArgs{}
			mockExecCommand := func(ctx context.Context, configPath string, cmdObj *s2hv1beta1.CommandAndArgs) ([]byte, error) {
				testCmdObj = cmdObj
				return []byte{}, nil
			}

			r := shell.New(shell.WithExecCommand(mockExecCommand))
			configCtrl := newMockConfigCtrl("env")

			g.Expect(os.Setenv("TEST_ENV", "s2h")).NotTo(HaveOccurred())
			comp := internal.NewComponentUpgradeReporter(&rpc.ComponentUpgrade{Status: 1}, internal.SamsahaiConfig{})
			err := r.SendComponentUpgrade(configCtrl, comp)
			g.Expect(err).NotTo(HaveOccurred())

			g.Expect(testCmdObj.Command).To(Equal([]string{"echo s2h"}))
		})
	})

	Describe("failure path", func() {
		It("should fail to execute command due to timeout", func() {
			r := shell.New(shell.WithTimeout(1 * time.Second))
			configCtrl := newMockConfigCtrl("failure")

			comp := internal.NewComponentUpgradeReporter(&rpc.ComponentUpgrade{}, internal.SamsahaiConfig{})
			err := r.SendComponentUpgrade(configCtrl, comp)
			g.Expect(err).To(HaveOccurred())
		})

		It("should not execute command if not define shell reporter configuration", func() {
			calls := 0
			mockExecCommand := func(ctx context.Context, configPath string, cmdObj *s2hv1beta1.CommandAndArgs) ([]byte, error) {
				calls++
				return []byte{}, nil
			}

			r := shell.New(shell.WithExecCommand(mockExecCommand))
			configCtrl := newMockConfigCtrl("empty")

			err := r.SendComponentUpgrade(configCtrl, &internal.ComponentUpgradeReporter{})
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(calls).To(Equal(0))

			err = r.SendActivePromotionStatus(configCtrl, &internal.ActivePromotionReporter{})
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(calls).To(Equal(0))

			err = r.SendImageMissing("mock", configCtrl, &rpc.Image{})
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(calls).To(Equal(0))
		})
	})
})

type mockConfigCtrl struct {
	configType string
}

func newMockConfigCtrl(configType string) internal.ConfigController {
	return &mockConfigCtrl{configType: configType}
}

func (c *mockConfigCtrl) Get(configName string) (*s2hv1beta1.Config, error) {
	switch c.configType {
	case "empty":
		return &s2hv1beta1.Config{}, nil
	case "env":
		return &s2hv1beta1.Config{
			Spec: s2hv1beta1.ConfigSpec{
				Reporter: &s2hv1beta1.ConfigReporter{
					Shell: &s2hv1beta1.Shell{
						ComponentUpgrade: &s2hv1beta1.CommandAndArgs{
							Command: []string{"echo {{ .Envs.TEST_ENV }}"},
						},
					},
				},
			},
		}, nil
	case "failure":
		return &s2hv1beta1.Config{
			Spec: s2hv1beta1.ConfigSpec{
				Reporter: &s2hv1beta1.ConfigReporter{
					Shell: &s2hv1beta1.Shell{
						ComponentUpgrade: &s2hv1beta1.CommandAndArgs{
							Command: []string{"/bin/sleep", "5"},
						},
					},
				},
			},
		}, nil
	default:
		return &s2hv1beta1.Config{
			Spec: s2hv1beta1.ConfigSpec{
				Reporter: &s2hv1beta1.ConfigReporter{
					Shell: &s2hv1beta1.Shell{
						ComponentUpgrade: &s2hv1beta1.CommandAndArgs{
							Command: []string{"/bin/sh", "-c"},
							Args:    []string{"echo executing\n echo upgraded component {{ .StatusStr }}"},
						},
						ActivePromotion: &s2hv1beta1.CommandAndArgs{
							Command: []string{"echo active promotion status {{ .Result }}"},
						},
						ImageMissing: &s2hv1beta1.CommandAndArgs{
							Command: []string{"/bin/sh", "-c"},
							Args:    []string{"echo image missing {{ .Repository }}:{{ .Tag }}"},
						},
					},
				},
			},
		}, nil
	}
}

func (c *mockConfigCtrl) GetComponents(configName string) (map[string]*s2hv1beta1.Component, error) {
	return map[string]*s2hv1beta1.Component{}, nil
}

func (c *mockConfigCtrl) GetParentComponents(configName string) (map[string]*s2hv1beta1.Component, error) {
	return map[string]*s2hv1beta1.Component{}, nil
}

func (c *mockConfigCtrl) EnsureComponentChanged(configName, namespace string) error {
	return nil
}

func (c *mockConfigCtrl) Update(config *s2hv1beta1.Config) error {
	return nil
}

func (c *mockConfigCtrl) Delete(configName string) error {
	return nil
}
