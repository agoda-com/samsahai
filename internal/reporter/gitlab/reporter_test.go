package gitlab_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"

	s2hv1 "github.com/agoda-com/samsahai/api/v1"
	"github.com/agoda-com/samsahai/internal"
	s2hgitlab "github.com/agoda-com/samsahai/internal/reporter/gitlab"
	"github.com/agoda-com/samsahai/internal/util/gitlab"
	"github.com/agoda-com/samsahai/internal/util/unittest"
	"github.com/agoda-com/samsahai/pkg/samsahai/rpc"
)

func TestUnit(t *testing.T) {
	unittest.InitGinkgo(t, "Gitlab Reporter")
}

var _ = Describe("publish commit status to gitlab", func() {
	g := NewGomegaWithT(GinkgoT())

	Describe("send pull request queue", func() {
		It("should correctly send pull request queue success", func() {
			configCtrl := newMockConfigCtrl("")
			g.Expect(configCtrl).ShouldNot(BeNil())

			rpcComp := &rpc.ComponentUpgrade{
				Name:             "bundle-1",
				Status:           rpc.ComponentUpgrade_UpgradeStatus_SUCCESS,
				TeamName:         "owner",
				QueueHistoryName: "bundle1-comp1-1234",
				PullRequestComponent: &rpc.TeamWithPullRequest{
					BundleName: "bundle-1",
					PRNumber:   "pr1234",
					CommitSHA:  "commit-sha-xxx",
				},
			}
			mockGitlabCli := &mockGitlab{}
			r := s2hgitlab.New(s2hgitlab.WithGitlabClient(mockGitlabCli))
			comp := internal.NewComponentUpgradeReporter(
				rpcComp,
				internal.SamsahaiConfig{SamsahaiExternalURL: "http://localhost:8080"},
				internal.WithQueueHistoryName("bundle1-comp1-5678"),
				internal.WithNamespace("pr-namespace"),
			)
			err := r.SendPullRequestQueue(configCtrl, comp)
			g.Expect(err).Should(BeNil())
			g.Expect(mockGitlabCli.publishCalls).Should(Equal(2))
			g.Expect(mockGitlabCli.repository).Should(Equal("samsahai/samsahai"))
			g.Expect(mockGitlabCli.commitSHA).Should(Equal("commit-sha-xxx"))
			g.Expect(mockGitlabCli.status).Should(Equal(gitlab.CommitStatusSuccess))
			g.Expect(mockGitlabCli.targetURLs).Should(Equal([]string{
				"http://localhost:8080/teams/owner/pullrequest/queue/histories/bundle1-comp1-5678",
				"http://localhost:8080/teams/owner/pullrequest/queue/histories/bundle1-comp1-5678/log",
			}))
		})

		It("should correctly send pull request queue failure", func() {
			configCtrl := newMockConfigCtrl("")
			g.Expect(configCtrl).ShouldNot(BeNil())

			rpcComp := &rpc.ComponentUpgrade{
				Name:     "bundle-1",
				Status:   rpc.ComponentUpgrade_UpgradeStatus_FAILURE,
				TeamName: "owner",
				PullRequestComponent: &rpc.TeamWithPullRequest{
					BundleName: "bundle-1",
				},
			}
			mockGitlabCli := &mockGitlab{}
			r := s2hgitlab.New(s2hgitlab.WithGitlabClient(mockGitlabCli))
			comp := internal.NewComponentUpgradeReporter(
				rpcComp,
				internal.SamsahaiConfig{SamsahaiExternalURL: "http://localhost:8080"},
			)
			err := r.SendPullRequestQueue(configCtrl, comp)
			g.Expect(err).Should(BeNil())
			g.Expect(mockGitlabCli.publishCalls).Should(Equal(2))
			g.Expect(mockGitlabCli.status).Should(Equal(gitlab.CommitStatusFailure))
		})
	})

	Describe("failure path", func() {
		It("should not send message if not define gitlab reporter configuration", func() {
			configCtrl := newMockConfigCtrl("empty")
			g.Expect(configCtrl).ShouldNot(BeNil())

			rpcComp := &rpc.ComponentUpgrade{}
			mockGitlabCli := &mockGitlab{}
			r := s2hgitlab.New(s2hgitlab.WithGitlabClient(mockGitlabCli))
			comp := internal.NewComponentUpgradeReporter(rpcComp, internal.SamsahaiConfig{})
			err := r.SendPullRequestQueue(configCtrl, comp)
			g.Expect(err).To(BeNil())
			g.Expect(mockGitlabCli.publishCalls).Should(Equal(0))
		})

		It("should fail to publish commit status", func() {
			configCtrl := newMockConfigCtrl("failure")
			g.Expect(configCtrl).ShouldNot(BeNil())

			rpcComp := &rpc.ComponentUpgrade{
				PullRequestComponent: &rpc.TeamWithPullRequest{
					BundleName: "bundle-1",
				},
			}
			mockGitlabCli := &mockGitlab{}
			r := s2hgitlab.New(s2hgitlab.WithGitlabClient(mockGitlabCli))
			comp := internal.NewComponentUpgradeReporter(rpcComp, internal.SamsahaiConfig{})
			err := r.SendPullRequestQueue(configCtrl, comp)
			g.Expect(err).To(HaveOccurred())
			g.Expect(mockGitlabCli.publishCalls).Should(Equal(0))
		})
	})
})

// mockGitlab mocks Gitlab interface
type mockGitlab struct {
	publishCalls int
	repository   string
	commitSHA    string
	status       gitlab.CommitStatus
	targetURLs   []string
}

// PostMessage mocks PostMessage function
func (s *mockGitlab) PublishCommitStatus(repository, commitSHA, labelName, targetURL, description string,
	status gitlab.CommitStatus) error {

	if repository == "error" {
		return errors.New("error")
	}

	s.publishCalls++
	s.repository = repository
	s.commitSHA = commitSHA
	s.status = status
	s.targetURLs = append(s.targetURLs, targetURL)

	return nil
}

type mockConfigCtrl struct {
	configType string
}

func newMockConfigCtrl(configType string) internal.ConfigController {
	return &mockConfigCtrl{
		configType: configType,
	}
}

func (c *mockConfigCtrl) Get(configName string) (*s2hv1.Config, error) {
	switch c.configType {
	case "empty":
		return &s2hv1.Config{}, nil
	case "failure":
		return &s2hv1.Config{
			Status: s2hv1.ConfigStatus{
				Used: s2hv1.ConfigSpec{
					Reporter: &s2hv1.ConfigReporter{
						Gitlab: &s2hv1.ReporterGitlab{
							Enabled: true,
							BaseURL: "https://gitlab.com",
						},
					},
					PullRequest: &s2hv1.ConfigPullRequest{
						Bundles: []*s2hv1.PullRequestBundle{
							{
								Name:          "bundle-1",
								GitRepository: "error",
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
						Gitlab: &s2hv1.ReporterGitlab{
							Enabled: true,
							BaseURL: "https://gitlab.com",
						},
					},
					PullRequest: &s2hv1.ConfigPullRequest{
						Bundles: []*s2hv1.PullRequestBundle{
							{
								Name: "bundle-1",
								Components: []*s2hv1.PullRequestComponent{
									{},
								},
								GitRepository: "samsahai/samsahai",
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
