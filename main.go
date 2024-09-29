package main

import (
	"context"
	"log"

	"github.com/gopaytech/istio-upgrade-worker/config"
	"github.com/gopaytech/istio-upgrade-worker/services/notification"
	"github.com/gopaytech/istio-upgrade-worker/services/upgrader"
	"github.com/gopaytech/istio-upgrade-worker/settings"

	"github.com/gopaytech/istio-upgrade-worker/services/kubernetes"
)

func main() {
	settings, err := settings.NewSettings()
	if err != nil {
		log.Fatal(err)
	}
	log.Println("successfully load the settings")

	kubernetesConfig, err := config.LoadKubernetes()
	if err != nil {
		log.Fatal(err)
	}
	log.Println("successfully load the kubernetes config")

	deploymentFreezeConfig, err := config.LoadDeploymentFreeze(settings)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("successfully load the deployment freeze config")

	notificationService, err := notification.NotificationFactory(settings)
	if err != nil {
		log.Fatal(err)
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

	if err := proxyUpgrader.Upgrade(context.Background()); err != nil {
		log.Println(err)
	}
}
