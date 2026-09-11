package model

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/kartverket/accesserator/api/v1alpha"
	k8svalidation "k8s.io/apimachinery/pkg/util/validation"
)

const (
	OpaClusterName = "opa_ext_authz"

	OpaRequestPolicyFailureModeDeny OpaRequestPolicyFailureMode = iota
	OpaRequestPolicyFailureModeForward
)

type OpaRequestPolicyFailureMode int

var (
	githubRepositoryPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9-]{0,38}/[a-zA-Z0-9_][a-zA-Z0-9._-]{0,99}$`)
	githubWorkflowPattern   = regexp.MustCompile(`^\.github/workflows/[a-zA-Z0-9_.-]+\.(yml|yaml)$`)
	githubRefPattern        = regexp.MustCompile(`^refs/(heads|tags|pull)/[a-zA-Z0-9/_.-]{1,243}$`)
)

type OpaEnvoyExtAuthzFilterConfig struct {
	OpaClusterName    string
	FailureMode       OpaRequestPolicyFailureMode
	RequestBodyConfig RequestBodyConfig
}

type RequestBodyConfig struct {
	IncludeRequestBody  bool
	MaxRequestBodyBytes int
}

type OpaBundle struct {
	Name         string          `json:"name"`
	URL          string          `json:"url"`
	BundleSource OpaBundleSource `json:"verification"`
}

type OpaBundleSource struct {
	Repository string `json:"repository"`
	Workflow   string `json:"workflow"`
	Ref        string `json:"ref"`
}

func ToOpaBundles(opaBundles []v1alpha.BundleSource) []OpaBundle {
	modelOpaBundles := make([]OpaBundle, 0, len(opaBundles))
	for _, opaBundle := range opaBundles {
		modelOpaBundles = append(
			modelOpaBundles,
			ToOpaBundle(opaBundle),
		)
	}
	return modelOpaBundles
}

func ToOpaBundle(fromOpaBundle v1alpha.BundleSource) OpaBundle {
	toOpaBundle := OpaBundle{
		Name: string(fromOpaBundle.Name),
		URL:  fromOpaBundle.URL,
	}
	if fromOpaBundle.Verification != nil {
		toOpaBundle.BundleSource.Repository = fromOpaBundle.Verification.Source.Repository
		toOpaBundle.BundleSource.Workflow = fromOpaBundle.Verification.Source.Workflow
		toOpaBundle.BundleSource.Ref = fromOpaBundle.Verification.Source.Ref
	}
	return toOpaBundle
}

func (bundle *OpaBundle) Decode(value string) error {
	return json.Unmarshal([]byte(value), bundle)
}

func (bundle OpaBundle) ValidateOpaBundle() error {
	if validationErrors := k8svalidation.IsConfigMapKey(bundle.Name); len(validationErrors) > 0 {
		return fmt.Errorf("invalid OPA bundle name %q: %s", bundle.Name, strings.Join(validationErrors, ", "))
	}

	if bundle.BundleSource.Repository == "" {
		return fmt.Errorf("invalid OPA bundle verification.repository: must be set")
	}
	if !githubRepositoryPattern.MatchString(bundle.BundleSource.Repository) {
		return fmt.Errorf("invalid OPA bundle verification.repository %q", bundle.BundleSource.Repository)
	}

	if !githubWorkflowPattern.MatchString(bundle.BundleSource.Workflow) {
		return fmt.Errorf("invalid OPA bundle verification.workflow %q", bundle.BundleSource.Workflow)
	}

	if !githubRefPattern.MatchString(bundle.BundleSource.Ref) {
		return fmt.Errorf("invalid OPA bundle verification.ref %q", bundle.BundleSource.Ref)
	}

	return nil
}

func (o OpaBundleSource) ToGitHubRepositoryURI() string {
	return fmt.Sprintf("https://github.com/%s", o.Repository)
}

func ToOpaEnvoyExtAuthzFilterConfig(requestPolicySpec v1alpha.OpaRequestPolicy) OpaEnvoyExtAuthzFilterConfig {
	opaEnvoyExtAuthzFilterConfig := OpaEnvoyExtAuthzFilterConfig{
		OpaClusterName: OpaClusterName,
		FailureMode:    toOpaRequestPolicyFailureMode(requestPolicySpec.FailureMode),
		RequestBodyConfig: RequestBodyConfig{
			IncludeRequestBody: false,
		},
	}
	if requestPolicySpec.RequestBody == nil {
		return opaEnvoyExtAuthzFilterConfig
	}

	opaEnvoyExtAuthzFilterConfig.RequestBodyConfig.IncludeRequestBody = requestPolicySpec.RequestBody.Include
	if opaEnvoyExtAuthzFilterConfig.RequestBodyConfig.IncludeRequestBody {
		opaEnvoyExtAuthzFilterConfig.RequestBodyConfig.MaxRequestBodyBytes = requestPolicySpec.RequestBody.MaxRequestBodyBytes
	}

	return opaEnvoyExtAuthzFilterConfig
}

func toOpaRequestPolicyFailureMode(fromFailureMode string) OpaRequestPolicyFailureMode {
	switch strings.ToLower(fromFailureMode) {
	case "deny":
		return OpaRequestPolicyFailureModeDeny
	case "forward":
		return OpaRequestPolicyFailureModeForward
	default:
		return OpaRequestPolicyFailureModeDeny
	}
}
