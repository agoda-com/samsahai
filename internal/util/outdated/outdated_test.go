package outdated

import (
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	s2hv1beta1 "github.com/agoda-com/samsahai/api/v1beta1"
	"github.com/agoda-com/samsahai/internal/util/stringutils"
	"github.com/agoda-com/samsahai/internal/util/unittest"
)

func TestUnit(t *testing.T) {
	unittest.InitGinkgo(t, "Component Outdated Duration")
}

var _ = Describe("set outdated duration when input objects are empty", func() {
	g := NewGomegaWithT(GinkgoT())
	It("should return empty outdated component when input objects are nil", func() {
		atpRpt := &s2hv1beta1.ActivePromotionStatus{}
		nowMockTime := time.Date(2019, 10, 3, 9, 0, 0, 0, time.UTC)
		oMock := newMock(nil, nil, nil, nowMockTime)
		oMock.SetOutdatedDuration(atpRpt)
		g.Expect(atpRpt.HasOutdatedComponent).To(BeFalse(), "should not have outdated duration")
		g.Expect(len(atpRpt.OutdatedComponents)).To(Equal(0))
	})

	It("should return empty outdated component when stable component list is nil", func() {
		atpRpt := &s2hv1beta1.ActivePromotionStatus{}
		desiredComps := make(map[string]map[string]s2hv1beta1.DesiredImageTime)
		nowMockTime := time.Date(2019, 10, 3, 9, 0, 0, 0, time.UTC)
		oMock := newMock(nil, desiredComps, nil, nowMockTime)
		oMock.SetOutdatedDuration(atpRpt)
		g.Expect(atpRpt.HasOutdatedComponent).To(BeFalse(), "should not have outdated duration")
		g.Expect(len(atpRpt.OutdatedComponents)).To(Equal(0))
	})

	It("should return empty outdated component when desired version time list is nil", func() {
		atpRpt := &s2hv1beta1.ActivePromotionStatus{}
		stableComps := []s2hv1beta1.StableComponent{
			{
				Spec: s2hv1beta1.StableComponentSpec{
					Name:       "comp1",
					Repository: "image-1",
					Version:    "1.1.0",
				},
			},
		}

		nowMockTime := time.Date(2019, 10, 3, 9, 0, 0, 0, time.UTC)
		oMock := newMock(nil, nil, stableComps, nowMockTime)
		oMock.SetOutdatedDuration(atpRpt)
		g.Expect(atpRpt.HasOutdatedComponent).To(BeFalse(), "should not have outdated duration")
		g.Expect(len(atpRpt.OutdatedComponents)).To(Equal(0))
	})

	It("should return empty outdated component when version in stable and desired do not match", func() {
		var comp1, repoComp1, v110, v1 = "comp1", "repo/comp1", "1.1.0", "v1"
		atpRpt := &s2hv1beta1.ActivePromotionStatus{}
		desiredComps := map[string]map[string]s2hv1beta1.DesiredImageTime{
			comp1: {
				stringutils.ConcatImageString(repoComp1, v110): s2hv1beta1.DesiredImageTime{
					Image:       &s2hv1beta1.Image{Repository: repoComp1, Tag: v110},
					CreatedTime: metav1.Time{Time: time.Date(2019, 10, 1, 9, 0, 0, 0, time.UTC)},
				},
			},
		}
		stableComps := []s2hv1beta1.StableComponent{
			{
				Spec: s2hv1beta1.StableComponentSpec{
					Name:       comp1,
					Repository: repoComp1,
					Version:    v1,
				},
			},
		}

		nowMockTime := time.Date(2019, 10, 3, 9, 0, 0, 0, time.UTC)
		oMock := newMock(nil, desiredComps, stableComps, nowMockTime)
		oMock.SetOutdatedDuration(atpRpt)
		g.Expect(atpRpt.HasOutdatedComponent).To(BeFalse(), "should not have outdated duration")
		g.Expect(len(atpRpt.OutdatedComponents)).To(Equal(0))
	})
})

