/*
Copyright 2019 Agoda DevOps Container.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1beta1

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Queue", func() {
	It("should successfully create/get/update delete CRD", func(done Done) {
		defer close(done)

		key := types.NamespacedName{
			Name:      "foo",
			Namespace: "default",
		}
		created := &Queue{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "default",
			}}

		// Test Create
		fetched := &Queue{}
		Expect(c.Create(context.TODO(), created)).NotTo(HaveOccurred())

		Expect(c.Get(context.TODO(), key, fetched)).NotTo(HaveOccurred())
		Expect(fetched).To(Equal(created))

		// Test Updating the Labels
		updated := fetched.DeepCopy()
		updated.Labels = map[string]string{"hello": "world"}
		Expect(c.Update(context.TODO(), updated)).NotTo(HaveOccurred())

		Expect(c.Get(context.TODO(), key, fetched)).NotTo(HaveOccurred())
		Expect(fetched).To(Equal(updated))
		Expect(updated.IsSame(fetched)).To(BeTrue())

		// List
		list := &QueueList{}
		Expect(c.List(context.TODO(), &client.ListOptions{}, list)).NotTo(HaveOccurred())
		Expect(len(list.Items)).To(Equal(1))

		// Test Delete
		Expect(c.Delete(context.TODO(), fetched)).NotTo(HaveOccurred())
		Expect(c.Get(context.TODO(), key, fetched)).To(HaveOccurred())

	}, 5)

	It("should successfully get first of queue in list", func(done Done) {
		defer close(done)

		list := QueueList{}
		list.Items = []Queue{
			{Spec: QueueSpec{Name: "ubuntu", Version: "16.04", Repository: "ubuntu", NoOfOrder: 1}},
			{Spec: QueueSpec{Name: "alpine", Version: "3.9.0", Repository: "alpine", NoOfOrder: 0}},
			{Spec: QueueSpec{Name: "redis", Version: "5", Repository: "redis", NoOfOrder: 3}},
		}

		By("Get first in QueueList")
		q := list.First()
		Expect(q.Spec).To(Equal(QueueSpec{Name: "alpine", Version: "3.9.0", Repository: "alpine", NoOfOrder: 0}))

		By("QueueList should be sorted")
		Expect(list.TopQueueOrder()).To(Equal(-1))
		Expect(list.LastQueueOrder()).To(Equal(4))

	}, 1)

	Specify("an empty QueueList", func() {
		list := QueueList{}
		Expect(list.LastQueueOrder()).To(Equal(1), "should always return 1 with empty queue")
		Expect(list.TopQueueOrder()).To(Equal(1), "should always return 1 with empty queue")
		Expect(list.First()).To(BeNil(), "should return nil when empty queue")
	})
})
