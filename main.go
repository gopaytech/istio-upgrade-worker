package main

import (
	"context"
	"log"

	"github.com/gopaytech/istio-upgrade-worker/config"
	"github.com/gopaytech/istio-upgrade-worker/services/k8s/statefulset"
	"github.com/gopaytech/istio-upgrade-worker/services/notification"
	"github.com/gopaytech/istio-upgrade-worker/settings"

	"github.com/gopaytech/istio-upgrade-worker/services/slack"
	"github.com/gopaytech/istio-upgrade-worker/usecases/upgrade_proxy"

	k8scm "github.com/gopaytech/istio-upgrade-worker/services/k8s/configmap"
	k8sdeployment "github.com/gopaytech/istio-upgrade-worker/services/k8s/deployment"
	k8snamespace "github.com/gopaytech/istio-upgrade-worker/services/k8s/namespace"
	k8spod "github.com/gopaytech/istio-upgrade-worker/services/k8s/pod"
)

func main() {
	settings, err := settings.NewSettings()
	if err != nil {
		log.Fatal(err)
	}

	_, err = config.LoadKubernetes()
	if err != nil {
		log.Fatal(err)
	}

	_, err = config.LoadDeploymentFreeze(settings)
	if err != nil {
		log.Fatal(err)
	}

	_, err = notification.NotificationFactory(settings)
	if err != nil {
		log.Fatal(err)
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	namespaceService := k8snamespace.New(cfg)
	deploymentService := k8sdeployment.New(cfg)
	statefulsetService := statefulset.New(cfg)
	podService := k8spod.New(cfg)
	configmapService := k8scm.New(cfg)
	slackService := slack.New(cfg.SlackWebhookURL())

	log.Println("worker will be running...")

	upgradeProxyUsecase := upgrade_proxy.New(cfg.ClusterName(),
		cfg.DeploymentFreeze(),
		slackService,
		namespaceService,
		deploymentService,
		statefulsetService,
		configmapService,
		podService)

	if err := upgradeProxyUsecase.Upgrade(context.Background()); err != nil {
		log.Println(err)
	}
	if err := upgradeProxyUsecase.SendNotificationIfAllProxyUpgraded(context.Background()); err != nil {
		log.Println(err)
	}
}
