package upgrader

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/gopaytech/istio-upgrade-worker/config"
	"github.com/gopaytech/istio-upgrade-worker/pkg/logger"
	"github.com/gopaytech/istio-upgrade-worker/pkg/metrics"
	"github.com/gopaytech/istio-upgrade-worker/services/kubernetes"
	"github.com/gopaytech/istio-upgrade-worker/services/notification"
	"github.com/gopaytech/istio-upgrade-worker/settings"
	"github.com/gopaytech/istio-upgrade-worker/types"
	"github.com/hashicorp/go-version"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
)

const (
	proxyContainerName = "istio-proxy"
)

type Proxy interface {
	Upgrade(ctx context.Context) error
}

type ProxyUpgrader struct {
	Settings               settings.Settings
	DeploymentFreezeConfig config.DeploymentFreezeConfiguration
	NotificationService    notification.NotificationInterface
	NamespaceService       kubernetes.NamespaceInterface
	DeploymentService      kubernetes.DeploymentInterface
	StatefulsetService     kubernetes.StatefulSetInterface
	ConfigMapService       kubernetes.ConfigMapInterface
	PodService             kubernetes.PodInterface
}

type DeploymentUpgrade struct {
	namespace string
	name      string
}

type StatefulSetUpgrade struct {
	namespace string
	name      string
}

func NewProxyUpgrader(settings settings.Settings, deploymentFreezeConfig config.DeploymentFreezeConfiguration, notificationService notification.NotificationInterface, namespaceService kubernetes.NamespaceInterface, deploymentService kubernetes.DeploymentInterface, statefulsetService kubernetes.StatefulSetInterface, configMapService kubernetes.ConfigMapInterface, podService kubernetes.PodInterface) ProxyUpgrader {
	return ProxyUpgrader{
		Settings:               settings,
		DeploymentFreezeConfig: deploymentFreezeConfig,
		NotificationService:    notificationService,
		NamespaceService:       namespaceService,
		DeploymentService:      deploymentService,
		StatefulsetService:     statefulsetService,
		ConfigMapService:       configMapService,
		PodService:             podService,
	}
}

