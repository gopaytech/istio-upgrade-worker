package settings

import (
	"errors"

	"github.com/kelseyhightower/envconfig"
)

var (
	ErrTotalRolloutLessThan100Percentage = errors.New("total services that will be rollouted is less then 100%, please increase environment variable for MAXIMUM_PERCENTAGE_ROLLOUT_SINGLE_EXECUTION or MAXIMUM_ITERATION")
)

type Settings struct {
	ClusterName                               string `required:"true" envconfig:"CLUSTER_NAME"`
	RolloutIntervalSecond                     int    `required:"true" envconfig:"ROLLOUT_INTERVAL_SECOND" default:"30"`
	MaximumPercentageRolloutInSingleExecution int    `required:"true" envconfig:"MAXIMUM_PERCENTAGE_ROLLOUT_SINGLE_EXECUTION" default:"20"`
	MaximumIteration                          int    `required:"true" envconfig:"MAXIMUM_ITERATION" default:"5"`
	PreUpgradeNotificationSecond              int    `required:"true" envconfig:"PRE_UPGRADE_NOTIFICATION_SECOND" default:"600"`

	EnableDeploymentFreeze         bool   `required:"true" envconfig:"ENABLE_DEPLOYMENT_FREEZE" default:"true"`
	DeploymentFreezeConfigFilePath string `required:"true" envconfig:"DEPLOYMENT_FREEZE_CONFIG_FILE_PATH"`
	EnableRolloutAtWeekend         bool   `required:"true" envconfig:"ENABLE_ROLLOUT_AT_WEEKEND" default:"false"`

	TimeLocation string `envconfig:"TIME_LOCATION" default:"Asia/Jakarta"`
	TimeFormat   string `envconfig:"TIME_FORMAT" default:"2006-01-02"`

	StorageMode               string `required:"true" envconfig:"STORAGE_MODE" default:"configmap"`
	StorageConfigMapName      string `envconfig:"STORAGE_CONFIGMAP_NAME" default:"istio-upgrade"`
	StorageConfigMapNameSpace string `envconfig:"STORAGE_CONFIGMAP_NAMESPACE" default:"istio-system"`

	NotificationMode         string `required:"true" envconfig:"NOTIFICATION_MODE" default:"slack"`
	NotificationLarkWebhook  string `envconfig:"NOTIFICATION_LARK_WEBHOOK"`
	NotificationSlackWebhook string `envconfig:"NOTIFICATION_SLACK_WEBHOOK"`

	IstioNamespaceCanaryLabel string `required:"true" envconfig:"ISTIO_NAMESPACE_CANARY_LABEL" default:"istio.io/rev=default"`
	IstioNamespaceLabel       string `required:"true" envconfig:"ISTIO_NAMESPACE_LABEL" default:"istio-injection=enabled"`
}

func (s Settings) Validation() error {
	if s.MaximumPercentageRolloutInSingleExecution*s.MaximumIteration < 100 {
		return ErrTotalRolloutLessThan100Percentage
	}

	return nil
}

func NewSettings() (Settings, error) {
	var settings Settings

	err := envconfig.Process("", &settings)
	if err != nil {
		return settings, err
	}

	if settings.Validation() != nil {
		return settings, err
	}

	return settings, nil
}
