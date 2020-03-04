package exporter

import (
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	s2hv1beta1 "github.com/agoda-com/samsahai/api/v1beta1"
	"github.com/agoda-com/samsahai/internal"
	"github.com/agoda-com/samsahai/internal/config"
	"github.com/agoda-com/samsahai/internal/config/git"
	"github.com/agoda-com/samsahai/internal/util/http"
	"github.com/agoda-com/samsahai/internal/util/unittest"
)

func TestExporter(t *testing.T) {
	unittest.InitGinkgo(t, "Samsahai Exporter")
}

var cfg *rest.Config
var c client.Client

func TestMain(m *testing.M) {
	var err error
	t := &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "..", "..", "config", "crds")},
	}

	err = s2hv1beta1.SchemeBuilder.AddToScheme(scheme.Scheme)
	if err != nil {
		log.Fatal(err)
	}

	if cfg, err = t.Start(); err != nil {
		logger.Error(err, "start testenv error")
		os.Exit(1)
	}

	if c, err = client.New(cfg, client.Options{Scheme: scheme.Scheme}); err != nil {
		log.Fatal(err)
	}

	code := m.Run()
	_ = t.Stop()
	os.Exit(code)
}

func startDate(y, mo, d, h, mi, s int) *metav1.Time {
	ti := metav1.Date(y, time.Month(mo), d, h, mi, s, 0, time.UTC)
	return &ti
}
func date(y, mo, d, h, mi, s int) metav1.Time {
	ti := metav1.Date(y, time.Month(mo), d, h, mi, s, 0, time.UTC)
	return ti
}

