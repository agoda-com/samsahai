package samsahai

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	s2hv1 "github.com/agoda-com/samsahai/api/v1"
	"github.com/agoda-com/samsahai/internal"
	"github.com/agoda-com/samsahai/internal/util/unittest"
)

func TestController(t *testing.T) {
	unittest.InitGinkgo(t, "Samsahai")
}

var _ = Describe("S2H Controller", func() {
	g := NewWithT(GinkgoT())

	Describe("plugins", func() {
		ctrl := controller{
			checkers: map[string]internal.DesiredComponentChecker{},
			plugins:  map[string]internal.Plugin{},
		}
		pluginName := "example"
		mockTeam := s2hv1.Team{
			Spec: s2hv1.TeamSpec{
				Owners: []string{"teamTest@samsahai.io"},
				StagingCtrl: &s2hv1.StagingCtrl{
					IsDeploy: false,
				},
			},
		}
		mockTeamUsingTemplate := s2hv1.Team{}

		It("should successfully load plugins", func() {
			ctrl.loadPlugins("plugin")

			g.Expect(len(ctrl.plugins)).To(Equal(1))
			name := ctrl.plugins[pluginName].GetName()
			g.Expect(name).To(Equal(pluginName))

			g.Expect(len(ctrl.checkers)).To(Equal(1))
			name = ctrl.checkers[pluginName].GetName()
			g.Expect(name).To(Equal(pluginName))
		})

		It("should apply template to team correctly", func() {
			g := NewWithT(GinkgoT())

			err := applyTeamTemplate(&mockTeamUsingTemplate, &mockTeam)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(mockTeamUsingTemplate.Status.Used.Owners).To(Equal(mockTeam.Spec.Owners))
			g.Expect(mockTeamUsingTemplate.Status.Used.StagingCtrl).To(Equal(mockTeam.Spec.StagingCtrl))
		})

		Specify("Non-existing plugin", func(done Done) {
			close(done)
			ctrl.loadPlugins("non-existing")
		}, 1)
	})
})