func (upgrader *ProxyUpgrader) Upgrade(ctx context.Context) error {
	upgradeConfig, err := upgrader.getUpgradeConfig(ctx)
	if err != nil {
		logger.Log().Error().Err(err).Msg("failed to get upgrade config")
		return err
	}

	// Set current iteration metric
	metrics.IterationCurrent.Set(float64(upgradeConfig.Iteration))

	currentDate, err := upgrader.currentDate()
	if err != nil {
		logger.Log().Error().Err(err).Msg("failed to get current date")
		return err
	}

	if upgrader.Settings.EnableDeploymentFreeze {
		if upgrader.isDeploymentFreeze(currentDate) {
			logger.Log().Info().Msg("Today is a deployment freeze, will skip rollout restart deployment")
			metrics.WorkloadsSkipped.WithLabelValues("deployment_freeze").Inc()
			return nil
		}
	}

	if !upgrader.Settings.EnableRolloutAtWeekend {
		if upgrader.isWeekend(currentDate) {
			logger.Log().Info().Msg("Today is a weekend, will skip rollout restart deployment")
			metrics.WorkloadsSkipped.WithLabelValues("weekend").Inc()
			return nil
		}
	}

	if currentDate.AddDate(0, 0, 1).Equal(upgradeConfig.RolloutRestartDate) {
		notification := types.Notification{
			Title:   fmt.Sprintf("[Upgrade Notification] Cluster %s Istio service mesh workload will be upgraded tomorrow to version %s!\n", upgradeConfig.ClusterName, upgradeConfig.Version.String()),
			Message: "Please make sure everything is ready!",
		}

		err := upgrader.NotificationService.Send(ctx, notification)
		if err != nil {
			logger.Log().Error().Err(err).Msg("failed to send 1 day upgrade notification")
			return err
		}
	}

	if (DateEqual(currentDate, upgradeConfig.RolloutRestartDate) || currentDate.After(upgradeConfig.RolloutRestartDate)) && upgradeConfig.Iteration <= upgrader.Settings.MaximumIteration {
		logger.Log().Info().Msg("start the upgrading process")

		logger.Log().Info().Msg("calculated upgrade istio deployments")
		upgradedDeployments, err := upgrader.calculatedUpgradedIstioDeployments(ctx, upgradeConfig)
		if err != nil {
			logger.Log().Error().Err(err).Msg("failed to calculate upgraded istio deployments")
			return err
		}

		logger.Log().Info().Msg("calculated upgrade istio statefulsets")
		upgradedStatefulSets, err := upgrader.calculatedUpgradedIstioStatefulSets(ctx, upgradeConfig)
		if err != nil {
			logger.Log().Error().Err(err).Msg("failed to calculate upgraded istio statefulsets")
			return err
		}

		logger.Log().Info().Msg("sending the pre upgrade notification")
		preUpgradeNotification := types.Notification{
			Title:   fmt.Sprintf("[Upgrade Notification] Cluster %s Istio service mesh workload will be upgraded in %s seconds to version %s!\n", upgradeConfig.ClusterName, strconv.Itoa(upgrader.Settings.PreUpgradeNotificationSecond), upgradeConfig.Version.String()),
			Message: fmt.Sprintf("%d of deployments and %d of statefulsets across namespaces will be restarted", len(upgradedDeployments), len(upgradedStatefulSets)),
		}

		err = upgrader.NotificationService.Send(ctx, preUpgradeNotification)
		if err != nil {
			logger.Log().Error().Err(err).Msg("failed to send pre-upgrade notification")
			return err
		}

		time.Sleep(time.Duration(upgrader.Settings.PreUpgradeNotificationSecond) * time.Second)

		logger.Log().Info().Msgf("start rollout restarting %d deployments & %d statefulsets\n", len(upgradedDeployments), len(upgradedStatefulSets))
		failedDeployments, failedStatefulSets := upgrader.Restart(ctx, upgradedDeployments, upgradedStatefulSets)

		logger.Log().Info().Msg("sending the post upgrade notification")
		postUpgradeNotification := types.Notification{
			Title:   fmt.Sprintf("[Upgrade Notification] Phase %d of Cluster %s Istio service mesh workload already upgraded to version %s!\n", upgradeConfig.Iteration, upgradeConfig.ClusterName, upgradeConfig.Version.String()),
			Message: fmt.Sprintf("%d of deployments and %d of statefulsets across namespaces already restarted. while %d of deployments & %d of statefulsets failed to restart", len(upgradedDeployments)-len(failedDeployments), len(upgradedStatefulSets)-len(failedStatefulSets), len(failedDeployments), len(failedStatefulSets)),
		}

		err = upgrader.NotificationService.Send(ctx, postUpgradeNotification)
		if err != nil {
			logger.Log().Error().Err(err).Msg("failed to send post upgrade notification")
			return err
		}

		if !upgrader.Settings.DryRun {
			logger.Log().Info().Msg("increasing iteration")
			err = upgrader.increaseIteration(ctx, upgradeConfig)
			if err != nil {
				logger.Log().Error().Err(err).Msg("failed to increase iteration")
				return err
			}
		} else {
			logger.Log().Info().Msg("[DRY-RUN] skipping iteration increment")
		}
	} else {
		if currentDate.Before(upgradeConfig.RolloutRestartDate) {
			logger.Log().Info().Msg("skip rollout restart since it's before the rollout restart date")
		}

		if upgradeConfig.Iteration > upgrader.Settings.MaximumIteration {
			logger.Log().Info().Msg("skip rollout restart since iteration is already more than maximum iteration configured")

			upgradedDeployments, err := upgrader.calculatedUpgradedIstioDeployments(ctx, upgradeConfig)
			if err != nil {
				logger.Log().Error().Err(err).Msg("failed to calculate upgraded istio deployments")
				return err
			}

			upgradedStatefulSets, err := upgrader.calculatedUpgradedIstioStatefulSets(ctx, upgradeConfig)
			if err != nil {
				logger.Log().Error().Err(err).Msg("failed to calculate upgraded istio statefulsets")
				return err
			}

			if len(upgradedDeployments)+len(upgradedStatefulSets) > 0 {
				iterationBreachUpgradeNotification := types.Notification{
					Title:   fmt.Sprintf("[Upgrade Notification] Cluster %s Istio service mesh still have old version of istio!\n", upgradeConfig.ClusterName),
					Message: fmt.Sprintf("%d of deployments and %d of statefulsets across namespaces still use old version and cannot be upgraded because already breach the maximum iteration!", len(upgradedDeployments), len(upgradedStatefulSets)),
				}

				err = upgrader.NotificationService.Send(ctx, iterationBreachUpgradeNotification)
				if err != nil {
					logger.Log().Error().Err(err).Msg("failed to send iteration breach upgrade notification")
					return err
				}
			}
		}
	}

	return nil
}

