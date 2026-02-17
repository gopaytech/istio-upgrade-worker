# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

istio-upgrade-worker is a Kubernetes CronJob that automates Istio sidecar proxy upgrades. It performs rolling restarts of Deployments and StatefulSets that have istio-proxy containers running versions older than the target version.

## Build Commands

```bash
# Build binary
make build.binaries

# Build Docker image
make image.build

# Generate Helm chart README from values.yaml
make readme

# Package and index Helm chart
make helm.create.releases
```

## Architecture

### Core Flow (main.go → services/upgrader/proxy.go)

1. Load settings from environment variables (from chart Settings ConfigMap)
2. Load Kubernetes config (`config/kubernetes.go`)
3. Load deployment freeze dates from mounted YAML file (from chart Deployment Freeze ConfigMap)
4. Read upgrade state from **external Storage ConfigMap** (contains: `version`, `cluster_name`, `iteration`, `rollout_restart_date`)
5. Skip if deployment freeze date or weekend (unless configured otherwise)
6. Find namespaces with Istio injection labels
7. Filter Deployments/StatefulSets with istio-proxy version < target version
8. Rollout restart a percentage of workloads based on iteration
9. Send notifications (Slack/Lark) and increment iteration counter in Storage ConfigMap

### ConfigMaps

**Chart-created (via Helm):**
1. **Settings ConfigMap** (`configmap.yaml`): Environment variables for worker configuration (cluster name, rollout interval, iteration settings, notification mode, etc.)
2. **Deployment Freeze ConfigMap** (`configmap-deployment-freeze.yaml`): Mounted as YAML file with dates to skip rollouts

**Externally created (before upgrade):**
3. **Storage ConfigMap** (e.g., `istio-auto-upgrade-config` in `istio-system`): Upgrade state that the worker reads and updates. Required fields: `version`, `cluster_name`, `iteration`, `rollout_restart_date`

### Key Packages

- `services/upgrader/proxy.go`: Main upgrade orchestration logic
- `services/kubernetes/`: K8s clients for Namespace, Deployment, StatefulSet, Pod, ConfigMap operations
- `services/notification/`: Slack and Lark notification implementations with factory pattern
- `settings/settings.go`: Environment variable configuration via `envconfig`

### Phased Rollout Logic

Workloads are rolled out in phases controlled by:
- `MAXIMUM_PERCENTAGE_ROLLOUT_SINGLE_EXECUTION` (default 20%)
- `MAXIMUM_ITERATION` (default 5)

Each CronJob run restarts `(percentage × iteration)%` of eligible workloads, incrementing the iteration counter after each run.

## Helm Chart

Located in `charts/istio-upgrade-worker/`. Key templates:
- `cronjob.yaml`: The main CronJob resource
- `configmap.yaml`: Settings as environment variables
- `configmap-deployment-freeze.yaml`: Deployment freeze dates mounted as YAML file
- ClusterRole grants read access to namespaces/pods/deployments/statefulsets and read/write to configmaps

**Note:** The Storage ConfigMap (upgrade state) must be created externally before the worker can operate. Example:
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: istio-auto-upgrade-config
  namespace: istio-system
data:
  cluster_name: "my-cluster"
  version: "1.20.0"
  iteration: "1"
  rollout_restart_date: "2024-01-15"
```
