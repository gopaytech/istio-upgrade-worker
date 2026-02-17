package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/gopaytech/istio-upgrade-worker/pkg/logger"
)

var (
	clusterName string

	DeploymentsRestarted = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "istio_upgrade_deployments_restarted_total",
			Help: "Total number of deployments restarted",
		},
		[]string{"namespace"},
	)

	StatefulSetsRestarted = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "istio_upgrade_statefulsets_restarted_total",
			Help: "Total number of statefulsets restarted",
		},
		[]string{"namespace"},
	)

	RestartsFailed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "istio_upgrade_restarts_failed_total",
			Help: "Total number of failed restart attempts",
		},
		[]string{"namespace", "kind"},
	)

	WorkloadsSkipped = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "istio_upgrade_workloads_skipped_total",
			Help: "Total number of workloads skipped",
		},
		[]string{"reason"},
	)

	IterationCurrent = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "istio_upgrade_iteration_current",
			Help: "Current iteration number",
		},
	)
)

func Init(cluster string) {
	clusterName = cluster
}

func LogMetrics() {
	logger.Log().Info().
		Str("cluster", clusterName).
		Msg("metrics logged at end of run")
}
