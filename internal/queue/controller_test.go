package queue

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/agoda-com/samsahai/api/v1beta1"
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

		queue := &v1beta1.Queue{
			Spec:   v1beta1.QueueSpec{Name: name, Repository: name, Version: "3.9.4"},
			Status: v1beta1.QueueStatus{},
		}
		queueList := &v1beta1.QueueList{
			Items: []v1beta1.Queue{
				{
					Spec:   v1beta1.QueueSpec{Name: name, Repository: name, Version: "3.9.0"},
					Status: v1beta1.QueueStatus{},
				},
				{
					Spec:   v1beta1.QueueSpec{Name: name, Repository: name, Version: "3.9.1"},
					Status: v1beta1.QueueStatus{},
				},
				{
					Spec:   v1beta1.QueueSpec{Name: "ubuntu", Repository: "ubuntu", Version: "18.04"},
					Status: v1beta1.QueueStatus{},
				},
			},
		}

		removing := c.removeSimilar(queue, queueList)

		g.Expect(len(queueList.Items)).To(Equal(1))
		g.Expect(len(removing)).To(Equal(2))
	})
})
