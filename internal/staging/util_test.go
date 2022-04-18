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
		g := NewWithT(GinkgoT())

		BeforeEach(func() {
			gitlabConf = &s2hv1.ConfigGitlab{
				ProjectID:            "123",
				PipelineTriggerToken: "xyz",
			}
			MRiid = "87878"
		})

		It("interBranch=True defaultBranch='', use branch from MR ", func() {
			gitlabConf.SetInferBranch(true)
			before := gitlabConf.DeepCopy()

			inferredBranch := "falsebranch"

			gitlabClientGetter := func() gitlab.Gitlab {
				return mockGitlab{
					G:                  g,
					ExpectedRepository: gitlabConf.ProjectID,
					ExpectedPRNumber:   MRiid,
					Branch:             inferredBranch,
					Error:              nil,
				}
			}

			tryInferPullRequestGitlabBranch(gitlabConf, MRiid, gitlabClientGetter)

			g.Expect(gitlabConf).ToNot(Equal(before))
			g.Expect(gitlabConf.Branch).To(Equal(inferredBranch))
		})

		It("interBranch=True defaultBranch='test2', use branch from MR", func() {
			gitlabConf.SetInferBranch(true)
			gitlabConf.Branch = "test2"
			before := gitlabConf.DeepCopy()

			inferredBranch := "falsebranch"

			gitlabClientGetter := func() gitlab.Gitlab {
				return mockGitlab{
					G:                  g,
					ExpectedRepository: gitlabConf.ProjectID,
					ExpectedPRNumber:   MRiid,
					Branch:             inferredBranch,
					Error:              nil,
				}
			}

			tryInferPullRequestGitlabBranch(gitlabConf, MRiid, gitlabClientGetter)

			g.Expect(gitlabConf).ToNot(Equal(before))
			g.Expect(gitlabConf.Branch).To(Equal(inferredBranch))
		})

		It("inferBranch=False defaultBranch='', use branch from MR", func() {
			gitlabConf.SetInferBranch(false)
			gitlabConf.Branch = ""
			before := gitlabConf.DeepCopy()

			inferredBranch := "truebranch"

			gitlabClientGetter := func() gitlab.Gitlab {
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
		})

		It("inferBranch=False defaultBranch='test4', use default branch", func() {
			gitlabConf.SetInferBranch(false)
			gitlabConf.Branch = "test4"
			before := gitlabConf.DeepCopy()

			gitlabClientGetter := func() gitlab.Gitlab {
				return mockGitlab{
					G:                  g,
					ExpectedRepository: before.ProjectID,
					ExpectedPRNumber:   MRiid,
					Branch:             "test4",
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