func (upgrader *ProxyUpgrader) calculatedUpgradedIstioDeployments(ctx context.Context, upgradeConfig types.UpgradeProxyConfig) ([]DeploymentUpgrade, error) {
	var upgradedDeployments []DeploymentUpgrade

	deployments, err := upgrader.getUpgradedIstioDeployments(ctx, upgradeConfig)
	if err != nil {
		logger.Log().Error().Err(err).Msg("failed to get upgraded istio deployment")
		return upgradedDeployments, err
	}

	totalDeplopymentsIteration := upgrader.getNumberOfRestartedByIteration(upgradeConfig.Iteration, len(deployments))
	logger.Log().Info().Msgf("%d out of %d deployments will be rollout restarted\n", totalDeplopymentsIteration, len(deployments))

	for iteration, deployment := range deployments {
		if iteration < totalDeplopymentsIteration {
			upgradeDeployment := DeploymentUpgrade{
				namespace: deployment.ObjectMeta.Namespace,
				name:      deployment.ObjectMeta.Name,
			}

			upgradedDeployments = append(upgradedDeployments, upgradeDeployment)
		}
	}
	logger.Log().Info().Msgf("%d out of %d deployments will be rollout restarted\n", len(upgradedDeployments), len(deployments))

	return upgradedDeployments, nil
}

func (upgrader *ProxyUpgrader) calculatedUpgradedIstioStatefulSets(ctx context.Context, upgradeConfig types.UpgradeProxyConfig) ([]StatefulSetUpgrade, error) {
	var upgradedStatefulSets []StatefulSetUpgrade

	statefulsets, err := upgrader.getUpgradedIstioStatefulSets(ctx, upgradeConfig)
	if err != nil {
		logger.Log().Error().Err(err).Msg("failed to get upgraded istio statefulset")
		return upgradedStatefulSets, err
	}

	totalStatefulSetsIteration := upgrader.getNumberOfRestartedByIteration(upgradeConfig.Iteration, len(statefulsets))
	logger.Log().Info().Msgf("%d out of %d statefulsets will be rollout restarted\n", totalStatefulSetsIteration, len(statefulsets))

	for iteration, statefulset := range statefulsets {
		if iteration < totalStatefulSetsIteration {
			upgradeStatefulSet := StatefulSetUpgrade{
				namespace: statefulset.ObjectMeta.Namespace,
				name:      statefulset.ObjectMeta.Name,
			}

			upgradedStatefulSets = append(upgradedStatefulSets, upgradeStatefulSet)
		}
	}
	logger.Log().Info().Msgf("%d out of %d statefulsets will be rollout restarted\n", len(upgradedStatefulSets), len(statefulsets))

	return upgradedStatefulSets, nil
}

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

