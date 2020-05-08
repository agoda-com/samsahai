package queue

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	s2hv1 "github.com/agoda-com/samsahai/api/v1"
	"github.com/agoda-com/samsahai/internal/util/unittest"
)

func TestQueue(t *testing.T) {
	unittest.InitGinkgo(t, "Queue Controller")
}

var _ = Describe("Queue Controller", func() {
	It("Should remove similar Queue", func() {

		g := NewWithT(GinkgoT())

		c := controller{}
		name := "alpine"

		queue := &s2hv1.Queue{
			Spec:   s2hv1.QueueSpec{Name: name, Repository: name, Version: "3.9.4"},
			Status: s2hv1.QueueStatus{},
		}
		queueList := &s2hv1.QueueList{
			Items: []s2hv1.Queue{
				{
					Spec:   s2hv1.QueueSpec{Name: name, Repository: name, Version: "3.9.0"},
					Status: s2hv1.QueueStatus{},
				},
				{
					Spec:   s2hv1.QueueSpec{Name: name, Repository: name, Version: "3.9.1"},
					Status: s2hv1.QueueStatus{},
				},
				{
					Spec:   s2hv1.QueueSpec{Name: "ubuntu", Repository: "ubuntu", Version: "18.04"},
					Status: s2hv1.QueueStatus{},
				},
			},
		}

		removing := c.removeSimilar(queue, queueList)

		g.Expect(len(queueList.Items)).To(Equal(1))
		g.Expect(len(removing)).To(Equal(2))
	})

	Describe("Reset Queue order", func() {
		It("Should reset order of all Queues correctly", func() {
			g := NewWithT(GinkgoT())

			c := controller{}

			queue := &s2hv1.Queue{
				ObjectMeta: metav1.ObjectMeta{Name: "comp1"},
				Spec:       s2hv1.QueueSpec{NoOfOrder: 4},
			}
			queueList := &s2hv1.QueueList{
				Items: []s2hv1.Queue{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "comp1"},
						Spec:       s2hv1.QueueSpec{NoOfOrder: 4},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: "comp2"},
						Spec:       s2hv1.QueueSpec{NoOfOrder: -1},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: "comp3"},
						Spec:       s2hv1.QueueSpec{NoOfOrder: 10},
					},
				},
			}

			c.resetQueueOrderWithCurrentQueue(queueList, queue)

			g.Expect(len(queueList.Items)).To(Equal(3))
			g.Expect(queueList.Items).To(ContainElement(
				s2hv1.Queue{
					ObjectMeta: metav1.ObjectMeta{Name: "comp1"},
					Spec:       s2hv1.QueueSpec{NoOfOrder: 1},
				},
			))
			g.Expect(queueList.Items).To(ContainElement(
				s2hv1.Queue{
					ObjectMeta: metav1.ObjectMeta{Name: "comp2"},
					Spec:       s2hv1.QueueSpec{NoOfOrder: 2},
				},
			))
			g.Expect(queueList.Items).To(ContainElement(
				s2hv1.Queue{
					ObjectMeta: metav1.ObjectMeta{Name: "comp3"},
					Spec:       s2hv1.QueueSpec{NoOfOrder: 3},
				},
			))
		})
	})
})
