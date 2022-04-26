package config

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	s2hv1 "github.com/agoda-com/samsahai/api/v1"
	"github.com/agoda-com/samsahai/internal"
	"github.com/agoda-com/samsahai/internal/util/unittest"
)

const (
	ContainerName          = "component-checker"
	ContainerImage         = "quay.io/samsahai/curl:latest"
	ContainerRestartPolicy = "OnFailure"
)

func TestConfig(t *testing.T) {
	unittest.InitGinkgo(t, "Config Controller")
}

var _ = Describe("Config Controller", func() {
	successfulJobsHistoryLimit := successfulJobsHistoryLimit

	teamTest := "teamtest"
	compSource := s2hv1.UpdatingSource("public-registry")
	redisCompName := "redis"
	redisConfigComp := s2hv1.Component{
		Name: redisCompName,
		Chart: s2hv1.ComponentChart{
			Repository: "https://charts.helm.sh/stable",
			Name:       redisCompName,
		},
		Image: s2hv1.ComponentImage{
			Repository: "bitnami/redis",
			Pattern:    "5.*debian-9.*",
		},
		Source: &compSource,
		Values: s2hv1.ComponentValues{
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

	mockConfig := s2hv1.ConfigSpec{
		Envs: map[s2hv1.EnvType]s2hv1.ChartValuesURLs{
			"staging": map[string][]string{
				redisCompName: {
					"https://raw.githubusercontent.com/agoda-com/samsahai/master/test/data/wordpress-redis/envs/staging/redis.yaml"},
			},
		},
		Components: []*s2hv1.Component{
			&redisConfigComp,
		},
	}

	mockConfigUsingTemplate := s2hv1.Config{
		Spec: s2hv1.ConfigSpec{
			Template: teamTest,
		}}

	It("should get env values by the env type correctly", func() {
		g := NewWithT(GinkgoT())

		config := mockConfig
		compValues, err := GetEnvValues(&config, s2hv1.EnvStaging, teamTest)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(compValues).To(Equal(map[string]s2hv1.ComponentValues{
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
		compValues, err := GetEnvComponentValues(&config, redisCompName, teamTest, s2hv1.EnvStaging)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(compValues).To(Equal(s2hv1.ComponentValues{
			"master": map[string]interface{}{
				"service": map[string]interface{}{
					"nodePort": float64(31001),
					"type":     "NodePort",
				},
			},
		}))
	})

	It("should render teamName values correctly", func() {
		g := NewWithT(GinkgoT())
		valueTemplate := `
wordpress:	
  ingress:	
    hosts:	
    - wordpress.{{ .TeamName }}-1
    - wordpress.{{ .Team.Missing.Data }}-2
`

		Values := teamNameRendering(teamTest, valueTemplate)
		g.Expect(string(Values)).To(Equal(`
wordpress:	
  ingress:	
    hosts:	
    - wordpress.teamtest-1
    - wordpress.{{.Team.Missing.Data}}-2
`,
		))
	})

	It("should apply template to config correctly", func() {
		g := NewWithT(GinkgoT())

		configTemplate := s2hv1.Config{
			Spec: mockConfig,
		}
		err := applyConfigTemplate(&mockConfigUsingTemplate, &configTemplate)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(mockConfigUsingTemplate.Status.Used.Envs).To(Equal(configTemplate.Spec.Envs))
		g.Expect(mockConfigUsingTemplate.Status.Used.Components).To(Equal(configTemplate.Spec.Components))
	})

	Describe("Component scheduler", func() {
		mockController := controller{
			s2hConfig: internal.SamsahaiConfig{SamsahaiExternalURL: "http://localhost:8080"},
		}
		teamTest := "teamtest"
		namespaceTest := "namespace"
		compSource := s2hv1.UpdatingSource("public-registry")
		redisCompName := "redis"
		redisSchedules := []string{"0 4 * * *", "*/5 2,3 * * *"}

		cronJobCmd := mockController.getCronJobCmd("redis", teamTest, "bitnami/redis")
		cronJobResources := mockController.getCronJobResources()
		redisConfigComp := s2hv1.Component{
			Name: redisCompName,
			Chart: s2hv1.ComponentChart{
				Repository: "https://charts.helm.sh/stable",
				Name:       redisCompName,
			},
			Image: s2hv1.ComponentImage{
				Repository: "bitnami/redis",
				Pattern:    "5.*debian-9.*",
			},
			Schedules: redisSchedules,
			Source:    &compSource,
			Values: s2hv1.ComponentValues{
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
						Schedule:                   "0 11 * * *",
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

			cronJobName04 := redisConfigComp.Name + "-checker-" + mockController.getCronJobSuffix(redisSchedules[0])
			cronJobLabels04 := mockController.getCronJobLabels(cronJobName04, teamTest, redisConfigComp.Name)
			cronJobName05 := redisConfigComp.Name + "-checker-" + mockController.getCronJobSuffix(redisSchedules[1])
			cronJobLabels05 := mockController.getCronJobLabels(cronJobName05, teamTest, redisConfigComp.Name)
			g.Expect(cronJobName04).To(Equal("redis-checker-0x4xxx"))
			g.Expect(cronJobName05).To(Equal("redis-checker-e5x2n3xxx"))
			expectedCronjob := []batchv1beta1.CronJob{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      cronJobName04,
						Namespace: namespaceTest,
						Labels:    cronJobLabels04,
					},
					Spec: batchv1beta1.CronJobSpec{
						SuccessfulJobsHistoryLimit: &successfulJobsHistoryLimit,
						Schedule:                   "0 4 * * *",
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
						Schedule:                   "*/5 2,3 * * *",
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
