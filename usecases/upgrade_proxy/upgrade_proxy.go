package upgrade_proxy

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata"

	"github.com/gopaytech/istio-upgrade-worker/services/k8s/configmap"
	"github.com/gopaytech/istio-upgrade-worker/services/k8s/deployment"
	"github.com/gopaytech/istio-upgrade-worker/services/k8s/namespace"
	"github.com/gopaytech/istio-upgrade-worker/services/k8s/pod"
	"github.com/gopaytech/istio-upgrade-worker/services/k8s/statefulset"
	"github.com/gopaytech/istio-upgrade-worker/services/slack"
	"github.com/hashicorp/go-version"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
)

var (
	ErrIstioVersionNotFound       = errors.New("configmap does not have istio version information")
	ErrRolloutRestartDateNotFound = errors.New("configmap does not have rollout restart date information information")
	ErrIterationDataNotFound      = errors.New("configmap does not have iteration data information")
	ErrInvalidIterationData       = errors.New("invalid iteration data")
)

const (
	defaultConfigMapName      = "istio-upgrade-worker-cm"
	defaultConfigMapNamespace = "istio-system"
	proxyContainerName        = "istio-proxy"
	defaultTimeFormat         = "2006-01-02"
	defaultTimeLocation       = "Asia/Jakarta"
	upperThresholdIteration   = 4
	defaultRolloutInterval    = 30 // In seconds
	defaultDashboardURL       = ""
)

type RolloutRestartStatus string

type UpgradeIstioProxyUsecase interface {
	Upgrade(ctx context.Context) error
	SendNotificationIfAllProxyUpgraded(ctx context.Context) error
}

type UpgradeIstioProxyImpl struct {
	ClusterName        string
	DeploymentFreeze   []time.Time
	SlackService       slack.Service
	NamespaceService   namespace.Service
	DeploymentService  deployment.Service
	StatefulsetService statefulset.Service
	ConfigMapService   configmap.Service
	PodService         pod.Service
}

func New(clusterName string,
	deploymentFreeze []time.Time,
	slackService slack.Service,
	namespaceService namespace.Service,
	deploymentService deployment.Service,
	statefulsetService statefulset.Service,
	configMapService configmap.Service,
	podService pod.Service) UpgradeIstioProxyUsecase {
	return &UpgradeIstioProxyImpl{
		ClusterName:        clusterName,
		DeploymentFreeze:   deploymentFreeze,
		SlackService:       slackService,
		NamespaceService:   namespaceService,
		DeploymentService:  deploymentService,
		StatefulsetService: statefulsetService,
		ConfigMapService:   configMapService,
		PodService:         podService,
	}
}

func (impl *UpgradeIstioProxyImpl) SendNotificationIfAllProxyUpgraded(ctx context.Context) error {
	confmap, err := impl.ConfigMapService.Get(ctx, impl.ConfigmapNamespace(), impl.ConfigmapName())
	if err != nil {
		return err
	}
	upgradeProxyVersion, err := impl.GetUpgradeProxyVersion(confmap)
	if err != nil {
		return err
	}
	istioNamespaces, err := impl.NamespaceService.GetIstioNamespaces(ctx)
	if err != nil {
		log.Println("failed getting istio namespaces: ", err.Error())
		return err
	}
	proxyUpgraded := true
	for _, ns := range istioNamespaces {
		deployments, err := impl.DeploymentService.FindByNamespace(ctx, ns.Name)
		if err != nil {
			log.Println("failed to get deployment by namespace: ", err.Error())
			return err
		}
		deployWithOldProxy, err := impl.FilterDeploymentsByProxyVersion(ctx, ns.Name, deployments, upgradeProxyVersion)
		if err != nil {
			return err
		}
		if len(deployWithOldProxy) > 0 {
			proxyUpgraded = false
			break
		}
		statefulsets, err := impl.StatefulsetService.FindByNamespace(ctx, ns.Name)
		if err != nil {
			return err
		}
		stsWithOldProxy, err := impl.FilterStatefulsetsByProxyVersion(ctx, ns.Name, statefulsets, upgradeProxyVersion)
		if err != nil {
			return err
		}
		if len(stsWithOldProxy) > 0 {
			proxyUpgraded = false
			break
		}
	}
	if proxyUpgraded {
		msg := fmt.Sprintf("All workloads in this cluster have been upgraded to new version %s", upgradeProxyVersion.String())
		attachment := slack.Attachment{}
		attachment.AddField(slack.Field{Title: "Cluster", Value: impl.ClusterName})
		attachment.AddField(slack.Field{Title: "Info", Value: msg})
		payload := slack.Payload{
			Text:        "Istio canary upgrade activity",
			Attachments: []slack.Attachment{attachment},
		}
		errs := impl.SlackService.Send(payload)
		if errs != nil {
			for _, err := range errs {
				log.Println("failed to send slack notification: ", err.Error())
			}
		}
	}
	return nil
}

