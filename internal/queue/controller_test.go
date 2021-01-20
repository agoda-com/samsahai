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
	Describe("Remove/Update similar Queue", func() {
		It("should remove similar component Queue", func() {

			g := NewWithT(GinkgoT())

			c := controller{}
			name := "alpine"

			queue := &s2hv1.Queue{
				Spec: s2hv1.QueueSpec{Name: name, Bundle: name,
					Components: s2hv1.QueueComponents{{Name: name, Repository: name, Version: "3.9.4"}},
				},
				Status: s2hv1.QueueStatus{},
			}
			queueList := &s2hv1.QueueList{
				Items: []s2hv1.Queue{
					{
						Spec: s2hv1.QueueSpec{Name: name,
							Components: s2hv1.QueueComponents{{Name: name, Repository: name, Version: "3.9.0"}},
						},
						Status: s2hv1.QueueStatus{},
					},
					{
						Spec: s2hv1.QueueSpec{Name: name,
							Components: s2hv1.QueueComponents{{Name: name, Repository: name, Version: "3.9.1"}},
						},
						Status: s2hv1.QueueStatus{},
					},
					{
						Spec: s2hv1.QueueSpec{Name: "group", Bundle: "group",
							Components: s2hv1.QueueComponents{
								{Name: name, Repository: name, Version: "3.9.1"},
								{Name: "ubuntu", Repository: "ubuntu", Version: "18.04"},
							},
						},
						Status: s2hv1.QueueStatus{},
					},
					{
						Spec: s2hv1.QueueSpec{Name: "ubuntu",
							Components: s2hv1.QueueComponents{{Name: "ubuntu", Repository: "ubuntu", Version: "18.04"}},
						},
						Status: s2hv1.QueueStatus{},
					},
				},
			}

			removing, updating := c.removeAndUpdateSimilarQueue(queue, queueList)

			g.Expect(len(queueList.Items)).To(Equal(2))
			g.Expect(len(removing)).To(Equal(2))

			newUpdating := getNonEmptyQueue(updating)
			g.Expect(len(newUpdating)).To(Equal(1))
			g.Expect(len(newUpdating[0].Spec.Components)).To(Equal(1))
		})

		It("should skip same component version", func() {

			g := NewWithT(GinkgoT())

			c := controller{}
			name := "alpine"

			queue := &s2hv1.Queue{
				Spec: s2hv1.QueueSpec{Name: name,
					Components: s2hv1.QueueComponents{{Name: name, Repository: name, Version: "3.9.4"}},
				},
				Status: s2hv1.QueueStatus{},
			}
			queueList := &s2hv1.QueueList{
				Items: []s2hv1.Queue{
					{
						Spec: s2hv1.QueueSpec{Name: name,
							Components: s2hv1.QueueComponents{{Name: name, Repository: name, Version: "3.9.4"}},
						},
						Status: s2hv1.QueueStatus{},
					},
				},
			}

			removing, updating := c.removeAndUpdateSimilarQueue(queue, queueList)

			g.Expect(len(queueList.Items)).To(Equal(1))
			g.Expect(len(removing)).To(Equal(0))

			newUpdating := getNonEmptyQueue(updating)
			g.Expect(len(newUpdating)).To(Equal(0))
		})
	})

	Describe("Add component into existing bundle Queue", func() {
		It("should add component into existing bundle Queue", func() {

			g := NewWithT(GinkgoT())

			c := controller{}
			alpine := "alpine"
			ubuntu := "ubuntu"

			queue := &s2hv1.Queue{
				Spec: s2hv1.QueueSpec{Name: "group", Bundle: "group",
					Components: s2hv1.QueueComponents{{Name: ubuntu, Repository: ubuntu, Version: "3.9.4"}},
				},
				Status: s2hv1.QueueStatus{},
			}
			queueList := &s2hv1.QueueList{
				Items: []s2hv1.Queue{
					{
						Spec: s2hv1.QueueSpec{Name: alpine,
							Components: s2hv1.QueueComponents{{Name: alpine, Repository: alpine, Version: "3.9.0"}},
						},
						Status: s2hv1.QueueStatus{},
					},
					{
						Spec: s2hv1.QueueSpec{Name: alpine, Bundle: "group",
							Components: s2hv1.QueueComponents{{Name: alpine, Repository: alpine, Version: "3.9.1"}},
						},
						Status: s2hv1.QueueStatus{},
					},
				},
			}

			updating := c.addExistingBundleQueue(queue, queueList)

			g.Expect(len(queueList.Items)).To(Equal(2))

			newUpdating := getNonEmptyQueue(updating)
			g.Expect(len(newUpdating)).To(Equal(1))
			g.Expect(len(newUpdating[0].Spec.Components)).To(Equal(2))
		})

		It("should skip same component version", func() {

			g := NewWithT(GinkgoT())

			c := controller{}
			name := "alpine"

			queue := &s2hv1.Queue{
				Spec: s2hv1.QueueSpec{Name: name,
					Components: s2hv1.QueueComponents{{Name: name, Repository: name, Version: "3.9.4"}},
				},
				Status: s2hv1.QueueStatus{},
			}
			queueList := &s2hv1.QueueList{
				Items: []s2hv1.Queue{
					{
						Spec: s2hv1.QueueSpec{Name: name,
							Components: s2hv1.QueueComponents{{Name: name, Repository: name, Version: "3.9.4"}},
						},
						Status: s2hv1.QueueStatus{},
					},
				},
			}

			updating := c.addExistingBundleQueue(queue, queueList)

			g.Expect(len(queueList.Items)).To(Equal(1))

			newUpdating := getNonEmptyQueue(updating)
			g.Expect(len(newUpdating)).To(Equal(0))
		})
	})

	Describe("Set queue order following priority queues", func() {
		It("should set order of the highest component queue following priority queues", func() {

			g := NewWithT(GinkgoT())

			c := controller{}
			alpine := "alpine"
			ubuntu := "ubuntu"

			priorityQueues := []string{"ubuntu", "alpine"}

			queue := &s2hv1.Queue{
				Spec: s2hv1.QueueSpec{Name: ubuntu,
					Components: s2hv1.QueueComponents{{Name: ubuntu, Repository: ubuntu, Version: "3.9.4"}},
				},
				Status: s2hv1.QueueStatus{},
			}
			queueList := &s2hv1.QueueList{
				Items: []s2hv1.Queue{
					{
						Spec: s2hv1.QueueSpec{
							Name:       alpine,
							NoOfOrder:  1,
							Components: s2hv1.QueueComponents{{Name: alpine, Repository: alpine, Version: "3.9.0"}},
						},
						Status: s2hv1.QueueStatus{},
					},
				},
			}

			updating := c.setQueueOrderFollowingPriorityQueues(queue, queueList, priorityQueues)

			g.Expect(len(queueList.Items)).To(Equal(1))
			g.Expect(queue.Spec.NoOfOrder).To(Equal(0))

			newUpdating := getNonEmptyQueue(updating)
			g.Expect(len(newUpdating)).To(Equal(1))
			g.Expect(newUpdating[0].Spec.Name).To(Equal(alpine))
			g.Expect(newUpdating[0].Spec.NoOfOrder).To(Equal(1))

		})

		It("should set order of lower component queue following priority queues", func() {

			g := NewWithT(GinkgoT())

			c := controller{}
			alpine := "alpine"
			ubuntu := "ubuntu"

			priorityQueues := []string{"alpine", "ubuntu"}

			queue := &s2hv1.Queue{
				Spec: s2hv1.QueueSpec{Name: ubuntu,
					Components: s2hv1.QueueComponents{{Name: ubuntu, Repository: ubuntu, Version: "3.9.4"}},
				},
				Status: s2hv1.QueueStatus{},
			}
			queueList := &s2hv1.QueueList{
				Items: []s2hv1.Queue{
					{
						Spec: s2hv1.QueueSpec{
							Name:       alpine,
							NoOfOrder:  1,
							Components: s2hv1.QueueComponents{{Name: alpine, Repository: alpine, Version: "3.9.0"}},
						},
						Status: s2hv1.QueueStatus{},
					},
				},
			}

			updating := c.setQueueOrderFollowingPriorityQueues(queue, queueList, priorityQueues)

			g.Expect(len(queueList.Items)).To(Equal(1))
			g.Expect(queue.Spec.NoOfOrder).To(Equal(2))

			newUpdating := getNonEmptyQueue(updating)
			g.Expect(len(newUpdating)).To(Equal(0))

		})

		It("should set order of the highest bundle queue following priority queues", func() {

			g := NewWithT(GinkgoT())

			c := controller{}
			alpine := "alpine"
			ubuntu := "ubuntu"
			stretch := "stretch"

			priorityQueues := []string{"group", "alpine"}

			queue := &s2hv1.Queue{
				Spec: s2hv1.QueueSpec{Name: "group", Bundle: "group",
					Components: s2hv1.QueueComponents{{Name: ubuntu, Repository: ubuntu, Version: "3.9.4"}},
				},
				Status: s2hv1.QueueStatus{},
			}
			queueList := &s2hv1.QueueList{
				Items: []s2hv1.Queue{
					{
						Spec: s2hv1.QueueSpec{
							Name:       alpine,
							NoOfOrder:  1,
							Components: s2hv1.QueueComponents{{Name: alpine, Repository: alpine, Version: "3.9.0"}},
						},
						Status: s2hv1.QueueStatus{},
					},
					{
						Spec: s2hv1.QueueSpec{
							Name:      "group",
							Bundle:    "group",
							NoOfOrder: 2,
							Components: s2hv1.QueueComponents{
								{Name: stretch, Repository: stretch, Version: "3.9.1"},
								{Name: ubuntu, Repository: ubuntu, Version: "3.9.2"},
							},
						},
						Status: s2hv1.QueueStatus{},
					},
				},
			}

			updating := c.setQueueOrderFollowingPriorityQueues(queue, queueList, priorityQueues)

			g.Expect(len(queueList.Items)).To(Equal(2))

			newUpdating := getNonEmptyQueue(updating)
			g.Expect(len(newUpdating)).To(Equal(2))
			g.Expect(newUpdating[0].Spec.Name).To(Equal(alpine))
			g.Expect(newUpdating[0].Spec.NoOfOrder).To(Equal(1))
			g.Expect(newUpdating[1].Spec.Name).To(Equal("group"))
			g.Expect(newUpdating[1].Spec.NoOfOrder).To(Equal(0))

		})

	})

	Describe("Reset Queue order", func() {
		It("should reset order of all Queues correctly", func() {
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

func getNonEmptyQueue(queues []s2hv1.Queue) []s2hv1.Queue {
	out := make([]s2hv1.Queue, 0)
	for _, q := range queues {
		if q.Spec.Name != "" {
			out = append(out, q)
		}
	}

	return out
}
