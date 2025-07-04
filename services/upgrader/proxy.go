package upgrader

import (
	"context"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/gopaytech/istio-upgrade-worker/config"
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
		log.Println("failed to get upgrade config")
		return err
	}

	currentDate, err := upgrader.currentDate()
	if err != nil {
		log.Println("failed to get current date")
		return err
	}

	if upgrader.Settings.EnableDeploymentFreeze {
		if upgrader.isDeploymentFreeze(currentDate) {
			log.Println("Today is a deployment freeze, will skip rollout restart deployment")
			return nil
		}
	}

	if !upgrader.Settings.EnableRolloutAtWeekend {
		if upgrader.isWeekend(currentDate) {
			log.Println("Today is a weekend, will skip rollout restart deployment")
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
			log.Println("failed to send 1 day upgrade notification")
			return err
		}
	}

	if (DateEqual(currentDate, upgradeConfig.RolloutRestartDate) || currentDate.After(upgradeConfig.RolloutRestartDate)) && upgradeConfig.Iteration <= upgrader.Settings.MaximumIteration {
		log.Println("start the upgrading process")

		log.Println("calculated upgrade istio deployments")
		upgradedDeployments, err := upgrader.calculatedUpgradedIstioDeployments(ctx, upgradeConfig)
		if err != nil {
			log.Println("failed to calculated upgraded istio deployments")
			return err
		}

		log.Println("calculated upgrade istio statefulsets")
		upgradedStatefulSets, err := upgrader.calculatedUpgradedIstioStatefulSets(ctx, upgradeConfig)
		if err != nil {
			log.Println("failed to calculated upgraded istio statefulsets")
			return err
		}

		log.Println("sending the pre upgrade notification")
		preUpgradeNotification := types.Notification{
			Title:   fmt.Sprintf("[Upgrade Notification] Cluster %s Istio service mesh workload will be upgraded in %s seconds to version %s!\n", upgradeConfig.ClusterName, strconv.Itoa(upgrader.Settings.PreUpgradeNotificationSecond), upgradeConfig.Version.String()),
			Message: fmt.Sprintf("%d of deployments and %d of statefulsets across namespaces will be restarted", len(upgradedDeployments), len(upgradedStatefulSets)),
		}

		err = upgrader.NotificationService.Send(ctx, preUpgradeNotification)
		if err != nil {
			log.Println("failed to send upgrade notification")
			return err
		}

		time.Sleep(time.Duration(upgrader.Settings.PreUpgradeNotificationSecond) * time.Second)

		log.Printf("start rollout restarting %d deployments & %d statefulsets\n", len(upgradedDeployments), len(upgradedStatefulSets))
		failedDeployments, failedStatefulSets := upgrader.Restart(ctx, upgradedDeployments, upgradedStatefulSets)

		log.Println("sending the post upgrade notification")
		postUpgradeNotification := types.Notification{
			Title:   fmt.Sprintf("[Upgrade Notification] Phase %d of Cluster %s Istio service mesh workload already upgraded to version %s!\n", upgradeConfig.Iteration, upgradeConfig.ClusterName, upgradeConfig.Version.String()),
			Message: fmt.Sprintf("%d of deployments and %d of statefulsets across namespaces already restarted. while %d of deployments & %d of statefulsets failed to restart", len(upgradedDeployments)-len(failedDeployments), len(upgradedStatefulSets)-len(failedStatefulSets), len(failedDeployments), len(failedStatefulSets)),
		}

		err = upgrader.NotificationService.Send(ctx, postUpgradeNotification)
		if err != nil {
			log.Println("failed to send post upgrade notification")
			return err
		}

		log.Println("increase iteration")
		err = upgrader.increaseIteration(ctx, upgradeConfig)
		if err != nil {
			log.Println("failed to increase the iteration: ", err)
			return err
		}
	} else {
		if currentDate.Before(upgradeConfig.RolloutRestartDate) {
			log.Println("skip rollout restart since it's before the rollout restart date")
		}

		if upgradeConfig.Iteration > upgrader.Settings.MaximumIteration {
			log.Println("skip rollout restart since iteration is already more than maximum iteration configured")

			upgradedDeployments, err := upgrader.calculatedUpgradedIstioDeployments(ctx, upgradeConfig)
			if err != nil {
				log.Println("failed to calculated upgraded istio deployments")
				return err
			}

			upgradedStatefulSets, err := upgrader.calculatedUpgradedIstioStatefulSets(ctx, upgradeConfig)
			if err != nil {
				log.Println("failed to calculated upgraded istio statefulsets")
				return err
			}

			if len(upgradedDeployments)+len(upgradedStatefulSets) > 0 {
				iterationBreachUpgradeNotification := types.Notification{
					Title:   fmt.Sprintf("[Upgrade Notification] Cluster %s Istio service mesh still have old version of istio!\n", upgradeConfig.ClusterName),
					Message: fmt.Sprintf("%d of deployments and %d of statefulsets across namespaces still use old version and cannot be upgraded because already breach the maximum iteration!", len(upgradedDeployments), len(upgradedStatefulSets)),
				}

				err = upgrader.NotificationService.Send(ctx, iterationBreachUpgradeNotification)
				if err != nil {
					log.Println("failed to send iteration breach upgrade notification")
					return err
				}
			}
		}

		log.Println("something is wrong with the codebase")
	}

	return nil
}