func (impl *UpgradeIstioProxyImpl) Upgrade(ctx context.Context) error {
	confmap, err := impl.ConfigMapService.Get(ctx, impl.ConfigmapNamespace(), impl.ConfigmapName())
	if err != nil {
		log.Println("failed getting configmap", err.Error())
		return err
	}
	rolloutDate, err := impl.RolloutRestartDate(confmap)
	if err != nil {
		return err
	}
	currentDate := impl.CurrentDate()
	iteration, err := impl.Iteration(confmap)
	if err != nil {
		return err
	}

	if impl.IsWeekend(currentDate) {
		log.Println("Today is a weekend, will skip rollout restart deployment")
		return nil
	}

	if impl.IsDeploymentFreeze(currentDate) {
		log.Println("Today is a deployment freeze, will skip rollout restart deployment")
		return nil
	}

	if (currentDate == rolloutDate || currentDate.After(rolloutDate)) && iteration < upperThresholdIteration {
		log.Println("worker is preparing to gather deployment information")
		istioNamespaces, err := impl.NamespaceService.GetIstioNamespaces(ctx)
		if err != nil {
			log.Println("failed getting istio namespaces: ", err.Error())
			return err
		}
		upgradeProxyVersion, err := impl.GetUpgradeProxyVersion(confmap)
		if err != nil {
			return err
		}
		restartedDeployments, err := impl.RestartDeployments(ctx, istioNamespaces, upgradeProxyVersion, iteration)
		if err != nil {
			return err
		}
		restartedStatefulsets, err := impl.RestartStatefulsets(ctx, istioNamespaces, upgradeProxyVersion, iteration)
		if err != nil {
			log.Println("failed to restart statefulsets: ", err.Error())
		}
		if len(restartedDeployments) > 0 {
			log.Println("sending slack notification....")
			impl.BulkNotify(impl.ClusterName, restartedDeployments, restartedStatefulsets)
		}
		iteration = iteration + 1
		nextIteration := strconv.Itoa(iteration)
		log.Printf("next iteration %s\n", nextIteration)
		confmap.Data["iteration"] = nextIteration
		if err := impl.ConfigMapService.Update(ctx, impl.ConfigmapNamespace(), confmap); err != nil {
			log.Println("failed to update configmap after rollout restart deployment: ", err.Error())
			return err
		}
	}
	return nil
}

func (impl *UpgradeIstioProxyImpl) FilterDeploymentsByProxyVersion(ctx context.Context, ns string, deployments []appsv1.Deployment, upgradeProxyVersion *version.Version) ([]appsv1.Deployment, error) {
	upgradedDeployments := make([]appsv1.Deployment, 0)
	for _, depl := range deployments {
		if *depl.Spec.Replicas > 0 {
			matchLabelPod := depl.Spec.Selector.MatchLabels
			pods, err := impl.PodService.FindByNamespaceAndLabels(ctx, ns, matchLabelPod)
			if err != nil {

				return nil, err
			}
			for _, pod := range pods {
				istioProxyVersion := impl.GetPodProxyVersion(pod)
				if istioProxyVersion != "" {
					currentProxyVersion, err := version.NewVersion(istioProxyVersion)
					if err != nil {
						return nil, err
					}
					if currentProxyVersion.LessThan(upgradeProxyVersion) {
						upgradedDeployments = append(upgradedDeployments, depl)
						break
					}
				}
			}
		}
	}
	return upgradedDeployments, nil
}

