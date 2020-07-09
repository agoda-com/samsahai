package config

import (
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	s2hv1beta1 "github.com/agoda-com/samsahai/api/v1beta1"
	"github.com/agoda-com/samsahai/internal"
	"github.com/agoda-com/samsahai/internal/util/unittest"
)

const (
	AppName                = "samsahai"
	ContainerName          = "component-checker"
	ContainerImage         = "quay.io/samsahai/curl:latest"
	ContainerRestartPolicy = "OnFailure"
)

func TestConfig(t *testing.T) {
	unittest.InitGinkgo(t, "Config Controller")
}

var _ = Describe("Config Controller", func() {
	compSource := s2hv1beta1.UpdatingSource("public-registry")
	redisCompName := "redis"
	redisConfigComp := s2hv1beta1.Component{
		Name: redisCompName,
		Chart: s2hv1beta1.ComponentChart{
			Repository: "https://kubernetes-charts.storage.googleapis.com",
			Name:       redisCompName,
		},
		Image: s2hv1beta1.ComponentImage{
			Repository: "bitnami/redis",
			Pattern:    "5.*debian-9.*",
		},
		Source: &compSource,
		Values: s2hv1beta1.ComponentValues{
			"image": map[string]interface{}{
				"repository": "bitnami/redis",
				"pullPolicy": "IfNotPresent",
			},
			"cluster": map[string]interface{}{
				"enabled": false,
			},
			"usePassword": false,
			"master": map[string]interface{}{
				"persistence": map[string]interface{}{
					"enabled": false,
				},
			},
		},
	}

	mockConfig := s2hv1beta1.ConfigSpec{
		Envs: map[s2hv1beta1.EnvType]s2hv1beta1.ChartValuesURLs{
			"staging": map[string][]string{
				redisCompName: {
					"https://raw.githubusercontent.com/agoda-com/samsahai/master/test/data/wordpress-redis/envs/staging/redis.yaml"},
			},
		},
		Components: []*s2hv1beta1.Component{
			&redisConfigComp,
		},
	}

	It("should get env values by the env type correctly", func() {
		g := NewWithT(GinkgoT())

		config := mockConfig
		compValues, err := GetEnvValues(&config, s2hv1beta1.EnvStaging)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(compValues).To(Equal(map[string]s2hv1beta1.ComponentValues{
			redisCompName: {
				"master": map[string]interface{}{
					"service": map[string]interface{}{
						"nodePort": float64(31001),
						"type":     "NodePort",
					},
				},
			},
		}))
	})

	It("should get env values by the env type and component name correctly", func() {
		g := NewWithT(GinkgoT())

		config := mockConfig
		compValues, err := GetEnvComponentValues(&config, redisCompName, s2hv1beta1.EnvStaging)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(compValues).To(Equal(s2hv1beta1.ComponentValues{
			"master": map[string]interface{}{
				"service": map[string]interface{}{
					"nodePort": float64(31001),
					"type":     "NodePort",
				},
			},
		}))
	})
})

