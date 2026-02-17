# istio-upgrade-worker Improvements Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add bug fixes, dry-run mode, structured logging (zerolog), Prometheus metrics, and graceful shutdown to istio-upgrade-worker.

**Architecture:** Layered improvements - start with bug fixes, add new settings, introduce logging package, add metrics package, then wire graceful shutdown. Each layer builds on the previous.

**Tech Stack:** Go 1.22, zerolog, prometheus/client_golang, Helm charts

---

## Task 1: Add zerolog Dependency

**Files:**
- Modify: `go.mod`

**Step 1: Add zerolog dependency**

Run:
```bash
go get github.com/rs/zerolog
```

**Step 2: Verify dependency added**

Run: `grep zerolog go.mod`
Expected: `github.com/rs/zerolog v1.x.x`

**Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "chore: add zerolog dependency"
```

---

## Task 2: Create Logger Package

**Files:**
- Create: `pkg/logger/logger.go`
- Create: `pkg/logger/logger_test.go`

**Step 1: Write the test**

```go
package logger

import (
	"bytes"
	"strings"
	"testing"

	"github.com/rs/zerolog"
)

func TestInitLogger_JSONFormat(t *testing.T) {
	var buf bytes.Buffer
	Init("info", "json", &buf)

	Log().Info().Msg("test message")

	output := buf.String()
	if !strings.Contains(output, `"level":"info"`) {
		t.Errorf("expected JSON output with level:info, got: %s", output)
	}
	if !strings.Contains(output, `"message":"test message"`) {
		t.Errorf("expected JSON output with message, got: %s", output)
	}
}

func TestInitLogger_ConsoleFormat(t *testing.T) {
	var buf bytes.Buffer
	Init("info", "console", &buf)

	Log().Info().Msg("test message")

	output := buf.String()
	if !strings.Contains(output, "INF") {
		t.Errorf("expected console output with INF, got: %s", output)
	}
}

