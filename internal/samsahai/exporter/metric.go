package exporter

import (
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	s2hv1beta1 "github.com/agoda-com/samsahai/api/v1beta1"
	s2hlog "github.com/agoda-com/samsahai/internal/log"
)

type ActivePromotionMetricState string
type QueueMetricState string

const (
	stateWaiting        ActivePromotionMetricState = "waiting"
	stateDeploying      ActivePromotionMetricState = "deploying"
	stateTesting        ActivePromotionMetricState = "testing"
	statePromoting      ActivePromotionMetricState = "promoting"
	stateDestroying     ActivePromotionMetricState = "destroying"
	queueStateWaiting   QueueMetricState           = "waiting"
	queueStateDeploying QueueMetricState           = "deploying"
	queueStateTesting   QueueMetricState           = "testing"
	queueStateCleaning  QueueMetricState           = "cleaning"
	queueStateFinished  QueueMetricState           = "finished"
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
}, []string{"teamName", "queueName", "component", "version", "state", "order", "no_of_processed"})

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

func SetHealthStatusMetric(version, gitCommit string, ts float64) {
	HealthStatusMetric.WithLabelValues(
		version,
		gitCommit).Set(ts)
}

func SetQueueMetric(queue *s2hv1beta1.Queue) {
	var queueState QueueMetricState
	switch queue.Status.State {
	case s2hv1beta1.Waiting:
		queueState = queueStateWaiting
	case s2hv1beta1.Testing, s2hv1beta1.Collecting:
		queueState = queueStateTesting
	case s2hv1beta1.DetectingImageMissing, s2hv1beta1.Creating:
		queueState = queueStateDeploying
	case s2hv1beta1.CleaningBefore:
		queueState = queueStateCleaning
	case s2hv1beta1.CleaningAfter:
		queueState = queueStateFinished
	}

	for _, qComp := range queue.Spec.Components {
		QueueMetric.WithLabelValues(
			queue.Spec.TeamName,
			queue.Name,
			qComp.Name,
			qComp.Version,
			string(queueState),
			strconv.Itoa(queue.Spec.NoOfOrder),
			strconv.Itoa(queue.Status.NoOfProcessed)).Set(float64(time.Now().Unix()))
	}
}

func SetActivePromotionMetric(atpComp *s2hv1beta1.ActivePromotion) {
	atpStateList := map[ActivePromotionMetricState]float64{stateWaiting: 0, stateDeploying: 0, stateTesting: 0, statePromoting: 0, stateDestroying: 0}
	atpState := atpComp.Status.State
	if atpState != "" {
		switch atpState {
		case s2hv1beta1.ActivePromotionWaiting:
			atpStateList[stateWaiting] = float64(time.Now().Unix())
			for state, val := range atpStateList {
				ActivePromotionMetric.WithLabelValues(
					atpComp.Name,
					string(state)).Set(val)
			}
		case s2hv1beta1.ActivePromotionDeployingComponents, s2hv1beta1.ActivePromotionCreatingPreActive:
			atpStateList[stateDeploying] = float64(time.Now().Unix())
			for state, val := range atpStateList {
				ActivePromotionMetric.WithLabelValues(
					atpComp.Name,
					string(state)).Set(val)
			}
		case s2hv1beta1.ActivePromotionTestingPreActive, s2hv1beta1.ActivePromotionCollectingPreActiveResult:
			atpStateList[stateTesting] = float64(time.Now().Unix())
			for state, val := range atpStateList {
				ActivePromotionMetric.WithLabelValues(
					atpComp.Name,
					string(state)).Set(val)
			}
		case s2hv1beta1.ActivePromotionActiveEnvironment, s2hv1beta1.ActivePromotionDemoting:
			atpStateList[statePromoting] = float64(time.Now().Unix())
			for state, val := range atpStateList {
				ActivePromotionMetric.WithLabelValues(
					atpComp.Name,
					string(state)).Set(val)
			}
		case s2hv1beta1.ActivePromotionDestroyingPreActive, s2hv1beta1.ActivePromotionDestroyingPreviousActive:
			atpStateList[stateDestroying] = float64(time.Now().Unix())
			for state, val := range atpStateList {
				ActivePromotionMetric.WithLabelValues(
					atpComp.Name,
					string(state)).Set(val)
			}
		case s2hv1beta1.ActivePromotionFinished:
			for state, val := range atpStateList {
				ActivePromotionMetric.WithLabelValues(
					atpComp.Name,
					string(state)).Set(val)
			}
		}
	}
}
