package types

import (
	"time"

	"github.com/hashicorp/go-version"
)

type UpgradeProxyConfig struct {
	Version            version.Version
	ClusterName        string
	Iteration          int
	RolloutRestartDate time.Time
}
