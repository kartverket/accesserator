package resolver

import (
	"context"
	"fmt"
	"time"

	"github.com/kartverket/accesserator/api/v1alpha"
	"github.com/kartverket/accesserator/internal/state"
	"github.com/kartverket/accesserator/pkg/config"
	"github.com/kartverket/accesserator/pkg/log"
	"github.com/kartverket/accesserator/pkg/validation"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/memory"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/credentials"
)

const (
	ociFetchTimeout = 60 * time.Second

	ociLayerMediaType = "application/vnd.oci.image.layer.v1.tar+gzip"
)

// BundleFetcher fetches OPA bundles from an OCI registry. It composes
// validation.AttestationFetcher (Resolve + FetchAttestation, used during
// signature verification) with FetchLayer for pulling the actual bundle
// content. Splitting these lets callers verify the manifest digest before
// pulling any untrusted layer bytes.
type BundleFetcher interface {
	validation.AttestationFetcher
	// FetchLayer pulls the bundle layer from the manifest identified by
	// manifestDigest. The lookup is by digest, so callers can be sure they pull
	// exactly what they verified.
	FetchLayer(ctx context.Context, credStore credentials.Store, reference string, manifestDigest string) ([]byte, error)
}

type ociBundleFetcher struct {
	validation.AttestationFetcher
}

func (ociBundleFetcher) FetchLayer(ctx context.Context, credStore credentials.Store, reference string, manifestDigest string) ([]byte, error) {
	return pullBundleLayer(ctx, credStore, reference, manifestDigest)
}

var defaultBundleFetcher BundleFetcher = ociBundleFetcher{
	AttestationFetcher: validation.DefaultAttestationFetcher,
}

// ResolveOpaConfig resolves the OPA configuration from the SecurityConfig.
// It fetches bundle files from OCI registries and returns them as byte slices.
func ResolveOpaConfig(logger log.Logger, securityConfig v1alpha.SecurityConfig) (*state.OpaConfig, error) {
	if securityConfig.Spec.Opa != nil && !config.Get().OpaEnabled {
		return nil, fmt.Errorf("OPA is not enabled on this cluster and 'spec.opa' can therefore not be set")
	}
	return ResolveOpaConfigWithFetcher(logger, defaultBundleFetcher, securityConfig)
}

// ResolveOpaConfigWithFetcher resolves the manifest digest for each bundle,
// verifies the cosign signature against that digest (if requested),
// and only then pulls the layer content.
func ResolveOpaConfigWithFetcher(logger log.Logger, fetcher BundleFetcher, securityConfig v1alpha.SecurityConfig) (*state.OpaConfig, error) {
	if securityConfig.Spec.Opa == nil || !securityConfig.Spec.Opa.Enabled {
		return &state.OpaConfig{
			Enabled: false,
		}, nil
	}
	logger.Info("OPA enabled, resolving OPA config", "name", securityConfig.Name, "namespace", securityConfig.Namespace)

	if err := validation.ValidateBundleUrls(securityConfig.Spec.Opa.BundleURLs); err != nil {
		return nil, fmt.Errorf("invalid OPA bundle URLs: %w", err)
	}

	// Setup authentication using default credentials (docker config)
	credStore, err := credentials.NewStoreFromDocker(credentials.StoreOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create credential store: %w", err)
	}

	bundleBinaryData := make(map[string][]byte)
	for _, bundleSource := range securityConfig.Spec.Opa.BundleURLs {
		bundleContent, bundleFetchErr := resolveAndFetchBundle(logger, fetcher, credStore, bundleSource)
		if bundleFetchErr != nil {
			return nil, bundleFetchErr
		}
		bundleBinaryData[string(bundleSource.Name)] = bundleContent
	}

	logger.Info("OPA config resolved", "name", securityConfig.Name, "namespace", securityConfig.Namespace)
	return &state.OpaConfig{
		Enabled:          true,
		BundleBinaryData: bundleBinaryData,
	}, nil
}

func resolveAndFetchBundle(
	logger log.Logger,
	fetcher BundleFetcher,
	credStore credentials.Store,
	bundleSource v1alpha.BundleSource,
) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), ociFetchTimeout)
	defer cancel()

	logger.Debug("Resolving OCI bundle", "url", bundleSource.URL)
	manifestDigest, err := fetcher.Resolve(ctx, credStore, bundleSource.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve OCI bundle from %s: %w", bundleSource.URL, err)
	}

	if bundleSource.Verification != nil {
		if bundleSource.Verification.Source.Repository == "" {
			return nil, fmt.Errorf("bundle URL %s specifies verification, but repository is empty", bundleSource.URL)
		}
		logger.Debug("Verifying OCI bundle signature", "url", bundleSource.URL)
		if verifyErr := validation.ValidateBundleSourceSignature(ctx, fetcher, credStore, bundleSource); verifyErr != nil {
			return nil, fmt.Errorf("failed to verify OCI bundle from %s: %w", bundleSource.URL, verifyErr)
		}
	}

	logger.Debug("Fetching OCI bundle layer", "url", bundleSource.URL)
	layerContent, err := fetcher.FetchLayer(ctx, credStore, bundleSource.URL, manifestDigest)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch OCI bundle from %s: %w", bundleSource.URL, err)
	}
	logger.Debug("Fetched OCI bundle layer", "url", bundleSource.URL)
	return layerContent, nil
}

// pullBundleLayer pulls the artifact identified by manifestDigest and returns
// the first tar+gzip layer's content. The lookup is by digest so this is
// safe to call after verification: callers know they're pulling the exact
// manifest they trusted.
func pullBundleLayer(ctx context.Context, credStore credentials.Store, reference string, manifestDigest string) ([]byte, error) {
	repo, err := remote.NewRepository(reference)
	if err != nil {
		return nil, fmt.Errorf("parse OCI reference %s: %w", reference, err)
	}
	repo.Client = &auth.Client{
		Cache:      auth.NewCache(),
		Credential: credentials.Credential(credStore),
	}

	memStore := memory.New()
	manifestDesc, err := oras.Copy(ctx, repo, manifestDigest, memStore, "", oras.DefaultCopyOptions)
	if err != nil {
		return nil, fmt.Errorf("pull OCI artifact %s@%s: %w", reference, manifestDigest, err)
	}

	successors, err := content.Successors(ctx, memStore, manifestDesc)
	if err != nil {
		return nil, fmt.Errorf("get bundle layers: %w", err)
	}

	for _, desc := range successors {
		if desc.MediaType == ociLayerMediaType {
			layerContent, fetchErr := content.FetchAll(ctx, memStore, desc)
			if fetchErr != nil {
				return nil, fmt.Errorf("fetch bundle layer: %w", fetchErr)
			}
			return layerContent, nil
		}
	}

	return nil, fmt.Errorf("no bundle content found in OCI artifact %s", reference)
}