var _ = Describe("Updating Cronjob Controller", func() {
	mockcontroller := controller{
		s2hConfig: internal.SamsahaiConfig{SamsahaiExternalURL: "http://localhost:8080"},
	}
	teamTest := "teamTest"
	namespaceTest := "namespaceTest"
	compSource := s2hv1beta1.UpdatingSource("public-registry")
	redisCompName := "redis"
	redisSchedules := []string{"0 4 * * *", "0 5 * * *"}

	redisConfigComp := s2hv1beta1.Component{
		Name: redisCompName,
		Chart: s2hv1beta1.ComponentChart{
			Repository: "https://kubernetes-charts.storage.googleapis.com",
			Name:       redisCompName,
		},
		Image: s2hv1beta1.ComponentImage{
			Repository: "bitnami/redis",
			Pattern:    "5.*debian-9.*",
		},
		Schedules: redisSchedules,
		Source:    &compSource,
		Values: s2hv1beta1.ComponentValues{
			"image": map[string]interface{}{
				"repository": "bitnami/redis",
				"pullPolicy": "IfNotPresent",
			},
			"cluster": map[string]interface{}{
				"enabled": false,
			},
			"usePassword": false,
			"master": map[string]interface{}{
				"persistence": map[string]interface{}{
					"enabled": false,
				},
			},
		},
	}

	mockCronjob := batchv1beta1.CronJobList{
		TypeMeta: metav1.TypeMeta{},
		ListMeta: metav1.ListMeta{},
		Items: []batchv1beta1.CronJob{
			{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Cronjob",
					APIVersion: "batch/v1beta1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      redisConfigComp.Name + "-checker-0",
					Namespace: namespaceTest,
					Labels: map[string]string{
						"app.kubernetes.io/managed-by": AppName,
						"component":                    redisConfigComp.Name,
						"samsahai.io/teamname":         teamTest,
					},
				},
				Spec: batchv1beta1.CronJobSpec{
					Schedule: "0 11 * * *",
					JobTemplate: batchv1beta1.JobTemplateSpec{
						Spec: batchv1.JobSpec{
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									Containers: []corev1.Container{
										{
											Name:  ContainerName,
											Image: ContainerImage,
											Args: []string{"/bin/sh", "-c", fmt.Sprintf(`set -eux

curl -X POST -k %s-d '{"component": %s ,"team": %s ,"repository": %s}'
`, mockcontroller.s2hConfig.SamsahaiExternalURL, "redis", "teamTest", "bitnami/redis")},
										},
									},
									RestartPolicy: ContainerRestartPolicy,
								},
							},
						},
					},
				},
			},
		},
	}

	It("should create/delete cronjob correctly", func() {
		g := NewWithT(GinkgoT())

		expectedCronjob := []batchv1beta1.CronJob{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      redisConfigComp.Name + "-checker-0",
					Namespace: namespaceTest,
					Labels: map[string]string{
						"app.kubernetes.io/managed-by": AppName,
						"samsahai.io/teamname":         teamTest,
						"component":                    redisConfigComp.Name,
					},
				},
				Spec: batchv1beta1.CronJobSpec{
					Schedule: "0 4 * * *",
					JobTemplate: batchv1beta1.JobTemplateSpec{
						Spec: batchv1.JobSpec{
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									Containers: []corev1.Container{
										{
											Name:  ContainerName,
											Image: ContainerImage,
											Args: []string{"/bin/sh", "-c", fmt.Sprintf(`set -eux

curl -X POST -k %v -d '{"component": %s, "team": %s, "repository": %s}'
`, mockcontroller.s2hConfig.SamsahaiExternalURL, "redis", "teamTest", "bitnami/redis")},
										},
									},
									RestartPolicy: ContainerRestartPolicy,
								},
							},
						},
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      redisConfigComp.Name + "-checker-1",
					Namespace: namespaceTest,
					Labels: map[string]string{
						"app.kubernetes.io/managed-by": AppName,
						"samsahai.io/teamname":         teamTest,
						"component":                    redisConfigComp.Name,
					},
				},
				Spec: batchv1beta1.CronJobSpec{
					Schedule: "0 5 * * *",
					JobTemplate: batchv1beta1.JobTemplateSpec{
						Spec: batchv1.JobSpec{
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									Containers: []corev1.Container{
										{
											Name:  ContainerName,
											Image: ContainerImage,
											Args: []string{"/bin/sh", "-c", fmt.Sprintf(`set -eux

curl -X POST -k %v -d '{"component": %s, "team": %s, "repository": %s}'
`, mockcontroller.s2hConfig.SamsahaiExternalURL, "redis", "teamTest", "bitnami/redis")},
										},
									},
									RestartPolicy: ContainerRestartPolicy,
								},
							},
						},
					},
				},
			},
		}

		c := controller{
			s2hConfig: internal.SamsahaiConfig{SamsahaiExternalURL: "http://localhost:8080"},
		}
		creatingResult, deletingResult := c.GetUpdateCronJobs(namespaceTest, teamTest, &redisConfigComp, mockCronjob)

		g.Expect(creatingResult).To(HaveLen(len(expectedCronjob)))
		g.Expect(creatingResult).To(ConsistOf(expectedCronjob))
		g.Expect(deletingResult).To(HaveLen(len(mockCronjob.Items)))
		g.Expect(deletingResult).To(ConsistOf(mockCronjob.Items))
	})

	It("should create/delete cronjob correctly when config have duplicate scheduler", func() {
		g := NewWithT(GinkgoT())

		redisConfigComp.Schedules = []string{"0 7 * * *", "0 7 * * *"}

		c := controller{}
		_, deletingResult := c.GetUpdateCronJobs(namespaceTest, teamTest, &redisConfigComp, mockCronjob)

		g.Expect(deletingResult).To(HaveLen(len(mockCronjob.Items)))
		g.Expect(deletingResult).To(ConsistOf(mockCronjob.Items))

	})

	It("should create cronjob/delete correctly when config have no scheduler", func() {
		g := NewWithT(GinkgoT())

		redisConfigComp.Schedules = []string{}
		c := controller{}
		creatingResult, deletingResult := c.GetUpdateCronJobs(namespaceTest, teamTest, &redisConfigComp, mockCronjob)

		g.Expect(len(creatingResult)).To(Equal(0))
		g.Expect(deletingResult).To(HaveLen(len(mockCronjob.Items)))
		g.Expect(deletingResult).To(ConsistOf(mockCronjob.Items))

	})

})
