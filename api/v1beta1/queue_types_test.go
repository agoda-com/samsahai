package v1beta1_test

import (
	"os"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	s2hv1beta1 "github.com/agoda-com/samsahai/api/v1beta1"
	s2hlog "github.com/agoda-com/samsahai/internal/log"
	"github.com/agoda-com/samsahai/internal/util/unittest"
)

func TestQueueList_Sort(t *testing.T) {
	unittest.InitGinkgo(t, "Queue List Sort")
}

var _ = BeforeSuite(func() {
	if os.Getenv("DEBUG") != "" {
		s2hlog.SetLogger(zap.New(func(o *zap.Options) {
			o.Development = true
		}))
	}
})

var _ = Describe("Sort by no of order", func() {
	g := NewWithT(GinkgoT())

	beforeNow := metav1.Time{Time: metav1.Now().Add(-10 * time.Minute)}
	afterNow := metav1.Time{Time: metav1.Now().Add(10 * time.Minute)}

	It("should sort queue list by no of order correctly", func() {
		queueList := s2hv1beta1.QueueList{
			Items: []s2hv1beta1.Queue{
				{Spec: s2hv1beta1.QueueSpec{Name: "comp1", NoOfOrder: 3}},
				{Spec: s2hv1beta1.QueueSpec{Name: "comp2", NoOfOrder: 1}},
				{Spec: s2hv1beta1.QueueSpec{Name: "comp3", NoOfOrder: 2}},
			},
		}

		expectedQueueList := s2hv1beta1.QueueList{
			Items: []s2hv1beta1.Queue{
				{Spec: s2hv1beta1.QueueSpec{Name: "comp2", NoOfOrder: 1}},
				{Spec: s2hv1beta1.QueueSpec{Name: "comp3", NoOfOrder: 2}},
				{Spec: s2hv1beta1.QueueSpec{Name: "comp1", NoOfOrder: 3}},
			},
		}

		queueList.Sort()
		g.Expect(queueList.Items).To(BeEquivalentTo(expectedQueueList.Items))
	})

	It("should sort queue list correctly in case of orders are the same", func() {
		queueList := s2hv1beta1.QueueList{
			Items: []s2hv1beta1.Queue{
				{Spec: s2hv1beta1.QueueSpec{Name: "comp1", NoOfOrder: 2}},
				{Spec: s2hv1beta1.QueueSpec{Name: "comp2", NoOfOrder: 1, NextProcessAt: &beforeNow}},
				{Spec: s2hv1beta1.QueueSpec{Name: "comp3", NoOfOrder: 1,
					NextProcessAt: &metav1.Time{Time: beforeNow.Add(-10 * time.Minute)}}},
			},
		}

		expectedQueueList := s2hv1beta1.QueueList{
			Items: []s2hv1beta1.Queue{
				{Spec: s2hv1beta1.QueueSpec{Name: "comp3", NoOfOrder: 1,
					NextProcessAt: &metav1.Time{Time: beforeNow.Add(-10 * time.Minute)}}},
				{Spec: s2hv1beta1.QueueSpec{Name: "comp2", NoOfOrder: 1, NextProcessAt: &beforeNow}},
				{Spec: s2hv1beta1.QueueSpec{Name: "comp1", NoOfOrder: 2}},
			},
		}

		queueList.Sort()
		g.Expect(queueList.Items).To(BeEquivalentTo(expectedQueueList.Items))
	})

	It("should sort queue list correctly in case of finishing reverify process, "+
		"next process at is after now", func() {
		queueList := s2hv1beta1.QueueList{
			Items: []s2hv1beta1.Queue{
				{Spec: s2hv1beta1.QueueSpec{Name: "comp1", NoOfOrder: 2}},
				{Spec: s2hv1beta1.QueueSpec{Name: "comp2", NoOfOrder: 1,
					NextProcessAt: &metav1.Time{Time: afterNow.Add(10 * time.Minute)}}}, // finished reverify process
				{Spec: s2hv1beta1.QueueSpec{Name: "comp3", NoOfOrder: 4, NextProcessAt: &afterNow}}, // finished reverify process
				{Spec: s2hv1beta1.QueueSpec{Name: "comp4", NoOfOrder: 3, NextProcessAt: &beforeNow}},
			},
		}

		expectedQueueList := s2hv1beta1.QueueList{
			Items: []s2hv1beta1.Queue{
				{Spec: s2hv1beta1.QueueSpec{Name: "comp1", NoOfOrder: 2}},
				{Spec: s2hv1beta1.QueueSpec{Name: "comp4", NoOfOrder: 3, NextProcessAt: &beforeNow}},
				{Spec: s2hv1beta1.QueueSpec{Name: "comp3", NoOfOrder: 4, NextProcessAt: &afterNow}}, // finished reverify process
				{Spec: s2hv1beta1.QueueSpec{Name: "comp2", NoOfOrder: 1,
					NextProcessAt: &metav1.Time{Time: afterNow.Add(10 * time.Minute)}}}, // finished reverify process
			},
		}

		queueList.Sort()
		g.Expect(queueList.Items).To(BeEquivalentTo(expectedQueueList.Items))
	})
})
