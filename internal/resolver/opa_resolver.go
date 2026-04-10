package resolver

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/kartverket/accesserator/api/v1alpha"
	"github.com/kartverket/accesserator/internal/state"
	"github.com/kartverket/accesserator/pkg/config"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/memory"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/credentials"
)

const ociFetchTimeout = 60 * time.Second

// ResolveOpaConfig resolves the OPA configuration from the SecurityConfig.
// It fetches bundle files from OCI registries and returns them as byte slices.
func ResolveOpaConfig(securityConfig v1alpha.SecurityConfig) (*state.OpaConfig, error) {
	if securityConfig.Spec.Opa == nil || !securityConfig.Spec.Opa.Enabled {
		return &state.OpaConfig{
			Enabled: false,
		}, nil
	}

	if err := ValidateBundleURLs(securityConfig.Spec.Opa.BundleURLs); err != nil {
		return nil, fmt.Errorf("invalid OPA bundle URLs: %w", err)
	}

	bundleBinaryData := make(map[string][]byte)
	for _, bundle := range securityConfig.Spec.Opa.BundleURLs {
		// Use a dedicated context with timeout for OCI fetching
		// to avoid being canceled by the reconciler's context
		fetchCtx, cancel := context.WithTimeout(context.Background(), ociFetchTimeout)
		bundleFile, err := fetchOCIBundle(fetchCtx, bundle.URL)
		cancel()
		if err != nil {
			return nil, fmt.Errorf("failed to fetch OCI bundle from %s: %w", bundle.URL, err)
		}
		bundleBinaryData[string(bundle.Name)] = bundleFile
	}

	return &state.OpaConfig{
		Enabled:          true,
		BundleBinaryData: bundleBinaryData,
	}, nil
}

// fetchOCIBundle fetches an OCI artifact from the given reference and returns its content as bytes.
// The reference should be in the format: <registry>/<repository>:<tag>
// Example: ghcr.io/kartverket/accesserator/opa-bundle:latest
func fetchOCIBundle(ctx context.Context, reference string) ([]byte, error) {
	// Parse the reference to get registry and repository
	repo, err := remote.NewRepository(reference)
	if err != nil {
		return nil, fmt.Errorf("failed to parse OCI reference %s: %w", reference, err)
	}

	// Setup authentication using default credentials (docker config)
	storeOpts := credentials.StoreOptions{}
	credStore, err := credentials.NewStoreFromDocker(storeOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to create credential store: %w", err)
	}

	repo.Client = &auth.Client{
		Cache:      auth.NewCache(),
		Credential: credentials.Credential(credStore),
	}

	// Create an in-memory store to hold the pulled content
	memStore := memory.New()

	// Pull the artifact (copies manifest + all layers to memStore)
	manifestDescriptor, err := oras.Copy(ctx, repo, repo.Reference.Reference, memStore, "", oras.DefaultCopyOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to pull OCI artifact %s: %w", reference, err)
	}

	// Get the layers from the manifest
	successors, err := content.Successors(ctx, memStore, manifestDescriptor)
	if err != nil {
		return nil, fmt.Errorf("failed to get layers from manifest: %w", err)
	}

	// For ORAS-pushed artifacts, find the layer with the bundle content
	for _, desc := range successors {
		// Look for the tar+gzip layer (the actual bundle)
		if desc.MediaType == "application/vnd.oci.image.layer.v1.tar+gzip" {
			// Fetch the layer content directly
			bundleContent, err := content.FetchAll(ctx, memStore, desc)
			if err != nil {
				return nil, fmt.Errorf("failed to fetch bundle layer: %w", err)
			}

			return bundleContent, nil
		}
	}

	return nil, fmt.Errorf("no bundle content found in OCI artifact %s", reference)
}

// ValidateBundleURLs validates that each bundle URL has an allowed registry prefix.
// Returns an error if any URL doesn't match the allowed prefixes from config.
func ValidateBundleURLs(bundleURLs []v1alpha.BundleSource) error {
	allowedPrefixes := config.Get().OpaAllowedBundleRegistryUrlPrefixes
	invalidURLs := make([]string, 0)

	for _, bundle := range bundleURLs {
		for _, prefix := range allowedPrefixes {
			if !strings.HasPrefix(bundle.URL, prefix) {
				invalidURLs = append(invalidURLs, bundle.URL)
			}
		}
	}

	if len(invalidURLs) > 0 {
		return fmt.Errorf(
			"bundle URLs are not allowed: %v; each URL must start with one of %v",
			invalidURLs,
			allowedPrefixes,
		)
	}

	return nil
}