func (impl *UpgradeIstioProxyImpl) RestartDeployments(ctx context.Context, istioNamespaces []namespace.Namespace, upgradeProxyVersion *version.Version, iteration int) (map[string][]appsv1.Deployment, error) {
	nsDeployMap := make(map[string][]appsv1.Deployment)
	for _, ns := range istioNamespaces {
		deployments, err := impl.DeploymentService.FindByNamespace(ctx, ns.Name)
		if err != nil {
			log.Println("failed to get deployment by namespace: ", err.Error())
			return nil, err
		}

		upgradedDeployments, err := impl.FilterDeploymentsByProxyVersion(ctx, ns.Name, deployments, upgradeProxyVersion)
		if err != nil {
			return nil, err
		}
		if len(upgradedDeployments) > 0 {
			nsDeployMap[ns.Name] = upgradedDeployments
		}
	}
	restartedDeployments := make(map[string][]appsv1.Deployment)
	for ns, deployments := range nsDeployMap {
		totalRolloutDeploy := impl.GetNumberOfRestartedByIteration(iteration, len(deployments))
		selectedDeployments := make([]appsv1.Deployment, 0)
		for i, deploy := range deployments {
			if i < totalRolloutDeploy {
				deploymentNamespace := deploy.ObjectMeta.Namespace
				deploymentName := deploy.ObjectMeta.Name
				if err := impl.DeploymentService.RolloutRestart(ctx, deploymentNamespace, deploymentName); err != nil {
					log.Printf("failed to rollout restart deployment: %s in namespace: %s reason: %s", deploymentName, deploymentNamespace, err.Error())
					continue
				}
				log.Printf("rolledout deployment: %s in namespace: %s\n", deploymentName, deploymentNamespace)
				log.Printf("waiting for 30s before next deployment\n")
				interval := impl.RolloutInterval()
				time.Sleep(time.Duration(interval) * time.Second)
				selectedDeployments = append(selectedDeployments, deploy)
			}
		}
		if len(selectedDeployments) > 0 {
			restartedDeployments[ns] = selectedDeployments
		}
	}
	return restartedDeployments, nil
}

func (impl *UpgradeIstioProxyImpl) FilterStatefulsetsByProxyVersion(ctx context.Context, ns string, statefulsets []appsv1.StatefulSet, upgradeProxyVersion *version.Version) ([]appsv1.StatefulSet, error) {
	upgradedStatefulsets := make([]appsv1.StatefulSet, 0)
	for _, sts := range statefulsets {
		if *sts.Spec.Replicas > 0 {
			matchLabelPod := sts.Spec.Selector.MatchLabels
			pods, err := impl.PodService.FindByNamespaceAndLabels(ctx, ns, matchLabelPod)
			if err != nil {

				return nil, err
			}
			for _, pod := range pods {
				istioProxyVersion := impl.GetPodProxyVersion(pod)
				if istioProxyVersion != "" {
					currentProxyVersion, err := version.NewVersion(istioProxyVersion)
					if err != nil {
						return nil, err
					}
					if currentProxyVersion.LessThan(upgradeProxyVersion) {
						upgradedStatefulsets = append(upgradedStatefulsets, sts)
						break
					}
				}
			}
		}
	}
	return upgradedStatefulsets, nil
}

