package staging

import (
	"testing"
	"time"

	s2hv1 "github.com/agoda-com/samsahai/api/v1"
	"github.com/agoda-com/samsahai/internal/util/gitlab"
	"github.com/agoda-com/samsahai/internal/util/unittest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestStagingUtil(t *testing.T) {
	unittest.InitGinkgo(t, "Staging util")
}

var _ = Describe("overrideTestRunner", func() {
	var testRunner *s2hv1.ConfigTestRunner
	var overrider s2hv1.ConfigTestRunnerOverrider
	var queue *s2hv1.Queue
	var gitlabClientGetter func(token string) gitlab.Gitlab
	g := NewWithT(GinkgoT())

	BeforeEach(func() {
		testRunner = nil
		overrider = s2hv1.ConfigTestRunnerOverrider{}
		queue = &s2hv1.Queue{}
		gitlabClientGetter = nil
	})

	It("should ignore testRunner if nothing to override", func() {
		testRunner = &s2hv1.ConfigTestRunner{
			Timeout:     v1.Duration{Duration: 40},
			PollingTime: v1.Duration{Duration: 50},
		}
		before := testRunner.DeepCopy()
		res := overrideTestRunner(testRunner, overrider, queue, gitlabClientGetter)
		g.Expect(res).To(Equal(before))
		g.Expect(testRunner).To(Equal(before))
	})

	It("should override testRunner field by field", func() {
		testRunner = &s2hv1.ConfigTestRunner{
			Timeout:     v1.Duration{Duration: 10},
			PollingTime: v1.Duration{Duration: 20},
			Gitlab: &s2hv1.ConfigGitlab{
				ProjectID:            "123",
				Branch:               "abc",
				PipelineTriggerToken: "xyz",
			},
			Teamcity: nil,
		}
		before := testRunner.DeepCopy()

		expectedProjectID := "456"
		expectedTeamcityBranch := "teamcity"

		expectedTestRunner := testRunner.DeepCopy()
		expectedTestRunner.Timeout = v1.Duration{Duration: time.Minute * 5}
		expectedTestRunner.Gitlab.ProjectID = expectedProjectID
		expectedTestRunner.Teamcity = &s2hv1.ConfigTeamcity{
			Branch: expectedTeamcityBranch,
		}

		overrider = s2hv1.ConfigTestRunnerOverrider{
			Timeout: expectedTestRunner.Timeout.DeepCopy(),
			Gitlab: &s2hv1.ConfigGitlabOverrider{
				ProjectID: &expectedProjectID,
			},
			Teamcity: &s2hv1.ConfigTeamcityOverrider{
				Branch: &expectedTeamcityBranch,
			},
		}

		res := overrideTestRunner(testRunner, overrider, queue, gitlabClientGetter)
		g.Expect(res).To(Equal(expectedTestRunner))
		g.Expect(testRunner).To(Equal(before))
	})

	Context("PullRequest Flow", func() {
		BeforeEach(func() {
			queue.Spec.Type = s2hv1.QueueTypePullRequest
		})

		It("should be able to fetch branch from gitlab", func() {
			testRunner = &s2hv1.ConfigTestRunner{
				Timeout:     v1.Duration{Duration: 10},
				PollingTime: v1.Duration{Duration: 20},
				Gitlab: &s2hv1.ConfigGitlab{
					ProjectID:            "123",
					Branch:               "abc",
					PipelineTriggerToken: "xyz",
				},
				Teamcity: nil,
			}
			before := testRunner.DeepCopy()

			expectedGitlabBranch := "realbranch"
			expectedPRNumber := "6767"

			expectedTestRunner := testRunner.DeepCopy()
			expectedTestRunner.Gitlab.Branch = expectedGitlabBranch

			infer := true
			overrider = s2hv1.ConfigTestRunnerOverrider{
				ConfigTestRunnerOverriderExtraParameters: s2hv1.ConfigTestRunnerOverriderExtraParameters{
					PullRequestInferGitlabMRBranch: &infer,
				},
			}

			gitlabClientGetter = func(token string) gitlab.Gitlab {
				g.Expect(token).To(Equal(before.Gitlab.PipelineTriggerToken))
				return mockGitlab{
					G:                  g,
					ExpectedRepository: before.Gitlab.ProjectID,
					ExpectedPRNumber:   expectedPRNumber,
					Branch:             expectedGitlabBranch,
					Error:              nil,
				}
			}

			queue.Spec.PRNumber = expectedPRNumber

			res := overrideTestRunner(testRunner, overrider, queue, gitlabClientGetter)

			g.Expect(res).To(Equal(expectedTestRunner))
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
