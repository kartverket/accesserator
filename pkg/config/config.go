package config

import (
	"fmt"
	"strings"

	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	RunsInProduction *bool `split_words:"true"`

	ClusterName string `split_words:"true"`

	TokenxName      string `split_words:"true" default:"tokendings"`
	TokenxNamespace string `split_words:"true"`

	TexasImageName     string `split_words:"true" default:"ghcr.io/nais/texas"`
	TexasImageTag      string `split_words:"true"`
	TexasPort          int32  `split_words:"true" default:"3000"`
	TexasUrlEnvVarName string `split_words:"true" default:"TEXAS_URL"`
}

var appCfg Config

func Load() error {
	cfg := Config{}
	if err := envconfig.Process("accesserator", &cfg); err != nil {
		return err
	}

	missing := make([]string, 0, 4)
	if cfg.RunsInProduction == nil {
		missing = append(missing, "ACCESSERATOR_RUNS_IN_PRODUCTION")
	}
	if cfg.ClusterName == "" {
		missing = append(missing, "ACCESSERATOR_CLUSTER_NAME")
	}
	if cfg.TokenxNamespace == "" {
		missing = append(missing, "ACCESSERATOR_TOKENX_NAMESPACE")
	}
	if cfg.TexasImageTag == "" {
		missing = append(missing, "ACCESSERATOR_TEXAS_IMAGE_TAG")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required config: %s", strings.Join(missing, ", "))
	}
	appCfg = cfg
	return nil
}

func Get() Config {
	return appCfg
}
