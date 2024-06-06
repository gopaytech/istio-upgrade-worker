package config

import (
	"errors"
	"log"
	"os"
	"strings"
	"time"

	_ "time/tzdata"

	"gopkg.in/yaml.v2"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var (
	ErrWebhookURLNotFound  = errors.New("slack webhook url not found")
	ErrClusterNameNotFound = errors.New("cluster name not found")
)

const (
	defaultTimeLocation = "Asia/Jakarta"
	defaultTimeFormat   = "2006-01-02"
)

type DeploymentFreeze struct {
	Dates []string `yaml:"dates"`
}

type Conf struct {
	k8sClient         *kubernetes.Clientset
	deployFreezeDates []time.Time
	webhookURL        string
	clusterName       string
}

func Load() (*Conf, error) {
	inClusterConf, err := rest.InClusterConfig()
	if err != nil {
		log.Print("failed when get in-cluster config")
		return nil, err
	}
	k8sClient, err := kubernetes.NewForConfig(inClusterConf)
	if err != nil {
		return nil, err
	}
	deploymentFreeze, err := loadDeploymentFreeze()
	if err != nil {
		return nil, err
	}
	clusterName := os.Getenv("CLUSTER_NAME")
	if clusterName == "" {
		return nil, ErrClusterNameNotFound
	}
	webhookURL := os.Getenv("ISTIO_CANARY_UPGRADE_WEBHOOK_URL")
	if webhookURL == "" {
		return nil, ErrWebhookURLNotFound
	}
	conf := Conf{
		webhookURL:        strings.TrimSpace(webhookURL),
		k8sClient:         k8sClient,
		deployFreezeDates: deploymentFreeze,
		clusterName:       clusterName,
	}
	return &conf, nil
}

func (c *Conf) K8SClient() *kubernetes.Clientset {
	return c.k8sClient
}

func (c *Conf) DeploymentFreeze() []time.Time {
	return c.deployFreezeDates
}

func (c *Conf) SlackWebhookURL() string {
	return c.webhookURL
}
func (c *Conf) ClusterName() string {
	return c.clusterName
}

func loadDeploymentFreeze() ([]time.Time, error) {
	var deploymentFreeze DeploymentFreeze
	yamlFile, err := os.ReadFile(os.Getenv("DEPLOYMENT_FREEZE_CONFIG"))
	if err != nil {
		return nil, err
	}
	err = yaml.Unmarshal(yamlFile, &deploymentFreeze)
	if err != nil {
		return nil, err
	}
	var deployFreezeDates []time.Time
	timeLocation := os.Getenv("TIME_LOCATION")
	if timeLocation == "" {
		timeLocation = defaultTimeLocation
	}
	timeFormat := os.Getenv("TIME_FORMAT")
	if timeFormat == "" {
		timeFormat = defaultTimeFormat
	}
	for _, dateStr := range deploymentFreeze.Dates {
		loc, err := time.LoadLocation(timeLocation)
		if err != nil {
			return nil, err
		}
		date, err := time.ParseInLocation(timeFormat, dateStr, loc)
		if err != nil {
			return nil, err
		}
		deployFreezeDates = append(deployFreezeDates, date)
	}
	log.Printf("deployFreezeDates %+v\n", deployFreezeDates)
	return deployFreezeDates, nil
}
