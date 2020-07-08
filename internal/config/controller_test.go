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
	"github.com/agoda-com/samsahai/internal/util/unittest"
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
	teamTest := "teamTest"
	namespaceTest := "namespaceTest"
	compSource := s2hv1beta1.UpdatingSource("public-registry")
	redisCompName := "redis"
	redisScheduler := []string{"0 4 * * *", "0 5 * * *"}

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
		Scheduler: redisScheduler,
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
						"app.kubernetes.io/managed-by": "samsahai",
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
											Name:  "kubectl",
											Image: "reg-hk.agodadev.io/aiab/utils:1.0.1",
											Args:  []string{"/bin/sh", "-c", "set -eux\n\ncurl -X POST -k\nhttps://1234/webhook/component\n-d {\"component\":\"redis\",\"team\":\"teamTest\",\"repository\":\"bitnami/redis\"}"},
										},
									},
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
						"app.kubernetes.io/managed-by": "samsahai",
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
											Name:  "kubectl",
											Image: "reg-hk.agodadev.io/aiab/utils:1.0.1",
											Args: []string{
												"/bin/sh",
												"-c",
												"set -eux\n\n" +
													"curl -X POST -k \n" +
													" https://1234/webhook/component \n" +
													"-d {\"component\":\"redis\",\"team\":\"teamTest\",\"repository\":\"bitnami/redis\"}"},
										},
									},
									RestartPolicy: "OnFailure",
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
						"app.kubernetes.io/managed-by": "samsahai",
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
											Name:  "kubectl",
											Image: "reg-hk.agodadev.io/aiab/utils:1.0.1",
											Args: []string{
												"/bin/sh",
												"-c",
												"set -eux\n\n" +
													"curl -X POST -k \n https://1234/webhook/component \n" +
													"-d {\"component\":\"redis\",\"team\":\"teamTest\",\"repository\":\"bitnami/redis\"}"},
										},
									},
									RestartPolicy: "OnFailure",
								},
							},
						},
					},
				},
			},
		}

		c := controller{}
		creatingResult, deletingResult := c.CheckCronjobChange(namespaceTest, teamTest, &redisConfigComp, mockCronjob)

		g.Expect(creatingResult).To(HaveLen(len(expectedCronjob)))
		g.Expect(creatingResult).To(ConsistOf(expectedCronjob))
		g.Expect(deletingResult).To(HaveLen(len(mockCronjob.Items)))
		g.Expect(deletingResult).To(ConsistOf(mockCronjob.Items))
	})

	It("should create/delete cronjob correctly when config have duplicate scheduler", func() {
		g := NewWithT(GinkgoT())

		redisConfigComp.Scheduler = []string{"0 7 * * *", "0 7 * * *"}
		//expectedCronjob := []batchv1beta1.CronJob{
		//	{
		//		ObjectMeta: metav1.ObjectMeta{
		//			Name:      redisConfigComp.Name + "-checker-0",
		//			Namespace: namespaceTest,
		//			Labels: map[string]string{
		//				"app.kubernetes.io/managed-by": "samsahai",
		//				"samsahai.io/teamname":         teamTest,
		//				"component":                    redisConfigComp.Name,
		//			},
		//		},
		//		Spec: batchv1beta1.CronJobSpec{
		//			Schedule: "0 7 * * *",
		//			JobTemplate: batchv1beta1.JobTemplateSpec{
		//				Spec: batchv1.JobSpec{
		//					Template: corev1.PodTemplateSpec{
		//						Spec: corev1.PodSpec{
		//							Containers: []corev1.Container{
		//								{
		//									Name:  "kubectl",
		//									Image: "reg-hk.agodadev.io/aiab/utils:1.0.1",
		//									Args: []string{
		//										"/bin/sh",
		//										"-c",
		//										"set -eux\n\n" +
		//											"curl -X POST -k \n" +
		//											" https://1234/webhook/component \n" +
		//											"-d {\"component\":\"redis\",\"team\":\"teamTest\",\"repository\":\"bitnami/redis\"}"},
		//								},
		//							},
		//							RestartPolicy: "OnFailure",
		//						},
		//					},
		//				},
		//			},
		//		},
		//	},
		//}

		c := controller{}
		_, deletingResult := c.CheckCronjobChange(namespaceTest, teamTest, &redisConfigComp, mockCronjob)

		//g.Expect(creatingResult).To(HaveLen(len(expectedCronjob)))
		//g.Expect(creatingResult).To(ConsistOf(expectedCronjob))
		g.Expect(deletingResult).To(HaveLen(len(mockCronjob.Items)))
		g.Expect(deletingResult).To(ConsistOf(mockCronjob.Items))

	})

	It("should create cronjob/delete correctly when config have no scheduler", func() {
		g := NewWithT(GinkgoT())

		redisConfigComp.Scheduler = []string{}
		//expectedCronjob := []batchv1beta1.CronJob{
		//	{
		//		ObjectMeta: metav1.ObjectMeta{
		//			Name:      redisConfigComp.Name + "-checker-0",
		//			Namespace: namespaceTest,
		//			Labels: map[string]string{
		//				"app.kubernetes.io/managed-by": "samsahai",
		//				"samsahai.io/teamname":         teamTest,
		//				"component":                    redisConfigComp.Name,
		//			},
		//		},
		//		Spec: batchv1beta1.CronJobSpec{
		//			Schedule: "0 7 * * *",
		//			JobTemplate: batchv1beta1.JobTemplateSpec{
		//				Spec: batchv1.JobSpec{
		//					Template: corev1.PodTemplateSpec{
		//						Spec: corev1.PodSpec{
		//							Containers: []corev1.Container{
		//								{
		//									Name:  "kubectl",
		//									Image: "reg-hk.agodadev.io/aiab/utils:1.0.1",
		//									Args: []string{
		//										"/bin/sh",
		//										"-c",
		//										"set -eux\n\n" +
		//											"curl -X POST -k \n" +
		//											" https://1234/webhook/component \n" +
		//											"-d {\"component\":\"redis\",\"team\":\"teamTest\",\"repository\":\"bitnami/redis\"}"},
		//								},
		//							},
		//							RestartPolicy: "OnFailure",
		//						},
		//					},
		//				},
		//			},
		//		},
		//	},
		//}

		c := controller{}
		creatingResult, deletingResult := c.CheckCronjobChange(namespaceTest, teamTest, &redisConfigComp, mockCronjob)

		g.Expect(len(creatingResult)).To(Equal(0))
		g.Expect(deletingResult).To(HaveLen(len(mockCronjob.Items)))
		g.Expect(deletingResult).To(ConsistOf(mockCronjob.Items))

	})

})
