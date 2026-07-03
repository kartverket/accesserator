package config

import (
	"fmt"
	"strings"

	"github.com/kelseyhightower/envconfig"
	"oras.land/oras-go/v2/registry/remote/credentials"
)

var CredStore *credentials.DynamicStore

type Config struct {
	RunsInProduction *bool `split_words:"true"`

	ClusterName string `split_words:"true"`

	TokenxEnabled   bool   `split_words:"true" default:"true"`
	TokenxName      string `split_words:"true" default:"tokendings"`
	TokenxNamespace string `split_words:"true"`

	TexasImageName     string `split_words:"true" default:"ghcr.io/nais/texas"`
	TexasImageTag      string `split_words:"true"`
	TexasImageSha      string `split_words:"true"`
	TexasPort          int32  `split_words:"true" default:"3000"`
	TexasProbePort     int32  `split_words:"true" default:"3001"`
	TexasUrlEnvVarName string `split_words:"true" default:"TEXAS_URL"`

	EntraTenantId string `split_words:"true"`

	OpaEnabled                          bool     `split_words:"true" default:"true"`
	OpaImageName                        string   `split_words:"true" default:"openpolicyagent/opa"`
	OpaImageTag                         string   `split_words:"true"`
	OpaImageSha                         string   `split_words:"true"`
	OpaPort                             int32    `split_words:"true" default:"3010"`
	OpaUrlEnvVarName                    string   `split_words:"true" default:"OPA_URL"`
	OpaAllowedBundleRegistryUrlPrefixes []string `split_words:"true"`
	OpaAllowedBundleSignatureSourceOrgs []string `split_words:"true"`

	SigstoreTufCachePath string `split_words:"true" default:"/tmp/sigstore-tuf"`
}

var appCfg Config

func Load() error {
	// Setup credential store for auth towards OCI registry
	var setupCredStoreErr error
	CredStore, setupCredStoreErr = credentials.NewStoreFromDocker(credentials.StoreOptions{})
	if setupCredStoreErr != nil {
		return fmt.Errorf("failed setting up credential store for auth towards OCI registry: %w", setupCredStoreErr)
	}

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
	if cfg.TexasImageTag == "" {
		missing = append(missing, "ACCESSERATOR_TEXAS_IMAGE_TAG")
	}
	if cfg.TexasImageSha == "" {
		missing = append(missing, "ACCESSERATOR_TEXAS_IMAGE_SHA")
	}
	if cfg.EntraTenantId == "" {
		missing = append(missing, "ACCESSERATOR_ENTRA_TENANT_ID")
	}
	if cfg.TokenxEnabled {
		if cfg.TokenxName == "" {
			missing = append(missing, "ACCESSERATOR_TOKENX_NAME")
		}
		if cfg.TokenxNamespace == "" {
			missing = append(missing, "ACCESSERATOR_TOKENX_NAMESPACE")
		}
	}
	if cfg.OpaEnabled {
		if cfg.OpaImageName == "" {
			missing = append(missing, "ACCESSERATOR_OPA_IMAGE_NAME")
		}
		if cfg.OpaImageTag == "" {
			missing = append(missing, "ACCESSERATOR_OPA_IMAGE_TAG")
		}
		if cfg.OpaImageSha == "" {
			missing = append(missing, "ACCESSERATOR_OPA_IMAGE_SHA")
		}
		if cfg.OpaPort == 0 {
			missing = append(missing, "ACCESSERATOR_OPA_PORT")
		}
		if cfg.OpaUrlEnvVarName == "" {
			missing = append(missing, "ACCESSERATOR_OPA_URL_ENV_VAR_NAME")
		}
		if len(cfg.OpaAllowedBundleRegistryUrlPrefixes) == 0 {
			missing = append(missing, "ACCESSERATOR_OPA_ALLOWED_BUNDLE_REGISTRY_URL_PREFIXES")
		}
		if len(cfg.OpaAllowedBundleSignatureSourceOrgs) == 0 {
			missing = append(missing, "ACCESSERATOR_OPA_ALLOWED_BUNDLE_SIGNATURE_SOURCE_ORGS")
		}
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
