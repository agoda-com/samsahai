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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Desired Component", func() {
	It("should successfully create/get/update/delete CRD", func(done Done) {
		key := types.NamespacedName{
			Name:      "foo",
			Namespace: "default",
		}
		now := metav1.Now()
		created := &DesiredComponent{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "default",
			},
			Status: DesiredComponentStatus{
				CreatedAt: &now,
				UpdatedAt: &now,
			},
			Spec: DesiredComponentSpec{
				Name:       "alpine",
				Version:    "3.9.3",
				Repository: "alpine",
			},
		}

		// Test Create
		fetched := &DesiredComponent{}
		Expect(c.Create(context.TODO(), created)).NotTo(HaveOccurred())

		Expect(c.Get(context.TODO(), key, fetched)).NotTo(HaveOccurred())
		Expect(fetched).To(Equal(created))

		// Test Updating the Labels
		updated := fetched.DeepCopy()
		updated.Labels = map[string]string{"hello": "world"}
		Expect(c.Update(context.TODO(), updated)).NotTo(HaveOccurred())

		Expect(c.Get(context.TODO(), key, fetched)).NotTo(HaveOccurred())
		Expect(fetched).To(Equal(updated))

		Expect(fetched.IsSame(updated)).To(BeTrue())

		Expect(fetched.IsSame(&DesiredComponent{
			Spec: DesiredComponentSpec{
				Name:       "alpine",
				Version:    "3.9.3",
				Repository: "alpine",
			},
		})).To(BeTrue())

		// Test Delete
		Expect(c.Delete(context.TODO(), fetched)).NotTo(HaveOccurred())
		Expect(c.Get(context.TODO(), key, fetched)).To(HaveOccurred())

		close(done)
	}, 5)
})