func (impl *UpgradeIstioProxyImpl) RestartStatefulsets(ctx context.Context, istioNamespaces []namespace.Namespace, upgradeProxyVersion *version.Version, iteration int) (map[string][]appsv1.StatefulSet, error) {
	nsStsMap := make(map[string][]appsv1.StatefulSet)
	for _, ns := range istioNamespaces {
		statefulsets, err := impl.StatefulsetService.FindByNamespace(ctx, ns.Name)
		if err != nil {
			log.Println("failed to get statefulsets by namespace: ", err.Error())
			return nil, err
		}
		upgradedStatefulsets, err := impl.FilterStatefulsetsByProxyVersion(ctx, ns.Name, statefulsets, upgradeProxyVersion)
		if err != nil {
			return nil, err
		}
		if len(upgradedStatefulsets) > 0 {
			nsStsMap[ns.Name] = upgradedStatefulsets
		}
	}
	restartedStatefulsets := make(map[string][]appsv1.StatefulSet)
	for ns, statefulsets := range nsStsMap {
		totalRolloutStatefulset := impl.GetNumberOfRestartedByIteration(iteration, len(statefulsets))
		selectedStatefulsets := make([]appsv1.StatefulSet, 0)
		for i, sts := range statefulsets {
			if i < totalRolloutStatefulset {
				stsNamespace := sts.ObjectMeta.Namespace
				stsName := sts.ObjectMeta.Name
				if err := impl.StatefulsetService.RolloutRestart(ctx, stsNamespace, stsName); err != nil {
					log.Printf("failed to rollout restart statefulsets: %s in namespace: %s reason: %s", stsName, stsNamespace, err.Error())
					continue
				}
				log.Printf("rolledout statefulset: %s in namespace: %s\n", stsName, stsNamespace)
				log.Printf("waiting for 30s before next statefulset\n")
				interval := impl.RolloutInterval()
				time.Sleep(time.Duration(interval) * time.Second)
				selectedStatefulsets = append(selectedStatefulsets, sts)
			}
		}
		if len(selectedStatefulsets) > 0 {
			restartedStatefulsets[ns] = selectedStatefulsets
		}
	}
	return restartedStatefulsets, nil
}

func (impl *UpgradeIstioProxyImpl) BulkNotify(clusterName string, deployments map[string][]appsv1.Deployment, statefulsets map[string][]appsv1.StatefulSet) []error {
	workloads := make([]string, 0, len(deployments))
	for ns, deploys := range deployments {
		deploymentNames := make([]string, 0, len(deploys))
		for _, deploy := range deploys {
			// Format: deployment-name|namespace
			name := fmt.Sprintf("%s | %s", deploy.Name, ns)
			deploymentNames = append(deploymentNames, name)
		}
		workloads = append(workloads, deploymentNames...)
	}
	for ns, sts := range statefulsets {
		stsNames := make([]string, 0, len(sts))
		for _, st := range sts {
			// Format: statefulset-name|namespace
			name := fmt.Sprintf("%s|%s", st.Name, ns)
			stsNames = append(stsNames, name)
		}
		workloads = append(workloads, stsNames...)
	}

	attachment := slack.Attachment{}
	attachment.AddField(slack.Field{Title: "Cluster", Value: clusterName})
	attachment.AddField(slack.Field{Title: "Action", Value: "Restarted"})
	attachment.AddField(slack.Field{Title: "Workload", Value: strings.Join(workloads, ", ")})
	attachment.AddField(slack.Field{Title: "Total", Value: fmt.Sprintf("%d", len(workloads))})
	attachment.AddAction(slack.Action{Type: "button", Text: "Dashboard", Url: defaultDashboardURL, Style: "primary"})
	payload := slack.Payload{
		Text:        "Istio canary upgrade activity",
		Attachments: []slack.Attachment{attachment},
	}
	errs := impl.SlackService.Send(payload)
	if errs != nil {
		for _, err := range errs {
			log.Println("failed to send slack notification: ", err.Error())
		}
	}
	return errs
}

func (impl *UpgradeIstioProxyImpl) GetNumberOfRestartedByIteration(iteration int, numberOfDeployments int) (total int) {
	var percentage float64
	switch iteration {
	case 0:
		percentage = 0.25
	case 1:
		percentage = 0.5
	case 2:
		percentage = 0.75
	case 3:
		percentage = 1.0
	}
	total = int(math.Ceil(percentage * float64(numberOfDeployments)))
	return
}

func (impl *UpgradeIstioProxyImpl) GetPodProxyVersion(pod v1.Pod) (ver string) {
	containers := pod.Spec.Containers
	for _, container := range containers {
		if container.Name == proxyContainerName {
			containerImage := container.Image
			splitImageNames := strings.Split(containerImage, ":")
			if len(splitImageNames) >= 2 {
				ver = splitImageNames[1]
			}
		}
	}
	return
}

