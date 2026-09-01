package resolver

import (
	"context"
	"fmt"
	"maps"
	"time"

	"github.com/kartverket/accesserator/api/v1alpha"
	"github.com/kartverket/accesserator/internal/model"
	"github.com/kartverket/accesserator/internal/state"
	"github.com/kartverket/accesserator/pkg/config"
	"github.com/kartverket/accesserator/pkg/log"
	"github.com/kartverket/accesserator/pkg/utilities"
	"github.com/kartverket/accesserator/pkg/validation"
	"google.golang.org/protobuf/types/known/structpb"
	"oras.land/oras-go/v2/registry/remote/credentials"
)

const (
	OpaBundleFetchLayerTimeout           = 60 * time.Second
	OpaEnvoyFilterClusterConfigPatchName = "opa_ext_authz"
)

// OpaBundleFetcher is the set of OCI lookups the OPA resolver needs.
// It extends validation.AttestationFetcher with the OPA bundle layer-pull operation.
type OpaBundleFetcher interface {
	validation.AttestationFetcher

	// FetchOpaBundleLayer fetches the OPA bundle layer from the OCI registry at the given repository and digest.
	// It only fetches the layer matching on mediaType to avoid unnecessary fetching of OCI wrapping layers.
	FetchOpaBundleLayer(ctx context.Context, ociRepoAndDigest utilities.OciRepositoryAndDigest) ([]byte, error)
}

type DefaultOpaBundleFetcher struct {
	validation.DefaultAttestationFetcher
}

func (DefaultOpaBundleFetcher) FetchOpaBundleLayer(
	ctx context.Context,
	ociRepoAndDigest utilities.OciRepositoryAndDigest,
) ([]byte, error) {
	return utilities.FetchLayerMatchingMediaType(ctx, ociRepoAndDigest, utilities.OpaBundleLayerMediaType)
}

// ResolveOpaConfig resolves the OPA configuration from the SecurityConfig. It returns an OpaConfig struct containing
// the resolved configuration, or an error if the resolution fails.
func ResolveOpaConfig(logger log.Logger, securityConfig v1alpha.SecurityConfig) (*state.OpaConfig, error) {
	return ResolveOpaConfigWithFetcher(logger, DefaultOpaBundleFetcher{}, securityConfig)
}

func ResolveOpaConfigWithFetcher(
	logger log.Logger,
	fetcher OpaBundleFetcher,
	securityConfig v1alpha.SecurityConfig,
) (*state.OpaConfig, error) {
	if securityConfig.Spec.Opa != nil && !config.Get().OpaEnabled {
		return nil, fmt.Errorf("OPA is not enabled on this cluster and 'spec.opa' can therefore not be set")
	}

	opaConfig := state.OpaConfig{
		Enabled: false,
	}
	if securityConfig.Spec.Opa == nil || !securityConfig.Spec.Opa.Enabled {
		return &opaConfig, nil
	}

	opaConfig.Enabled = true
	logger.Info(
		"OPA enabled, resolving OPA config",
		"name", securityConfig.Name, "namespace", securityConfig.Namespace,
	)

	if len(securityConfig.Spec.Opa.BundleURLs) == 0 {
		return nil, fmt.Errorf(
			"no OPA bundle URLs found in SecurityConfig %s/%s",
			securityConfig.Namespace,
			securityConfig.Name,
		)
	}

	ctx, cancel := context.WithTimeout(context.Background(), OpaBundleFetchLayerTimeout)
	defer cancel()

	logger.Debug("Resolving OPA bundles from OCI registry",
		"name", securityConfig.Name,
		"namespace", securityConfig.Namespace,
	)
	bundles := model.ToOpaBundles(securityConfig.Spec.Opa.BundleURLs)
	opaConfig.BundleBinaryData = maps.Clone(config.OpaSelfAuthorizationBundleBinaryData)
	for _, bundle := range bundles {
		data, resolveBundleErr := resolveOpaBundle(ctx, logger, fetcher, config.CredStore, bundle)
		if resolveBundleErr != nil {
			return nil, resolveBundleErr
		}
		opaConfig.BundleBinaryData[bundle.Name] = data
	}

	if securityConfig.Spec.Opa.RequestPolicy == nil {
		opaConfig.RequestPolicy.Enabled = false
		return &opaConfig, nil
	}

	logger.Debug("Resolving OPA request authorization",
		"name", securityConfig.Name,
		"namespace", securityConfig.Namespace,
	)
	requestAuthorizationConfig, err := ResolveOpaRequestAuthorization(
		string(securityConfig.Spec.ApplicationRef),
		*securityConfig.Spec.Opa.RequestPolicy,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve OPA request authorization config: %w", err)
	}
	opaConfig.RequestPolicy = *requestAuthorizationConfig

	return &opaConfig, nil
}