var _ = Describe("set outdated duration when active stable version is eq to latest desired version", func() {
	g := NewGomegaWithT(GinkgoT())
	cfg := &s2hv1beta1.ConfigSpec{}

	It("should return outdated duration as zero", func() {
		var comp1, repoComp1, v110 = "comp1", "repo/comp1", "1.1.0"
		var comp2, repoComp2, v212 = "comp2", "repo/comp2", "2.1.2"
		atpRpt := &s2hv1beta1.ActivePromotionStatus{}
		desiredComps := map[string]map[string]s2hv1beta1.DesiredImageTime{
			comp1: {
				stringutils.ConcatImageString(repoComp1, v110): s2hv1beta1.DesiredImageTime{
					Image:       &s2hv1beta1.Image{Repository: repoComp1, Tag: v110},
					CreatedTime: metav1.Time{Time: time.Date(2019, 10, 1, 9, 0, 0, 0, time.UTC)},
				},
			},
			comp2: {
				stringutils.ConcatImageString(repoComp2, v212): s2hv1beta1.DesiredImageTime{
					Image:       &s2hv1beta1.Image{Repository: repoComp2, Tag: v212},
					CreatedTime: metav1.Time{Time: time.Date(2019, 10, 2, 9, 0, 0, 0, time.UTC)},
				},
			},
		}
		stableComps := []s2hv1beta1.StableComponent{
			{
				Spec: s2hv1beta1.StableComponentSpec{
					Name:       comp1,
					Repository: repoComp1,
					Version:    v110,
				},
			},
			{
				Spec: s2hv1beta1.StableComponentSpec{
					Name:       comp2,
					Repository: repoComp2,
					Version:    v212,
				},
			},
		}
		nowMockTime := time.Date(2019, 10, 3, 9, 0, 0, 0, time.UTC)
		oMock := newMock(cfg, desiredComps, stableComps, nowMockTime)
		oMock.SetOutdatedDuration(atpRpt)
		g.Expect(atpRpt.HasOutdatedComponent).To(BeFalse(), "should not have outdated duration")

		outdatedComp1 := atpRpt.OutdatedComponents[comp1]
		g.Expect(outdatedComp1.CurrentImage.Tag).To(Equal(v110))
		g.Expect(outdatedComp1.DesiredImage.Tag).To(Equal(v110))
		g.Expect(outdatedComp1.OutdatedDuration).To(Equal(time.Duration(0)))

		outdatedComp2 := atpRpt.OutdatedComponents[comp2]
		g.Expect(outdatedComp2.CurrentImage.Tag).To(Equal(v212))
		g.Expect(outdatedComp2.DesiredImage.Tag).To(Equal(v212))
		g.Expect(outdatedComp2.OutdatedDuration).To(Equal(time.Duration(0)))
	})

	It("should return outdated duration as zero when rollback version is latest desired version", func() {
		var comp1, repoComp1 = "comp1", "repo/comp1"
		var v110, v113, v114 = "1.1.0", "1.1.3", "1.1.4"
		atpRpt := &s2hv1beta1.ActivePromotionStatus{}
		desiredComps := map[string]map[string]s2hv1beta1.DesiredImageTime{
			comp1: {
				stringutils.ConcatImageString(repoComp1, v110): s2hv1beta1.DesiredImageTime{
					Image:       &s2hv1beta1.Image{Repository: repoComp1, Tag: v110},
					CreatedTime: metav1.Time{Time: time.Date(2019, 10, 1, 2, 0, 0, 0, time.UTC)},
				},
				stringutils.ConcatImageString(repoComp1, v114): s2hv1beta1.DesiredImageTime{
					Image:       &s2hv1beta1.Image{Repository: repoComp1, Tag: v114},
					CreatedTime: metav1.Time{Time: time.Date(2019, 10, 5, 2, 0, 0, 0, time.UTC)},
				},
				stringutils.ConcatImageString(repoComp1, v113): s2hv1beta1.DesiredImageTime{
					Image:       &s2hv1beta1.Image{Repository: repoComp1, Tag: v113},
					CreatedTime: metav1.Time{Time: time.Date(2019, 10, 6, 2, 0, 0, 0, time.UTC)},
				},
			},
		}
		stableComps := []s2hv1beta1.StableComponent{
			{
				Spec: s2hv1beta1.StableComponentSpec{
					Name:       comp1,
					Repository: repoComp1,
					Version:    v113,
				},
			},
		}
		nowMockTime := time.Date(2019, 10, 3, 9, 0, 0, 0, time.UTC)
		oMock := newMock(cfg, desiredComps, stableComps, nowMockTime)
		oMock.SetOutdatedDuration(atpRpt)
		g.Expect(atpRpt.HasOutdatedComponent).To(BeFalse(), "should not have outdated duration")

		outdatedComp1 := atpRpt.OutdatedComponents[comp1]
		g.Expect(outdatedComp1.CurrentImage.Tag).To(Equal(v113))
		g.Expect(outdatedComp1.DesiredImage.Tag).To(Equal(v113))
		g.Expect(outdatedComp1.OutdatedDuration).To(Equal(time.Duration(0)))
	})
})