func (upgrader *ProxyUpgrader) getUpgradeConfig(ctx context.Context) (types.UpgradeProxyConfig, error) {
	upgradeConfigMap, err := upgrader.ConfigMapService.Get(ctx, upgrader.Settings.StorageConfigMapNameSpace, upgrader.Settings.StorageConfigMapName)
	if err != nil {
		logger.Log().Error().Err(err).Str("namespace", upgrader.Settings.StorageConfigMapNameSpace).Str("configmap", upgrader.Settings.StorageConfigMapName).Msg("failed to get configmap")
		return types.UpgradeProxyConfig{}, err
	}

	configMapClusterName, ok := upgradeConfigMap.Data["cluster_name"]
	if !ok {
		logger.Log().Error().Msg("missing cluster_name in configmap")
		return types.UpgradeProxyConfig{}, fmt.Errorf("missing cluster_name in configmap")
	}

	configMapVersion, ok := upgradeConfigMap.Data["version"]
	if !ok {
		logger.Log().Error().Msg("missing version in configmap")
		return types.UpgradeProxyConfig{}, fmt.Errorf("missing version in configmap")
	}

	configMapIteration, ok := upgradeConfigMap.Data["iteration"]
	if !ok {
		logger.Log().Error().Msg("missing iteration in configmap")
		return types.UpgradeProxyConfig{}, fmt.Errorf("missing iteration in configmap")
	}

	configMapRolloutRestartDate, ok := upgradeConfigMap.Data["rollout_restart_date"]
	if !ok {
		logger.Log().Error().Msg("missing rollout_restart_date in configmap")
		return types.UpgradeProxyConfig{}, fmt.Errorf("missing rollout_restart_date in configmap")
	}

	// version
	version, err := version.NewVersion(configMapVersion)
	if err != nil {
		logger.Log().Error().Err(err).Msg("failed to initialize new semantic version")
		return types.UpgradeProxyConfig{}, err
	}

	// config iteration
	iteration, err := strconv.Atoi(configMapIteration)
	if err != nil {
		logger.Log().Error().Err(err).Str("iteration", configMapIteration).Msg("failed to convert iteration to integer")
		return types.UpgradeProxyConfig{}, err
	}

	// rollout restart date
	timeLocation, err := time.LoadLocation(upgrader.Settings.TimeLocation)
	if err != nil {
		logger.Log().Error().Err(err).Str("location", upgrader.Settings.TimeLocation).Msg("failed to load time location")
		return types.UpgradeProxyConfig{}, err
	}

	rolloutRestartDate, err := time.Parse(upgrader.Settings.TimeFormat, configMapRolloutRestartDate)
	if err != nil {
		logger.Log().Error().Err(err).Msg("failed to parse rollout restart date from configmap")
		return types.UpgradeProxyConfig{}, err
	}

	return types.UpgradeProxyConfig{
		Version:            *version,
		ClusterName:        configMapClusterName,
		Iteration:          iteration,
		RolloutRestartDate: time.Date(rolloutRestartDate.Year(), rolloutRestartDate.Month(), rolloutRestartDate.Day(), 0, 0, 0, 0, timeLocation),
	}, nil
}

func (upgrader *ProxyUpgrader) increaseIteration(ctx context.Context, upgradeConfig types.UpgradeProxyConfig) error {
	upgradeConfigMap, err := upgrader.ConfigMapService.Get(ctx, upgrader.Settings.StorageConfigMapNameSpace, upgrader.Settings.StorageConfigMapName)
	if err != nil {
		logger.Log().Error().Err(err).Str("namespace", upgrader.Settings.StorageConfigMapNameSpace).Str("configmap", upgrader.Settings.StorageConfigMapName).Msg("failed to get configmap")
		return err
	}

	upgradeConfigMap.Data["iteration"] = strconv.Itoa(upgradeConfig.Iteration + 1)
	if err := upgrader.ConfigMapService.Update(ctx, upgradeConfigMap.Namespace, upgradeConfigMap); err != nil {
		logger.Log().Error().Err(err).Msg("failed to update configmap after rollout restart")
		return err
	}

	return nil
}

func (upgrader *ProxyUpgrader) currentDate() (time.Time, error) {
	timeLocation, err := time.LoadLocation(upgrader.Settings.TimeLocation)
	if err != nil {
		logger.Log().Error().Err(err).Str("location", upgrader.Settings.TimeLocation).Msg("failed to load time location")
		return time.Now(), err
	}

	currentTime := time.Now()
	return time.Date(currentTime.Year(), currentTime.Month(), currentTime.Day(), 0, 0, 0, 0, timeLocation), nil
}

func (upgrader *ProxyUpgrader) isWeekend(currentTime time.Time) bool {
	switch currentTime.Weekday() {
	case time.Saturday, time.Sunday:
		return true
	default:
		return false
	}
}

func (upgrader *ProxyUpgrader) isDeploymentFreeze(currentTime time.Time) bool {
	for _, deploymentFreezeDate := range upgrader.DeploymentFreezeConfig.Dates {
		if deploymentFreezeDate.Year() == currentTime.Year() && deploymentFreezeDate.Month() == currentTime.Month() && deploymentFreezeDate.Day() == currentTime.Day() {
			return true
		}
	}
	return false
}

