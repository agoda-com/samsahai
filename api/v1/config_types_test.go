package v1_test

import (
	"testing"

	v1 "github.com/agoda-com/samsahai/api/v1"
	"github.com/agoda-com/samsahai/internal/util/unittest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestConfigOverrider(t *testing.T) {
	unittest.InitGinkgo(t, "Test Config Overrider")
}

var _ = Describe("Config Overrider", func() {
	g := NewWithT(GinkgoT())

	Describe("ConfigTeamcityOverrider", func() {
		var overrider v1.ConfigTeamcityOverrider
		var confTeamcity *v1.ConfigTeamcity
		var beforeOverride *v1.ConfigTeamcity
		var res *v1.ConfigTeamcity

		BeforeEach(func() {
			overrider = v1.ConfigTeamcityOverrider{}
			confTeamcity = nil
			beforeOverride = nil
			res = nil
		})

		It("should return new pointer if the old is nil", func() {
			buildTypeID := "123"
			branch := "branch"
			overrider = v1.ConfigTeamcityOverrider{
				BuildTypeID: &buildTypeID,
				Branch:      &branch,
			}
			res = overrider.Override(nil)
			g.Expect(res).To(Equal(&v1.ConfigTeamcity{
				BuildTypeID: buildTypeID,
				Branch:      branch,
			}))
		})

		Context("nothing to override", func() {
			Specify("overridden is nil", func() {
				res = overrider.Override(nil)
				g.Expect(res).To(BeNil())
			})

			Specify("overridden is not nil", func() {
				confTeamcity = &v1.ConfigTeamcity{
					BuildTypeID: "123",
					Branch:      "branch",
				}
				beforeOverride = confTeamcity.DeepCopy()
				res = overrider.Override(confTeamcity)

				// expect to not change anything
				g.Expect(confTeamcity).To(Equal(beforeOverride))
				// expect to yield the input
				g.Expect(res).To(BeIdenticalTo(confTeamcity))
			})
		})

		Context("there is something to override", func() {
			Specify("override one field", func() {
				branch := "456"
				overrider = v1.ConfigTeamcityOverrider{
					Branch: &branch,
				}
				confTeamcity = &v1.ConfigTeamcity{
					BuildTypeID: "123",
					Branch:      "branch",
				}
				beforeOverride = confTeamcity.DeepCopy()
				res = overrider.Override(confTeamcity)
				// expect to override 1 field
				g.Expect(res).To(Equal(&v1.ConfigTeamcity{
					BuildTypeID: beforeOverride.BuildTypeID,
					Branch:      branch,
				}))
				// expect to yield the input pointer
				g.Expect(res).To(BeIdenticalTo(confTeamcity))
			})

			Specify("override more than one field", func() {
				buildTypeID := "id"
				branch := "456"
				overrider = v1.ConfigTeamcityOverrider{
					BuildTypeID: &buildTypeID,
					Branch:      &branch,
				}
				confTeamcity = &v1.ConfigTeamcity{
					BuildTypeID: "123",
					Branch:      "branch",
				}
				beforeOverride = confTeamcity.DeepCopy()
				res = overrider.Override(confTeamcity)
				// expect to override fields
				g.Expect(res).To(Equal(&v1.ConfigTeamcity{
					BuildTypeID: buildTypeID,
					Branch:      branch,
				}))
				// expect to yield the input pointer
				g.Expect(res).To(BeIdenticalTo(confTeamcity))
			})
		})
	})

	Describe("ConfigGitlabOverrider", func() {
		g := NewWithT(GinkgoT())
		var overrider v1.ConfigGitlabOverrider
		var confGitlab *v1.ConfigGitlab
		var beforeOverride *v1.ConfigGitlab
		var res *v1.ConfigGitlab

		BeforeEach(func() {
			overrider = v1.ConfigGitlabOverrider{}
			confGitlab = nil
			beforeOverride = nil
			res = nil
		})

		It("should return new pointer if the old is nil", func() {
			projectID := "123"
			branch := "branch"
			overrider = v1.ConfigGitlabOverrider{
				ProjectID: &projectID,
				Branch:    &branch,
			}
			res = overrider.Override(nil)
			g.Expect(res).To(Equal(&v1.ConfigGitlab{
				ProjectID: projectID,
				Branch:    branch,
			}))
		})

		Context("nothing to override", func() {
			Specify("overridden is nil", func() {
				res = overrider.Override(nil)
				g.Expect(res).To(BeNil())
			})

			Specify("overridden is not nil", func() {
				confGitlab = &v1.ConfigGitlab{
					ProjectID:            "123",
					Branch:               "branch",
					PipelineTriggerToken: "token",
				}
				beforeOverride = confGitlab.DeepCopy()
				res = overrider.Override(confGitlab)

				// expect to not change anything
				g.Expect(confGitlab).To(Equal(beforeOverride))
				// expect to yield the input
				g.Expect(res).To(BeIdenticalTo(confGitlab))
			})
		})

		Context("there is something to override", func() {
			Specify("override one field", func() {
				branch := "456"
				overrider = v1.ConfigGitlabOverrider{
					Branch: &branch,
				}
				confGitlab = &v1.ConfigGitlab{
					ProjectID:            "123",
					Branch:               "branch",
					PipelineTriggerToken: "token",
				}
				beforeOverride = confGitlab.DeepCopy()
				res = overrider.Override(confGitlab)
				// expect to override 1 field
				g.Expect(res).To(Equal(&v1.ConfigGitlab{
					ProjectID:            beforeOverride.ProjectID,
					Branch:               branch,
					PipelineTriggerToken: beforeOverride.PipelineTriggerToken,
				}))
				// expect to yield the input pointer
				g.Expect(res).To(BeIdenticalTo(confGitlab))
			})

			Specify("override more than one field", func() {
				projectID := "id"
				branch := "456"
				pipelineTriggerToken := "pipelinetriggertoken"
				overrider = v1.ConfigGitlabOverrider{
					ProjectID:            &projectID,
					Branch:               &branch,
					PipelineTriggerToken: &pipelineTriggerToken,
				}
				confGitlab = &v1.ConfigGitlab{
					ProjectID:            "123",
					Branch:               "branch",
					PipelineTriggerToken: "token",
				}
				beforeOverride = confGitlab.DeepCopy()
				res = overrider.Override(confGitlab)
				// expect to override fields
				g.Expect(res).To(Equal(&v1.ConfigGitlab{
					ProjectID:            projectID,
					Branch:               branch,
					PipelineTriggerToken: pipelineTriggerToken,
				}))
				// expect to yield the input pointer
				g.Expect(res).To(BeIdenticalTo(confGitlab))
			})
		})
	})
})
