package settings

import (
	"github.com/kelseyhightower/envconfig"
)

type DeploymentFreeze struct {
	Dates []string `yaml:"dates"`
}

type Settings struct {
	ClusterName                    string `required:"true" envconfig:"CLUSTER_NAME"`
	DeploymentFreezeConfigFilePath string `required:"true" envconfig:"DEPLOYMENT_FREEZE_CONFIG_FILE_PATH"`
	RolloutIntervalSecond          int    `required:"true" envconfig:"ROLLOUT_INTERVAL_SECOND"`

	TimeLocation string `envconfig:"TIME_LOCATION" default:"Asia/Jakarta"`
	TimeFormat   string `envconfig:"TIME_FORMAT" default:"2006-01-02"`

	StorageMode               string `required:"true" envconfig:"STORAGE_MODE" default:"configmap"`
	StorageConfigMapName      string `envconfig:"STORAGE_CONFIGMAP_NAME" default:"istio-upgrade"`
	StorageConfigMapNameSpace string `envconfig:"STORAGE_CONFIGMAP_NAMESPACE" default:"istio-system"`

	NotificationMode         string `required:"true" envconfig:"NOTIFICATION_MODE" default:"slack"`
	NotificationLarkWebhook  string `envconfig:"NOTIFICATION_LARK_WEBHOOK"`
	NotificationSlackWebhook string `envconfig:"NOTIFICATION_SLACK_WEBHOOK"`

	IstioNamespaceCanaryLabel string `required:"true" envconfig:"ISTIO_NAMESPACE_CANARY_LABEL" default:"istio.io/rev=default"`
	IstioNamespaceLabel       string `required:"true" envconfig:"ISTIO_NAMESPACE_CANARY_LABEL" default:"istio-injection=enabled"`
}

func (s Settings) Validation() error {
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