var _ = Describe("Samsahai Exporter", func() {
	timeout := float64(3000)
	namespace := "default"
	g := NewWithT(GinkgoT())
	var wgStop *sync.WaitGroup
	var chStop chan struct{}
	var SamsahaiURL = "aaa"
	var updatedDate = metav1.Date(2020, 01, 01, 01, 01, 01, 01, time.UTC)
	var configMgr internal.ConfigManager
	var err error

	RegisterMetrics()

	BeforeEach(func(done Done) {
		defer GinkgoRecover()
		defer close(done)

		configMgr, err = config.NewWithGitClient(&git.Client{}, "example", path.Join("..", "..", "..", "test", "data", "wordpress-redis"))
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(configMgr).NotTo(BeNil())

		chStop = make(chan struct{})

		mgr, err := manager.New(cfg, manager.Options{Namespace: namespace, MetricsBindAddress: ":8008"})
		Expect(err).NotTo(HaveOccurred(), "should create manager successfully")

		tOld := map[string]internal.ConfigManager{
			"testQTeamNameOld": configMgr,
		}
		qOld := &s2hv1beta1.QueueList{
			Items: []s2hv1beta1.Queue{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "qNameOld",
						Namespace: namespace,
					},
					Spec: s2hv1beta1.QueueSpec{
						TeamName:  "testQTeamNameOld",
						Version:   "10.9.8.7-Old",
						NoOfOrder: 0,
					},
					Status: s2hv1beta1.QueueStatus{
						NoOfProcessed: 1,
						State:         "waiting",
					},
				},
			},
		}
		qhOld := &s2hv1beta1.QueueHistoryList{
			Items: []s2hv1beta1.QueueHistory{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "testQHnameOld",
						Namespace: namespace,
					},
					Spec: s2hv1beta1.QueueHistorySpec{
						Queue: &s2hv1beta1.Queue{
							Spec: s2hv1beta1.QueueSpec{
								TeamName: "testQHTeamNameOld",
								Version:  "1.2.3.4-Old",
							},
							Status: s2hv1beta1.QueueStatus{
								UpdatedAt:     &updatedDate,
								NoOfProcessed: 9,
							},
						},
						IsDeploySuccess: true,
						IsTestSuccess:   true,
					},
				},
			},
		}
		apOld := &s2hv1beta1.ActivePromotionList{
			Items: []s2hv1beta1.ActivePromotion{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "testAPNameOld",
						Namespace: namespace,
					},
					Status: s2hv1beta1.ActivePromotionStatus{
						State: s2hv1beta1.ActivePromotionFinished,
					},
				},
			},
		}
		aphOld := &s2hv1beta1.ActivePromotionHistoryList{
			Items: []s2hv1beta1.ActivePromotionHistory{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "testAPHNameOld",
						Namespace: namespace,
					},
					Spec: s2hv1beta1.ActivePromotionHistorySpec{
						TeamName: "testAPHTeamnameOld",
						ActivePromotion: &s2hv1beta1.ActivePromotion{
							Status: s2hv1beta1.ActivePromotionStatus{
								State:     s2hv1beta1.ActivePromotionDestroyingPreActive,
								StartedAt: startDate(2019, 12, 10, 2, 22, 02),
								PreActiveQueue: s2hv1beta1.QueueStatus{
									Conditions: []s2hv1beta1.QueueCondition{
										{
											Type:               "QueueDeployed",
											LastTransitionTime: date(2019, 12, 10, 3, 3, 36),
										},
									},
								},
								Conditions: []s2hv1beta1.ActivePromotionCondition{
									{
										Type:               "ActivePromotionStarted",
										LastTransitionTime: date(2019, 12, 10, 2, 22, 02),
										Message:            "reason message",
									},
									{
										Type:               "PreActiveVerified",
										LastTransitionTime: date(2019, 12, 10, 3, 38, 21),
										Message:            "reason message",
									},
									{
										Type:               "ActivePromoted",
										LastTransitionTime: date(2019, 12, 10, 3, 39, 01),
										Message:            "reason message",
									},
									{
										Type:               "Finished",
										LastTransitionTime: date(2019, 12, 10, 4, 11, 13),
										Message:            "reason message",
									},
								},
								Result: s2hv1beta1.ActivePromotionSuccess,
							},
						},
					},
				},
			},
		}
		ocOld := &s2hv1beta1.ActivePromotion{
			ObjectMeta: metav1.ObjectMeta{
				Name: "testOCTeamNameOld",
			},
			Status: s2hv1beta1.ActivePromotionStatus{
				OutdatedComponents: map[string]s2hv1beta1.OutdatedComponent{
					"testOCNameOld": {
						CurrentImage: &s2hv1beta1.Image{
							Tag: "2019.12.07.00-Old",
						},
						DesiredImage: &s2hv1beta1.Image{
							Tag: "2019.12.10.00-Old",
						},
						OutdatedDuration: 99540000000000,
					},
					"testOCName2": {
						CurrentImage: &s2hv1beta1.Image{
							Tag: "2019.12.07.00",
						},
						DesiredImage: &s2hv1beta1.Image{
							Tag: "2019.12.10.00",
						},
						OutdatedDuration: 99599999999999,
					},
				},
			},
		}
		SetTeamNameMetric(tOld)
		SetQueueMetric(qOld, tOld)
		SetQueueHistoriesMetric(qhOld, SamsahaiURL)
		SetActivePromotionMetric(apOld)
		SetActivePromotionHistoriesMetric(aphOld)
		SetOutdatedComponentMetric(ocOld)
		SetHealthStatusMetric("9.9.9.8", "777888999", 234000)

		wgStop = &sync.WaitGroup{}
		wgStop.Add(1)
		go func() {
			defer wgStop.Done()
			Expect(mgr.Start(chStop)).To(BeNil())
		}()
	}, timeout)

	AfterEach(func(done Done) {
		defer close(done)
		close(chStop)
		wgStop.Wait()
	}, timeout)

	It("Should show team name correctly ", func() {
		data, err := http.Get("http://localhost:8008/metrics")
		g.Expect(err).NotTo(HaveOccurred())
		expectedData := strings.Contains(string(data), `samsahai_team{teamName="testQTeamNameOld"} 1`)
		g.Expect(expectedData).To(BeTrue())

		t := map[string]internal.ConfigManager{
			"testQTeamName1": configMgr,
		}
		SetTeamNameMetric(t)
		data, err = http.Get("http://localhost:8008/metrics")
		g.Expect(err).NotTo(HaveOccurred())
		expectedData = strings.Contains(string(data), `samsahai_team{teamName="testQTeamName1"} 1`)
		g.Expect(expectedData).To(BeTrue())
		expectedData = strings.Contains(string(data), `samsahai_team{teamName="testQTeamNameOld"} 1`)
		g.Expect(expectedData).To(BeFalse())

	}, timeout)

	It("Should show queue metric correctly  ", func(done Done) {
		defer close(done)
		data, err := http.Get("http://localhost:8008/metrics")
		g.Expect(err).NotTo(HaveOccurred())
		expectedData := strings.Contains(string(data), `samsahai_queue{component="qNameOld",no_of_processed="1",order="0",state="waiting",teamName="testQTeamNameOld",version="10.9.8.7-Old"} 1`)
		g.Expect(expectedData).To(BeTrue())

		t := map[string]internal.ConfigManager{
			"testQTeamName1": configMgr,
		}
		q := &s2hv1beta1.QueueList{
			Items: []s2hv1beta1.Queue{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "qName1",
						Namespace: namespace,
					},
					Spec: s2hv1beta1.QueueSpec{
						TeamName:  "testQTeamName1",
						Version:   "10.9.8.7",
						NoOfOrder: 0,
					},
					Status: s2hv1beta1.QueueStatus{
						NoOfProcessed: 1,
						State:         "waiting",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "qName2",
						Namespace: namespace,
					},
					Spec: s2hv1beta1.QueueSpec{
						TeamName:  "testQTeamName2",
						Version:   "7.8.9.10",
						NoOfOrder: 0,
					},
					Status: s2hv1beta1.QueueStatus{
						NoOfProcessed: 1,
						State:         "waiting",
					},
				},
			},
		}
		SetQueueMetric(q, t)
		data, err = http.Get("http://localhost:8008/metrics")
		g.Expect(err).NotTo(HaveOccurred())
		expectedData = strings.Contains(string(data), `samsahai_queue{component="qName1",no_of_processed="1",order="0",state="waiting",teamName="testQTeamName1",version="10.9.8.7"} 1`)
		g.Expect(expectedData).To(BeTrue())
		expectedData = strings.Contains(string(data), `samsahai_queue{component="qName2",no_of_processed="1",order="0",state="waiting",teamName="testQTeamName2",version="7.8.9.10"} 1`)
		g.Expect(expectedData).To(BeTrue())
		expectedData = strings.Contains(string(data), `samsahai_queue{component="",`)
		g.Expect(expectedData).To(BeFalse())
		expectedData = strings.Contains(string(data), `samsahai_queue{component="qNameOld",no_of_processed="1",order="0",state="waiting",teamName="testQTeamNameOld",version="10.9.8.7-Old"} 1`)
		g.Expect(expectedData).To(BeFalse())
	}, timeout)

	It("Should show queue histories metric correctly ", func(done Done) {
		defer close(done)
		data, err := http.Get("http://localhost:8008/metrics")
		g.Expect(err).NotTo(HaveOccurred())
		expectedData := strings.Contains(string(data), `samsahai_queue_histories{component="testQHnameOld",date="2020-01-01T01:01:01Z",log="aaa/teams/testQHTeamNameOld/queue/histories/testQHnameOld/log",result="success",teamName="testQHTeamNameOld",version="1.2.3.4-Old"} 9`)
		g.Expect(expectedData).To(BeTrue())

		qh := &s2hv1beta1.QueueHistoryList{
			Items: []s2hv1beta1.QueueHistory{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "testQHname1",
						Namespace: namespace,
					},
					Spec: s2hv1beta1.QueueHistorySpec{
						Queue: &s2hv1beta1.Queue{
							Spec: s2hv1beta1.QueueSpec{
								TeamName: "testQHTeamName1",
								Version:  "1.2.3.4",
							},
							Status: s2hv1beta1.QueueStatus{
								UpdatedAt:     &updatedDate,
								NoOfProcessed: 9,
							},
						},
						IsDeploySuccess: true,
						IsTestSuccess:   true,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "testQHname2",
						Namespace: namespace,
					},
					Spec: s2hv1beta1.QueueHistorySpec{
						Queue: &s2hv1beta1.Queue{
							Spec: s2hv1beta1.QueueSpec{
								TeamName: "testQHTeamName2",
								Version:  "4.3.2.1",
							},
							Status: s2hv1beta1.QueueStatus{
								UpdatedAt:     &updatedDate,
								NoOfProcessed: 9,
							},
						},
						IsDeploySuccess: true,
						IsTestSuccess:   true,
					},
				},
			},
		}
		SetQueueHistoriesMetric(qh, SamsahaiURL)
		data, err = http.Get("http://localhost:8008/metrics")
		g.Expect(err).NotTo(HaveOccurred())
		expectedData = strings.Contains(string(data), `samsahai_queue_histories{component="testQHname1",date="2020-01-01T01:01:01Z",log="aaa/teams/testQHTeamName1/queue/histories/testQHname1/log",result="success",teamName="testQHTeamName1",version="1.2.3.4"} 9`)
		g.Expect(expectedData).To(BeTrue())
		expectedData = strings.Contains(string(data), `samsahai_queue_histories{component="testQHname2",date="2020-01-01T01:01:01Z",log="aaa/teams/testQHTeamName2/queue/histories/testQHname2/log",result="success",teamName="testQHTeamName2",version="4.3.2.1"} 9`)
		g.Expect(expectedData).To(BeTrue())
		expectedData = strings.Contains(string(data), `samsahai_queue_histories{component="",`)
		g.Expect(expectedData).To(BeFalse())
		expectedData = strings.Contains(string(data), `samsahai_queue_histories{component="testQHnameOld",date="2020-01-01T01:01:01Z",log="aaa/teams/testQHTeamNameOld/queue/histories/testQHnameOld/log",result="success",teamName="testQHTeamNameOld",version="1.2.3.4-Old"} 9`)
		g.Expect(expectedData).To(BeFalse())
	}, timeout)

	It("Should show active promotion correctly", func(done Done) {
		defer close(done)
		data, err := http.Get("http://localhost:8008/metrics")
		g.Expect(err).NotTo(HaveOccurred())
		expectedData := strings.Contains(string(data), `samsahai_active_promotion{state="destroying",teamName="testAPNameOld"} 1`)
		g.Expect(expectedData).To(BeTrue())

		ap := &s2hv1beta1.ActivePromotionList{
			Items: []s2hv1beta1.ActivePromotion{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "testAPName1",
						Namespace: namespace,
					},
					Status: s2hv1beta1.ActivePromotionStatus{
						State: s2hv1beta1.ActivePromotionWaiting,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "testAPName2",
						Namespace: namespace,
					},
					Status: s2hv1beta1.ActivePromotionStatus{
						State: s2hv1beta1.ActivePromotionFinished,
					},
				},
			},
		}
		SetActivePromotionMetric(ap)
		data, err = http.Get("http://localhost:8008/metrics")
		g.Expect(err).NotTo(HaveOccurred())
		expectedData = strings.Contains(string(data), `samsahai_active_promotion{state="destroying",teamName="testAPName2"} 1`)
		g.Expect(expectedData).To(BeTrue())
		expectedData = strings.Contains(string(data), `samsahai_active_promotion{state="waiting",teamName="testAPName1"} 1`)
		g.Expect(expectedData).To(BeTrue())
		expectedData = strings.Contains(string(data), `samsahai_active_promotion{state="destroying",teamName="testAPNameOld"} 1`)
		g.Expect(expectedData).To(BeFalse())

	}, timeout)

	It("Should show active promotion histories correctly", func(done Done) {
		defer close(done)
		data, err := http.Get("http://localhost:8008/metrics")
		g.Expect(err).NotTo(HaveOccurred())
		expectedData := strings.Contains(string(data), `samsahai_active_promotion_histories{failureReason="reason message",name="testAPHNameOld",result="Success",startTime="2019-12-10T02:22:02Z",state="deploying",teamName="testAPHTeamnameOld"} 2494`)
		g.Expect(expectedData).To(BeTrue())
		expectedData = strings.Contains(string(data), `samsahai_active_promotion_histories{failureReason="reason message",name="testAPHNameOld",result="Success",startTime="2019-12-10T02:22:02Z",state="destroying",teamName="testAPHTeamnameOld"} 1932`)
		g.Expect(expectedData).To(BeTrue())
		expectedData = strings.Contains(string(data), `samsahai_active_promotion_histories{failureReason="reason message",name="testAPHNameOld",result="Success",startTime="2019-12-10T02:22:02Z",state="promoting",teamName="testAPHTeamnameOld"} 40`)
		g.Expect(expectedData).To(BeTrue())
		expectedData = strings.Contains(string(data), `samsahai_active_promotion_histories{failureReason="reason message",name="testAPHNameOld",result="Success",startTime="2019-12-10T02:22:02Z",state="testing",teamName="testAPHTeamnameOld"} 2085`)
		g.Expect(expectedData).To(BeTrue())
		expectedData = strings.Contains(string(data), `samsahai_active_promotion_histories{failureReason="reason message",name="testAPHNameOld",result="Success",startTime="2019-12-10T02:22:02Z",state="waiting",teamName="testAPHTeamnameOld"} 0`)
		g.Expect(expectedData).To(BeTrue())

		aph := &s2hv1beta1.ActivePromotionHistoryList{
			Items: []s2hv1beta1.ActivePromotionHistory{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "testAPHName1",
						Namespace: namespace,
					},
					Spec: s2hv1beta1.ActivePromotionHistorySpec{
						TeamName: "testAPHTeamname1",
						ActivePromotion: &s2hv1beta1.ActivePromotion{
							Status: s2hv1beta1.ActivePromotionStatus{
								State:     s2hv1beta1.ActivePromotionDestroyingPreActive,
								StartedAt: startDate(2019, 12, 10, 2, 22, 02),
								PreActiveQueue: s2hv1beta1.QueueStatus{
									Conditions: []s2hv1beta1.QueueCondition{
										{
											Type:               "QueueDeployed",
											LastTransitionTime: date(2019, 12, 10, 3, 3, 36),
										},
									},
								},
								Conditions: []s2hv1beta1.ActivePromotionCondition{
									{
										Type:               "ActivePromotionStarted",
										LastTransitionTime: date(2019, 12, 10, 2, 22, 02),
										Message:            "reason message",
									},
									{
										Type:               "PreActiveVerified",
										LastTransitionTime: date(2019, 12, 10, 3, 38, 21),
										Message:            "reason message",
									},
									{
										Type:               "ActivePromoted",
										LastTransitionTime: date(2019, 12, 10, 3, 39, 01),
										Message:            "reason message",
									},
									{
										Type:               "Finished",
										LastTransitionTime: date(2019, 12, 10, 4, 11, 13),
										Message:            "reason message",
									},
								},
								Result: s2hv1beta1.ActivePromotionSuccess,
							},
						},
					},
				},
			},
		}
		SetActivePromotionHistoriesMetric(aph)
		data, err = http.Get("http://localhost:8008/metrics")
		g.Expect(err).NotTo(HaveOccurred())
		expectedData = strings.Contains(string(data), `samsahai_active_promotion_histories{failureReason="reason message",name="testAPHName1",result="Success",startTime="2019-12-10T02:22:02Z",state="deploying",teamName="testAPHTeamname1"} 2494`)
		g.Expect(expectedData).To(BeTrue())
		expectedData = strings.Contains(string(data), `samsahai_active_promotion_histories{failureReason="reason message",name="testAPHName1",result="Success",startTime="2019-12-10T02:22:02Z",state="destroying",teamName="testAPHTeamname1"} 1932`)
		g.Expect(expectedData).To(BeTrue())
		expectedData = strings.Contains(string(data), `samsahai_active_promotion_histories{failureReason="reason message",name="testAPHName1",result="Success",startTime="2019-12-10T02:22:02Z",state="promoting",teamName="testAPHTeamname1"} 40`)
		g.Expect(expectedData).To(BeTrue())
		expectedData = strings.Contains(string(data), `samsahai_active_promotion_histories{failureReason="reason message",name="testAPHName1",result="Success",startTime="2019-12-10T02:22:02Z",state="testing",teamName="testAPHTeamname1"} 2085`)
		g.Expect(expectedData).To(BeTrue())
		expectedData = strings.Contains(string(data), `samsahai_active_promotion_histories{failureReason="reason message",name="testAPHName1",result="Success",startTime="2019-12-10T02:22:02Z",state="waiting",teamName="testAPHTeamname1"} 0`)
		g.Expect(expectedData).To(BeTrue())
		expectedData = strings.Contains(string(data), `samsahai_active_promotion_histories{failureReason="reason message",name="testAPHNameOld",result="Success",startTime="2019-12-10T02:22:02Z",state="deploying",teamName="testAPHTeamnameOld"} 2494`)
		g.Expect(expectedData).To(BeFalse())
		expectedData = strings.Contains(string(data), `samsahai_active_promotion_histories{failureReason="reason message",name="testAPHNameOld",result="Success",startTime="2019-12-10T02:22:02Z",state="destroying",teamName="testAPHTeamnameOld"} 1932`)
		g.Expect(expectedData).To(BeFalse())
		expectedData = strings.Contains(string(data), `samsahai_active_promotion_histories{failureReason="reason message",name="testAPHNameOld",result="Success",startTime="2019-12-10T02:22:02Z",state="promoting",teamName="testAPHTeamnameOld"} 40`)
		g.Expect(expectedData).To(BeFalse())
		expectedData = strings.Contains(string(data), `samsahai_active_promotion_histories{failureReason="reason message",name="testAPHNameOld",result="Success",startTime="2019-12-10T02:22:02Z",state="testing",teamName="testAPHTeamnameOld"} 2085`)
		g.Expect(expectedData).To(BeFalse())
		expectedData = strings.Contains(string(data), `samsahai_active_promotion_histories{name="testAPHNameOld",result="Success",startTime="2019-12-10T02:22:02Z",state="waiting",teamName="testAPHTeamnameOld"} 0`)
		g.Expect(expectedData).To(BeFalse())
	}, timeout)

	It("Should show outdated component correctly", func(done Done) {
		defer close(done)
		data, err := http.Get("http://localhost:8008/metrics")
		g.Expect(err).NotTo(HaveOccurred())
		data2 := string(data)
		expectedData := strings.Contains(data2, `samsahai_outdated_component{component="testOCNameOld",currentVer="2019.12.07.00-Old",desiredVer="2019.12.10.00-Old",teamName="testOCTeamNameOld"} 1`)
		g.Expect(expectedData).To(BeTrue())
		OutdatedComponentMetric.Reset()

		oc := &s2hv1beta1.ActivePromotion{
			ObjectMeta: metav1.ObjectMeta{
				Name: "testOCTeamName",
			},
			Status: s2hv1beta1.ActivePromotionStatus{
				OutdatedComponents: map[string]s2hv1beta1.OutdatedComponent{
					"testOCName1": {
						CurrentImage: &s2hv1beta1.Image{
							Tag: "2019.12.07.00",
						},
						DesiredImage: &s2hv1beta1.Image{
							Tag: "2019.12.10.00",
						},
						OutdatedDuration: 99540000000000,
					},
					"testOCName2": {
						CurrentImage: &s2hv1beta1.Image{
							Tag: "2019.12.07.00",
						},
						DesiredImage: &s2hv1beta1.Image{
							Tag: "2019.12.10.00",
						},
						OutdatedDuration: 99599999999999,
					},
				},
			},
		}
		SetOutdatedComponentMetric(oc)
		data, err = http.Get("http://localhost:8008/metrics")
		g.Expect(err).NotTo(HaveOccurred())
		expectedData = strings.Contains(string(data), `samsahai_outdated_component{component="testOCName1",currentVer="2019.12.07.00",desiredVer="2019.12.10.00",teamName="testOCTeamName"} 1`)
		g.Expect(expectedData).To(BeTrue())
		expectedData = strings.Contains(string(data), `samsahai_outdated_component{component="testOCName2",currentVer="2019.12.07.00",desiredVer="2019.12.10.00",teamName="testOCTeamName"} 1`)
		g.Expect(expectedData).To(BeTrue())
		expectedData = strings.Contains(string(data), `samsahai_outdated_component{component="testOCNameOld",currentVer="2019.12.07.00-Old",desiredVer="2019.12.10.00-Old",teamName="testOCTeamNameOld"} 1`)
		g.Expect(expectedData).To(BeFalse())
	}, timeout)

	It("Should show health metric correctly", func(done Done) {
		defer close(done)
		data, err := http.Get("http://localhost:8008/metrics")
		g.Expect(err).NotTo(HaveOccurred())
		expectedData := strings.Contains(string(data), `samsahai_health{gitCommit="777888999",version="9.9.9.8"} 234000`)
		g.Expect(expectedData).To(BeTrue())
	}, timeout)
})