func (upgrader *ProxyUpgrader) calculatedUpgradedIstioDeployments(ctx context.Context, upgradeConfig types.UpgradeProxyConfig) ([]DeploymentUpgrade, error) {
	var upgradedDeployments []DeploymentUpgrade

	deployments, err := upgrader.getUpgradedIstioDeployments(ctx, upgradeConfig)
	if err != nil {
		log.Println("failed to get upgraded istio deployment: ", err)
		return upgradedDeployments, err
	}

	totalDeplopymentsIteration := upgrader.getNumberOfRestartedByIteration(upgradeConfig.Iteration, len(deployments))
	log.Printf("%d out of %d deployments will be rollout restarted\n", totalDeplopymentsIteration, len(deployments))

	for iteration, deployment := range deployments {
		if iteration < totalDeplopymentsIteration {
			upgradeDeployment := DeploymentUpgrade{
				namespace: deployment.ObjectMeta.Namespace,
				name:      deployment.ObjectMeta.Name,
			}

			upgradedDeployments = append(upgradedDeployments, upgradeDeployment)
		}
	}
	log.Printf("%d out of %d deployments will be rollout restarted\n", len(upgradedDeployments), len(deployments))

	return upgradedDeployments, nil
}

func (upgrader *ProxyUpgrader) calculatedUpgradedIstioStatefulSets(ctx context.Context, upgradeConfig types.UpgradeProxyConfig) ([]StatefulSetUpgrade, error) {
	var upgradedStatefulSets []StatefulSetUpgrade

	statefulsets, err := upgrader.getUpgradedIstioStatefulSets(ctx, upgradeConfig)
	if err != nil {
		log.Println("failed to get upgraded istio statefulset: ", err)
		return upgradedStatefulSets, err
	}

	totalStatefulSetsIteration := upgrader.getNumberOfRestartedByIteration(upgradeConfig.Iteration, len(statefulsets))
	log.Printf("%d out of %d statefulsets will be rollout restarted\n", totalStatefulSetsIteration, len(statefulsets))

	for iteration, statefulset := range statefulsets {
		if iteration < totalStatefulSetsIteration {
			upgradeStatefulSet := StatefulSetUpgrade{
				namespace: statefulset.ObjectMeta.Namespace,
				name:      statefulset.ObjectMeta.Name,
			}

			upgradedStatefulSets = append(upgradedStatefulSets, upgradeStatefulSet)
		}
	}
	log.Printf("%d out of %d deployments will be rollout restarted\n", len(upgradedStatefulSets), len(statefulsets))

	return upgradedStatefulSets, nil
}

func (upgrader *ProxyUpgrader) Restart(ctx context.Context, upgradedDeployments []DeploymentUpgrade, upgradedStatefulSets []StatefulSetUpgrade) ([]DeploymentUpgrade, []StatefulSetUpgrade) {
	var failedDeployments []DeploymentUpgrade
	var failedStatefulSets []StatefulSetUpgrade

	for _, deployment := range upgradedDeployments {
		if err := upgrader.DeploymentService.RolloutRestart(ctx, deployment.namespace, deployment.name); err != nil {
			log.Printf("failed to rollout restart deployment: %s in namespace: %s reason: %s\n", deployment.namespace, deployment.name, err.Error())
			failedDeployments = append(failedDeployments, deployment)
			continue
		}

		time.Sleep(time.Duration(upgrader.Settings.RolloutIntervalSecond) * time.Second)
	}

	for _, statefulSet := range upgradedStatefulSets {
		if err := upgrader.StatefulsetService.RolloutRestart(ctx, statefulSet.namespace, statefulSet.name); err != nil {
			log.Printf("failed to rollout restart statefulSet: %s in namespace: %s reason: %s\n", statefulSet.namespace, statefulSet.name, err.Error())
			failedStatefulSets = append(failedStatefulSets, statefulSet)
			continue
		}

		time.Sleep(time.Duration(upgrader.Settings.RolloutIntervalSecond) * time.Second)
	}

	return failedDeployments, failedStatefulSets
}

