# istio-upgrade-worker

![Version: 1.0.0](https://img.shields.io/badge/Version-1.0.0-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: 1.0.0](https://img.shields.io/badge/AppVersion-1.0.0-informational?style=flat-square) [![made with Go](https://img.shields.io/badge/made%20with-Go-brightgreen)](http://golang.org) [![Github main branch build](https://img.shields.io/github/workflow/status/gopaytech/istio-upgrade-worker/Main)](https://github.com/gopaytech/istio-upgrade-worker/actions/workflows/main.yml) [![GitHub issues](https://img.shields.io/github/issues/gopaytech/istio-upgrade-worker)](https://github.com/gopaytech/istio-upgrade-worker/issues) [![GitHub pull requests](https://img.shields.io/github/issues-pr/gopaytech/istio-upgrade-worker)](https://github.com/gopaytech/istio-upgrade-worker/pulls)[![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/istio-upgrade-worker)](https://artifacthub.io/packages/search?repo=istio-upgrade-worker)

## Installing

To install the chart with the release name `my-release`:

```console
helm repo add istio-upgrade-worker https://gopaytech.github.io/istio-upgrade-worker/charts/releases/
helm install my-istio-upgrade-worker istio-upgrade-worker/istio-upgrade-worker --values values.yaml
```

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| configuration.clusterName | string | `"my-cluster"` |  |
| configuration.deploymentFreezeDates[0] | string | `"2024-01-01"` |  |
| configuration.enableDeploymentFreeze | bool | `true` |  |
| configuration.enableRolloutAtWeekend | bool | `false` |  |
| configuration.istioNamespaceCanaryLabel | string | `"istio.io/rev=default"` |  |
| configuration.istioNamespaceLabel | string | `"istio-injection=enabled"` |  |
| configuration.maximumIteration | int | `5` |  |
| configuration.maximumPercentageRolloutInSingleExecution | int | `20` |  |
| configuration.notificationMode | string | `"slack"` |  |
| configuration.preUpgradeNotificationSecond | int | `600` |  |
| configuration.rolloutIntervalSecond | int | `30` |  |
| configuration.slackWebhookSecretName | string | `"slack-webhook-secret"` |  |
| configuration.storageConfigMapName | string | `"istio-auto-upgrade-config"` |  |
| configuration.storageConfigMapNameSpace | string | `"istio-system"` |  |
| configuration.storageMode | string | `"configmap"` |  |
| configuration.timeFormat | string | `"2006-01-02"` |  |
| configuration.timeLocation | string | `"Asia/Jakarta"` |  |
| cronjob.image | string | `"ghcr.io/gopaytech/istio-upgrade-worker"` |  |
| cronjob.schedule | string | `"32 5 * * *"` |  |
| cronjob.tag | string | `"master"` |  |
| cronjob.timeZone | string | `"Asia/Jakarta"` |  |
| podLabels | object | `{}` |  |
| resources.limits.cpu | string | `"200m"` |  |
| resources.limits.memory | string | `"100Mi"` |  |
| resources.requests.cpu | string | `"100m"` |  |
| resources.requests.memory | string | `"20Mi"` |  |
| serviceAccount.automountServiceAccountToken | bool | `true` |  |
| serviceAccount.imagePullSecrets | list | `[]` |  |