var _ = Describe("set outdated duration when active stable version is not eq to latest desired version", func() {
	g := NewGomegaWithT(GinkgoT())
	cfg := &s2hv1beta1.ConfigSpec{
		ActivePromotion: &s2hv1beta1.ConfigActivePromotion{
			OutdatedNotification: &s2hv1beta1.OutdatedNotification{},
		},
	}

	It("should return outdated duration correctly when outdated for 1 version", func() {
		var comp1, repoComp1, v110, v113 = "comp1", "repo/comp1", "1.1.0", "1.1.3"
		var comp2, repoComp2, v210, v212 = "comp2", "repo/comp2", "2.1.0", "2.1.2"
		atpRpt := &s2hv1beta1.ActivePromotionStatus{}
		desiredComps := map[string]map[string]s2hv1beta1.DesiredImageTime{
			comp1: {
				stringutils.ConcatImageString(repoComp1, v110): s2hv1beta1.DesiredImageTime{
					Image:       &s2hv1beta1.Image{Repository: repoComp1, Tag: v110},
					CreatedTime: metav1.Time{Time: time.Date(2019, 10, 1, 2, 0, 0, 0, time.UTC)},
				},
				stringutils.ConcatImageString(repoComp1, v113): s2hv1beta1.DesiredImageTime{
					Image:       &s2hv1beta1.Image{Repository: repoComp1, Tag: v113},
					CreatedTime: metav1.Time{Time: time.Date(2019, 10, 3, 2, 0, 0, 0, time.UTC)},
				},
			},
			comp2: {
				stringutils.ConcatImageString(repoComp2, v210): s2hv1beta1.DesiredImageTime{
					Image:       &s2hv1beta1.Image{Repository: repoComp2, Tag: v210},
					CreatedTime: metav1.Time{Time: time.Date(2019, 10, 1, 2, 0, 0, 0, time.UTC)},
				},
				stringutils.ConcatImageString(repoComp2, v212): s2hv1beta1.DesiredImageTime{
					Image:       &s2hv1beta1.Image{Repository: repoComp2, Tag: v212},
					CreatedTime: metav1.Time{Time: time.Date(2019, 10, 2, 2, 0, 0, 0, time.UTC)},
				},
			},
		}
		stableComps := []s2hv1beta1.StableComponent{
			{
				Spec: s2hv1beta1.StableComponentSpec{
					Name:       comp1,
					Repository: repoComp1,
					Version:    v110,
				},
			},
			{
				Spec: s2hv1beta1.StableComponentSpec{
					Name:       comp2,
					Repository: repoComp2,
					Version:    v210,
				},
			},
		}
		nowMockTime := time.Date(2019, 10, 3, 9, 0, 0, 0, time.UTC)
		oMock := newMock(cfg, desiredComps, stableComps, nowMockTime)
		oMock.SetOutdatedDuration(atpRpt)
		g.Expect(atpRpt.HasOutdatedComponent).To(BeTrue(), "should have outdated components")

		outdatedComp1 := atpRpt.OutdatedComponents[comp1]
		g.Expect(outdatedComp1.CurrentImage.Tag).To(Equal(v110))
		g.Expect(outdatedComp1.DesiredImage.Tag).To(Equal(v113))
		g.Expect(outdatedComp1.OutdatedDuration).To(Equal(time.Duration(25260000000000))) // 0d7h1m

		outdatedComp2 := atpRpt.OutdatedComponents[comp2]
		g.Expect(outdatedComp2.CurrentImage.Tag).To(Equal(v210))
		g.Expect(outdatedComp2.DesiredImage.Tag).To(Equal(v212))
		g.Expect(outdatedComp2.OutdatedDuration).To(Equal(time.Duration(111660000000000))) // 1d7h1m
	})

	It("should return outdated duration correctly when outdated for 2 versions", func() {
		var comp1, repoComp1 = "comp1", "repo/comp1"
		var v110, v113, v114, v115, v116 = "1.1.0", "1.1.3", "1.1.4", "1.1.5", "1.1.6"
		atpRpt := &s2hv1beta1.ActivePromotionStatus{}
		desiredComps := map[string]map[string]s2hv1beta1.DesiredImageTime{
			comp1: {
				stringutils.ConcatImageString(repoComp1, v110): s2hv1beta1.DesiredImageTime{
					Image:       &s2hv1beta1.Image{Repository: repoComp1, Tag: v110},
					CreatedTime: metav1.Time{Time: time.Date(2019, 10, 1, 2, 0, 0, 0, time.UTC)},
				},
				stringutils.ConcatImageString(repoComp1, v113): s2hv1beta1.DesiredImageTime{
					Image:       &s2hv1beta1.Image{Repository: repoComp1, Tag: v113},
					CreatedTime: metav1.Time{Time: time.Date(2019, 10, 3, 2, 0, 0, 0, time.UTC)},
				},
				stringutils.ConcatImageString(repoComp1, v114): s2hv1beta1.DesiredImageTime{
					Image:       &s2hv1beta1.Image{Repository: repoComp1, Tag: v114},
					CreatedTime: metav1.Time{Time: time.Date(2019, 10, 5, 2, 0, 0, 0, time.UTC)},
				},
				stringutils.ConcatImageString(repoComp1, v115): s2hv1beta1.DesiredImageTime{
					Image:       &s2hv1beta1.Image{Repository: repoComp1, Tag: v115},
					CreatedTime: metav1.Time{Time: time.Date(2019, 10, 5, 15, 0, 0, 0, time.UTC)},
				},
				stringutils.ConcatImageString(repoComp1, v116): s2hv1beta1.DesiredImageTime{
					Image:       &s2hv1beta1.Image{Repository: repoComp1, Tag: v116},
					CreatedTime: metav1.Time{Time: time.Date(2019, 10, 7, 2, 0, 0, 0, time.UTC)},
				},
			},
		}
		stableComps := []s2hv1beta1.StableComponent{
			{
				Spec: s2hv1beta1.StableComponentSpec{
					Name:       comp1,
					Repository: repoComp1,
					Version:    v114,
				},
			},
		}
		nowMockTime := time.Date(2019, 10, 7, 9, 0, 0, 0, time.UTC)
		oMock := newMock(cfg, desiredComps, stableComps, nowMockTime)
		oMock.SetOutdatedDuration(atpRpt)
		g.Expect(atpRpt.HasOutdatedComponent).To(BeTrue(), "should have outdated components")

		outdatedComp1 := atpRpt.OutdatedComponents[comp1]
		g.Expect(outdatedComp1.CurrentImage.Tag).To(Equal(v114))
		g.Expect(outdatedComp1.DesiredImage.Tag).To(Equal(v116))
		g.Expect(outdatedComp1.OutdatedDuration).To(Equal(time.Duration(151260000000000))) // 1d18h1m
	})

	It("should return outdated duration correctly when there are 2 new versions in the same day", func() {
		var comp1, repoComp1 = "comp1", "repo/comp1"
		var v110, v113, v114, v115 = "1.1.0", "1.1.3", "1.1.4", "1.1.5"
		atpRpt := &s2hv1beta1.ActivePromotionStatus{}
		desiredComps := map[string]map[string]s2hv1beta1.DesiredImageTime{
			comp1: {
				stringutils.ConcatImageString(repoComp1, v110): s2hv1beta1.DesiredImageTime{
					Image:       &s2hv1beta1.Image{Repository: repoComp1, Tag: v110},
					CreatedTime: metav1.Time{Time: time.Date(2019, 10, 1, 2, 0, 0, 0, time.UTC)},
				},
				stringutils.ConcatImageString(repoComp1, v113): s2hv1beta1.DesiredImageTime{
					Image:       &s2hv1beta1.Image{Repository: repoComp1, Tag: v113},
					CreatedTime: metav1.Time{Time: time.Date(2019, 10, 3, 2, 0, 0, 0, time.UTC)},
				},
				stringutils.ConcatImageString(repoComp1, v114): s2hv1beta1.DesiredImageTime{
					Image:       &s2hv1beta1.Image{Repository: repoComp1, Tag: v114},
					CreatedTime: metav1.Time{Time: time.Date(2019, 10, 5, 2, 0, 0, 0, time.UTC)},
				},
				stringutils.ConcatImageString(repoComp1, v115): s2hv1beta1.DesiredImageTime{
					Image:       &s2hv1beta1.Image{Repository: repoComp1, Tag: v115},
					CreatedTime: metav1.Time{Time: time.Date(2019, 10, 5, 15, 0, 0, 0, time.UTC)},
				},
			},
		}
		stableComps := []s2hv1beta1.StableComponent{
			{
				Spec: s2hv1beta1.StableComponentSpec{
					Name:       comp1,
					Repository: repoComp1,
					Version:    v114,
				},
			},
		}
		nowMockTime := time.Date(2019, 10, 5, 18, 0, 0, 0, time.UTC)
		oMock := newMock(cfg, desiredComps, stableComps, nowMockTime)
		oMock.SetOutdatedDuration(atpRpt)
		g.Expect(atpRpt.HasOutdatedComponent).To(BeTrue(), "should have outdated components")

		outdatedComp1 := atpRpt.OutdatedComponents[comp1]
		g.Expect(outdatedComp1.CurrentImage.Tag).To(Equal(v114))
		g.Expect(outdatedComp1.DesiredImage.Tag).To(Equal(v115))
		g.Expect(outdatedComp1.OutdatedDuration).To(Equal(time.Duration(10860000000000))) // 0d3h1m
	})

	It("should return outdated duration correctly when rollback but there is another latest desired version", func() {
		var comp1, repoComp1 = "comp1", "repo/comp1"
		var v110, v113, v114 = "1.1.0", "1.1.3", "1.1.4"
		atpRpt := &s2hv1beta1.ActivePromotionStatus{}
		desiredComps := map[string]map[string]s2hv1beta1.DesiredImageTime{
			"comp1": {
				stringutils.ConcatImageString(repoComp1, v110): s2hv1beta1.DesiredImageTime{
					Image:       &s2hv1beta1.Image{Repository: repoComp1, Tag: v110},
					CreatedTime: metav1.Time{Time: time.Date(2019, 10, 1, 2, 0, 0, 0, time.UTC)},
				},
				stringutils.ConcatImageString(repoComp1, v113): s2hv1beta1.DesiredImageTime{
					Image:       &s2hv1beta1.Image{Repository: repoComp1, Tag: v113},
					CreatedTime: metav1.Time{Time: time.Date(2019, 10, 6, 2, 0, 0, 0, time.UTC)},
				},
				stringutils.ConcatImageString(repoComp1, v114): s2hv1beta1.DesiredImageTime{
					Image:       &s2hv1beta1.Image{Repository: repoComp1, Tag: v114},
					CreatedTime: metav1.Time{Time: time.Date(2019, 10, 7, 9, 0, 0, 0, time.UTC)},
				},
			},
		}
		stableComps := []s2hv1beta1.StableComponent{
			{
				Spec: s2hv1beta1.StableComponentSpec{
					Name:       comp1,
					Repository: repoComp1,
					Version:    v113,
				},
			},
		}
		nowMockTime := time.Date(2019, 10, 7, 9, 0, 0, 0, time.UTC)
		oMock := newMock(cfg, desiredComps, stableComps, nowMockTime)
		oMock.SetOutdatedDuration(atpRpt)
		g.Expect(atpRpt.HasOutdatedComponent).To(BeTrue(), "should have outdated components")

		outdatedComp1 := atpRpt.OutdatedComponents[comp1]
		g.Expect(outdatedComp1.CurrentImage.Tag).To(Equal(v113))
		g.Expect(outdatedComp1.DesiredImage.Tag).To(Equal(v114))
		g.Expect(outdatedComp1.OutdatedDuration).To(Equal(time.Duration(60000000000))) // 0d0h1m
	})

	It("should return outdated duration correctly when outdated for 1 version by different repository", func() {
		var comp1, repoComp1, repoComp11, v110 = "comp1", "repo/comp1", "repo/comp11", "1.1.0"
		atpRpt := &s2hv1beta1.ActivePromotionStatus{}
		desiredComps := map[string]map[string]s2hv1beta1.DesiredImageTime{
			comp1: {
				stringutils.ConcatImageString(repoComp1, v110): s2hv1beta1.DesiredImageTime{
					Image:       &s2hv1beta1.Image{Repository: repoComp1, Tag: v110},
					CreatedTime: metav1.Time{Time: time.Date(2019, 10, 1, 2, 0, 0, 0, time.UTC)},
				},
				stringutils.ConcatImageString(repoComp11, v110): s2hv1beta1.DesiredImageTime{
					Image:       &s2hv1beta1.Image{Repository: repoComp11, Tag: v110},
					CreatedTime: metav1.Time{Time: time.Date(2019, 10, 3, 2, 0, 0, 0, time.UTC)},
				},
			},
		}
		stableComps := []s2hv1beta1.StableComponent{
			{
				Spec: s2hv1beta1.StableComponentSpec{
					Name:       comp1,
					Repository: repoComp1,
					Version:    v110,
				},
			},
		}
		nowMockTime := time.Date(2019, 10, 3, 9, 0, 0, 0, time.UTC)
		oMock := newMock(cfg, desiredComps, stableComps, nowMockTime)
		oMock.SetOutdatedDuration(atpRpt)
		g.Expect(atpRpt.HasOutdatedComponent).To(BeTrue(), "should have outdated components")

		outdatedComp1 := atpRpt.OutdatedComponents[comp1]
		g.Expect(outdatedComp1.CurrentImage.Repository).To(Equal(repoComp1))
		g.Expect(outdatedComp1.CurrentImage.Tag).To(Equal(v110))
		g.Expect(outdatedComp1.DesiredImage.Repository).To(Equal(repoComp11))
		g.Expect(outdatedComp1.DesiredImage.Tag).To(Equal(v110))
		g.Expect(outdatedComp1.OutdatedDuration).To(Equal(time.Duration(25260000000000))) // 0d7h1m
	})
})

