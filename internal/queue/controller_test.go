package queue

import (
	"testing"

	"github.com/agoda-com/samsahai/internal/apis/env/v1beta1"

	"github.com/onsi/gomega"
)

func TestController_removeSimilar(t *testing.T) {
	g := gomega.NewWithT(t)

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

	g.Expect(len(queueList.Items)).To(gomega.Equal(1))
	g.Expect(len(removing)).To(gomega.Equal(2))
}
