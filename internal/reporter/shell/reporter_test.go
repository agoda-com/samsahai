package shell_test

import (
	"context"
	"os"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	s2hv1 "github.com/agoda-com/samsahai/api/v1"
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
			testCmdObj := &s2hv1.CommandAndArgs{}
			mockExecCommand := func(ctx context.Context, configPath string, cmdObj *s2hv1.CommandAndArgs) ([]byte, error) {
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

		It("should correctly execute pull request queue", func() {
			testCmdObj := &s2hv1.CommandAndArgs{}
			mockExecCommand := func(ctx context.Context, configPath string, cmdObj *s2hv1.CommandAndArgs) ([]byte, error) {
				testCmdObj = cmdObj
				return []byte{}, nil
			}

			r := shell.New(shell.WithExecCommand(mockExecCommand))
			configCtrl := newMockConfigCtrl("")

			comp := internal.NewComponentUpgradeReporter(&rpc.ComponentUpgrade{
				Status: 1,
				PullRequestComponent: &rpc.TeamWithPullRequest{
					BundleName: "bundle-1",
					PRNumber:   "pr1234",
				},
			}, internal.SamsahaiConfig{})
			err := r.SendPullRequestQueue(configCtrl, comp)
			g.Expect(err).NotTo(HaveOccurred())

			g.Expect(testCmdObj.Command).To(Equal([]string{"/bin/sh", "-c"}))
			g.Expect(testCmdObj.Args).To(Equal([]string{"echo executing\n echo pull request #pr1234: Success"}))
		})

		It("should correctly execute active promotion status", func() {
			testCmdObj := &s2hv1.CommandAndArgs{}
			mockExecCommand := func(ctx context.Context, configPath string, cmdObj *s2hv1.CommandAndArgs) ([]byte, error) {
				testCmdObj = cmdObj
				return []byte{}, nil
			}

			r := shell.New(shell.WithExecCommand(mockExecCommand))
			configCtrl := newMockConfigCtrl("")

			status := s2hv1.ActivePromotionStatus{
				Result: s2hv1.ActivePromotionSuccess,
			}
			atpRpt := internal.NewActivePromotionReporter(status, internal.SamsahaiConfig{}, "", "",
				2)

			err := r.SendActivePromotionStatus(configCtrl, atpRpt)
			g.Expect(err).NotTo(HaveOccurred())

			g.Expect(testCmdObj.Command).To(Equal([]string{"echo active promotion status Success #2"}))
			g.Expect(testCmdObj.Args).To(BeNil())
		})

		It("should correctly execute image missing", func() {
			testCmdObj := &s2hv1.CommandAndArgs{}
			mockExecCommand := func(ctx context.Context, configPath string, cmdObj *s2hv1.CommandAndArgs) ([]byte, error) {
				testCmdObj = cmdObj
				return []byte{}, nil
			}

			r := shell.New(shell.WithExecCommand(mockExecCommand))
			configCtrl := newMockConfigCtrl("")

			img := s2hv1.Image{Repository: "docker.io/hello-a", Tag: "2018.01.01"}
			imageMissingRpt := internal.NewImageMissingReporter(img, internal.SamsahaiConfig{},
				"owner", "comp1", "")
			err := r.SendImageMissing(configCtrl, imageMissingRpt)
			g.Expect(err).NotTo(HaveOccurred())

			g.Expect(testCmdObj.Command).To(Equal([]string{"/bin/sh", "-c"}))
			g.Expect(testCmdObj.Args).To(Equal([]string{"echo image missing docker.io/hello-a:2018.01.01 of comp1"}))
		})

		It("should correctly execute pull request trigger", func() {
			testCmdObj := &s2hv1.CommandAndArgs{}
			mockExecCommand := func(ctx context.Context, configPath string, cmdObj *s2hv1.CommandAndArgs) ([]byte, error) {
				testCmdObj = cmdObj
				return []byte{}, nil
			}

			r := shell.New(shell.WithExecCommand(mockExecCommand))
			configCtrl := newMockConfigCtrl("")

			status := s2hv1.PullRequestTriggerStatus{}
			prComps := []*s2hv1.PullRequestTriggerComponent{
				{
					ComponentName: "bundle1-comp1",
					Image:         &s2hv1.Image{Repository: "registry/comp-1", Tag: "1.0.0"},
				},
				{
					ComponentName: "bundle1-comp2",
					Image:         &s2hv1.Image{Repository: "registry/comp-2", Tag: "2.0.0"},
				},
			}
			prTriggerRpt := internal.NewPullRequestTriggerResultReporter(status, internal.SamsahaiConfig{},
				"owner", "bundle-1", "1234", "Failure", 0, prComps)
			err := r.SendPullRequestTriggerResult(configCtrl, prTriggerRpt)
			g.Expect(err).NotTo(HaveOccurred())

			g.Expect(testCmdObj.Command).To(Equal([]string{"/bin/sh", "-c"}))
			g.Expect(testCmdObj.Args).To(Equal([]string{"echo pull request trigger of 1234: Failure"}))
		})

		It("should correctly execute deleted active namespace", func() {
			testCmdObj := &s2hv1.CommandAndArgs{}
			mockExecCommand := func(ctx context.Context, configPath string, cmdObj *s2hv1.CommandAndArgs) ([]byte, error) {
				testCmdObj = cmdObj
				return []byte{}, nil
			}

			r := shell.New(shell.WithExecCommand(mockExecCommand))
			configCtrl := newMockConfigCtrl("")

			activeNsDeleted := internal.NewActiveEnvironmentDeletedReporter(
				"teamtest", "s2h-active-ns-test", "user", "2020-11-06T05:14:23")
			err := r.SendActiveEnvironmentDeleted(configCtrl, activeNsDeleted)
			g.Expect(err).NotTo(HaveOccurred())

			g.Expect(testCmdObj.Command).To(Equal([]string{"/bin/sh", "-c"}))
			g.Expect(testCmdObj.Args).To(Equal([]string{"echo executing deleted active namespace command of teamtest , namespace : s2h-active-ns-test ,deleted-by : user, deleted-at : 2020-11-06T05:14:23"}))
		})

		It("should correctly execute command with environment variables", func() {
			testCmdObj := &s2hv1.CommandAndArgs{}
			mockExecCommand := func(ctx context.Context, configPath string, cmdObj *s2hv1.CommandAndArgs) ([]byte, error) {
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
			mockExecCommand := func(ctx context.Context, configPath string, cmdObj *s2hv1.CommandAndArgs) ([]byte, error) {
				calls++
				return []byte{}, nil
			}

			r := shell.New(shell.WithExecCommand(mockExecCommand))
			configCtrl := newMockConfigCtrl("empty")

			err := r.SendComponentUpgrade(configCtrl, &internal.ComponentUpgradeReporter{ComponentUpgrade: &rpc.ComponentUpgrade{}})
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(calls).To(Equal(0))

			err = r.SendPullRequestQueue(configCtrl, &internal.ComponentUpgradeReporter{ComponentUpgrade: &rpc.ComponentUpgrade{}})
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(calls).To(Equal(0))

			err = r.SendActivePromotionStatus(configCtrl, &internal.ActivePromotionReporter{})
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(calls).To(Equal(0))

			err = r.SendImageMissing(configCtrl, &internal.ImageMissingReporter{})
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(calls).To(Equal(0))

			err = r.SendPullRequestTriggerResult(configCtrl, &internal.PullRequestTriggerReporter{})
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

func (c *mockConfigCtrl) Get(configName string) (*s2hv1.Config, error) {
	switch c.configType {
	case "empty":
		return &s2hv1.Config{}, nil
	case "env":
		return &s2hv1.Config{
			Status: s2hv1.ConfigStatus{
				Used: s2hv1.ConfigSpec{
					Reporter: &s2hv1.ConfigReporter{
						Shell: &s2hv1.ReporterShell{
							ComponentUpgrade: &s2hv1.CommandAndArgs{
								Command: []string{"echo {{ .Envs.TEST_ENV }}"},
							},
						},
					},
				},
			},
		}, nil
	case "failure":
		return &s2hv1.Config{
			Status: s2hv1.ConfigStatus{
				Used: s2hv1.ConfigSpec{
					Reporter: &s2hv1.ConfigReporter{
						Shell: &s2hv1.ReporterShell{
							ComponentUpgrade: &s2hv1.CommandAndArgs{
								Command: []string{"/bin/sleep", "5"},
							},
						},
					},
				},
			},
		}, nil
	default:
		return &s2hv1.Config{
			Status: s2hv1.ConfigStatus{
				Used: s2hv1.ConfigSpec{
					Reporter: &s2hv1.ConfigReporter{
						Shell: &s2hv1.ReporterShell{
							ComponentUpgrade: &s2hv1.CommandAndArgs{
								Command: []string{"/bin/sh", "-c"},
								Args:    []string{"echo executing\n echo upgraded component {{ .StatusStr }}"},
							},
							PullRequestQueue: &s2hv1.CommandAndArgs{
								Command: []string{"/bin/sh", "-c"},
								Args:    []string{"echo executing\n echo pull request #{{ .PullRequestComponent.PRNumber }}: {{ .StatusStr }}"},
							},
							ActivePromotion: &s2hv1.CommandAndArgs{
								Command: []string{"echo active promotion status {{ .Result }} #{{ .Runs }}"},
							},
							ImageMissing: &s2hv1.CommandAndArgs{
								Command: []string{"/bin/sh", "-c"},
								Args:    []string{"echo image missing {{ .Repository }}:{{ .Tag }} of {{ .ComponentName }}"},
							},
							PullRequestTrigger: &s2hv1.CommandAndArgs{
								Command: []string{"/bin/sh", "-c"},
								Args:    []string{"echo pull request trigger of {{ .PRNumber }}: {{ .Result }}"},
							},
							ActiveEnvironmentDeleted: &s2hv1.CommandAndArgs{
								Command: []string{"/bin/sh", "-c"},
								Args:    []string{"echo executing deleted active namespace command of {{ .TeamName }} , namespace : {{ .ActiveNamespace }} ,deleted-by : {{ .DeletedBy }}, deleted-at : {{ .DeletedAt }}"},
							},
						},
					},
				},
			},
		}, nil
	}
}

func (c *mockConfigCtrl) GetComponents(configName string) (map[string]*s2hv1.Component, error) {
	return map[string]*s2hv1.Component{}, nil
}

func (c *mockConfigCtrl) GetParentComponents(configName string) (map[string]*s2hv1.Component, error) {
	return map[string]*s2hv1.Component{}, nil
}

func (c *mockConfigCtrl) GetPullRequestComponents(configName, prBundleName string, depIncluded bool) (map[string]*s2hv1.Component, error) {
	return map[string]*s2hv1.Component{}, nil
}

func (c *mockConfigCtrl) GetBundles(configName string) (s2hv1.ConfigBundles, error) {
	return s2hv1.ConfigBundles{}, nil
}

func (c *mockConfigCtrl) GetPriorityQueues(configName string) ([]string, error) {
	return nil, nil
}

func (c *mockConfigCtrl) GetStagingConfig(configName string) (*s2hv1.ConfigStaging, error) {
	return nil, nil
}

func (c *mockConfigCtrl) GetPullRequestConfig(configName string) (*s2hv1.ConfigPullRequest, error) {
	return nil, nil
}

func (c *mockConfigCtrl) GetPullRequestBundleDependencies(configName, prBundleName string) ([]string, error) {
	return nil, nil
}

func (c *mockConfigCtrl) Update(config *s2hv1.Config) error {
	return nil
}

func (c *mockConfigCtrl) Delete(configName string) error {
	return nil
}

func (c *mockConfigCtrl) EnsureConfigTemplateChanged(config *s2hv1.Config) error {
	return nil
}