var _ = Describe("set outdated duration when exceed duration configuration is 24h", func() {
	g := NewGomegaWithT(GinkgoT())
	cfg := &s2hv1beta1.ConfigSpec{
		ActivePromotion: &s2hv1beta1.ConfigActivePromotion{
			OutdatedNotification: &s2hv1beta1.OutdatedNotification{
				ExceedDuration: metav1.Duration{Duration: 24 * time.Hour},
			},
		},
	}

	It("should return no outdated duration correctly when outdated duration does not exceed than config", func() {
		var comp1, repoComp1 = "comp1", "repo/comp1"
		var v110, v113 = "1.1.0", "1.1.3"
		atpRpt := &s2hv1beta1.ActivePromotionStatus{}
		desiredComps := map[string]map[string]s2hv1beta1.DesiredImageTime{
			comp1: {
				stringutils.ConcatImageString(repoComp1, v110): s2hv1beta1.DesiredImageTime{
					Image:       &s2hv1beta1.Image{Repository: repoComp1, Tag: v110},
					CreatedTime: metav1.Time{Time: time.Date(2019, 10, 1, 2, 0, 0, 0, time.UTC)},
				},
				stringutils.ConcatImageString(repoComp1, v113): s2hv1beta1.DesiredImageTime{
					Image:       &s2hv1beta1.Image{Repository: repoComp1, Tag: v113},
					CreatedTime: metav1.Time{Time: time.Date(2019, 10, 3, 2, 0, 0, 0, time.UTC)},
				},
			},
		}
		stableComps := []s2hv1beta1.StableComponent{
			{
				Spec: s2hv1beta1.StableComponentSpec{
					Name:       comp1,
					Repository: repoComp1,
					Version:    v110,
				},
			},
		}
		nowMockTime := time.Date(2019, 10, 3, 9, 0, 0, 0, time.UTC)
		oMock := newMock(cfg, desiredComps, stableComps, nowMockTime)
		oMock.SetOutdatedDuration(atpRpt)
		g.Expect(atpRpt.HasOutdatedComponent).To(BeFalse(), "should not have outdated components")

		outdatedComp1 := atpRpt.OutdatedComponents[comp1]
		g.Expect(outdatedComp1.CurrentImage.Tag).To(Equal(v110))
		g.Expect(outdatedComp1.DesiredImage.Tag).To(Equal(v113))
		g.Expect(outdatedComp1.OutdatedDuration).To(Equal(time.Duration(0))) // 0d0h0m
	})

	It("should return outdated duration correctly when exceed duration config is set", func() {
		var comp1, repoComp1 = "comp1", "repo/comp1"
		var v110, v113 = "1.1.0", "1.1.3"
		var comp2, repoComp2 = "comp2", "repo/comp2"
		var v210, v212 = "2.1.0", "2.1.2"
		atpRpt := &s2hv1beta1.ActivePromotionStatus{}
		desiredComps := map[string]map[string]s2hv1beta1.DesiredImageTime{
			comp1: {
				stringutils.ConcatImageString(repoComp1, v110): s2hv1beta1.DesiredImageTime{
					Image:       &s2hv1beta1.Image{Repository: repoComp1, Tag: v110},
					CreatedTime: metav1.Time{Time: time.Date(2019, 10, 1, 2, 0, 0, 0, time.UTC)},
				},
				stringutils.ConcatImageString(repoComp1, v113): s2hv1beta1.DesiredImageTime{
					Image:       &s2hv1beta1.Image{Repository: repoComp1, Tag: v113},
					CreatedTime: metav1.Time{Time: time.Date(2019, 10, 3, 2, 0, 0, 0, time.UTC)},
				},
			},
			comp2: {
				stringutils.ConcatImageString(repoComp2, v210): s2hv1beta1.DesiredImageTime{
					Image:       &s2hv1beta1.Image{Repository: repoComp2, Tag: v210},
					CreatedTime: metav1.Time{Time: time.Date(2019, 10, 1, 2, 0, 0, 0, time.UTC)},
				},
				stringutils.ConcatImageString(repoComp2, v212): s2hv1beta1.DesiredImageTime{
					Image:       &s2hv1beta1.Image{Repository: repoComp2, Tag: v212},
					CreatedTime: metav1.Time{Time: time.Date(2019, 10, 2, 2, 0, 0, 0, time.UTC)},
				},
			},
		}
		stableComps := []s2hv1beta1.StableComponent{
			{
				Spec: s2hv1beta1.StableComponentSpec{
					Name:       comp1,
					Repository: repoComp1,
					Version:    v110,
				},
			},
			{
				Spec: s2hv1beta1.StableComponentSpec{
					Name:       comp2,
					Repository: repoComp2,
					Version:    v210,
				},
			},
		}
		nowMockTime := time.Date(2019, 10, 3, 9, 0, 0, 0, time.UTC)
		oMock := newMock(cfg, desiredComps, stableComps, nowMockTime)
		oMock.SetOutdatedDuration(atpRpt)
		g.Expect(atpRpt.HasOutdatedComponent).To(BeTrue(), "should have outdated components")

		outdatedComp1 := atpRpt.OutdatedComponents[comp1]
		g.Expect(outdatedComp1.CurrentImage.Tag).To(Equal(v110))
		g.Expect(outdatedComp1.DesiredImage.Tag).To(Equal(v113))
		g.Expect(outdatedComp1.OutdatedDuration).To(Equal(time.Duration(0))) // 0d0h0m

		outdatedComp2 := atpRpt.OutdatedComponents[comp2]
		g.Expect(outdatedComp2.CurrentImage.Tag).To(Equal(v210))
		g.Expect(outdatedComp2.DesiredImage.Tag).To(Equal(v212))
		g.Expect(outdatedComp2.OutdatedDuration).To(Equal(time.Duration(111660000000000))) // 1d7h1m
	})
})