func (impl *UpgradeIstioProxyImpl) ConfigmapName() string {
	cmName := os.Getenv("CONFIGMAP_NAME")
	if cmName != "" {
		return cmName
	}
	return defaultConfigMapName
}

func (impl *UpgradeIstioProxyImpl) ConfigmapNamespace() string {
	cmNamespace := os.Getenv("CONFIGMAP_NAMESPACE")
	if cmNamespace != "" {
		return cmNamespace
	}
	return defaultConfigMapNamespace
}

func (impl *UpgradeIstioProxyImpl) RolloutRestartDate(cm *v1.ConfigMap) (time.Time, error) {
	rolloutRestartDate, ok := cm.Data["rollout_restart_date"]
	if !ok {
		log.Println("failed to get rollout_restart_date in the configmap")
		return time.Time{}, ErrRolloutRestartDateNotFound
	}
	timeFormat := os.Getenv("TIME_FORMAT")
	if timeFormat == "" {
		timeFormat = defaultTimeFormat
	}
	t, err := time.Parse(timeFormat, rolloutRestartDate)
	if err != nil {
		log.Println("failed to parse rollout restart date from configmap: ", err.Error())
		return time.Time{}, err
	}
	return t, nil
}

func (impl *UpgradeIstioProxyImpl) CurrentDate() time.Time {
	timeLoc := os.Getenv("TIME_LOCATION")
	if timeLoc == "" {
		timeLoc = defaultTimeLocation
	}
	loc, err := time.LoadLocation(timeLoc)
	if err == nil {
		t := time.Now()
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
	}
	log.Printf("error while loading time location %v\n", err)
	return time.Now()
}

func (impl *UpgradeIstioProxyImpl) Iteration(cm *v1.ConfigMap) (int, error) {
	iterationStr, ok := cm.Data["iteration"]
	if !ok {
		return -1, ErrIterationDataNotFound
	}
	iteration, err := strconv.Atoi(iterationStr)
	if err != nil {
		return -1, ErrInvalidIterationData
	}
	return iteration, nil
}

func (impl *UpgradeIstioProxyImpl) GetUpgradeProxyVersion(confmap *v1.ConfigMap) (*version.Version, error) {
	istioVersion, ok := confmap.Data["istio_version"]
	if !ok {
		return nil, ErrIstioVersionNotFound
	}
	upgradeProxyVersion, err := version.NewVersion(istioVersion)
	if err != nil {
		log.Println("failed to initialize new semantic version: ", err.Error())
		return nil, err
	}
	return upgradeProxyVersion, nil
}

func (impl *UpgradeIstioProxyImpl) IsDeploymentFreeze(rolloutDate time.Time) bool {
	for _, deploymentFreezeDate := range impl.DeploymentFreeze {
		log.Printf("rollout date %s deploymentFreezeDate %s\n", rolloutDate.String(), deploymentFreezeDate.String())
		if deploymentFreezeDate.Year() == rolloutDate.Year() && deploymentFreezeDate.Month() == rolloutDate.Month() && deploymentFreezeDate.Day() == rolloutDate.Day() {
			return true
		}
	}
	return false
}

func (impl *UpgradeIstioProxyImpl) IsWeekend(d time.Time) bool {
	switch d.Weekday() {
	case time.Saturday, time.Sunday:
		return true
	default:
		return false
	}
}

func (impl *UpgradeIstioProxyImpl) RolloutInterval() int64 {
	intervalStr := os.Getenv("ROLLOUT_INTERVAL")
	if intervalStr == "" {
		log.Printf("ROLLOUT_INTERVAL configuration is empty, returning default rollout interval %d\n", defaultRolloutInterval)
		return defaultRolloutInterval
	}
	interval, err := strconv.ParseInt(intervalStr, 10, 64)
	if err != nil {
		log.Printf("ROLLOUT_INTERVAL configuration is invalid: %s, returning default rollout interval %d\n", intervalStr, defaultRolloutInterval)
		return defaultRolloutInterval
	}
	return interval
}
