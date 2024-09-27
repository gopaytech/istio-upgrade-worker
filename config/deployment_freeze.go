package config

import (
	"os"
	"time"

	_ "time/tzdata"

	"github.com/gopaytech/istio-upgrade-worker/settings"
	"gopkg.in/yaml.v2"
)

type DeploymentFreezeFile struct {
	Dates []string
}

type DeploymentFreezeConfiguration struct {
	Dates []time.Time
}

func LoadDeploymentFreeze(settings settings.Settings) (DeploymentFreezeConfiguration, error) {
	var configFile DeploymentFreezeFile
	var config DeploymentFreezeConfiguration

	yamlFile, err := os.ReadFile(settings.DeploymentFreezeConfigFilePath)
	if err != nil {
		return config, err
	}

	err = yaml.Unmarshal(yamlFile, &configFile)
	if err != nil {
		return config, err
	}

	var deployFreezeDates []time.Time
	for _, dateStr := range configFile.Dates {
		loc, err := time.LoadLocation(settings.TimeLocation)
		if err != nil {
			return config, err
		}

		date, err := time.ParseInLocation(settings.TimeFormat, dateStr, loc)
		if err != nil {
			return config, err
		}

		deployFreezeDates = append(deployFreezeDates, date)
	}

	config.Dates = deployFreezeDates
	return config, nil
}
