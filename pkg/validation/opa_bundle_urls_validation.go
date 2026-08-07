package validation

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/kartverket/accesserator/internal/model"
	"github.com/kartverket/accesserator/pkg/config"
	"github.com/kartverket/accesserator/pkg/log"
	"github.com/kartverket/accesserator/pkg/utilities"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/registry/remote/credentials"
)

const (
	githubActionsOIDCIssuer = "https://token.actions.githubusercontent.com"

	SigstoreBundleMediaType                  = "application/vnd.dev.sigstore.bundle.v0.3+json"
	SigstoreBundleContentAnnotationKey       = "dev.sigstore.bundle.content"
	SigstoreBundleContentAnnotationValue     = "dsse-envelope"
	SigstoreBundlePredicateTypeAnnotationKey = "dev.sigstore.bundle.predicateType"
	SigstoreSlsaProvenanceV1                 = "https://slsa.dev/provenance/v1"
)

// AttestationFetcher abstracts the OCI lookups required to verify an OPA
// bundle's Sigstore SLSA provenance attestation.
type AttestationFetcher interface {
	// ResolveOciRepositoryAndDigest resolves the OCI repository and digest for the given OCI reference.
	ResolveOciRepositoryAndDigest(
		ctx context.Context,
		credStore credentials.Store,
		ociReference string,
	) (*utilities.OciRepositoryAndDigest, error)

	// GetSLSAProvenanceReferrers fetches the SLSA provenance attestation referrers for the given OCI
	// repository and digest.
	GetSLSAProvenanceReferrers(
		ctx context.Context,
		ociRepoAndDigest utilities.OciRepositoryAndDigest,
	) ([]ocispec.Descriptor, error)

	// ValidateSigstoreBundlesMatchesExpectedSource returns an error if the list of OCI referrers does not refer to
	// any valid Sigstore bundles whose signer certificate is not signed keyless by GitHub or matches the expected
	// OPA build source.
	ValidateSigstoreBundlesMatchesExpectedSource(
		ctx context.Context,
		ociRepositoryAndDigest utilities.OciRepositoryAndDigest,
		sigstoreReferrers []ocispec.Descriptor,
		verificationSource model.OpaBundleSource,
	) error
}

type DefaultAttestationFetcher struct{}

func (DefaultAttestationFetcher) ResolveOciRepositoryAndDigest(
	ctx context.Context,
	credStore credentials.Store,
	ociReference string,
) (*utilities.OciRepositoryAndDigest, error) {
	return utilities.ResolveOciRepositoryAndDigest(ctx, credStore, ociReference)
}

func (DefaultAttestationFetcher) GetSLSAProvenanceReferrers(
	ctx context.Context,
	ociRepoAndDigest utilities.OciRepositoryAndDigest,
) ([]ocispec.Descriptor, error) {
	return utilities.GetOciReferrersMatchingMediaTypeAndPredicate(
		ctx,
		ociRepoAndDigest,
		SigstoreBundleMediaType,
		func(referrer ocispec.Descriptor) bool {
			return referrer.ArtifactType == SigstoreBundleMediaType &&
				referrer.Annotations[SigstoreBundleContentAnnotationKey] == SigstoreBundleContentAnnotationValue &&
				referrer.Annotations[SigstoreBundlePredicateTypeAnnotationKey] == SigstoreSlsaProvenanceV1
		},
	)
}

func (DefaultAttestationFetcher) ValidateSigstoreBundlesMatchesExpectedSource(
	ctx context.Context,
	ociRepositoryAndDigest utilities.OciRepositoryAndDigest,
	sigstoreReferrers []ocispec.Descriptor,
	verificationSource model.OpaBundleSource,
) error {
	return ValidateSigstoreBundlesMatchesExpectedSource(
		ctx,
		ociRepositoryAndDigest,
		sigstoreReferrers,
		verificationSource,
	)
}