func TestInitLogger_DebugLevel(t *testing.T) {
	var buf bytes.Buffer
	Init("debug", "json", &buf)

	Log().Debug().Msg("debug message")

	output := buf.String()
	if !strings.Contains(output, `"level":"debug"`) {
		t.Errorf("expected debug level in output, got: %s", output)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/logger/... -v`
Expected: FAIL (package doesn't exist)

**Step 3: Write minimal implementation**

```go
package logger

import (
	"io"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

var log zerolog.Logger

// Init initializes the global logger with specified level and format.
// format: "json" (default) or "console"
// level: "debug", "info", "warn", "error" (default: "info")
func Init(level, format string, w ...io.Writer) {
	var output io.Writer = os.Stdout
	if len(w) > 0 {
		output = w[0]
	}

	if strings.ToLower(format) == "console" {
		output = zerolog.ConsoleWriter{Out: output, TimeFormat: time.RFC3339}
	}

	lvl, err := zerolog.ParseLevel(strings.ToLower(level))
	if err != nil {
		lvl = zerolog.InfoLevel
	}

	log = zerolog.New(output).Level(lvl).With().Timestamp().Logger()
}

// Log returns the global logger instance.
func Log() *zerolog.Logger {
	return &log
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/logger/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add pkg/logger/
git commit -m "feat: add logger package with zerolog"
```

---

## Task 3: Add New Settings (DryRun, LogLevel, LogFormat)

**Files:**
- Modify: `settings/settings.go`
- Modify: `settings/settings_test.go` (create if doesn't exist)

**Step 1: Write the test**

```go
package settings

import (
	"os"
	"testing"
)

func TestSettings_DryRunDefault(t *testing.T) {
	os.Setenv("CLUSTER_NAME", "test")
	os.Setenv("DEPLOYMENT_FREEZE_CONFIG_FILE_PATH", "/tmp/test")
	defer os.Unsetenv("CLUSTER_NAME")
	defer os.Unsetenv("DEPLOYMENT_FREEZE_CONFIG_FILE_PATH")

	s, err := NewSettings()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if s.DryRun != false {
		t.Errorf("expected DryRun default to be false, got %v", s.DryRun)
	}
}

func TestSettings_LogDefaults(t *testing.T) {
	os.Setenv("CLUSTER_NAME", "test")
	os.Setenv("DEPLOYMENT_FREEZE_CONFIG_FILE_PATH", "/tmp/test")
	defer os.Unsetenv("CLUSTER_NAME")
	defer os.Unsetenv("DEPLOYMENT_FREEZE_CONFIG_FILE_PATH")

	s, err := NewSettings()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if s.LogLevel != "info" {
		t.Errorf("expected LogLevel default to be 'info', got %v", s.LogLevel)
	}
	if s.LogFormat != "json" {
		t.Errorf("expected LogFormat default to be 'json', got %v", s.LogFormat)
	}
}

func TestSettings_WebhookValidation_SlackMissing(t *testing.T) {
	os.Setenv("CLUSTER_NAME", "test")
	os.Setenv("DEPLOYMENT_FREEZE_CONFIG_FILE_PATH", "/tmp/test")
	os.Setenv("NOTIFICATION_MODE", "slack")
	os.Setenv("NOTIFICATION_SLACK_WEBHOOK", "")
	defer func() {
		os.Unsetenv("CLUSTER_NAME")
		os.Unsetenv("DEPLOYMENT_FREEZE_CONFIG_FILE_PATH")
		os.Unsetenv("NOTIFICATION_MODE")
		os.Unsetenv("NOTIFICATION_SLACK_WEBHOOK")
	}()

	_, err := NewSettings()
	if err == nil {
		t.Error("expected error for missing slack webhook")
	}
}

func TestSettings_WebhookValidation_LarkMissing(t *testing.T) {
	os.Setenv("CLUSTER_NAME", "test")
	os.Setenv("DEPLOYMENT_FREEZE_CONFIG_FILE_PATH", "/tmp/test")
	os.Setenv("NOTIFICATION_MODE", "lark")
	os.Setenv("NOTIFICATION_LARK_WEBHOOK", "")
	defer func() {
		os.Unsetenv("CLUSTER_NAME")
		os.Unsetenv("DEPLOYMENT_FREEZE_CONFIG_FILE_PATH")
		os.Unsetenv("NOTIFICATION_MODE")
		os.Unsetenv("NOTIFICATION_LARK_WEBHOOK")
	}()

	_, err := NewSettings()
	if err == nil {
		t.Error("expected error for missing lark webhook")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./settings/... -v`
Expected: FAIL (fields don't exist)

**Step 3: Update settings.go**

Add to Settings struct:
```go
DryRun    bool   `envconfig:"DRY_RUN" default:"false"`
LogLevel  string `envconfig:"LOG_LEVEL" default:"info"`
LogFormat string `envconfig:"LOG_FORMAT" default:"json"`
```

Add error variables:
```go
ErrSlackWebhookRequired = errors.New("NOTIFICATION_SLACK_WEBHOOK is required when NOTIFICATION_MODE is slack")
ErrLarkWebhookRequired  = errors.New("NOTIFICATION_LARK_WEBHOOK is required when NOTIFICATION_MODE is lark")
```

Update Validation():
```go
func (s Settings) Validation() error {
	if s.MaximumPercentageRolloutInSingleExecution*s.MaximumIteration < 100 {
		return ErrTotalRolloutLessThan100Percentage
	}

	if s.NotificationMode == "slack" && s.NotificationSlackWebhook == "" {
		return ErrSlackWebhookRequired
	}

	if s.NotificationMode == "lark" && s.NotificationLarkWebhook == "" {
		return ErrLarkWebhookRequired
	}

	return nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./settings/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add settings/
git commit -m "feat: add DryRun, LogLevel, LogFormat settings with webhook validation"
```

---

## Task 4: Fix Namespace Deduplication

**Files:**
- Modify: `services/kubernetes/namespace.go`
- Create: `services/kubernetes/namespace_test.go`

**Step 1: Write the test**

```go
package kubernetes

import (
	"testing"
)

func TestDeduplicateNamespaces(t *testing.T) {
	input := []Namespace{
		{Name: "app-a"},
		{Name: "app-b"},
		{Name: "app-a"}, // duplicate
		{Name: "app-c"},
		{Name: "app-b"}, // duplicate
	}

	result := deduplicateNamespaces(input)

	if len(result) != 3 {
		t.Errorf("expected 3 namespaces, got %d", len(result))
	}

	expected := map[string]bool{"app-a": true, "app-b": true, "app-c": true}
	for _, ns := range result {
		if !expected[ns.Name] {
			t.Errorf("unexpected namespace: %s", ns.Name)
		}
		delete(expected, ns.Name)
	}

	if len(expected) != 0 {
		t.Errorf("missing namespaces: %v", expected)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./services/kubernetes/... -v -run TestDeduplicate`
Expected: FAIL (function doesn't exist)

**Step 3: Add deduplication function and update GetIstioNamespaces**

Add to namespace.go:
```go
func deduplicateNamespaces(namespaces []Namespace) []Namespace {
	seen := make(map[string]bool)
	result := make([]Namespace, 0)

	for _, ns := range namespaces {
		if !seen[ns.Name] {
			seen[ns.Name] = true
			result = append(result, ns)
		}
	}

	return result
}
```

Update GetIstioNamespaces:
```go
func (s *NamespaceService) GetIstioNamespaces(ctx context.Context) ([]Namespace, error) {
	canaryNamespaces, err := s.GetNamespaceByLabel(ctx, s.canaryLabel)
	if err != nil {
		return nil, err
	}
	nonCanaryNamespaces, err := s.GetNamespaceByLabel(ctx, s.label)
	if err != nil {
		return nil, err
	}
	var namespaces []Namespace
	namespaces = append(namespaces, canaryNamespaces...)
	namespaces = append(namespaces, nonCanaryNamespaces...)

	return deduplicateNamespaces(namespaces), nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./services/kubernetes/... -v -run TestDeduplicate`
Expected: PASS

**Step 5: Commit**

```bash
git add services/kubernetes/
git commit -m "fix: deduplicate namespaces to prevent double processing"
```

---

## Task 5: Add Context Support to Slack Notification

**Files:**
- Modify: `services/notification/slack/slack.go`

**Step 1: Update Slack Send to use context**

Replace `PostWebhook` with `PostWebhookContext`:

```go
func (s Slack) Send(ctx context.Context, upgrade types.Notification) error {
	attachment := slack.Attachment{
		AuthorName: "istio-upgrade-worker",
		Title:      upgrade.Title,
		Text:       upgrade.Message,
		Ts:         json.Number(strconv.FormatInt(time.Now().Unix(), 10)),
	}

	msg := slack.WebhookMessage{
		Attachments: []slack.Attachment{attachment},
	}

	err := slack.PostWebhookContext(ctx, s.Settings.NotificationSlackWebhook, &msg)
	if err != nil {
		log.Printf("failed to send slack notification: %v\n", err.Error())
		return err
	}

	return nil
}
```

**Step 2: Verify build passes**

Run: `go build ./...`
Expected: Success

**Step 3: Commit**

```bash
git add services/notification/slack/
git commit -m "fix: use context in Slack notification for cancellation support"
```

---

## Task 6: Add Context Support to Lark Notification

**Files:**
- Modify: `services/notification/lark/lark.go`

**Step 1: Update Lark Send to respect context**

```go
func (s Lark) Send(ctx context.Context, upgrade types.Notification) error {
	bot := golark.NewNotificationBot(s.Settings.NotificationLarkWebhook)

	content := golark.NewPostBuilder().
		Title(upgrade.Title).
		TextTag(upgrade.Message, 1, true).
		Render()
	buffer := golark.NewMsgBuffer(golark.MsgPost).Post(content).Build()

	// Use channel to handle context cancellation
	type result struct {
		err error
	}
	done := make(chan result, 1)

	go func() {
		_, err := bot.PostNotificationV2(buffer)
		done <- result{err: err}
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case r := <-done:
		if r.err != nil {
			log.Printf("failed to send lark notification: %v\n", r.err.Error())
			return r.err
		}
		return nil
	}
}
```

**Step 2: Verify build passes**

Run: `go build ./...`
Expected: Success

**Step 3: Commit**

```bash
git add services/notification/lark/
git commit -m "fix: respect context cancellation in Lark notification"
```

---

## Task 7: Create Metrics Package

**Files:**
- Create: `pkg/metrics/metrics.go`
- Create: `pkg/metrics/metrics_test.go`

**Step 1: Add prometheus dependency**

Run:
```bash
go get github.com/prometheus/client_golang/prometheus
go get github.com/prometheus/client_golang/prometheus/promauto
```

**Step 2: Write the test**

```go
package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestMetricsRegistered(t *testing.T) {
	Init("test-cluster")

	// Verify counters can be incremented without panic
	DeploymentsRestarted.WithLabelValues("default").Inc()
	StatefulSetsRestarted.WithLabelValues("default").Inc()
	RestartsFailed.WithLabelValues("default", "deployment").Inc()
	WorkloadsSkipped.WithLabelValues("weekend").Inc()
	IterationCurrent.Set(1)
}

func TestLogMetrics(t *testing.T) {
	Init("test-cluster")
	DeploymentsRestarted.WithLabelValues("ns1").Add(5)

	// Should not panic
	LogMetrics()
}
```

**Step 3: Write implementation**

```go
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/rs/zerolog/log"
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
	log.Info().
		Str("cluster", clusterName).
		Msg("metrics logged at end of run")
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/metrics/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add pkg/metrics/ go.mod go.sum
git commit -m "feat: add Prometheus metrics package"
```

---

## Task 8: Add Graceful Shutdown to main.go

**Files:**
- Modify: `main.go`

**Step 1: Update main.go with signal handling and logger init**

```go
package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"

	"github.com/gopaytech/istio-upgrade-worker/config"
	"github.com/gopaytech/istio-upgrade-worker/pkg/logger"
	"github.com/gopaytech/istio-upgrade-worker/pkg/metrics"
	"github.com/gopaytech/istio-upgrade-worker/services/kubernetes"
	"github.com/gopaytech/istio-upgrade-worker/services/notification"
	"github.com/gopaytech/istio-upgrade-worker/services/upgrader"
	"github.com/gopaytech/istio-upgrade-worker/settings"
)

func main() {
	settings, err := settings.NewSettings()
	if err != nil {
		// Use stdlib log for early errors before logger is initialized
		panic(err)
	}

	// Initialize logger
	logger.Init(settings.LogLevel, settings.LogFormat)
	log := logger.Log()

	log.Info().Msg("successfully loaded settings")

	// Initialize metrics
	metrics.Init(settings.ClusterName)

	kubernetesConfig, err := config.LoadKubernetes()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load kubernetes config")
	}
	log.Info().Msg("successfully loaded kubernetes config")

	deploymentFreezeConfig, err := config.LoadDeploymentFreeze(settings)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load deployment freeze config")
	}
	log.Info().Msg("successfully loaded deployment freeze config")

	notificationService, err := notification.NotificationFactory(settings)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create notification service")
	}

	namespaceService := kubernetes.NewNamespaceService(kubernetesConfig, settings)
	deploymentService := kubernetes.NewDeploymentService(kubernetesConfig)
	statefulsetService := kubernetes.NewStatefulSetService(kubernetesConfig)
	podService := kubernetes.NewPodService(kubernetesConfig)
	configmapService := kubernetes.NewConfigMapService(kubernetesConfig)

	proxyUpgrader := upgrader.NewProxyUpgrader(
		settings,
		deploymentFreezeConfig,
		notificationService,
		namespaceService,
		deploymentService,
		statefulsetService,
		configmapService,
		podService,
	)

	// Setup cancellable context with signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		sig := <-sigChan
		log.Warn().Str("signal", sig.String()).Msg("received shutdown signal, cancelling...")
		cancel()
	}()

	if err := proxyUpgrader.Upgrade(ctx); err != nil {
		if errors.Is(err, context.Canceled) {
			log.Info().Msg("upgrade interrupted by shutdown signal")
			metrics.LogMetrics()
			os.Exit(0)
		}
		log.Error().Err(err).Msg("upgrade failed")
		metrics.LogMetrics()
		os.Exit(1)
	}

	log.Info().Msg("upgrade completed successfully")
	metrics.LogMetrics()
}
```

**Step 2: Verify build passes**

Run: `go build ./...`
Expected: Success

**Step 3: Commit**

```bash
git add main.go
git commit -m "feat: add graceful shutdown and initialize logger/metrics"
```

---

## Task 9: Add Dry-Run Mode and Migrate Logging in proxy.go

**Files:**
- Modify: `services/upgrader/proxy.go`

**Step 1: Update imports**

Add to imports:
```go
"github.com/gopaytech/istio-upgrade-worker/pkg/logger"
"github.com/gopaytech/istio-upgrade-worker/pkg/metrics"
```

**Step 2: Update Restart method for dry-run and metrics**

```go
func (upgrader *ProxyUpgrader) Restart(ctx context.Context, upgradedDeployments []DeploymentUpgrade, upgradedStatefulSets []StatefulSetUpgrade) ([]DeploymentUpgrade, []StatefulSetUpgrade) {
	log := logger.Log()
	var failedDeployments []DeploymentUpgrade
	var failedStatefulSets []StatefulSetUpgrade

	for _, deployment := range upgradedDeployments {
		// Check for context cancellation
		if ctx.Err() != nil {
			log.Warn().Msg("context cancelled, stopping restart loop")
			break
		}

		if upgrader.Settings.DryRun {
			log.Info().
				Str("namespace", deployment.namespace).
				Str("deployment", deployment.name).
				Msg("[DRY-RUN] would restart deployment")
			continue
		}

		if err := upgrader.DeploymentService.RolloutRestart(ctx, deployment.namespace, deployment.name); err != nil {
			log.Error().
				Err(err).
				Str("namespace", deployment.namespace).
				Str("deployment", deployment.name).
				Msg("failed to rollout restart deployment")
			failedDeployments = append(failedDeployments, deployment)
			metrics.RestartsFailed.WithLabelValues(deployment.namespace, "deployment").Inc()
			continue
		}

		log.Info().
			Str("namespace", deployment.namespace).
			Str("deployment", deployment.name).
			Msg("successfully restarted deployment")
		metrics.DeploymentsRestarted.WithLabelValues(deployment.namespace).Inc()

		time.Sleep(time.Duration(upgrader.Settings.RolloutIntervalSecond) * time.Second)
	}

	for _, statefulSet := range upgradedStatefulSets {
		// Check for context cancellation
		if ctx.Err() != nil {
			log.Warn().Msg("context cancelled, stopping restart loop")
			break
		}

		if upgrader.Settings.DryRun {
			log.Info().
				Str("namespace", statefulSet.namespace).
				Str("statefulset", statefulSet.name).
				Msg("[DRY-RUN] would restart statefulset")
			continue
		}

		if err := upgrader.StatefulsetService.RolloutRestart(ctx, statefulSet.namespace, statefulSet.name); err != nil {
			log.Error().
				Err(err).
				Str("namespace", statefulSet.namespace).
				Str("statefulset", statefulSet.name).
				Msg("failed to rollout restart statefulset")
			failedStatefulSets = append(failedStatefulSets, statefulSet)
			metrics.RestartsFailed.WithLabelValues(statefulSet.namespace, "statefulset").Inc()
			continue
		}

		log.Info().
			Str("namespace", statefulSet.namespace).
			Str("statefulset", statefulSet.name).
			Msg("successfully restarted statefulset")
		metrics.StatefulSetsRestarted.WithLabelValues(statefulSet.namespace).Inc()

		time.Sleep(time.Duration(upgrader.Settings.RolloutIntervalSecond) * time.Second)
	}

	return failedDeployments, failedStatefulSets
}
```

**Step 3: Update Upgrade method to skip iteration increment in dry-run**

In Upgrade(), after rollout restart, wrap the iteration increment:
```go
if !upgrader.Settings.DryRun {
	log.Info().Msg("increasing iteration")
	err = upgrader.increaseIteration(ctx, upgradeConfig)
	if err != nil {
		log.Error().Err(err).Msg("failed to increase iteration")
		return err
	}
} else {
	log.Info().Msg("[DRY-RUN] skipping iteration increment")
}
```

**Step 4: Update notification title for dry-run**

Add prefix to notification titles when dry-run is enabled:
```go
titlePrefix := ""
if upgrader.Settings.DryRun {
	titlePrefix = "[DRY-RUN] "
}
```

**Step 5: Verify build passes**

Run: `go build ./...`
Expected: Success

**Step 6: Commit**

```bash
git add services/upgrader/
git commit -m "feat: add dry-run mode with structured logging and metrics"
```

---

## Task 10: Update Helm Chart with New Configuration

**Files:**
- Modify: `charts/istio-upgrade-worker/values.yaml`
- Modify: `charts/istio-upgrade-worker/templates/configmap.yaml`

**Step 1: Update values.yaml**

Add under `configuration:`:
```yaml
  dryRun: false
  logLevel: "info"
  logFormat: "json"
```

**Step 2: Update configmap.yaml**

Add new environment variables:
```yaml
  DRY_RUN: "{{ .Values.configuration.dryRun }}"
  LOG_LEVEL: "{{ .Values.configuration.logLevel }}"
  LOG_FORMAT: "{{ .Values.configuration.logFormat }}"
```

**Step 3: Verify helm template renders correctly**

Run: `helm template test charts/istio-upgrade-worker | grep -A5 DRY_RUN`
Expected: Shows DRY_RUN: "false"

**Step 4: Commit**

```bash
git add charts/istio-upgrade-worker/
git commit -m "feat: add dryRun, logLevel, logFormat to Helm chart"
```

---

## Task 11: Migrate Remaining log.Println to zerolog

**Files:**
- Modify: `services/upgrader/proxy.go` (remaining log statements)
- Modify: `config/deployment_freeze.go`
- Modify: `config/kubernetes.go`

**Step 1: Replace all log.Println/Printf with zerolog equivalents**

Example pattern:
```go
// Before
log.Println("failed to get upgrade config")

// After
logger.Log().Error().Msg("failed to get upgrade config")

// Before
log.Printf("find %d of namespaces\n", len(namespaces))

// After
logger.Log().Info().Int("count", len(namespaces)).Msg("found istio namespaces")
```

**Step 2: Verify build passes**

Run: `go build ./...`
Expected: Success

**Step 3: Run existing tests**

Run: `go test ./... -v`
Expected: All tests pass

**Step 4: Commit**

```bash
git add services/ config/
git commit -m "refactor: migrate all logging to zerolog"
```

---

## Task 12: Final Verification

**Step 1: Run all tests**

Run: `go test ./... -v`
Expected: All tests pass

**Step 2: Build binary**

Run: `make build.binaries`
Expected: Binary created in ./bin/

**Step 3: Verify binary runs with dry-run**

Run: `DRY_RUN=true CLUSTER_NAME=test ./bin/istio-upgrade-worker`
Expected: Should fail fast (no k8s config) but shows structured logs

**Step 4: Generate updated Helm README**

Run: `make readme`

**Step 5: Final commit**

```bash
git add .
git commit -m "docs: update README with new configuration options"
```

---

## Summary

| Task | Description | Files |
|------|-------------|-------|
| 1 | Add zerolog dependency | go.mod |
| 2 | Create logger package | pkg/logger/ |
| 3 | Add new settings + webhook validation | settings/ |
| 4 | Fix namespace deduplication | services/kubernetes/ |
| 5 | Add context to Slack | services/notification/slack/ |
| 6 | Add context to Lark | services/notification/lark/ |
| 7 | Create metrics package | pkg/metrics/ |
| 8 | Add graceful shutdown | main.go |
| 9 | Add dry-run + migrate proxy.go | services/upgrader/ |
| 10 | Update Helm chart | charts/ |
| 11 | Migrate remaining logs | services/, config/ |
| 12 | Final verification | all |