func (upgrader *ProxyUpgrader) getNumberOfRestartedByIteration(iteration, total int) (restarted int) {
	percentage := float64(upgrader.Settings.MaximumPercentageRolloutInSingleExecution) / 100
	return int(math.Ceil(percentage * float64(total)))
}

func (upgrader *ProxyUpgrader) getUpgradedIstioDeployments(ctx context.Context, upgradeConfig types.UpgradeProxyConfig) ([]appsv1.Deployment, error) {
	namespaces, err := upgrader.NamespaceService.GetIstioNamespaces(ctx)
	if err != nil {
		logger.Log().Error().Err(err).Msg("failed getting istio namespaces")
		return nil, err
	}
	logger.Log().Info().Msgf("find %d of namespaces is on Istio mesh\n", len(namespaces))

	upgradedDeployments := make([]appsv1.Deployment, 0)
	for _, namespace := range namespaces {
		deployments, err := upgrader.DeploymentService.FindByNamespace(ctx, namespace.Name)
		if err != nil {
			logger.Log().Error().Err(err).Str("namespace", namespace.Name).Msg("failed to get deployments by namespace")
			return nil, err
		}
		logger.Log().Info().Msgf("find %d of deployments is on Istio mesh in namespace %s\n", len(deployments), namespace.Name)

		namespaceUpgradedDeployments, err := upgrader.filterDeploymentsByProxyVersion(ctx, namespace.Name, deployments, &upgradeConfig.Version)
		if err != nil {
			logger.Log().Error().Err(err).Msg("failed to filter deployments by proxy version")
			return nil, err
		}
		logger.Log().Info().Msgf("find %d of deployments is on Istio mesh in namespace %s can be upgraded\n", len(namespaceUpgradedDeployments), namespace.Name)

		upgradedDeployments = append(upgradedDeployments, namespaceUpgradedDeployments...)
	}

	return upgradedDeployments, nil
}

func (upgrader *ProxyUpgrader) filterDeploymentsByProxyVersion(ctx context.Context, namespace string, deployments []appsv1.Deployment, upgradeProxyVersion *version.Version) ([]appsv1.Deployment, error) {
	upgradedDeployments := make([]appsv1.Deployment, 0)
	for _, deployment := range deployments {
		// Replicas defaults to 1 if nil
		if deployment.Spec.Replicas == nil || *deployment.Spec.Replicas > 0 {
			pods, err := upgrader.PodService.FindByNamespaceAndLabels(ctx, namespace, deployment.Spec.Selector.MatchLabels)
			if err != nil {
				logger.Log().Error().Err(err).Str("deployment", deployment.Name).Str("namespace", deployment.Namespace).Msg("failed to find pods for deployment")
				return nil, err
			}

			for _, pod := range pods {
				istioProxyVersion := upgrader.getPodProxyVersion(pod)
				if istioProxyVersion != "" {
					currentProxyVersion, err := version.NewVersion(istioProxyVersion)
					if err != nil {
						logger.Log().Error().Err(err).Str("deployment", deployment.Name).Str("namespace", deployment.Namespace).Str("pod", pod.Name).Msg("failed to parse istio proxy version")
						return nil, err
					}

					if currentProxyVersion.LessThan(upgradeProxyVersion) {
						upgradedDeployments = append(upgradedDeployments, deployment)
						break
					}
				}
			}
		}
	}
	return upgradedDeployments, nil
}

