package helm3

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"helm.sh/helm/v3/pkg/release"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/agoda-com/samsahai/internal/util/unittest"
)

func TestHelm3Engine(t *testing.T) {
	unittest.InitGinkgo(t, "Helm3 Engine")
}

var _ = Describe("Helm3 Engine", func() {
	g := NewWithT(GinkgoT())
	mockNamespace := "test"
	timeNow := metav1.Now()

	Describe("pre-hook jobs", func() {

		It("should successfully check pre-hooks job - all are ready", func() {
			mockJobs := batchv1.JobList{
				Items: []batchv1.Job{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "pre-hook-job-1",
							Namespace: mockNamespace,
							Annotations: map[string]string{
								release.HookAnnotation: "pre-install,pre-upgrade",
							},
						},
						Status: batchv1.JobStatus{
							CompletionTime: &timeNow,
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "pre-hook-job-2",
							Namespace: mockNamespace,
							Annotations: map[string]string{
								release.HookAnnotation: "pre-install",
							},
						},
						Status: batchv1.JobStatus{
							CompletionTime: &timeNow,
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "job-1",
							Namespace: mockNamespace,
						},
					},
				},
			}

			isReady := isPreHookJobsReady(&mockJobs)
			g.Expect(isReady).To(BeTrue(), "all pre-hook jobs are completed")
		})

		It("should successfully check pre-hooks job - no pre-hook jobs", func() {
			mockJobs := batchv1.JobList{
				Items: []batchv1.Job{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "job-1",
							Namespace: mockNamespace,
						},
					},
				},
			}

			isReady := isPreHookJobsReady(&mockJobs)
			g.Expect(isReady).To(BeTrue(), "all pre-hook jobs are completed")
		})

	})
})
