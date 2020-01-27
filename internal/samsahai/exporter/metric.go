package exporter

import (
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/agoda-com/samsahai/api/v1beta1"
	"github.com/agoda-com/samsahai/internal"
	s2hlog "github.com/agoda-com/samsahai/internal/log"
)

var logger = s2hlog.S2HLog.WithName("exporter")

var TeamMetric = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Name: "samsahai_team",
	Help: "List team name",
}, []string{"teamName"})

var QueueMetric = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Name: "samsahai_queue",
	Help: "Show components in queue",
}, []string{"order", "teamName", "component", "version", "state", "no_of_processed"})

var QueueHistoriesMetric = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Name: "samsahai_queue_histories",
	Help: "Get queue histories",
}, []string{"teamName", "component", "version", "result", "log", "date"})

var HealthStatusMetric = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Name: "samsahai_health",
	Help: "show s2h's health status",
}, []string{"version", "gitCommit"})

var ActivePromotionMetric = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Name: "samsahai_active_promotion",
	Help: "Get values from samsahai active promotion",
}, []string{"teamName", "state"})

var ActivePromotionHistoriesMetric = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Name: "samsahai_active_promotion_histories",
	Help: "Get values from samsahai active promotion histories",
}, []string{"teamName", "name", "startTime", "result", "state"})

var OutdatedComponentMetric = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Name: "samsahai_outdated_component",
	Help: "Get values from samsahai active promotion histories",
}, []string{"teamName", "component", "currentVer", "desiredVer"})

func RegisterMetrics() {
	metrics.Registry.MustRegister(TeamMetric)
	metrics.Registry.MustRegister(QueueMetric)
	metrics.Registry.MustRegister(QueueHistoriesMetric)
	metrics.Registry.MustRegister(ActivePromotionMetric)
	metrics.Registry.MustRegister(ActivePromotionHistoriesMetric)
	metrics.Registry.MustRegister(OutdatedComponentMetric)
	metrics.Registry.MustRegister(HealthStatusMetric)
}

func SetTeamNameMetric(teamList map[string]internal.ConfigManager) {
	TeamMetric.Reset()
	for teamName := range teamList {
		TeamMetric.WithLabelValues(teamName).Set(1)
	}
}

func SetQueueMetric(queueList *v1beta1.QueueList, teamList map[string]internal.ConfigManager) {
	QueueMetric.Reset()
	for teamName := range teamList {
		componentList := teamList[teamName].GetComponents()
		for componentName := range componentList {
			queueStateList := map[string]float64{"waiting": 0, "testing": 0, "finished": 0, "deploying": 0, "cleaning": 0}
			if ok := isExist(queueList.Items, componentName, teamName); ok {
				continue
			} else {
				for state, val := range queueStateList {
					QueueMetric.WithLabelValues("", teamName, componentName, "", state, "").Set(val)
				}
			}
		}
		for i := range queueList.Items {
			queueStateList := map[string]float64{"waiting": 0, "testing": 0, "finished": 0, "deploying": 0, "cleaning": 0}
			queue := queueList.Items[i]
			switch queue.Status.State {
			case v1beta1.Waiting:
				queueStateList["waiting"] = 1
				for state, val := range queueStateList {
					QueueMetric.WithLabelValues(
						strconv.Itoa(queue.Spec.NoOfOrder),
						queue.Spec.TeamName,
						queue.Name,
						queue.Spec.Version,
						state,
						strconv.Itoa(queue.Status.NoOfProcessed)).Set(val)
				}
			case v1beta1.Testing, v1beta1.Collecting:
				queueStateList["testing"] = 1
				for state, val := range queueStateList {
					QueueMetric.WithLabelValues(
						strconv.Itoa(queue.Spec.NoOfOrder),
						queue.Spec.TeamName,
						queue.Name,
						queue.Spec.Version,
						state,
						strconv.Itoa(queue.Status.NoOfProcessed)).Set(val)
				}
			case v1beta1.Finished:
				queueStateList["finished"] = 1
				for state, val := range queueStateList {
					QueueMetric.WithLabelValues(
						strconv.Itoa(queue.Spec.NoOfOrder),
						queue.Spec.TeamName,
						queue.Name,
						queue.Spec.Version,
						state,
						strconv.Itoa(queue.Status.NoOfProcessed)).Set(val)
				}
			case v1beta1.DetectingImageMissing, v1beta1.Creating:
				queueStateList["deploying"] = 1
				for state, val := range queueStateList {
					QueueMetric.WithLabelValues(
						strconv.Itoa(queue.Spec.NoOfOrder),
						queue.Spec.TeamName,
						queue.Name,
						queue.Spec.Version,
						state,
						strconv.Itoa(queue.Status.NoOfProcessed)).Set(val)
				}
			case v1beta1.CleaningBefore, v1beta1.CleaningAfter:
				queueStateList["cleaning"] = 1
				for state, val := range queueStateList {
					QueueMetric.WithLabelValues(
						strconv.Itoa(queue.Spec.NoOfOrder),
						queue.Spec.TeamName,
						queue.Name,
						queue.Spec.Version,
						state,
						strconv.Itoa(queue.Status.NoOfProcessed)).Set(val)
				}
			}
		}
	}
}