func (upgrader *ProxyUpgrader) getUpgradeConfig(ctx context.Context) (types.UpgradeProxyConfig, error) {
	upgradeConfigMap, err := upgrader.ConfigMapService.Get(ctx, upgrader.Settings.StorageConfigMapNameSpace, upgrader.Settings.StorageConfigMapName)
	if err != nil {
		log.Printf("failed getting configmap %s on namespace %s\n", upgrader.Settings.StorageConfigMapNameSpace, upgrader.Settings.StorageConfigMapName)
		return types.UpgradeProxyConfig{}, err
	}

	configMapClusterName, ok := upgradeConfigMap.Data["cluster_name"]
	if !ok {
		log.Println("failed to get rollout_restart_date in the configmap")
		return types.UpgradeProxyConfig{}, err
	}

	configMapVersion, ok := upgradeConfigMap.Data["version"]
	if !ok {
		log.Println("failed to get rollout_restart_date in the configmap")
		return types.UpgradeProxyConfig{}, err
	}

	configMapIteration, ok := upgradeConfigMap.Data["iteration"]
	if !ok {
		log.Println("failed to get rollout_restart_date in the configmap")
		return types.UpgradeProxyConfig{}, err
	}

	configMapRolloutRestartDate, ok := upgradeConfigMap.Data["rollout_restart_date"]
	if !ok {
		log.Println("failed to get rollout_restart_date in the configmap")
		return types.UpgradeProxyConfig{}, err
	}

	// version
	version, err := version.NewVersion(configMapVersion)
	if err != nil {
		log.Println("failed to initialize new semantic version: ", err.Error())
		return types.UpgradeProxyConfig{}, err
	}

	// config iteration
	iteration, err := strconv.Atoi(configMapIteration)
	if err != nil {
		log.Println("failed to convert iteration configmap to integer")
		return types.UpgradeProxyConfig{}, err
	}

	// rollout restart date
	timeLocation, err := time.LoadLocation(upgrader.Settings.TimeLocation)
	if err != nil {
		log.Printf("error while loading time location %v\n", err)
		return types.UpgradeProxyConfig{}, err
	}

	rolloutRestartDate, err := time.Parse(upgrader.Settings.TimeFormat, configMapRolloutRestartDate)
	if err != nil {
		log.Println("failed to parse rollout restart date from configmap: ", err.Error())
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
		log.Printf("failed getting configmap %s on namespace %s\n", upgrader.Settings.StorageConfigMapNameSpace, upgrader.Settings.StorageConfigMapName)
		return err
	}

	upgradeConfigMap.Data["iteration"] = strconv.Itoa(upgradeConfig.Iteration + 1)
	if err := upgrader.ConfigMapService.Update(ctx, upgradeConfigMap.Namespace, upgradeConfigMap); err != nil {
		log.Println("failed to update configmap after rollout restart deployment & statefulsets: ", err.Error())
		return err
	}

	return nil
}

