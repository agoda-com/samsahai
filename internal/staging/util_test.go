package staging

import (
	s2hv1 "github.com/agoda-com/samsahai/api/v1"
	"github.com/agoda-com/samsahai/internal/util/gitlab"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Util", func() {
	Describe("tryInferPullRequestGitlabBranch", func() {
		var testRunner *s2hv1.ConfigTestRunner
		var queue *s2hv1.Queue
		var gitlabClientGetter func(token string) gitlab.Gitlab
		g := NewWithT(GinkgoT())

		BeforeEach(func() {
			testRunner = &s2hv1.ConfigTestRunner{
				Timeout:     v1.Duration{Duration: 10},
				PollingTime: v1.Duration{Duration: 20},
				Gitlab: &s2hv1.ConfigGitlab{
					ProjectID:            "123",
					Branch:               "abc",
					PipelineTriggerToken: "xyz",
				},
				Teamcity: &s2hv1.ConfigTeamcity{
					BuildTypeID: "alpha",
					Branch:      "beta",
				},
			}
			queue = &s2hv1.Queue{
				Spec: s2hv1.QueueSpec{
					PRNumber: "87878",
					Type:     s2hv1.QueueTypePullRequest,
				},
			}
			gitlabClientGetter = nil
		})

		It("should not infer if false", func() {
			testRunner.PullRequestInferGitlabMRBranch = false
			before := testRunner.DeepCopy()

			inferredBranch := "falsebranch"

			gitlabClientGetter = func(token string) gitlab.Gitlab {
				g.Expect(token).To(Equal(before.Gitlab.PipelineTriggerToken))
				return mockGitlab{
					G:                  g,
					ExpectedRepository: before.Gitlab.ProjectID,
					ExpectedPRNumber:   queue.Spec.PRNumber,
					Branch:             inferredBranch,
					Error:              nil,
				}
			}

			tryInferPullRequestGitlabBranch(testRunner, queue, gitlabClientGetter)

			g.Expect(testRunner).To(Equal(before))
		})

		It("should infer if true", func() {
			testRunner.PullRequestInferGitlabMRBranch = true
			before := testRunner.DeepCopy()

			inferredBranch := "truebranch"

			gitlabClientGetter = func(token string) gitlab.Gitlab {
				g.Expect(token).To(Equal(before.Gitlab.PipelineTriggerToken))
				return mockGitlab{
					G:                  g,
					ExpectedRepository: before.Gitlab.ProjectID,
					ExpectedPRNumber:   queue.Spec.PRNumber,
					Branch:             inferredBranch,
					Error:              nil,
				}
			}

			tryInferPullRequestGitlabBranch(testRunner, queue, gitlabClientGetter)

			g.Expect(testRunner).ToNot(Equal(before))
			g.Expect(testRunner.Gitlab.Branch).To(Equal(inferredBranch))
			testRunner.Gitlab.Branch = before.Gitlab.Branch
			// should change only branch field
			g.Expect(testRunner).To(Equal(before))
		})
	})
})

type mockGitlab struct {
	G                  *GomegaWithT
	ExpectedRepository string
	ExpectedPRNumber   string
	Branch             string
	Error              error
}

func (m mockGitlab) PublishCommitStatus(repository, commitSHA, labelName, targetURL, description string, status gitlab.CommitStatus) error {
	panic("expect not to invoke PublishCommitStatus")
}

func (m mockGitlab) GetMRSourceBranch(repository, MRiid string) (string, error) {
	m.G.Expect(repository).To(Equal(m.ExpectedRepository))
	m.G.Expect(MRiid).To(Equal(m.ExpectedPRNumber))
	return m.Branch, m.Error
}