func SetQueueHistoriesMetric(queueHistoriesList *v1beta1.QueueHistoryList, SamsahaiURL string) {
	QueueHistoriesMetric.Reset()
	for i := range queueHistoriesList.Items {
		queueHistoriesResult := "failed"
		queueHist := queueHistoriesList.Items[i]
		if queueHist.Spec.IsReverify {
			continue
		}
		if queueHist.Spec.IsDeploySuccess && queueHist.Spec.IsTestSuccess {
			queueHistoriesResult = "success"
		}
		QueueHistoriesMetric.WithLabelValues(
			queueHist.Spec.Queue.Spec.TeamName,
			queueHist.Name,
			queueHist.Spec.Queue.Spec.Version,
			queueHistoriesResult,
			SamsahaiURL+"/teams/"+queueHist.Spec.Queue.Spec.TeamName+"/queue/histories/"+queueHist.Name+"/log",
			queueHist.Spec.Queue.Status.UpdatedAt.Format(time.RFC3339),
		).Set(float64(queueHist.Spec.Queue.Status.NoOfProcessed))
	}
}

func SetHealthStatusMetric(version, gitCommit string, ts float64) {
	HealthStatusMetric.Reset()
	HealthStatusMetric.WithLabelValues(
		version,
		gitCommit).Set(ts)
}

