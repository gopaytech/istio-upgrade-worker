# istio-upgrade-worker Improvements Design

**Date:** 2026-02-17
**Status:** Approved

## Overview

Comprehensive improvements to istio-upgrade-worker covering bug fixes, dry-run mode, structured logging, Prometheus metrics, and graceful shutdown.

## 1. Bug Fixes

### 1.1 Duplicate Namespace Deduplication

**Problem:** If a namespace has both `istio-injection=enabled` AND `istio.io/rev=default` labels, it appears twice in the namespace list, causing deployments to be processed twice.

**Solution:** Add a `seen` map in `GetIstioNamespaces()` to deduplicate by namespace name.

**File:** `services/kubernetes/namespace.go`

### 1.2 Webhook URL Validation

**Problem:** App starts with `notificationMode=slack` but empty `NOTIFICATION_SLACK_WEBHOOK`, fails at runtime when sending notification.

**Solution:** Add validation in `settings.Validation()`:
- If `NotificationMode == "slack"` → require `NotificationSlackWebhook` non-empty
- If `NotificationMode == "lark"` → require `NotificationLarkWebhook` non-empty

**File:** `settings/settings.go`

### 1.3 Context Usage in Notifications

**Problem:** Slack/Lark `Send()` accepts `ctx` but doesn't use it for timeouts/cancellation.

**Solution:**
- Slack: Use `slack.PostWebhookContext(ctx, ...)` instead of `PostWebhook`
- Lark: Wrap with goroutine and select on context cancellation

**Files:** `services/notification/slack/slack.go`, `services/notification/lark/lark.go`

## 2. Dry-Run Mode

### 2.1 New Setting

```go
DryRun bool `envconfig:"DRY_RUN" default:"false"`
```

### 2.2 Behavior

When `DRY_RUN=true`:
- Log all workloads that would be restarted
- Skip actual `RolloutRestart()` calls
- Skip iteration increment
- Send notifications with `[DRY-RUN]` prefix

### 2.3 Implementation

- Add check in `Restart()` before each `RolloutRestart` call
- Log: `[DRY-RUN] Would restart deployment: namespace/name`

### 2.4 Helm Chart

- Add `configuration.dryRun: false` to values.yaml
- Add `DRY_RUN` to configmap.yaml

## 3. Structured Logging (zerolog)

### 3.1 Dependencies

```go
github.com/rs/zerolog
```

### 3.2 Logger Initialization

Create `pkg/logger/logger.go`:
- JSON output (default)
- Pretty console if `LOG_FORMAT=console`
- Log level via `LOG_LEVEL` (default: `info`)

### 3.3 New Settings

```go
LogLevel  string `envconfig:"LOG_LEVEL" default:"info"`
LogFormat string `envconfig:"LOG_FORMAT" default:"json"`
```

### 3.4 Migration

Replace all `log.Println`/`log.Printf` with zerolog equivalents with contextual fields.

## 4. Prometheus Metrics

### 4.1 Dependencies

```go
github.com/prometheus/client_golang/prometheus
github.com/prometheus/client_golang/prometheus/promauto
```

### 4.2 Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `istio_upgrade_deployments_restarted_total` | Counter | `namespace`, `cluster` | Deployments successfully restarted |
| `istio_upgrade_statefulsets_restarted_total` | Counter | `namespace`, `cluster` | StatefulSets successfully restarted |
| `istio_upgrade_restarts_failed_total` | Counter | `namespace`, `cluster`, `kind` | Failed restart attempts |
| `istio_upgrade_workloads_skipped_total` | Counter | `cluster`, `reason` | Skipped workloads |
| `istio_upgrade_iteration_current` | Gauge | `cluster` | Current iteration |

### 4.3 Exposure

Log metrics as JSON at end of run. Pushgateway support can be added later.

**File:** `pkg/metrics/metrics.go`

## 5. Graceful Shutdown

### 5.1 Implementation

- Add signal handler in `main.go` for `SIGTERM` and `SIGINT`
- Create cancellable context passed through call chain
- On signal: log, cancel context, exit cleanly

### 5.2 Behavior

- Current Kubernetes API call completes (atomic)
- Loop exits before next restart
- Iteration NOT incremented (next run resumes)
- Exit code 0

### 5.3 Context Propagation

Check `ctx.Err()` between rollout restarts. Return early with `context.Canceled` if cancelled.

## New Files

- `pkg/logger/logger.go` - zerolog initialization
- `pkg/metrics/metrics.go` - Prometheus metric definitions

## Modified Files

- `go.mod` - add zerolog, prometheus dependencies
- `main.go` - logger init, signal handler, cancellable context
- `settings/settings.go` - new settings, webhook validation
- `services/kubernetes/namespace.go` - deduplication
- `services/notification/slack/slack.go` - context usage
- `services/notification/lark/lark.go` - context usage
- `services/upgrader/proxy.go` - dry-run, metrics, structured logging, context checks
- `charts/istio-upgrade-worker/values.yaml` - new config options
- `charts/istio-upgrade-worker/templates/configmap.yaml` - new env vars
