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

	compName := "comp1"

	It("should correctly set deployment issue `WaitForInitContainer` in Queue", func() {
		queue := s2hv1beta1.Queue{Status: s2hv1beta1.QueueStatus{}}
		pods := corev1.PodList{Items: []corev1.Pod{{
			ObjectMeta: metav1.ObjectMeta{
				Name: compName,
			},
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

		err := stagingCtrl.setDeploymentIssues(&queue, &pods, &batchv1.JobList{Items: []batchv1.Job{}})
		g.Expect(err).NotTo(HaveOccurred())

		g.Expect(queue.Status.DeploymentIssues).To(HaveLen(1))
		g.Expect(queue.Status.DeploymentIssues[0].IssueType).To(Equal(s2hv1beta1.DeploymentIssueWaitForInitContainer))
		g.Expect(queue.Status.DeploymentIssues[0].FailureComponents).To(HaveLen(1))
		g.Expect(queue.Status.DeploymentIssues[0].FailureComponents[0].ComponentName).To(Equal(compName))
		g.Expect(queue.Status.DeploymentIssues[0].FailureComponents[0].FirstFailureContainerName).To(Equal("wait-for-dep1"))
		g.Expect(queue.Status.DeploymentIssues[0].FailureComponents[0].RestartCount).To(Equal(int32(1)))
	})

	It("should correctly set deployment issue `ImagePullBackOff` in Queue", func() {
		queue := s2hv1beta1.Queue{Status: s2hv1beta1.QueueStatus{}}
		pods := corev1.PodList{Items: []corev1.Pod{{
			ObjectMeta: metav1.ObjectMeta{
				Name: compName,
			},
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

		err := stagingCtrl.setDeploymentIssues(&queue, &pods, &batchv1.JobList{Items: []batchv1.Job{}})
		g.Expect(err).NotTo(HaveOccurred())

		g.Expect(queue.Status.DeploymentIssues).To(HaveLen(1))
		g.Expect(queue.Status.DeploymentIssues[0].IssueType).To(Equal(s2hv1beta1.DeploymentIssueImagePullBackOff))
		g.Expect(queue.Status.DeploymentIssues[0].FailureComponents).To(HaveLen(1))
		g.Expect(queue.Status.DeploymentIssues[0].FailureComponents[0].ComponentName).To(Equal(compName))
		g.Expect(queue.Status.DeploymentIssues[0].FailureComponents[0].FirstFailureContainerName).To(Equal(compName))
		g.Expect(queue.Status.DeploymentIssues[0].FailureComponents[0].RestartCount).To(Equal(int32(0)))
	})

	It("should correctly set deployment issue `CrashLoopBackOff` in Queue", func() {
		queue := s2hv1beta1.Queue{Status: s2hv1beta1.QueueStatus{}}
		pods := corev1.PodList{Items: []corev1.Pod{{
			ObjectMeta: metav1.ObjectMeta{
				Name: compName,
			},
			Status: corev1.PodStatus{
				ContainerStatuses: []corev1.ContainerStatus{{
					Name:  compName,
					Ready: false,
					State: corev1.ContainerState{
						Waiting: &corev1.ContainerStateWaiting{
							Reason: "CrashLoopBackOff",
						},
					},
					RestartCount: 1,
				}},
			},
		}}}

		err := stagingCtrl.setDeploymentIssues(&queue, &pods, &batchv1.JobList{Items: []batchv1.Job{}})
		g.Expect(err).NotTo(HaveOccurred())

		g.Expect(queue.Status.DeploymentIssues).To(HaveLen(1))
		g.Expect(queue.Status.DeploymentIssues[0].IssueType).To(Equal(s2hv1beta1.DeploymentIssueCrashLoopBackOff))
		g.Expect(queue.Status.DeploymentIssues[0].FailureComponents).To(HaveLen(1))
		g.Expect(queue.Status.DeploymentIssues[0].FailureComponents[0].ComponentName).To(Equal(compName))
		g.Expect(queue.Status.DeploymentIssues[0].FailureComponents[0].FirstFailureContainerName).To(Equal(compName))
		g.Expect(queue.Status.DeploymentIssues[0].FailureComponents[0].RestartCount).To(Equal(int32(1)))
	})

	It("should correctly set deployment issue `CrashLoopBackOff` for Running 0/1 in Queue", func() {
		queue := s2hv1beta1.Queue{Status: s2hv1beta1.QueueStatus{}}
		pods := corev1.PodList{Items: []corev1.Pod{{
			ObjectMeta: metav1.ObjectMeta{
				Name: compName,
			},
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

		err := stagingCtrl.setDeploymentIssues(&queue, &pods, &batchv1.JobList{Items: []batchv1.Job{}})
		g.Expect(err).NotTo(HaveOccurred())

		g.Expect(queue.Status.DeploymentIssues).To(HaveLen(1))
		g.Expect(queue.Status.DeploymentIssues[0].IssueType).To(Equal(s2hv1beta1.DeploymentIssueCrashLoopBackOff))
		g.Expect(queue.Status.DeploymentIssues[0].FailureComponents).To(HaveLen(1))
		g.Expect(queue.Status.DeploymentIssues[0].FailureComponents[0].ComponentName).To(Equal(compName))
		g.Expect(queue.Status.DeploymentIssues[0].FailureComponents[0].FirstFailureContainerName).To(Equal(compName))
		g.Expect(queue.Status.DeploymentIssues[0].FailureComponents[0].RestartCount).To(Equal(int32(3)))
	})

	It("should correctly set deployment issue `ContainerCreating` in Queue", func() {
		queue := s2hv1beta1.Queue{Status: s2hv1beta1.QueueStatus{}}
		pods := corev1.PodList{Items: []corev1.Pod{{
			ObjectMeta: metav1.ObjectMeta{
				Name: compName,
			},
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

		err := stagingCtrl.setDeploymentIssues(&queue, &pods, &batchv1.JobList{Items: []batchv1.Job{}})
		g.Expect(err).NotTo(HaveOccurred())

		g.Expect(queue.Status.DeploymentIssues).To(HaveLen(1))
		g.Expect(queue.Status.DeploymentIssues[0].IssueType).To(Equal(s2hv1beta1.DeploymentIssueContainerCreating))
		g.Expect(queue.Status.DeploymentIssues[0].FailureComponents).To(HaveLen(1))
		g.Expect(queue.Status.DeploymentIssues[0].FailureComponents[0].ComponentName).To(Equal(compName))
		g.Expect(queue.Status.DeploymentIssues[0].FailureComponents[0].FirstFailureContainerName).To(Equal(compName))
		g.Expect(queue.Status.DeploymentIssues[0].FailureComponents[0].RestartCount).To(Equal(int32(0)))
	})

	It("should correctly set deployment issue `JobNotComplete` in Queue", func() {
		queue := s2hv1beta1.Queue{Status: s2hv1beta1.QueueStatus{}}
		jobs := batchv1.JobList{Items: []batchv1.Job{{
			ObjectMeta: metav1.ObjectMeta{
				Name: compName,
			},
			Status: batchv1.JobStatus{
				Active: 1,
			},
		}}}

		err := stagingCtrl.setDeploymentIssues(&queue, &corev1.PodList{Items: []corev1.Pod{}}, &jobs)
		g.Expect(err).NotTo(HaveOccurred())

		g.Expect(queue.Status.DeploymentIssues).To(HaveLen(1))
		g.Expect(queue.Status.DeploymentIssues[0].IssueType).To(Equal(s2hv1beta1.DeploymentIssueJobNotComplete))
		g.Expect(queue.Status.DeploymentIssues[0].FailureComponents).To(HaveLen(1))
		g.Expect(queue.Status.DeploymentIssues[0].FailureComponents[0].ComponentName).To(Equal(compName))
		g.Expect(queue.Status.DeploymentIssues[0].FailureComponents[0].FirstFailureContainerName).To(BeEmpty())
		g.Expect(queue.Status.DeploymentIssues[0].FailureComponents[0].RestartCount).To(Equal(int32(0)))
	})

	It("should correctly set deployment issue `Pending` in Queue", func() {
		queue := s2hv1beta1.Queue{Status: s2hv1beta1.QueueStatus{}}
		pods := corev1.PodList{Items: []corev1.Pod{{
			ObjectMeta: metav1.ObjectMeta{
				Name: compName,
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodPending,
			},
		}}}

		err := stagingCtrl.setDeploymentIssues(&queue, &pods, &batchv1.JobList{Items: []batchv1.Job{}})
		g.Expect(err).NotTo(HaveOccurred())

		g.Expect(queue.Status.DeploymentIssues).To(HaveLen(1))
		g.Expect(queue.Status.DeploymentIssues[0].IssueType).To(Equal(s2hv1beta1.DeploymentIssuePending))
		g.Expect(queue.Status.DeploymentIssues[0].FailureComponents).To(HaveLen(1))
		g.Expect(queue.Status.DeploymentIssues[0].FailureComponents[0].ComponentName).To(Equal(compName))
		g.Expect(queue.Status.DeploymentIssues[0].FailureComponents[0].FirstFailureContainerName).To(BeEmpty())
		g.Expect(queue.Status.DeploymentIssues[0].FailureComponents[0].RestartCount).To(Equal(int32(0)))
	})

	It("should correctly set multiple deployment issues in Queue", func() {
		queue := s2hv1beta1.Queue{Status: s2hv1beta1.QueueStatus{}}
		pods := corev1.PodList{Items: []corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "multi-comp1",
				},
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
					Name: "multi-comp2",
				},
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
				Active: 1,
			},
		}}}

		err := stagingCtrl.setDeploymentIssues(&queue, &pods, &jobs)
		g.Expect(err).NotTo(HaveOccurred())

		g.Expect(queue.Status.DeploymentIssues).To(HaveLen(3))
		g.Expect(queue.Status.DeploymentIssues).To(ContainElement(s2hv1beta1.DeploymentIssue{
			IssueType: s2hv1beta1.DeploymentIssueImagePullBackOff,
			FailureComponents: []s2hv1beta1.FailureComponent{{
				ComponentName:             "multi-comp1",
				FirstFailureContainerName: "multi-comp1",
				RestartCount:              0,
			}},
		}))
		g.Expect(queue.Status.DeploymentIssues).To(ContainElement(s2hv1beta1.DeploymentIssue{
			IssueType: s2hv1beta1.DeploymentIssueCrashLoopBackOff,
			FailureComponents: []s2hv1beta1.FailureComponent{{
				ComponentName:             "multi-comp2",
				FirstFailureContainerName: "multi-comp2",
				RestartCount:              10,
			}},
		}))
		g.Expect(queue.Status.DeploymentIssues).To(ContainElement(s2hv1beta1.DeploymentIssue{
			IssueType: s2hv1beta1.DeploymentIssueJobNotComplete,
			FailureComponents: []s2hv1beta1.FailureComponent{{
				ComponentName:             "job-1",
				FirstFailureContainerName: "",
				RestartCount:              0,
			}},
		}))
	})
})