func SetActivePromotionMetric(activePromotionList *v1beta1.ActivePromotionList) {
	ActivePromotionMetric.Reset()
	for i := range activePromotionList.Items {
		activePromStateList := map[string]float64{"waiting": 0, "deploying": 0, "testing": 0, "promoting": 0, "destroying": 0}
		activeProm := activePromotionList.Items[i]
		atpState := activeProm.Status.State
		if atpState != "" {
			switch atpState {
			case v1beta1.ActivePromotionWaiting:
				activePromStateList["waiting"] = 1
				for state, val := range activePromStateList {
					ActivePromotionMetric.WithLabelValues(
						activeProm.Name,
						state).Set(val)
				}
			case v1beta1.ActivePromotionDeployingComponents, v1beta1.ActivePromotionCreatingPreActive:
				activePromStateList["deploying"] = 1
				for state, val := range activePromStateList {
					ActivePromotionMetric.WithLabelValues(
						activeProm.Name,
						state).Set(val)
				}
			case v1beta1.ActivePromotionTestingPreActive, v1beta1.ActivePromotionCollectingPreActiveResult:
				activePromStateList["testing"] = 1
				for state, val := range activePromStateList {
					ActivePromotionMetric.WithLabelValues(
						activeProm.Name,
						state).Set(val)
				}
			case v1beta1.ActivePromotionActiveEnvironment, v1beta1.ActivePromotionDemoting:
				activePromStateList["promoting"] = 1
				for state, val := range activePromStateList {
					ActivePromotionMetric.WithLabelValues(
						activeProm.Name,
						state).Set(val)
				}
			case v1beta1.ActivePromotionDestroyingPreActive, v1beta1.ActivePromotionDestroyingPreviousActive, v1beta1.ActivePromotionFinished:
				activePromStateList["destroying"] = 1
				for state, val := range activePromStateList {
					ActivePromotionMetric.WithLabelValues(
						activeProm.Name,
						state).Set(val)
				}
			}
		}
	}
}
func SetActivePromotionHistoriesMetric(activePromotionHistories *v1beta1.ActivePromotionHistoryList) {
	ActivePromotionHistoriesMetric.Reset()
	for i := range activePromotionHistories.Items {
		atpHistories := activePromotionHistories.Items[i]
		if atpHistories.Spec.ActivePromotion == nil {
			continue
		}
		startTime := atpHistories.Spec.ActivePromotion.Status.StartedAt
		if startTime == nil {
			t := v1.Now()
			startTime = &t
		}
		atpConDict := map[string]v1.Time{}
		for _, o := range atpHistories.Spec.ActivePromotion.Status.Conditions {
			atpConDict[string(o.Type)] = o.LastTransitionTime
		}
		preAtpConDict := map[string]v1.Time{}
		for _, o := range atpHistories.Spec.ActivePromotion.Status.PreActiveQueue.Conditions {
			preAtpConDict[string(o.Type)] = o.LastTransitionTime
		}
		//waiting time
		var waitingDuration time.Duration
		if t1, ok := atpConDict["ActivePromotionStarted"]; ok {
			waitingDuration = duration(startTime.Time, t1.Time) / time.Second
		} else {
			waitingDuration = 0
		}
		// deploy time
		var deployDuration time.Duration
		if t1, ok := atpConDict["ActivePromotionStarted"]; ok {
			if t2, ok := preAtpConDict["QueueDeployed"]; ok {
				deployDuration = duration(t1.Time, t2.Time) / time.Second
			} else {
				deployDuration = 0
			}
		}
		// test time
		var testDuration time.Duration
		if t1, ok := preAtpConDict["QueueDeployed"]; ok {
			if t2, ok := atpConDict["PreActiveVerified"]; ok {
				testDuration = duration(t1.Time, t2.Time) / time.Second
			} else {
				testDuration = 0
			}
		}
		// promote time
		var promoteDuration time.Duration
		if t1, ok := atpConDict["PreActiveVerified"]; ok {
			if t2, ok := atpConDict["ActivePromoted"]; ok {
				promoteDuration = duration(t1.Time, t2.Time) / time.Second
			} else {
				promoteDuration = 0
			}
		}
		// destroy time
		var destroyDuration time.Duration
		if t1, ok := atpConDict["ActivePromoted"]; ok {
			if t2, ok := atpConDict["Finished"]; ok {
				destroyDuration = duration(t1.Time, t2.Time) / time.Second
			} else {
				destroyDuration = 0
			}
		}

		ActivePromotionHistoriesMetric.WithLabelValues(
			//TODO : Change Label to teamname field.
			atpHistories.Labels["samsahai.io/teamname"],
			atpHistories.Name,
			startTime.Format(time.RFC3339),
			string(atpHistories.Spec.ActivePromotion.Status.Result),
			"waiting").Set(float64(waitingDuration))
		ActivePromotionHistoriesMetric.WithLabelValues(
			atpHistories.Labels["samsahai.io/teamname"],
			atpHistories.Name,
			startTime.Format(time.RFC3339),
			string(atpHistories.Spec.ActivePromotion.Status.Result),
			"deploying").Set(float64(deployDuration))
		ActivePromotionHistoriesMetric.WithLabelValues(
			atpHistories.Labels["samsahai.io/teamname"],
			atpHistories.Name,
			startTime.Format(time.RFC3339),
			string(atpHistories.Spec.ActivePromotion.Status.Result),
			"testing").Set(float64(testDuration))
		ActivePromotionHistoriesMetric.WithLabelValues(
			atpHistories.Labels["samsahai.io/teamname"],
			atpHistories.Name,
			startTime.Format(time.RFC3339),
			string(atpHistories.Spec.ActivePromotion.Status.Result),
			"promoting").Set(float64(promoteDuration))
		ActivePromotionHistoriesMetric.WithLabelValues(
			atpHistories.Labels["samsahai.io/teamname"],
			atpHistories.Name,
			startTime.Format(time.RFC3339),
			string(atpHistories.Spec.ActivePromotion.Status.Result),
			"destroying").Set(float64(destroyDuration))
	}

}

func SetOutdatedComponentMetric(outdatedComponent *v1beta1.ActivePromotion) {
	teamName := outdatedComponent.Name
	for i := range outdatedComponent.Status.OutdatedComponents {
		outdated := outdatedComponent.Status.OutdatedComponents[i]
		outdatedDays := outdated.OutdatedDuration / (24 * time.Hour)
		OutdatedComponentMetric.WithLabelValues(
			teamName,
			outdated.Name,
			outdated.CurrentImage.Tag,
			outdated.LatestImage.Tag,
		).Set(float64(outdatedDays))
	}
}

func duration(start, end time.Time) time.Duration {
	var d time.Duration
	if !start.IsZero() && !end.IsZero() {
		d = end.Sub(start)
	}
	return d
}

func isExist(slice []v1beta1.Queue, comName, teamName string) bool {
	for _, item := range slice {
		if item.Name == comName && item.Spec.TeamName == teamName {
			return true
		}
	}
	return false
}