func (upgrader *ProxyUpgrader) getUpgradedIstioStatefulSets(ctx context.Context, upgradeConfig types.UpgradeProxyConfig) ([]appsv1.StatefulSet, error) {
	namespaces, err := upgrader.NamespaceService.GetIstioNamespaces(ctx)
	if err != nil {
		logger.Log().Error().Err(err).Msg("failed getting istio namespaces")
		return nil, err
	}
	logger.Log().Info().Msgf("find %d of namespaces is on Istio mesh\n", len(namespaces))

	upgradedStatefulSets := make([]appsv1.StatefulSet, 0)
	for _, namespace := range namespaces {
		statefulsets, err := upgrader.StatefulsetService.FindByNamespace(ctx, namespace.Name)
		if err != nil {
			logger.Log().Error().Err(err).Str("namespace", namespace.Name).Msg("failed to get statefulsets by namespace")
			return nil, err
		}
		logger.Log().Info().Msgf("find %d of statefulsets is on Istio mesh in namespace %s\n", len(statefulsets), namespace.Name)

		namespaceUpgradedStatefulSets, err := upgrader.filterStatefulSetsByProxyVersion(ctx, namespace.Name, statefulsets, &upgradeConfig.Version)
		if err != nil {
			logger.Log().Error().Err(err).Msg("failed to filter deployments by proxy version")
			return nil, err
		}
		logger.Log().Info().Msgf("find %d of statefulsets is on Istio mesh in namespace %s can be upgraded\n", len(namespaceUpgradedStatefulSets), namespace.Name)

		upgradedStatefulSets = append(upgradedStatefulSets, namespaceUpgradedStatefulSets...)
	}

	return upgradedStatefulSets, nil
}

func (upgrader *ProxyUpgrader) filterStatefulSetsByProxyVersion(ctx context.Context, namespace string, statefulsets []appsv1.StatefulSet, upgradeProxyVersion *version.Version) ([]appsv1.StatefulSet, error) {
	upgradedStatefulsets := make([]appsv1.StatefulSet, 0)
	for _, statefulset := range statefulsets {
		// Replicas defaults to 1 if nil
		if statefulset.Spec.Replicas == nil || *statefulset.Spec.Replicas > 0 {
			pods, err := upgrader.PodService.FindByNamespaceAndLabels(ctx, namespace, statefulset.Spec.Selector.MatchLabels)
			if err != nil {
				logger.Log().Error().Err(err).Str("statefulset", statefulset.Name).Str("namespace", statefulset.Namespace).Msg("failed to find pods for statefulset")
				return nil, err
			}

			for _, pod := range pods {
				istioProxyVersion := upgrader.getPodProxyVersion(pod)
				if istioProxyVersion != "" {
					currentProxyVersion, err := version.NewVersion(istioProxyVersion)
					if err != nil {
						logger.Log().Error().Err(err).Str("statefulset", statefulset.Name).Str("namespace", statefulset.Namespace).Str("pod", pod.Name).Msg("failed to parse istio proxy version")
						return nil, err
					}

					if currentProxyVersion.LessThan(upgradeProxyVersion) {
						upgradedStatefulsets = append(upgradedStatefulsets, statefulset)
						break
					}
				}
			}
		}
	}
	return upgradedStatefulsets, nil
}

func (upgrader *ProxyUpgrader) getPodProxyVersion(pod v1.Pod) (ver string) {
	containers := pod.Spec.Containers
	initContainers := pod.Spec.InitContainers

	for _, container := range containers {
		if container.Name == proxyContainerName {
			ver = extractImageTag(container.Image)
			if ver != "" {
				return ver
			}
		}
	}

	for _, initContainer := range initContainers {
		if initContainer.Name == proxyContainerName {
			ver = extractImageTag(initContainer.Image)
			if ver != "" {
				return ver
			}
		}
	}

	return ver
}

// extractImageTag extracts the tag from a container image reference.
// Handles formats like:
//   - image:tag
//   - registry/image:tag
//   - registry:port/image:tag
//   - image@sha256:digest (returns empty string for digest format)
func extractImageTag(image string) string {
	// Skip digest format (image@sha256:...)
	if strings.Contains(image, "@") {
		return ""
	}

	// Find the last colon - the tag is after it
	// But we need to ensure it's not a port in the registry (e.g., registry:5000/image)
	lastColon := strings.LastIndex(image, ":")
	if lastColon == -1 {
		return ""
	}

	// Check if there's a slash after the last colon (meaning it's a port, not a tag)
	afterColon := image[lastColon+1:]
	if strings.Contains(afterColon, "/") {
		return ""
	}

	return afterColon
}

func DateEqual(date1, date2 time.Time) bool {
	return date1.Year() == date2.Year() && date1.Month() == date2.Month() && date1.Day() == date2.Day()
}
