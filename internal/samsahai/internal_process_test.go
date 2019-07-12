package samsahai

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/agoda-com/samsahai/internal/util/stringutils"
	s2hv1beta1 "github.com/agoda-com/samsahai/pkg/apis/env/v1beta1"
)

var _ = Describe("S2H internal process", func() {
	g := NewGomegaWithT(GinkgoT())
	It("should correctly delete desired mapping out of range", func() {
		var comp1, repoComp1, comp2, repoComp2 = "comp1", "repo/comp1", "comp2", "repoComp2"
		var v110, v111, v112, v113, v114, v115, v116 = "1.1.0", "1.1.1", "1.1.2", "1.1.3", "1.1.4", "1.1.5", "1.1.6"
		maxDesiredMapping := 5
		timeNow := time.Now()
		team := &s2hv1beta1.Team{
			Status: s2hv1beta1.TeamStatus{
				DesiredComponentImageCreatedTime: map[string]map[string]s2hv1beta1.DesiredImageTime{
					comp1: {
						stringutils.ConcatImageString(repoComp1, v110): s2hv1beta1.DesiredImageTime{
							Image:       &s2hv1beta1.Image{Repository: repoComp1, Tag: v110},
							CreatedTime: metav1.Time{Time: timeNow.Add(-1 * time.Hour)},
						},
						stringutils.ConcatImageString(repoComp1, v111): s2hv1beta1.DesiredImageTime{
							Image:       &s2hv1beta1.Image{Repository: repoComp1, Tag: v111},
							CreatedTime: metav1.Time{Time: timeNow.Add(-1 * time.Hour)},
						},
						stringutils.ConcatImageString(repoComp1, v112): s2hv1beta1.DesiredImageTime{
							Image:       &s2hv1beta1.Image{Repository: repoComp1, Tag: v112},
							CreatedTime: metav1.Time{Time: timeNow.Add(-1 * time.Hour)},
						},
					},
					comp2: {
						stringutils.ConcatImageString(repoComp2, v110): s2hv1beta1.DesiredImageTime{
							Image:       &s2hv1beta1.Image{Repository: repoComp2, Tag: v110},
							CreatedTime: metav1.Time{Time: timeNow.Add(-1 * time.Hour)},
						},
						stringutils.ConcatImageString(repoComp2, v111): s2hv1beta1.DesiredImageTime{
							Image:       &s2hv1beta1.Image{Repository: repoComp2, Tag: v111},
							CreatedTime: metav1.Time{Time: timeNow.Add(-1 * time.Hour)},
						},
						stringutils.ConcatImageString(repoComp2, v112): s2hv1beta1.DesiredImageTime{
							Image:       &s2hv1beta1.Image{Repository: repoComp2, Tag: v112},
							CreatedTime: metav1.Time{Time: timeNow.Add(-1 * time.Hour)},
						},
						stringutils.ConcatImageString(repoComp2, v113): s2hv1beta1.DesiredImageTime{
							Image:       &s2hv1beta1.Image{Repository: repoComp2, Tag: v113},
							CreatedTime: metav1.Time{Time: timeNow.Add(-1 * time.Hour)},
						},
						stringutils.ConcatImageString(repoComp2, v114): s2hv1beta1.DesiredImageTime{
							Image:       &s2hv1beta1.Image{Repository: repoComp2, Tag: v114},
							CreatedTime: metav1.Time{Time: timeNow.Add(-1 * time.Hour)},
						},
						stringutils.ConcatImageString(repoComp2, v115): s2hv1beta1.DesiredImageTime{
							Image:       &s2hv1beta1.Image{Repository: repoComp2, Tag: v115},
							CreatedTime: metav1.Time{Time: timeNow.Add(-1 * time.Hour)},
						},
						stringutils.ConcatImageString(repoComp2, v116): s2hv1beta1.DesiredImageTime{
							Image:       &s2hv1beta1.Image{Repository: repoComp2, Tag: v116},
							CreatedTime: metav1.Time{Time: timeNow.Add(-1 * time.Hour)},
						},
					},
				},
			},
		}

		deleteDesiredMappingOutOfRange(team, maxDesiredMapping)
		m := team.Status.DesiredComponentImageCreatedTime
		g.Expect(len(m["comp1"])).To(Equal(3), "size of mapping should be matched")
		g.Expect(len(m["comp2"])).To(Equal(5), "size of mapping should be matched")
		g.Expect(m["comp2"]).ShouldNot(HaveKey("1.1.5"),
			"outdated desired component should be deleted")
		g.Expect(m["comp2"]).ShouldNot(HaveKey("1.1.6"),
			"outdated desired component should be deleted")
	})
})
