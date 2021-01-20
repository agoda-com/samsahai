package k8sobject

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

	"github.com/agoda-com/samsahai/internal/util/unittest"
)

func TestK8sObject(t *testing.T) {
	unittest.InitGinkgo(t, "K8s Object")
}

var _ = Describe("K8s Object", func() {
	It("should correctly sort env vars", func() {
		firstContainers := corev1.Container{
			Env: []corev1.EnvVar{
				{Name: "env-1", Value: "1"},
				{Name: "env-2", Value: "2"},
				{Name: "env-3", Value: "3"},
			},
		}
		secondContainers := corev1.Container{
			Env: []corev1.EnvVar{
				{Name: "env-2", Value: "2"},
				{Name: "env-1", Value: "1"},
				{Name: "env-3", Value: "3"},
			},
		}

		result := areContainersEqual(firstContainers, secondContainers)
		Expect(result).To(BeTrue())
	})
})
