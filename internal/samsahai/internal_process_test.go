package samsahai

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	s2hv1 "github.com/agoda-com/samsahai/api/v1"
	"github.com/agoda-com/samsahai/internal/util/stringutils"
)

var _ = Describe("S2H internal process", func() {
	g := NewGomegaWithT(GinkgoT())
	It("should correctly remove desired image that out of range", func() {
		var comp1, repoComp1, comp2, repoComp2 = "comp1", "repo/comp1", "comp2", "repoComp2"
		var v009, v110, v111, v112, v113, v114, v115 = "0.0.9", "1.1.0", "1.1.1", "1.1.2", "1.1.3", "1.1.4", "1.1.5"
		maxDesiredMapping := 5
		timeNow := time.Now()
		team := &s2hv1.Team{
			Status: s2hv1.TeamStatus{
				DesiredComponentImageCreatedTime: map[string]map[string]s2hv1.DesiredImageTime{
					comp1: {
						stringutils.ConcatImageString(repoComp1, v110): s2hv1.DesiredImageTime{
							Image:       &s2hv1.Image{Repository: repoComp1, Tag: v110},
							CreatedTime: metav1.Time{Time: timeNow.Add(-1 * time.Hour)},
						},
						stringutils.ConcatImageString(repoComp1, v111): s2hv1.DesiredImageTime{
							Image:       &s2hv1.Image{Repository: repoComp1, Tag: v111},
							CreatedTime: metav1.Time{Time: timeNow.Add(-1 * time.Hour)},
						},
						stringutils.ConcatImageString(repoComp1, v112): s2hv1.DesiredImageTime{
							Image:       &s2hv1.Image{Repository: repoComp1, Tag: v112},
							CreatedTime: metav1.Time{Time: timeNow.Add(-1 * time.Hour)},
						},
					},
					comp2: {
						stringutils.ConcatImageString(repoComp2, v110): {
							Image:       &s2hv1.Image{Repository: repoComp2, Tag: v110},
							CreatedTime: metav1.Time{Time: timeNow.Add(-6 * time.Hour)},
						},
						stringutils.ConcatImageString(repoComp2, v111): {
							Image:       &s2hv1.Image{Repository: repoComp2, Tag: v111},
							CreatedTime: metav1.Time{Time: timeNow.Add(-5 * time.Hour)},
						},
						stringutils.ConcatImageString(repoComp2, v112): {
							Image:       &s2hv1.Image{Repository: repoComp2, Tag: v112},
							CreatedTime: metav1.Time{Time: timeNow.Add(-4 * time.Hour)},
						},
						stringutils.ConcatImageString(repoComp2, v113): {
							Image:       &s2hv1.Image{Repository: repoComp2, Tag: v113},
							CreatedTime: metav1.Time{Time: timeNow.Add(-3 * time.Hour)},
						},
						stringutils.ConcatImageString(repoComp2, v114): {
							Image:       &s2hv1.Image{Repository: repoComp2, Tag: v114},
							CreatedTime: metav1.Time{Time: timeNow.Add(-2 * time.Hour)},
						},
						stringutils.ConcatImageString(repoComp2, v115): {
							Image:       &s2hv1.Image{Repository: repoComp2, Tag: v115},
							CreatedTime: metav1.Time{Time: timeNow.Add(-1 * time.Hour)},
						},
						stringutils.ConcatImageString(repoComp2, v009): {
							Image:       &s2hv1.Image{Repository: repoComp2, Tag: v009},
							CreatedTime: metav1.Time{Time: timeNow.Add(1 * time.Hour)},
						},
					},
				},
			},
		}

		deleteDesiredMappingOutOfRange(team, maxDesiredMapping)
		desired := team.Status.DesiredComponentImageCreatedTime
		desiredCompImageCreatedTime1 := desired[comp1]
		g.Expect(len(desiredCompImageCreatedTime1)).To(Equal(3), "size of comp1 mapping should be matched")
		g.Expect(desiredCompImageCreatedTime1).Should(HaveKey(stringutils.ConcatImageString(repoComp1, v110)))
		g.Expect(desiredCompImageCreatedTime1).Should(HaveKey(stringutils.ConcatImageString(repoComp1, v111)))
		g.Expect(desiredCompImageCreatedTime1).Should(HaveKey(stringutils.ConcatImageString(repoComp1, v112)))

		desiredCompImageCreatedTime2 := desired[comp2]
		g.Expect(len(desiredCompImageCreatedTime2)).To(Equal(5), "size of comp2 mapping should be matched")
		g.Expect(desiredCompImageCreatedTime2).ShouldNot(HaveKey(stringutils.ConcatImageString(repoComp2, v110)))
		g.Expect(desiredCompImageCreatedTime2).ShouldNot(HaveKey(stringutils.ConcatImageString(repoComp2, v111)))
		g.Expect(desiredCompImageCreatedTime2).Should(HaveKey(stringutils.ConcatImageString(repoComp2, v112)))
		g.Expect(desiredCompImageCreatedTime2).Should(HaveKey(stringutils.ConcatImageString(repoComp2, v113)))
		g.Expect(desiredCompImageCreatedTime2).Should(HaveKey(stringutils.ConcatImageString(repoComp2, v114)))
		g.Expect(desiredCompImageCreatedTime2).Should(HaveKey(stringutils.ConcatImageString(repoComp2, v115)))
		g.Expect(desiredCompImageCreatedTime2).Should(HaveKey(stringutils.ConcatImageString(repoComp2, v009)))
	})

	It("should correctly not remove desired image that still in active components", func() {
		var comp1, repoComp1, comp2, repoComp2 = "comp1", "repo/comp1", "comp2", "repoComp2"
		var v009, v110, v111, v112, v113, v114, v115, v116 = "0.0.9", "1.1.0", "1.1.1", "1.1.2", "1.1.3", "1.1.4", "1.1.5", "1.1.6"
		maxDesiredMapping := 5
		timeNow := time.Now()
		team := &s2hv1.Team{
			Status: s2hv1.TeamStatus{
				ActiveComponents: map[string]s2hv1.StableComponent{
					comp1: {
						Spec: s2hv1.StableComponentSpec{
							Name:       comp1,
							Repository: repoComp1,
							Version:    v110,
						},
					},
					comp2: {
						Spec: s2hv1.StableComponentSpec{
							Name:       comp2,
							Repository: repoComp2,
							Version:    v114,
						},
					},
				},
				DesiredComponentImageCreatedTime: map[string]map[string]s2hv1.DesiredImageTime{
					comp1: {
						stringutils.ConcatImageString(repoComp1, v116): {
							Image:       &s2hv1.Image{Repository: repoComp1, Tag: v009},
							CreatedTime: metav1.Time{Time: timeNow.Add(-7 * time.Hour)},
						},
						stringutils.ConcatImageString(repoComp1, v110): {
							Image:       &s2hv1.Image{Repository: repoComp1, Tag: v110},
							CreatedTime: metav1.Time{Time: timeNow.Add(-6 * time.Hour)},
						},
						stringutils.ConcatImageString(repoComp1, v111): {
							Image:       &s2hv1.Image{Repository: repoComp1, Tag: v111},
							CreatedTime: metav1.Time{Time: timeNow.Add(-5 * time.Hour)},
						},
						stringutils.ConcatImageString(repoComp1, v112): {
							Image:       &s2hv1.Image{Repository: repoComp1, Tag: v112},
							CreatedTime: metav1.Time{Time: timeNow.Add(-4 * time.Hour)},
						},
						stringutils.ConcatImageString(repoComp1, v113): {
							Image:       &s2hv1.Image{Repository: repoComp1, Tag: v113},
							CreatedTime: metav1.Time{Time: timeNow.Add(-3 * time.Hour)},
						},
						stringutils.ConcatImageString(repoComp1, v114): {
							Image:       &s2hv1.Image{Repository: repoComp1, Tag: v114},
							CreatedTime: metav1.Time{Time: timeNow.Add(-2 * time.Hour)},
						},
						stringutils.ConcatImageString(repoComp1, v115): {
							Image:       &s2hv1.Image{Repository: repoComp1, Tag: v115},
							CreatedTime: metav1.Time{Time: timeNow.Add(-1 * time.Hour)},
						},
						stringutils.ConcatImageString(repoComp1, v009): {
							Image:       &s2hv1.Image{Repository: repoComp1, Tag: v116},
							CreatedTime: metav1.Time{Time: timeNow.Add(1 * time.Hour)},
						},
					},
					comp2: {
						stringutils.ConcatImageString(repoComp2, v110): {
							Image:       &s2hv1.Image{Repository: repoComp2, Tag: v110},
							CreatedTime: metav1.Time{Time: timeNow.Add(-6 * time.Hour)},
						},
						stringutils.ConcatImageString(repoComp2, v111): {
							Image:       &s2hv1.Image{Repository: repoComp2, Tag: v111},
							CreatedTime: metav1.Time{Time: timeNow.Add(-5 * time.Hour)},
						},
						stringutils.ConcatImageString(repoComp2, v112): {
							Image:       &s2hv1.Image{Repository: repoComp2, Tag: v112},
							CreatedTime: metav1.Time{Time: timeNow.Add(-4 * time.Hour)},
						},
						stringutils.ConcatImageString(repoComp2, v113): {
							Image:       &s2hv1.Image{Repository: repoComp2, Tag: v113},
							CreatedTime: metav1.Time{Time: timeNow.Add(-3 * time.Hour)},
						},
						stringutils.ConcatImageString(repoComp2, v114): {
							Image:       &s2hv1.Image{Repository: repoComp2, Tag: v114},
							CreatedTime: metav1.Time{Time: timeNow.Add(-2 * time.Hour)},
						},
						stringutils.ConcatImageString(repoComp2, v115): {
							Image:       &s2hv1.Image{Repository: repoComp2, Tag: v115},
							CreatedTime: metav1.Time{Time: timeNow.Add(-1 * time.Hour)},
						},
						stringutils.ConcatImageString(repoComp2, v116): {
							Image:       &s2hv1.Image{Repository: repoComp2, Tag: v116},
							CreatedTime: metav1.Time{Time: timeNow.Add(1 * time.Hour)},
						},
					},
				},
			},
		}

		deleteDesiredMappingOutOfRange(team, maxDesiredMapping)
		desired := team.Status.DesiredComponentImageCreatedTime
		desiredCompImageCreatedTime1 := desired[comp1]
		g.Expect(len(desiredCompImageCreatedTime1)).To(Equal(7), "size of comp1 mapping should be matched")
		g.Expect(desiredCompImageCreatedTime1).ShouldNot(HaveKey(stringutils.ConcatImageString(repoComp1, v116)))
		g.Expect(desiredCompImageCreatedTime1).Should(HaveKey(stringutils.ConcatImageString(repoComp1, v110)))
		g.Expect(desiredCompImageCreatedTime1).Should(HaveKey(stringutils.ConcatImageString(repoComp1, v111)))
		g.Expect(desiredCompImageCreatedTime1).Should(HaveKey(stringutils.ConcatImageString(repoComp1, v112)))
		g.Expect(desiredCompImageCreatedTime1).Should(HaveKey(stringutils.ConcatImageString(repoComp1, v113)))
		g.Expect(desiredCompImageCreatedTime1).Should(HaveKey(stringutils.ConcatImageString(repoComp1, v114)))
		g.Expect(desiredCompImageCreatedTime1).Should(HaveKey(stringutils.ConcatImageString(repoComp1, v115)))
		g.Expect(desiredCompImageCreatedTime1).Should(HaveKey(stringutils.ConcatImageString(repoComp1, v009)))

		desiredCompImageCreatedTime2 := desired[comp2]
		g.Expect(len(desiredCompImageCreatedTime2)).To(Equal(5), "size of comp2 mapping should be matched")
		g.Expect(desiredCompImageCreatedTime2).ShouldNot(HaveKey(stringutils.ConcatImageString(repoComp2, v110)))
		g.Expect(desiredCompImageCreatedTime2).ShouldNot(HaveKey(stringutils.ConcatImageString(repoComp2, v111)))
		g.Expect(desiredCompImageCreatedTime2).Should(HaveKey(stringutils.ConcatImageString(repoComp2, v113)))
		g.Expect(desiredCompImageCreatedTime2).Should(HaveKey(stringutils.ConcatImageString(repoComp2, v114)))
		g.Expect(desiredCompImageCreatedTime2).Should(HaveKey(stringutils.ConcatImageString(repoComp2, v115)))
		g.Expect(desiredCompImageCreatedTime2).Should(HaveKey(stringutils.ConcatImageString(repoComp2, v116)))
	})
})