var _ = Describe("calculate outdated duration without weekend duration", func() {
	g := NewGomegaWithT(GinkgoT())
	cfg := &s2hv1beta1.ConfigSpec{
		ActivePromotion: &s2hv1beta1.ConfigActivePromotion{
			OutdatedNotification: &s2hv1beta1.OutdatedNotification{
				ExcludeWeekendCalculation: true,
			},
		},
	}
	desiredComps := make(map[string]map[string]s2hv1beta1.DesiredImageTime)
	var stableComps []s2hv1beta1.StableComponent

	It("should return outdated duration correctly when atp stable desired is on working day", func() {
		nowMockTime := time.Date(2019, 10, 7, 9, 0, 0, 0, time.UTC)
		atpStableDesiredTime := time.Date(2019, 10, 4, 9, 0, 0, 0, time.UTC)
		oMock := newMock(cfg, desiredComps, stableComps, nowMockTime)
		totalOutdatedDuration := oMock.calculateOutdatedDuration(atpStableDesiredTime)
		// overAllOutdatedDuration = 259200000000000 72h0m0s
		// totalWeekendDuration = 172800000000000 48h0m0s
		g.Expect(totalOutdatedDuration).To(Equal(time.Duration(86400000000000))) // 24h0m0s
	})

	It("should return outdated duration correctly when atp stable desired is on Saturday", func() {
		nowMockTime := time.Date(2019, 10, 7, 9, 0, 0, 0, time.UTC)
		atpStableDesiredTime := time.Date(2019, 10, 5, 9, 0, 0, 0, time.UTC)
		oMock := newMock(cfg, desiredComps, stableComps, nowMockTime)
		totalOutdatedDuration := oMock.calculateOutdatedDuration(atpStableDesiredTime)
		// overAllOutdatedDuration = 172800000000000 48h0m0s
		// totalWeekendDuration = 140400000000000 39h0m0s
		g.Expect(totalOutdatedDuration).To(Equal(time.Duration(32400000000000))) // 9h0m0s
	})

	It("should return outdated duration correctly when atp stable desired is on Sunday", func() {
		nowMockTime := time.Date(2019, 10, 7, 9, 0, 0, 0, time.UTC)
		atpStableDesiredTime := time.Date(2019, 10, 6, 9, 0, 0, 0, time.UTC)
		oMock := newMock(cfg, desiredComps, stableComps, nowMockTime)
		totalOutdatedDuration := oMock.calculateOutdatedDuration(atpStableDesiredTime)
		// overAllOutdatedDuration = 86400000000000 24h0m0s
		// totalWeekendDuration = 54000000000000 15h0m0s
		g.Expect(totalOutdatedDuration).To(Equal(time.Duration(32400000000000))) // 9h0m0s
	})

	It("should return outdated duration correctly when atp stable desired and now is on weekend", func() {
		nowMockTime := time.Date(2019, 11, 24, 14, 0, 0, 0, time.UTC)
		atpStableDesiredTime := time.Date(2019, 11, 23, 19, 0, 0, 0, time.UTC)
		oMock := newMock(cfg, desiredComps, stableComps, nowMockTime)
		totalOutdatedDuration := oMock.calculateOutdatedDuration(atpStableDesiredTime)
		// overAllOutdatedDuration = 68400000000000 19h0m0s
		// totalWeekendDuration = 68400000000000 19h0m0s
		g.Expect(totalOutdatedDuration).To(Equal(time.Duration(0)))
	})
})

func newMock(cfg *s2hv1beta1.ConfigSpec, desiredComps map[string]map[string]s2hv1beta1.DesiredImageTime, stableComps []s2hv1beta1.StableComponent, nowMockTime time.Time) *Outdated {
	o := &Outdated{
		cfg:                   cfg,
		desiredCompsImageTime: desiredComps,
		stableComps:           stableComps,
		nowTime:               nowMockTime,
	}

	return o
}
