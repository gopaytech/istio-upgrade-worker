package metrics

import (
	"bytes"
	"testing"

	"github.com/gopaytech/istio-upgrade-worker/pkg/logger"
)

func TestMetricsRegistered(t *testing.T) {
	// Initialize logger first (required for LogMetrics)
	var buf bytes.Buffer
	logger.Init("info", "json", &buf)

	Init("test-cluster")

	// Verify counters can be incremented without panic
	DeploymentsRestarted.WithLabelValues("default").Inc()
	StatefulSetsRestarted.WithLabelValues("default").Inc()
	RestartsFailed.WithLabelValues("default", "deployment").Inc()
	WorkloadsSkipped.WithLabelValues("weekend").Inc()
	IterationCurrent.Set(1)
}

func TestLogMetrics(t *testing.T) {
	var buf bytes.Buffer
	logger.Init("info", "json", &buf)

	Init("test-cluster")
	DeploymentsRestarted.WithLabelValues("ns1").Add(5)

	// Should not panic and should log something
	LogMetrics()

	output := buf.String()
	if output == "" {
		t.Error("expected log output, got empty string")
	}
}
