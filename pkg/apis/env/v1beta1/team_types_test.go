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
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"golang.org/x/net/context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/agoda-com/samsahai/internal/util/stringutils"
)

var _ = Describe("Team", func() {
	g := NewGomegaWithT(GinkgoT())
	ctx := context.TODO()
	It("should successfully create/update/get/delete CRD", func() {
		key := types.NamespacedName{
			Name: "foo",
		}
		created := &Team{
			ObjectMeta: metav1.ObjectMeta{
				Name: "foo",
			},
		}

		// Test Create
		fetched := &Team{}
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

var _ = Describe("Update desired components to team", func() {
	g := NewGomegaWithT(GinkgoT())
	var comp1, repoComp1, v100, v110 = "comp1", "repo/comp1", "1.0.0", "1.1.0"
	var d20191001t090000 = metav1.Time{Time: time.Date(2019, 10, 1, 9, 0, 0, 0, time.UTC)}
	var d20191002t090000 = metav1.Time{Time: time.Date(2019, 10, 2, 9, 0, 0, 0, time.UTC)}
	var d20191003t090000 = metav1.Time{Time: time.Date(2019, 10, 3, 9, 0, 0, 0, time.UTC)}
	It("should successfully add new desired component", func() {
		team := &Team{
			Status: TeamStatus{
				DesiredComponentImageCreatedTime: map[string]map[string]DesiredImageTime{},
			},
		}
		desiredImageTime := DesiredImageTime{
			Image: &Image{
				Repository: repoComp1,
				Tag:        v100,
			},
			CreatedTime: d20191001t090000,
		}
		desiredImage := stringutils.ConcatImageString(repoComp1, v100)

		team.Status.UpdateDesiredComponentImageCreatedTime(comp1, desiredImage, desiredImageTime)
		desiredComp1 := team.Status.DesiredComponentImageCreatedTime[comp1]
		g.Expect(len(desiredComp1)).To(Equal(1))
		g.Expect(desiredComp1[desiredImage]).ToNot(BeNil())
		g.Expect(desiredComp1[desiredImage].CreatedTime).To(Equal(d20191001t090000))
	})

	It("should successfully add new desired version", func() {
		team := &Team{
			Status: TeamStatus{
				DesiredComponentImageCreatedTime: map[string]map[string]DesiredImageTime{
					comp1: {
						stringutils.ConcatImageString(repoComp1, v100): DesiredImageTime{
							Image:       &Image{Repository: repoComp1, Tag: v100},
							CreatedTime: d20191001t090000,
						},
					},
				},
			},
		}
		desiredImageTime := DesiredImageTime{
			Image: &Image{
				Repository: repoComp1,
				Tag:        v110,
			},
			CreatedTime: d20191002t090000,
		}
		desiredImage := stringutils.ConcatImageString(repoComp1, v110)

		team.Status.UpdateDesiredComponentImageCreatedTime(comp1, desiredImage, desiredImageTime)
		desiredComp1 := team.Status.DesiredComponentImageCreatedTime[comp1]
		g.Expect(len(desiredComp1)).To(Equal(2))
		g.Expect(desiredComp1[desiredImage]).ToNot(BeNil())
		g.Expect(desiredComp1[desiredImage].CreatedTime).To(Equal(d20191002t090000))
	})

	It("should not update desired created time when desired version is already the latest", func() {
		team := &Team{
			Status: TeamStatus{
				DesiredComponentImageCreatedTime: map[string]map[string]DesiredImageTime{
					comp1: {
						stringutils.ConcatImageString(repoComp1, v100): DesiredImageTime{
							Image:       &Image{Repository: repoComp1, Tag: v100},
							CreatedTime: d20191002t090000,
						},
						stringutils.ConcatImageString(repoComp1, v110): DesiredImageTime{
							Image:       &Image{Repository: repoComp1, Tag: v110},
							CreatedTime: d20191001t090000,
						},
					},
				},
			},
		}
		desiredImageTime := DesiredImageTime{
			Image: &Image{
				Repository: repoComp1,
				Tag:        v100,
			},
			CreatedTime: d20191003t090000,
		}
		desiredImage := stringutils.ConcatImageString(repoComp1, v100)

		team.Status.UpdateDesiredComponentImageCreatedTime(comp1, desiredImage, desiredImageTime)
		desiredComp1 := team.Status.DesiredComponentImageCreatedTime[comp1]
		g.Expect(len(desiredComp1)).To(Equal(2))
		g.Expect(desiredComp1[desiredImage].CreatedTime).To(Equal(d20191002t090000))
	})

	It("should update new desired time of existing version when desired version is not the latest", func() {
		team := &Team{
			Status: TeamStatus{
				DesiredComponentImageCreatedTime: map[string]map[string]DesiredImageTime{
					comp1: {
						stringutils.ConcatImageString(repoComp1, v100): DesiredImageTime{
							Image:       &Image{Repository: repoComp1, Tag: v100},
							CreatedTime: d20191001t090000,
						},
						stringutils.ConcatImageString(repoComp1, v110): DesiredImageTime{
							Image:       &Image{Repository: repoComp1, Tag: v110},
							CreatedTime: d20191002t090000,
						},
					},
				},
			},
		}
		desiredImageTime := DesiredImageTime{
			Image: &Image{
				Repository: repoComp1,
				Tag:        v100,
			},
			CreatedTime: d20191003t090000,
		}
		desiredImage := stringutils.ConcatImageString(repoComp1, v100)

		team.Status.UpdateDesiredComponentImageCreatedTime(comp1, desiredImage, desiredImageTime)
		desiredComp1 := team.Status.DesiredComponentImageCreatedTime[comp1]
		g.Expect(len(desiredComp1)).To(Equal(2))
		g.Expect(desiredComp1[desiredImage].CreatedTime).To(Equal(d20191003t090000))
	})
})
