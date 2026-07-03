package resolver

import (
	"context"
	"fmt"
	"time"

	"github.com/kartverket/accesserator/api/v1alpha"
	"github.com/kartverket/accesserator/internal/state"
	"github.com/kartverket/accesserator/pkg/config"
	"github.com/kartverket/accesserator/pkg/log"
	"github.com/kartverket/accesserator/pkg/utilities"
	"github.com/kartverket/accesserator/pkg/validation"
	"oras.land/oras-go/v2/registry/remote/credentials"
)

const (
	OpaBundleFetchLayerTimeout = 60 * time.Second
	OpaBundleLayerMediaType    = "application/vnd.oci.image.layer.v1.tar+gzip"
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
	credStore credentials.Store,
	ociRepoAndDigest utilities.OciRepositoryAndDigest,
) ([]byte, error) {
	return utilities.FetchLayerMatchingMediaType(ctx, ociRepoAndDigest, OpaBundleLayerMediaType)
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
	if securityConfig.Spec.Opa == nil || !securityConfig.Spec.Opa.Enabled {
		return &state.OpaConfig{
			Enabled: false,
		}, nil
	}

	logger.Info(
		"OPA enabled, resolving OPA config",
		"name", securityConfig.Name, "namespace", securityConfig.Namespace)

	bundles := securityConfig.Spec.Opa.BundleURLs
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
	binaryData := make(map[string][]byte, len(bundles))
	for _, bundle := range bundles {
		data, resolveBundleErr := resolveOpaBundle(ctx, logger, fetcher, config.CredStore, bundle)
		if resolveBundleErr != nil {
			return nil, resolveBundleErr
		}
		binaryData[string(bundle.Name)] = data
	}

	logger.Info(
		"OPA config resolved",
		"name", securityConfig.Name, "namespace", securityConfig.Namespace)
	return &state.OpaConfig{Enabled: true, BundleBinaryData: binaryData}, nil
}

// resolveOpaBundle resolves the OPA bundle from the OCI registry and returns the binary data of the bundle layer.
// It first resolves the OCI repository and digest for the given bundle URL, then fetches the OPA bundle layer matching
// the specified media type. If any step fails, it returns an error.
func resolveOpaBundle(
	ctx context.Context,
	logger log.Logger,
	fetcher OpaBundleFetcher,
	credStore credentials.Store,
	bundle v1alpha.BundleSource,
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