// resolveOpaBundle resolves the OPA bundle from the OCI registry and returns the binary data of the bundle layer.
// It first resolves the OCI repository and digest for the given bundle URL, then fetches the OPA bundle layer matching
// the specified media type. If any step fails, it returns an error.
func resolveOpaBundle(
	ctx context.Context,
	logger log.Logger,
	fetcher OpaBundleFetcher,
	credStore credentials.Store,
	bundle model.OpaBundle,
) ([]byte, error) {
	logger.Debug("Resolving OPA bundle digest", "bundleURL", bundle.URL)
	ociRepoAndDigest, err := fetcher.ResolveOciRepositoryAndDigest(ctx, credStore, bundle.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve OCI bundle digest for %s: %w", bundle.URL, err)
	}

	logger.Debug("Fetching OPA bundle layer", "bundleURL", bundle.URL, "digest", ociRepoAndDigest.Digest)
	layer, err := fetcher.FetchOpaBundleLayer(ctx, *ociRepoAndDigest)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch OCI bundle layer for %s: %w", bundle.URL, err)
	}
	return layer, nil
}

func ResolveOpaRequestAuthorization(
	skiperatorAppName string, requestAuthorizationSpec v1alpha.OpaRequestPolicy,
) (*state.RequestPolicyConfig, error) {
	requestAuthorizationConfig := state.RequestPolicyConfig{
		Enabled: requestAuthorizationSpec.Enabled,
		WorkloadLabels: map[string]string{
			utilities.SkiperatorApplicationRefLabel: skiperatorAppName,
		},
	}
	if !requestAuthorizationSpec.Enabled {
		return &requestAuthorizationConfig, nil
	}
	clusterConfigPatchValue, err := getClusterConfigPatchValue()
	if err != nil {
		return nil, fmt.Errorf(
			"failed to parse EnvoyFilter cluster config patch value as protobuf struct: %w",
			err,
		)
	}
	externalAuthorizationConfigPatchValue, err := getExternalAuthorizationConfigPatchValue(
		model.ToOpaRequestPolicyFailureMode(requestAuthorizationSpec.FailureMode),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to parse EnvoyFilter external authorization config patch value as protobuf struct: %w",
			err,
		)
	}
	requestAuthorizationConfig.ClusterConfigPatchValue = clusterConfigPatchValue
	requestAuthorizationConfig.ExternalAuthorizationConfigPatchValue = externalAuthorizationConfigPatchValue
	return &requestAuthorizationConfig, nil
}

func getClusterConfigPatchValue() (*structpb.Struct, error) {
	clusterConfigPatchValue := map[string]any{
		"name":            OpaEnvoyFilterClusterConfigPatchName,
		"type":            "STRICT_DNS",
		"connect_timeout": "1s",
		"typed_extension_protocol_options": map[string]any{
			"envoy.extensions.upstreams.http.v3.HttpProtocolOptions": map[string]any{
				"@type": "type.googleapis.com/envoy.extensions.upstreams.http.v3.HttpProtocolOptions",
				"explicit_http_config": map[string]any{
					"http2_protocol_options": map[string]any{},
				},
			},
		},
		"load_assignment": map[string]any{
			"cluster_name": OpaEnvoyFilterClusterConfigPatchName,
			"endpoints": []any{
				map[string]any{
					"lb_endpoints": []any{
						map[string]any{
							"endpoint": map[string]any{
								"address": map[string]any{
									"socket_address": map[string]any{
										"address":    "127.0.0.1",
										"port_value": config.Get().OpaGrpcPort,
									},
								},
							},
						},
					},
				},
			},
		},
	}
	return structpb.NewStruct(clusterConfigPatchValue)
}

func getExternalAuthorizationConfigPatchValue(
	failureMode model.OpaRequestPolicyFailureMode,
) (*structpb.Struct, error) {
	failureModeAllow := failureMode == model.OpaRequestPolicyFailureModeForward

	externalAuthorizationConfigPatchValue := map[string]any{
		"name": "envoy.filters.http.ext_authz",
		"typed_config": map[string]any{
			"@type":                 "type.googleapis.com/envoy.extensions.filters.http.ext_authz.v3.ExtAuthz",
			"transport_api_version": "V3",
			"grpc_service": map[string]any{
				"envoy_grpc": map[string]any{
					"cluster_name": OpaEnvoyFilterClusterConfigPatchName,
				},
				"timeout": "1s",
			},
			// The field failure_mode_allow specifies whether the request should be denied (false), i.e. returning
			// 403 Access denied, or forwarded to the upstream application (true).
			"failure_mode_allow": failureModeAllow,
			// If failure_mode_allow AND field failure_mode_allow_header_add is set to true, the header
			// `x-envoy-auth-failure-mode-allowed: true` is added to the request if envoy failed to reach OPA or if OPA
			// returned a 5xx response.
			"failure_mode_allow_header_add": true,
			"with_request_body": map[string]any{
				"max_request_bytes":     8192,
				"allow_partial_message": true,
			},
		},
	}
	return structpb.NewStruct(externalAuthorizationConfigPatchValue)
}
