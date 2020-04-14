package exporter

import (
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	s2hv1beta1 "github.com/agoda-com/samsahai/api/v1beta1"
	s2hlog "github.com/agoda-com/samsahai/internal/log"
)

var logger = s2hlog.S2HLog.WithName("exporter")

var TeamMetric = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Name: "samsahai_team",
	Help: "List team name",
}, []string{"teamName"})

var HealthStatusMetric = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Name: "samsahai_health",
	Help: "show s2h's health status",
}, []string{"version", "gitCommit"})

var QueueMetric = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Name: "samsahai_queue",
	Help: "Show components in queue",
}, []string{ "teamName", "component", "version", "state","order", "no_of_processed"})

var ActivePromotionMetric = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Name: "samsahai_active_promotion",
	Help: "Get values from samsahai active promotion",
}, []string{"teamName", "state"})

func RegisterMetrics() {
	metrics.Registry.MustRegister(TeamMetric)
	metrics.Registry.MustRegister(QueueMetric)
	metrics.Registry.MustRegister(ActivePromotionMetric)
	metrics.Registry.MustRegister(HealthStatusMetric)
}

func SetTeamNameMetric(teamList *s2hv1beta1.TeamList) {
	for _, teamComp := range teamList.Items {
		TeamMetric.WithLabelValues(teamComp.Name).Set(1)
	}
}

func SetQueueMetric(queue *s2hv1beta1.Queue) {
	var queueState string
	switch queue.Status.State {
	case s2hv1beta1.Waiting:
		queueState = "waiting"
	case s2hv1beta1.Testing, s2hv1beta1.Collecting:
		queueState = "testing"
	case s2hv1beta1.Finished:
		queueState = "finished"
	case s2hv1beta1.DetectingImageMissing, s2hv1beta1.Creating:
		queueState = "deploying"
	case s2hv1beta1.CleaningBefore, s2hv1beta1.CleaningAfter:
		queueState = "cleaning"
	}
	QueueMetric.WithLabelValues(
		queue.Spec.TeamName,
		queue.Name,
		queue.Spec.Version,
		queueState,
		strconv.Itoa(queue.Spec.NoOfOrder),
		strconv.Itoa(queue.Status.NoOfProcessed)).Set(float64(time.Now().Unix()))
}

func SetHealthStatusMetric(version, gitCommit string, ts float64) {
	HealthStatusMetric.WithLabelValues(
		version,
		gitCommit).Set(ts)
}

func SetActivePromotionMetric(activeProm *s2hv1beta1.ActivePromotion) {
	activePromStateList := map[string]float64{"waiting": 0, "deploying": 0, "testing": 0, "promoting": 0, "destroying": 0}
	atpState := activeProm.Status.State
	if atpState != "" {
		switch atpState {
		case s2hv1beta1.ActivePromotionWaiting:
			activePromStateList["waiting"] = float64(time.Now().Unix())
			for state, val := range activePromStateList {
				ActivePromotionMetric.WithLabelValues(
					activeProm.Name,
					state).Set(val)
			}
		case s2hv1beta1.ActivePromotionDeployingComponents, s2hv1beta1.ActivePromotionCreatingPreActive:
			activePromStateList["deploying"] = float64(time.Now().Unix())
			for state, val := range activePromStateList {
				ActivePromotionMetric.WithLabelValues(
					activeProm.Name,
					state).Set(val)
			}
		case s2hv1beta1.ActivePromotionTestingPreActive, s2hv1beta1.ActivePromotionCollectingPreActiveResult:
			activePromStateList["testing"] = float64(time.Now().Unix())
			for state, val := range activePromStateList {
				ActivePromotionMetric.WithLabelValues(
					activeProm.Name,
					state).Set(val)
			}
		case s2hv1beta1.ActivePromotionActiveEnvironment, s2hv1beta1.ActivePromotionDemoting:
			activePromStateList["promoting"] = float64(time.Now().Unix())
			for state, val := range activePromStateList {
				ActivePromotionMetric.WithLabelValues(
					activeProm.Name,
					state).Set(val)
			}
		case s2hv1beta1.ActivePromotionDestroyingPreActive, s2hv1beta1.ActivePromotionDestroyingPreviousActive:
			activePromStateList["destroying"] = float64(time.Now().Unix())
			for state, val := range activePromStateList {
				ActivePromotionMetric.WithLabelValues(
					activeProm.Name,
					state).Set(val)
			}
		case s2hv1beta1.ActivePromotionFinished:
			for state, val := range activePromStateList {
				ActivePromotionMetric.WithLabelValues(
					activeProm.Name,
					state).Set(val)
			}
		}
	}
}
