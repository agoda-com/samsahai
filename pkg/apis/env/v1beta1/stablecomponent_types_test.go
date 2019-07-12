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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"golang.org/x/net/context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Stable Component", func() {
	g := NewGomegaWithT(GinkgoT())
	ctx := context.TODO()
	It("should successfully create/update/get/delete CRD", func() {
		key := types.NamespacedName{
			Name:      "foo",
			Namespace: "default",
		}
		created := &StableComponent{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "default",
			},
		}

		// Test Create
		fetched := &StableComponent{}
		g.Expect(c.Create(ctx, created)).NotTo(HaveOccurred())
		g.Expect(c.Get(ctx, key, fetched)).NotTo(HaveOccurred())
		g.Expect(fetched).To(Equal(created))

		// Test Updating the Labels
		updated := fetched.DeepCopy()
		updated.Labels = map[string]string{"hello": "world"}
		g.Expect(c.Update(ctx, updated)).NotTo(HaveOccurred())
		g.Expect(c.Get(ctx, key, fetched)).NotTo(HaveOccurred())
		g.Expect(fetched).To(Equal(updated))

		// Test Delete
		g.Expect(c.Delete(ctx, fetched)).NotTo(HaveOccurred())
		g.Expect(c.Get(ctx, key, fetched)).To(HaveOccurred())
	})
})
