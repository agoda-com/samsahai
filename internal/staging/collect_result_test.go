package staging

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	s2hv1beta1 "github.com/agoda-com/samsahai/api/v1beta1"
)

var _ = Describe("Set deployment issues in Queue", func() {
	g := NewWithT(GinkgoT())
	stagingCtrl := controller{}

	compName := "comp-1"
	nodeName := "node-1"

	Describe("Extract deployment issues", func() {
		It("should correctly get `WaitForInitContainer` deployment issue from k8s resources", func() {
			pods := corev1.PodList{Items: []corev1.Pod{{
				ObjectMeta: metav1.ObjectMeta{Name: compName},
				Spec:       corev1.PodSpec{NodeName: nodeName},
				Status: corev1.PodStatus{
					InitContainerStatuses: []corev1.ContainerStatus{{
						Name:         "wait-for-dep1",
						Ready:        false,
						RestartCount: 1,
					}},
					ContainerStatuses: []corev1.ContainerStatus{{
						Name:  compName,
						Ready: false,
					}},
				},
			}}}

			issuesMaps := make(map[s2hv1beta1.DeploymentIssueType][]s2hv1beta1.FailureComponent)
			stagingCtrl.extractDeploymentIssues(&pods, &batchv1.JobList{Items: []batchv1.Job{}}, issuesMaps)

			g.Expect(issuesMaps).To(HaveLen(1))

			failureComps := issuesMaps[s2hv1beta1.DeploymentIssueWaitForInitContainer]
			g.Expect(failureComps).To(HaveLen(1))
			g.Expect(failureComps[0].ComponentName).To(Equal(compName))
			g.Expect(failureComps[0].FirstFailureContainerName).To(Equal("wait-for-dep1"))
			g.Expect(failureComps[0].RestartCount).To(Equal(int32(1)))
			g.Expect(failureComps[0].NodeName).To(Equal(nodeName))
		})

		It("should correctly get `ImagePullBackOff` deployment issue from k8s resources", func() {
			pods := corev1.PodList{Items: []corev1.Pod{{
				ObjectMeta: metav1.ObjectMeta{Name: compName},
				Spec:       corev1.PodSpec{NodeName: nodeName},
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{{
						Name:  compName,
						Ready: false,
						State: corev1.ContainerState{
							Waiting: &corev1.ContainerStateWaiting{
								Reason: "ErrImagePull",
							},
						},
					}},
				},
			}}}

			issuesMaps := make(map[s2hv1beta1.DeploymentIssueType][]s2hv1beta1.FailureComponent)
			stagingCtrl.extractDeploymentIssues(&pods, &batchv1.JobList{Items: []batchv1.Job{}}, issuesMaps)

			g.Expect(issuesMaps).To(HaveLen(1))

			failureComps := issuesMaps[s2hv1beta1.DeploymentIssueImagePullBackOff]
			g.Expect(failureComps).To(HaveLen(1))
			g.Expect(failureComps[0].ComponentName).To(Equal(compName))
			g.Expect(failureComps[0].FirstFailureContainerName).To(Equal(compName))
			g.Expect(failureComps[0].RestartCount).To(BeZero())
			g.Expect(failureComps[0].NodeName).To(Equal(nodeName))
		})

		It("should correctly get `CrashLoopBackOff` deployment issue from k8s resources", func() {
			pods := corev1.PodList{Items: []corev1.Pod{{
				ObjectMeta: metav1.ObjectMeta{Name: compName},
				Spec:       corev1.PodSpec{NodeName: nodeName},
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{{
						Name:  compName,
						Ready: false,
						State: corev1.ContainerState{
							Waiting: &corev1.ContainerStateWaiting{
								Reason: "Error",
							},
						},
						RestartCount: 1,
					}},
				},
			}}}

			issuesMaps := make(map[s2hv1beta1.DeploymentIssueType][]s2hv1beta1.FailureComponent)
			stagingCtrl.extractDeploymentIssues(&pods, &batchv1.JobList{Items: []batchv1.Job{}}, issuesMaps)

			g.Expect(issuesMaps).To(HaveLen(1))

			failureComps := issuesMaps[s2hv1beta1.DeploymentIssueCrashLoopBackOff]
			g.Expect(failureComps).To(HaveLen(1))
			g.Expect(failureComps[0].ComponentName).To(Equal(compName))
			g.Expect(failureComps[0].FirstFailureContainerName).To(Equal(compName))
			g.Expect(failureComps[0].RestartCount).To(Equal(int32(1)))
			g.Expect(failureComps[0].NodeName).To(Equal(nodeName))
		})

		It("should correctly get `CrashLoopBackOff` deployment issue from k8s resources (Running 0/1)", func() {
			pods := corev1.PodList{Items: []corev1.Pod{{
				ObjectMeta: metav1.ObjectMeta{Name: compName},
				Spec:       corev1.PodSpec{NodeName: nodeName},
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{{
						Name:  compName,
						Ready: false,
						State: corev1.ContainerState{
							Running: &corev1.ContainerStateRunning{
								StartedAt: metav1.Now(),
							},
						},
						RestartCount: 3,
					}},
				},
			}}}

			issuesMaps := make(map[s2hv1beta1.DeploymentIssueType][]s2hv1beta1.FailureComponent)
			stagingCtrl.extractDeploymentIssues(&pods, &batchv1.JobList{Items: []batchv1.Job{}}, issuesMaps)

			g.Expect(issuesMaps).To(HaveLen(1))

			failureComps := issuesMaps[s2hv1beta1.DeploymentIssueCrashLoopBackOff]
			g.Expect(failureComps).To(HaveLen(1))
			g.Expect(failureComps[0].ComponentName).To(Equal(compName))
			g.Expect(failureComps[0].FirstFailureContainerName).To(Equal(compName))
			g.Expect(failureComps[0].RestartCount).To(Equal(int32(3)))
			g.Expect(failureComps[0].NodeName).To(Equal(nodeName))
		})

		It("should correctly get `ReadinessProbeFailed` deployment issue from k8s resources", func() {
			pods := corev1.PodList{Items: []corev1.Pod{{
				ObjectMeta: metav1.ObjectMeta{Name: compName},
				Spec:       corev1.PodSpec{NodeName: nodeName},
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{{
						Name:  compName,
						Ready: false,
						State: corev1.ContainerState{
							Running: &corev1.ContainerStateRunning{
								StartedAt: metav1.Now(),
							},
						},
						RestartCount: 0,
					}},
				},
			}}}

			issuesMaps := make(map[s2hv1beta1.DeploymentIssueType][]s2hv1beta1.FailureComponent)
			stagingCtrl.extractDeploymentIssues(&pods, &batchv1.JobList{Items: []batchv1.Job{}}, issuesMaps)

			g.Expect(issuesMaps).To(HaveLen(1))

			failureComps := issuesMaps[s2hv1beta1.DeploymentIssueReadinessProbeFailed]
			g.Expect(failureComps).To(HaveLen(1))
			g.Expect(failureComps[0].ComponentName).To(Equal(compName))
			g.Expect(failureComps[0].FirstFailureContainerName).To(Equal(compName))
			g.Expect(failureComps[0].RestartCount).To(Equal(int32(0)))
			g.Expect(failureComps[0].NodeName).To(Equal(nodeName))
		})

		It("should correctly get `ContainerCreating` deployment issue from k8s resources", func() {
			pods := corev1.PodList{Items: []corev1.Pod{{
				ObjectMeta: metav1.ObjectMeta{Name: compName},
				Spec:       corev1.PodSpec{NodeName: nodeName},
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{{
						Name:  compName,
						Ready: false,
						State: corev1.ContainerState{
							Waiting: &corev1.ContainerStateWaiting{
								Reason: "ContainerCreating",
							},
						},
					}},
				},
			}}}

			issuesMaps := make(map[s2hv1beta1.DeploymentIssueType][]s2hv1beta1.FailureComponent)
			stagingCtrl.extractDeploymentIssues(&pods, &batchv1.JobList{Items: []batchv1.Job{}}, issuesMaps)

			g.Expect(issuesMaps).To(HaveLen(1))

			failureComps := issuesMaps[s2hv1beta1.DeploymentIssueContainerCreating]
			g.Expect(failureComps).To(HaveLen(1))
			g.Expect(failureComps[0].ComponentName).To(Equal(compName))
			g.Expect(failureComps[0].FirstFailureContainerName).To(Equal(compName))
			g.Expect(failureComps[0].RestartCount).To(BeZero())
			g.Expect(failureComps[0].NodeName).To(Equal(nodeName))
		})

		It("should correctly get `JobNotComplete` deployment issue from k8s resources", func() {
			jobs := batchv1.JobList{Items: []batchv1.Job{{
				ObjectMeta: metav1.ObjectMeta{Name: compName},
				Status: batchv1.JobStatus{
					Active: 1,
					Failed: 4,
				},
			}}}

			issuesMaps := make(map[s2hv1beta1.DeploymentIssueType][]s2hv1beta1.FailureComponent)
			stagingCtrl.extractDeploymentIssues(&corev1.PodList{Items: []corev1.Pod{}}, &jobs, issuesMaps)

			g.Expect(issuesMaps).To(HaveLen(1))

			failureComps := issuesMaps[s2hv1beta1.DeploymentIssueJobNotComplete]
			g.Expect(failureComps).To(HaveLen(1))
			g.Expect(failureComps[0].ComponentName).To(Equal(compName))
			g.Expect(failureComps[0].FirstFailureContainerName).To(BeEmpty())
			g.Expect(failureComps[0].RestartCount).To(Equal(int32(4)))
		})

		It("should correctly get `Pending` deployment issue from k8s resources", func() {
			pods := corev1.PodList{Items: []corev1.Pod{{
				ObjectMeta: metav1.ObjectMeta{Name: compName},
				Spec:       corev1.PodSpec{NodeName: nodeName},
				Status: corev1.PodStatus{
					Phase: corev1.PodPending,
				},
			}}}

			issuesMaps := make(map[s2hv1beta1.DeploymentIssueType][]s2hv1beta1.FailureComponent)
			stagingCtrl.extractDeploymentIssues(&pods, &batchv1.JobList{Items: []batchv1.Job{}}, issuesMaps)

			g.Expect(issuesMaps).To(HaveLen(1))

			failureComps := issuesMaps[s2hv1beta1.DeploymentIssuePending]
			g.Expect(failureComps).To(HaveLen(1))
			g.Expect(failureComps[0].ComponentName).To(Equal(compName))
			g.Expect(failureComps[0].FirstFailureContainerName).To(BeEmpty())
			g.Expect(failureComps[0].RestartCount).To(BeZero())
			g.Expect(failureComps[0].NodeName).To(Equal(nodeName))
		})

		It("should correctly set multiple deployment issues in Queue", func() {
			timeNow := metav1.Now()
			pods := corev1.PodList{Items: []corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "multi-comp1",
						Namespace: "namespace",
					},
					Spec: corev1.PodSpec{NodeName: "node-11"},
					Status: corev1.PodStatus{
						ContainerStatuses: []corev1.ContainerStatus{{
							Name:  "multi-comp1",
							Ready: false,
							State: corev1.ContainerState{
								Waiting: &corev1.ContainerStateWaiting{
									Reason: "ImagePullBackOff",
								},
							},
						}},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "multi-comp2",
						Namespace: "namespace",
					},
					Spec: corev1.PodSpec{NodeName: "node-12"},
					Status: corev1.PodStatus{
						ContainerStatuses: []corev1.ContainerStatus{{
							Name:  "multi-comp2",
							Ready: false,
							State: corev1.ContainerState{
								Waiting: &corev1.ContainerStateWaiting{
									Reason: "CrashLoopBackOff",
								},
							},
							RestartCount: 10,
						}},
					},
				},
			}}

			jobs := batchv1.JobList{Items: []batchv1.Job{{
				ObjectMeta: metav1.ObjectMeta{
					Name: "job-1",
				},
				Status: batchv1.JobStatus{
					CompletionTime: &timeNow,
				},
			}}}

			issuesMaps := make(map[s2hv1beta1.DeploymentIssueType][]s2hv1beta1.FailureComponent)
			stagingCtrl.extractDeploymentIssues(&pods, &jobs, issuesMaps)

			g.Expect(issuesMaps).To(HaveLen(2))

			failureComps1 := issuesMaps[s2hv1beta1.DeploymentIssueImagePullBackOff]
			g.Expect(failureComps1).To(HaveLen(1))
			g.Expect(failureComps1[0].ComponentName).To(Equal("multi-comp1"))
			g.Expect(failureComps1[0].FirstFailureContainerName).To(Equal("multi-comp1"))
			g.Expect(failureComps1[0].RestartCount).To(BeZero())
			g.Expect(failureComps1[0].NodeName).To(Equal("node-11"))

			failureComps2 := issuesMaps[s2hv1beta1.DeploymentIssueCrashLoopBackOff]
			g.Expect(failureComps2).To(HaveLen(1))
			g.Expect(failureComps2[0].ComponentName).To(Equal("multi-comp2"))
			g.Expect(failureComps2[0].FirstFailureContainerName).To(Equal("multi-comp2"))
			g.Expect(failureComps2[0].RestartCount).To(Equal(int32(10)))
			g.Expect(failureComps2[0].NodeName).To(Equal("node-12"))
		})
	})

	Describe("Convert deployment issues maps into list", func() {
		It("should correctly convert deployment issues maps into list", func() {
			issuesMaps := map[s2hv1beta1.DeploymentIssueType][]s2hv1beta1.FailureComponent{
				s2hv1beta1.DeploymentIssueUndefined: {
					{
						ComponentName:             "comp-1",
						FirstFailureContainerName: "comp-1",
						RestartCount:              0,
						NodeName:                  "node-1",
					},
					{
						ComponentName:             "comp-2",
						FirstFailureContainerName: "comp-2",
						RestartCount:              3,
						NodeName:                  "node-2",
					},
				},
				s2hv1beta1.DeploymentIssueWaitForInitContainer: {
					{
						ComponentName:             "comp-3",
						FirstFailureContainerName: "wait-for-dep3",
						RestartCount:              10,
						NodeName:                  "node-3",
					},
				},
			}

			issues := stagingCtrl.convertToDeploymentIssues(issuesMaps)
			g.Expect(issues).To(HaveLen(2))
			g.Expect(issues).Should(ContainElement(s2hv1beta1.DeploymentIssue{
				IssueType: s2hv1beta1.DeploymentIssueUndefined,
				FailureComponents: []s2hv1beta1.FailureComponent{
					{
						ComponentName:             "comp-1",
						FirstFailureContainerName: "comp-1",
						RestartCount:              0,
						NodeName:                  "node-1",
					},
					{
						ComponentName:             "comp-2",
						FirstFailureContainerName: "comp-2",
						RestartCount:              3,
						NodeName:                  "node-2",
					},
				},
			}))
			g.Expect(issues).Should(ContainElement(s2hv1beta1.DeploymentIssue{
				IssueType: s2hv1beta1.DeploymentIssueWaitForInitContainer,
				FailureComponents: []s2hv1beta1.FailureComponent{
					{
						ComponentName:             "comp-3",
						FirstFailureContainerName: "wait-for-dep3",
						RestartCount:              10,
						NodeName:                  "node-3",
					},
				},
			}))
		})
	})
})
