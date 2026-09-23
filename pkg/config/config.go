package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/kartverket/accesserator/pkg/model"
	"github.com/kelseyhightower/envconfig"
	"github.com/sigstore/sigstore-go/pkg/fulcio/certificate"
	"github.com/sigstore/sigstore-go/pkg/verify"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"oras.land/oras-go/v2/registry/remote/credentials"
)

var (
	githubActionsOIDCIssuer = "https://token.actions.githubusercontent.com"

	CertificateIdentityForGithubActions verify.CertificateIdentity

	CredStore *credentials.DynamicStore

	OpaSelfAuthorizationBundleBinaryData map[string][]byte
)

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

	TexasCPURequest    string `split_words:"true" default:"1m"`
	TexasCPULimit      string `split_words:"true" default:"200m"`
	TexasMemoryRequest string `split_words:"true" default:"16Mi"`
	TexasMemoryLimit   string `split_words:"true" default:"128Mi"`

	EntraTenantId string `split_words:"true"`

	OpaEnabled                          bool             `split_words:"true" default:"true"`
	OpaImageName                        string           `split_words:"true" default:"openpolicyagent/opa"`
	OpaImageTag                         string           `split_words:"true"`
	OpaImageSha                         string           `split_words:"true"`
	OpaPort                             int32            `split_words:"true" default:"3010"`
	OpaGrpcPort                         int32            `split_words:"true" default:"9191"`
	OpaUrlEnvVarName                    string           `split_words:"true" default:"OPA_URL"`
	OpaAllowedBundleRegistryUrlPrefixes []string         `split_words:"true"`
	OpaAllowedBundleSignatureSourceOrgs []string         `split_words:"true"`
	OpaSelfAuthorizationBundle          *model.OpaBundle `split_words:"true"`

	OpaCPURequest    string `split_words:"true" default:"2m"`
	OpaCPULimit      string `split_words:"true" default:"500m"`
	OpaMemoryRequest string `split_words:"true" default:"32Mi"`
	OpaMemoryLimit   string `split_words:"true" default:"256Mi"`

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

	OpaSelfAuthorizationBundleBinaryData = make(map[string][]byte, 1)

	cfg := Config{}
	if err := envconfig.Process("accesserator", &cfg); err != nil {
		return err
	}

	raw, ok := os.LookupEnv("ACCESSERATOR_OPA_SELF_AUTHORIZATION_BUNDLE")
	if !ok || strings.TrimSpace(raw) == "" {
		cfg.OpaSelfAuthorizationBundle = nil
	} else {
		if err := cfg.OpaSelfAuthorizationBundle.ValidateOpaBundle(); err != nil {
			return fmt.Errorf("invalid ACCESSERATOR_OPA_SELF_AUTHORIZATION_BUNDLE: %w", err)
		}
	}

	missing := make([]string, 0, 11)
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
		if cfg.OpaImageTag == "" {
			missing = append(missing, "ACCESSERATOR_OPA_IMAGE_TAG")
		}
		if cfg.OpaImageSha == "" {
			missing = append(missing, "ACCESSERATOR_OPA_IMAGE_SHA")
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

	if err := validateResourceQuantities(map[string]string{
		"ACCESSERATOR_TEXAS_CPU_REQUEST":    cfg.TexasCPURequest,
		"ACCESSERATOR_TEXAS_CPU_LIMIT":      cfg.TexasCPULimit,
		"ACCESSERATOR_TEXAS_MEMORY_REQUEST": cfg.TexasMemoryRequest,
		"ACCESSERATOR_TEXAS_MEMORY_LIMIT":   cfg.TexasMemoryLimit,
		"ACCESSERATOR_OPA_CPU_REQUEST":      cfg.OpaCPURequest,
		"ACCESSERATOR_OPA_CPU_LIMIT":        cfg.OpaCPULimit,
		"ACCESSERATOR_OPA_MEMORY_REQUEST":   cfg.OpaMemoryRequest,
		"ACCESSERATOR_OPA_MEMORY_LIMIT":     cfg.OpaMemoryLimit,
	}); err != nil {
		return err
	}

	if cfg.OpaEnabled {
		certificateIdentityForGitHubActions, err := GetCertificateIdentityForGitHubOrgs(
			cfg.OpaAllowedBundleSignatureSourceOrgs,
		)
		if err != nil {
			return fmt.Errorf("failed setting up CertificateIdentity for GitHub Actions: %w", err)
		}
		CertificateIdentityForGithubActions = *certificateIdentityForGitHubActions
	}

	appCfg = cfg
	return nil
}

func Get() Config {
	return appCfg
}

// validateResourceQuantities checks that every configured CPU/memory resource string
// (keyed by its env var name for error reporting) parses as a valid k8s resource.Quantity.
func validateResourceQuantities(quantitiesByEnvVarName map[string]string) error {
	for envVarName, value := range quantitiesByEnvVarName {
		if _, err := resource.ParseQuantity(value); err != nil {
			return fmt.Errorf("invalid %s value %q: %w", envVarName, value, err)
		}
	}
	return nil
}

// TexasResources returns the configured resource requests and limits for the Texas sidecar container.
func TexasResources() corev1.ResourceRequirements {
	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(appCfg.TexasCPURequest),
			corev1.ResourceMemory: resource.MustParse(appCfg.TexasMemoryRequest),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(appCfg.TexasCPULimit),
			corev1.ResourceMemory: resource.MustParse(appCfg.TexasMemoryLimit),
		},
	}
}

// OpaResources returns the configured resource requests and limits for the OPA sidecar container.
func OpaResources() corev1.ResourceRequirements {
	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(appCfg.OpaCPURequest),
			corev1.ResourceMemory: resource.MustParse(appCfg.OpaMemoryRequest),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(appCfg.OpaCPULimit),
			corev1.ResourceMemory: resource.MustParse(appCfg.OpaMemoryLimit),
		},
	}
}

func GetCertificateIdentityForGitHubOrgs(orgs []string) (*verify.CertificateIdentity, error) {
	sanRegex, err := BuildGitHubSANRegex(orgs)
	if err != nil {
		return nil, fmt.Errorf("failed to build GitHub SAN regex: %w", err)
	}
	certificateIdentity, err := verify.NewCertificateIdentity(
		verify.SubjectAlternativeNameMatcher{
			Regexp: *sanRegex,
		},
		verify.IssuerMatcher{
			Issuer: githubActionsOIDCIssuer,
		},
		certificate.Extensions{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create CertificateIdentity: %w", err)
	}
	return &certificateIdentity, nil
}

// BuildGitHubSANRegex returns an anchored regex that matches the SAN of any
// keyless cert signed by a workflow in one of the given GitHub orgs.
func BuildGitHubSANRegex(orgs []string) (*regexp.Regexp, error) {
	if len(orgs) == 0 {
		return nil, fmt.Errorf("at least one org is required")
	}
	escaped := make([]string, len(orgs))
	for i, o := range orgs {
		escaped[i] = regexp.QuoteMeta(o)
	}
	sanRegex := fmt.Sprintf(
		`^https://github\.com/(?:%s)/[^/]+/\.github/workflows/[^@]+\.ya?ml@.+`,
		strings.Join(escaped, "|"),
	)
	compiledSanRegex, err := regexp.Compile(sanRegex)
	if err != nil {
		return nil, fmt.Errorf("failed to compile SAN regex: %w", err)
	}
	return compiledSanRegex, nil
}
