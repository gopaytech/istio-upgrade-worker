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
	appSettings, err := settings.NewSettings()
	if err != nil {
		panic(err)
	}

	// Initialize logger
	logger.Init(appSettings.LogLevel, appSettings.LogFormat)
	log := logger.Log()

	log.Info().Msg("successfully loaded settings")

	// Initialize metrics
	metrics.Init(appSettings.ClusterName)

	kubernetesConfig, err := config.LoadKubernetes()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load kubernetes config")
	}
	log.Info().Msg("successfully loaded kubernetes config")

	deploymentFreezeConfig, err := config.LoadDeploymentFreeze(appSettings)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load deployment freeze config")
	}
	log.Info().Msg("successfully loaded deployment freeze config")

	notificationService, err := notification.NotificationFactory(appSettings)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create notification service")
	}

	namespaceService := kubernetes.NewNamespaceService(kubernetesConfig, appSettings)
	deploymentService := kubernetes.NewDeploymentService(kubernetesConfig)
	statefulsetService := kubernetes.NewStatefulSetService(kubernetesConfig)
	podService := kubernetes.NewPodService(kubernetesConfig)
	configmapService := kubernetes.NewConfigMapService(kubernetesConfig)

	proxyUpgrader := upgrader.NewProxyUpgrader(
		appSettings,
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