func ValidateCollisionWithConfiguredSelfAuthBundle(bundles []model.OpaBundle) error {
	if config.Get().OpaSelfAuthorizationBundle == nil {
		return nil
	}
	for _, bundle := range bundles {
		if bundle.Name == config.Get().OpaSelfAuthorizationBundle.Name {
			return fmt.Errorf(
				"OPA bundle with name: %s has name collision with pre-configured OPA bundle with same name. "+
					"Rename your bundle to avoid this error",
				bundle.Name,
			)
		}
	}
	return nil
}

// ValidateBundleUrlPrefixes validates that each bundle URL has an allowed registry
// prefix as configured via ACCESSERATOR_OPA_ALLOWED_BUNDLE_REGISTRY_URL_PREFIXES.
func ValidateBundleUrlPrefixes(opaBundles []model.OpaBundle) error {
	if len(opaBundles) == 0 {
		return fmt.Errorf("bundle URLs cannot be nil or empty")
	}
	allowedPrefixes := config.Get().OpaAllowedBundleRegistryUrlPrefixes

	var invalid []string
	for _, opaBundle := range opaBundles {
		if !hasAllowedPrefix(opaBundle.URL, allowedPrefixes) {
			invalid = append(invalid, opaBundle.URL)
		}
	}
	if len(invalid) == 0 {
		return nil
	}
	return fmt.Errorf(
		"bundle URLs are not allowed: %v; each URL must start with one of %v",
		invalid, allowedPrefixes,
	)
}

func hasAllowedPrefix(url string, prefixes []string) bool {
	return slices.ContainsFunc(prefixes, func(prefix string) bool { return strings.HasPrefix(url, prefix) })
}

// VerifyBundleSource verifies that the given OPA bundle source has a valid Sigstore SLSA provenance attestation that
// matches the configured verification source.
func VerifyBundleSource(
	ctx context.Context,
	fetcher AttestationFetcher,
	credStore credentials.Store,
	bundleSource model.OpaBundle,
) error {
	logger := log.GetLogger(ctx)

	if bundleSource.BundleSource.Repository == "" {
		return fmt.Errorf("bundle source must have a repository")
	}

	ociRepoAndDigest, err := fetcher.ResolveOciRepositoryAndDigest(ctx, credStore, bundleSource.URL)
	if err != nil {
		logger.Error(
			err,
			"failed to resolve digest for OPA bundle",
			"bundleURL", bundleSource.URL,
		)
		return fmt.Errorf("failed to resolve digest for %s", bundleSource.URL)
	}

	sigstoreProvenanceReferrers, err := fetcher.GetSLSAProvenanceReferrers(ctx, *ociRepoAndDigest)
	if err != nil {
		logger.Error(err, "failed to fetch SLSA referrers", "bundleURL", bundleSource.URL)
		return fmt.Errorf("failed to fetch SLSA referrers for %s", bundleSource.URL)
	}

	logger.Debug(
		"validating OPA bundle build source",
		"bundleURL", bundleSource.URL,
		"verification", bundleSource.BundleSource,
	)
	if validateErr := fetcher.ValidateSigstoreBundlesMatchesExpectedSource(
		ctx,
		*ociRepoAndDigest,
		sigstoreProvenanceReferrers,
		bundleSource.BundleSource,
	); validateErr != nil {
		if errors.Is(validateErr, ErrSourceMismatch) ||
			errors.Is(validateErr, ErrNoMatchingSigstoreBundleFound) {
			logger.Warning(
				fmt.Sprintf("OPA bundle verification failed: %s", validateErr.Error()),
				"bundleURL", bundleSource.URL,
			)
			return fmt.Errorf(
				"OPA bundle verification failed for %s: %w",
				bundleSource.URL,
				validateErr,
			)
		}
		logger.Error(err, "OPA bundle verification failed", "bundleURL", bundleSource.URL)
		return fmt.Errorf("OPA bundle verification failed for %s", bundleSource.URL)
	}
	return nil
}
