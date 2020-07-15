package config

import (
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
	successfulJobsHistoryLimit := successfulJobsHistoryLimit

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

	Describe("Component scheduler", func() {
		mockController := controller{
			s2hConfig: internal.SamsahaiConfig{SamsahaiExternalURL: "http://localhost:8080"},
		}
		teamTest := "teamtest"
		namespaceTest := "namespace"
		compSource := s2hv1beta1.UpdatingSource("public-registry")
		redisCompName := "redis"
		redisSchedules := []string{"0 4 * * *", "0 5 * * *"}

		cronJobCmd := mockController.getCronJobCmd("redis", teamTest, "bitnami/redis")
		cronJobResources := mockController.getCronJobResources()
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

		redisCronJobName := redisConfigComp.Name + "-checker-0x11xxx"
		redisCronJobLabels := mockController.getCronJobLabels(redisCronJobName, teamTest, redisCompName)
		mockCronJobs := &batchv1beta1.CronJobList{
			TypeMeta: metav1.TypeMeta{},
			ListMeta: metav1.ListMeta{},
			Items: []batchv1beta1.CronJob{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Cronjob",
						APIVersion: "batch/v1beta1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      redisCronJobName,
						Namespace: namespaceTest,
						Labels:    redisCronJobLabels,
					},
					Spec: batchv1beta1.CronJobSpec{
						SuccessfulJobsHistoryLimit: &successfulJobsHistoryLimit,
						Schedule: "0 11 * * *",
						JobTemplate: batchv1beta1.JobTemplateSpec{
							Spec: batchv1.JobSpec{
								Template: corev1.PodTemplateSpec{
									ObjectMeta: metav1.ObjectMeta{
										Labels: redisCronJobLabels,
									},
									Spec: corev1.PodSpec{
										Containers: []corev1.Container{
											{
												Name:      ContainerName,
												Image:     ContainerImage,
												Args:      []string{"/bin/sh", "-c", cronJobCmd},
												Resources: cronJobResources,
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

			cronJobName04 := redisConfigComp.Name + "-checker-0x4xxx"
			cronJobLabels04 := mockController.getCronJobLabels(cronJobName04, teamTest, redisConfigComp.Name)
			cronJobName05 := redisConfigComp.Name + "-checker-0x5xxx"
			cronJobLabels05 := mockController.getCronJobLabels(cronJobName05, teamTest, redisConfigComp.Name)
			expectedCronjob := []batchv1beta1.CronJob{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      cronJobName04,
						Namespace: namespaceTest,
						Labels:    cronJobLabels04,
					},
					Spec: batchv1beta1.CronJobSpec{
						SuccessfulJobsHistoryLimit: &successfulJobsHistoryLimit,
						Schedule: "0 4 * * *",
						JobTemplate: batchv1beta1.JobTemplateSpec{
							Spec: batchv1.JobSpec{
								Template: corev1.PodTemplateSpec{
									ObjectMeta: metav1.ObjectMeta{
										Labels: cronJobLabels04,
									},
									Spec: corev1.PodSpec{
										Containers: []corev1.Container{
											{
												Name:      ContainerName,
												Image:     ContainerImage,
												Args:      []string{"/bin/sh", "-c", cronJobCmd},
												Resources: cronJobResources,
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
						Name:      cronJobName05,
						Namespace: namespaceTest,
						Labels:    cronJobLabels05,
					},
					Spec: batchv1beta1.CronJobSpec{
						SuccessfulJobsHistoryLimit: &successfulJobsHistoryLimit,
						Schedule: "0 5 * * *",
						JobTemplate: batchv1beta1.JobTemplateSpec{
							Spec: batchv1.JobSpec{
								Template: corev1.PodTemplateSpec{
									ObjectMeta: metav1.ObjectMeta{
										Labels: cronJobLabels05,
									},
									Spec: corev1.PodSpec{
										Containers: []corev1.Container{
											{
												Name:      ContainerName,
												Image:     ContainerImage,
												Args:      []string{"/bin/sh", "-c", cronJobCmd},
												Resources: cronJobResources,
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
			creatingResult, deletingResult := c.getUpdatedCronJobs(namespaceTest, teamTest, &redisConfigComp, mockCronJobs)

			g.Expect(creatingResult).To(HaveLen(len(expectedCronjob)))
			g.Expect(creatingResult).To(ConsistOf(expectedCronjob))
			g.Expect(deletingResult).To(HaveLen(len(mockCronJobs.Items)))
			g.Expect(deletingResult).To(ConsistOf(mockCronJobs.Items))
		})

		It("should create/delete cronjob correctly when config have duplicate scheduler", func() {
			g := NewWithT(GinkgoT())

			redisConfigComp.Schedules = []string{"0 7 * * *", "0 7 * * *"}

			c := controller{}
			_, deletingResult := c.getUpdatedCronJobs(namespaceTest, teamTest, &redisConfigComp, mockCronJobs)

			g.Expect(deletingResult).To(HaveLen(len(mockCronJobs.Items)))
			g.Expect(deletingResult).To(ConsistOf(mockCronJobs.Items))
		})

		It("should create/delete cronjob correctly when config have no scheduler", func() {
			g := NewWithT(GinkgoT())

			redisConfigComp.Schedules = make([]string, 0)
			c := controller{}
			creatingResult, deletingResult := c.getUpdatedCronJobs(namespaceTest, teamTest, &redisConfigComp, mockCronJobs)

			g.Expect(len(creatingResult)).To(Equal(0))
			g.Expect(deletingResult).To(HaveLen(len(mockCronJobs.Items)))
			g.Expect(deletingResult).To(ConsistOf(mockCronJobs.Items))

		})

	})
})
