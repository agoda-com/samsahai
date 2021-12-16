package staging

import (
	s2hv1 "github.com/agoda-com/samsahai/api/v1"
	"github.com/agoda-com/samsahai/internal/util/gitlab"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Util", func() {
	Describe("tryInferPullRequestGitlabBranch", func() {
		var gitlabConf *s2hv1.ConfigGitlab
		var MRiid string
		var gitlabClientGetter func(token string) gitlab.Gitlab
		g := NewWithT(GinkgoT())

		BeforeEach(func() {
			gitlabConf = &s2hv1.ConfigGitlab{
				ProjectID:            "123",
				PipelineTriggerToken: "xyz",
			}
			MRiid = "87878"
			gitlabClientGetter = nil
		})

		It("should not infer if false", func() {
			gitlabConf.InferBranch = false
			before := gitlabConf.DeepCopy()

			inferredBranch := "falsebranch"

			gitlabClientGetter = func(token string) gitlab.Gitlab {
				g.Expect(token).To(Equal(gitlabConf.PipelineTriggerToken))
				return mockGitlab{
					G:                  g,
					ExpectedRepository: gitlabConf.ProjectID,
					ExpectedPRNumber:   MRiid,
					Branch:             inferredBranch,
					Error:              nil,
				}
			}

			tryInferPullRequestGitlabBranch(gitlabConf, MRiid, gitlabClientGetter)

			g.Expect(gitlabConf).To(Equal(before))
		})

		It("should infer if true and branch empty", func() {
			gitlabConf.InferBranch = true
			gitlabConf.Branch = ""
			before := gitlabConf.DeepCopy()

			inferredBranch := "truebranch"

			gitlabClientGetter = func(token string) gitlab.Gitlab {
				g.Expect(token).To(Equal(gitlabConf.PipelineTriggerToken))
				return mockGitlab{
					G:                  g,
					ExpectedRepository: before.ProjectID,
					ExpectedPRNumber:   MRiid,
					Branch:             inferredBranch,
					Error:              nil,
				}
			}

			tryInferPullRequestGitlabBranch(gitlabConf, MRiid, gitlabClientGetter)

			g.Expect(gitlabConf).ToNot(Equal(before))
			g.Expect(gitlabConf.Branch).To(Equal(inferredBranch))
			gitlabConf.Branch = before.Branch
			// should change only branch field
			g.Expect(gitlabConf).To(Equal(before))
		})

		It("should not infer if true and branch not empty", func() {
			gitlabConf.InferBranch = true
			gitlabConf.Branch = "abc"
			before := gitlabConf.DeepCopy()

			inferredBranch := "truebranch"

			gitlabClientGetter = func(token string) gitlab.Gitlab {
				g.Expect(token).To(Equal(gitlabConf.PipelineTriggerToken))
				return mockGitlab{
					G:                  g,
					ExpectedRepository: before.ProjectID,
					ExpectedPRNumber:   MRiid,
					Branch:             inferredBranch,
					Error:              nil,
				}
			}

			tryInferPullRequestGitlabBranch(gitlabConf, MRiid, gitlabClientGetter)

			g.Expect(gitlabConf).To(Equal(before))
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