func (upgrader *ProxyUpgrader) currentDate() (time.Time, error) {
	timeLocation, err := time.LoadLocation(upgrader.Settings.TimeLocation)
	if err != nil {
		log.Printf("error while loading time location %v\n", err)
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
	percentage := float64(upgrader.Settings.MaximumPercentageRolloutInSingleExecution) / 100 * float64(iteration)
	return int(math.Ceil(percentage * float64(total)))
}

func (upgrader *ProxyUpgrader) getUpgradedIstioDeployments(ctx context.Context, upgradeConfig types.UpgradeProxyConfig) ([]appsv1.Deployment, error) {
	namespaces, err := upgrader.NamespaceService.GetIstioNamespaces(ctx)
	if err != nil {
		log.Println("failed getting istio namespaces: ", err.Error())
		return nil, err
	}
	log.Printf("find %d of namespaces is on Istio mesh\n", len(namespaces))

	upgradedDeployments := make([]appsv1.Deployment, 0)
	for _, namespace := range namespaces {
		deployments, err := upgrader.DeploymentService.FindByNamespace(ctx, namespace.Name)
		if err != nil {
			log.Println("failed to get deployments by namespace:", namespace.Name, err.Error())
			return nil, err
		}
		log.Printf("find %d of deployments is on Istio mesh in namespace %s\n", len(deployments), namespace.Name)

		namespaceUpgradedDeployments, err := upgrader.filterDeploymentsByProxyVersion(ctx, namespace.Name, deployments, &upgradeConfig.Version)
		if err != nil {
			log.Println("failed to filter deployments by the currently upgraded proxy version: ", err.Error())
			return nil, err
		}
		log.Printf("find %d of deployments is on Istio mesh in namespace %s can be upgraded\n", len(namespaceUpgradedDeployments), namespace.Name)

		upgradedDeployments = append(upgradedDeployments, namespaceUpgradedDeployments...)
	}

	return upgradedDeployments, nil
}

func (upgrader *ProxyUpgrader) filterDeploymentsByProxyVersion(ctx context.Context, namespace string, deployments []appsv1.Deployment, upgradeProxyVersion *version.Version) ([]appsv1.Deployment, error) {
	upgradedDeployments := make([]appsv1.Deployment, 0)
	for _, deployment := range deployments {
		if *deployment.Spec.Replicas > 0 {
			pods, err := upgrader.PodService.FindByNamespaceAndLabels(ctx, namespace, deployment.Spec.Selector.MatchLabels)
			if err != nil {
				log.Printf("failed to find pods based on namespace and labels on the deployment %s namespace %s: %v\n", deployment.Name, deployment.Namespace, err)
				return nil, err
			}

			for _, pod := range pods {
				istioProxyVersion := upgrader.getPodProxyVersion(pod)
				if istioProxyVersion != "" {
					currentProxyVersion, err := version.NewVersion(istioProxyVersion)
					if err != nil {
						log.Println("failed to find parse istio proxy on deployment", deployment.Name, "namespace", deployment.Namespace, "pod", pod.Name, ":", err.Error())
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
		log.Println("failed getting istio namespaces: ", err.Error())
		return nil, err
	}
	log.Printf("find %d of namespaces is on Istio mesh\n", len(namespaces))

	upgradedStatefulSets := make([]appsv1.StatefulSet, 0)
	for _, namespace := range namespaces {
		statefulsets, err := upgrader.StatefulsetService.FindByNamespace(ctx, namespace.Name)
		if err != nil {
			log.Printf("failed to get deployments by namespace %s: %v\n", namespace.Name, err.Error())
			return nil, err
		}
		log.Printf("find %d of statefulsets is on Istio mesh in namespace %s\n", len(statefulsets), namespace.Name)

		namespaceUpgradedStatefulSets, err := upgrader.filterStatefulSetsByProxyVersion(ctx, namespace.Name, statefulsets, &upgradeConfig.Version)
		if err != nil {
			log.Println("failed to filter deployments by the currently upgraded proxy version: ", err.Error())
			return nil, err
		}
		log.Printf("find %d of statefulsets is on Istio mesh in namespace %s can be upgraded\n", len(namespaceUpgradedStatefulSets), namespace.Name)

		upgradedStatefulSets = append(upgradedStatefulSets, namespaceUpgradedStatefulSets...)
	}

	return upgradedStatefulSets, nil
}

func (upgrader *ProxyUpgrader) filterStatefulSetsByProxyVersion(ctx context.Context, namespace string, statefulsets []appsv1.StatefulSet, upgradeProxyVersion *version.Version) ([]appsv1.StatefulSet, error) {
	upgradedStatefulsets := make([]appsv1.StatefulSet, 0)
	for _, statefulset := range statefulsets {
		if *statefulset.Spec.Replicas > 0 {
			pods, err := upgrader.PodService.FindByNamespaceAndLabels(ctx, namespace, statefulset.Spec.Selector.MatchLabels)
			if err != nil {
				log.Printf("failed to find pods based on namespace and labels on the statefulset %s namespace %s: %v\n", statefulset.Name, statefulset.Namespace, err)
				return nil, err
			}

			for _, pod := range pods {
				istioProxyVersion := upgrader.getPodProxyVersion(pod)
				if istioProxyVersion != "" {
					currentProxyVersion, err := version.NewVersion(istioProxyVersion)
					if err != nil {
						log.Printf("failed to find parse istio proxy on statefulset %s namespace %s pod %s: %v\n", statefulset.Name, statefulset.Namespace, pod.Name, err.Error())
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
			containerImage := container.Image
			splitImageNames := strings.Split(containerImage, ":")
			if len(splitImageNames) >= 2 {
				ver = splitImageNames[1]
			}
		}
	}

	if ver == "" {
		for _, initContainer := range initContainers {
			if initContainer.Name == proxyContainerName {
				initContainerImage := initContainer.Image
				splitImageNames := strings.Split(initContainerImage, ":")
				if len(splitImageNames) >= 2 {
					ver = splitImageNames[1]
				}
			}
		}
	}

	return ver
}

func DateEqual(date1, date2 time.Time) bool {
	return date1.Year() == date2.Year() && date1.Month() == date2.Month() && date1.Day() == date2.Day()
}
