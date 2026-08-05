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
	sigstorebundle "github.com/sigstore/sigstore-go/pkg/bundle"
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

	// GetSigstoreProvenanceReferrers fetches the Sigstore SLSA provenance attestation referrers for the given OCI
	// repository and digest.
	GetSigstoreProvenanceReferrers(
		ctx context.Context,
		ociRepoAndDigest utilities.OciRepositoryAndDigest,
	) ([]ocispec.Descriptor, error)

	// GetSigstoreBundleMatchingVerificationSource fetches the first Sigstore bundle matching the given verification
	// source from the provided Sigstore referrers.
	GetSigstoreBundleMatchingVerificationSource(
		ctx context.Context,
		ociRepositoryAndDigest utilities.OciRepositoryAndDigest,
		sigstoreReferrers []ocispec.Descriptor,
		verificationSource model.OpaBundleSource,
	) (*sigstorebundle.Bundle, error)
}

type DefaultAttestationFetcher struct{}

func (DefaultAttestationFetcher) ResolveOciRepositoryAndDigest(
	ctx context.Context,
	credStore credentials.Store,
	ociReference string,
) (*utilities.OciRepositoryAndDigest, error) {
	return utilities.ResolveOciRepositoryAndDigest(ctx, credStore, ociReference)
}

func (DefaultAttestationFetcher) GetSigstoreProvenanceReferrers(
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

func (DefaultAttestationFetcher) GetSigstoreBundleMatchingVerificationSource(
	ctx context.Context,
	ociRepositoryAndDigest utilities.OciRepositoryAndDigest,
	sigstoreReferrers []ocispec.Descriptor,
	verificationSource model.OpaBundleSource,
) (*sigstorebundle.Bundle, error) {
	return GetSigstoreBundleMatchingVerification(
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

	sigstoreProvenanceReferrers, err := fetcher.GetSigstoreProvenanceReferrers(ctx, *ociRepoAndDigest)
	if err != nil {
		logger.Error(err, "failed to fetch Sigstore provenance referrers", "bundleURL", bundleSource.URL)
		return fmt.Errorf("failed to fetch Sigstore provenance referrers for %s", bundleSource.URL)
	}

	sigstoreBundleMatchingVerification, err := fetcher.GetSigstoreBundleMatchingVerificationSource(
		ctx,
		*ociRepoAndDigest,
		sigstoreProvenanceReferrers,
		bundleSource.BundleSource,
	)
	if err != nil {
		if errors.Is(err, ErrSourceMismatch) {
			logger.Warning(
				fmt.Sprintf("OPA bundle verification failed: %s", err.Error()),
				"bundleURL", bundleSource.URL,
			)
			return err
		}
		logger.Error(err, "failed to fetch Sigstore bundle matching verification", "bundleURL", bundleSource.URL)
		return errors.New("failed to fetch Sigstore bundle matching verification")
	}
	logger.Debug(
		"Found sigstore bundle matching OPA bundle verification source",
		"bundleURL", bundleSource.URL,
	)

	logger.Debug(
		"Stripping alg prefix from Sigstore bundle digest",
		"bundleURL", bundleSource.URL,
	)
	artifactSHA256, err := utilities.StripAlgPrefix(ociRepoAndDigest.Digest)
	if err != nil {
		logger.Error(err, "failed to strip alg prefix artifact SHA256", "bundleURL", bundleSource.URL)
		return fmt.Errorf("failed to strip alg prefix artifact SHA256 for digest %s", ociRepoAndDigest.Digest)
	}
	logger.Debug(
		"Sigstore bundle digest stripped of alg prefix successfully",
		"bundleURL", bundleSource.URL,
	)

	logger.Debug(
		"Validating Sigstore bundle signature",
		"bundleURL", bundleSource.URL,
		"sigstoreBundleDigest", "sha256:"+string(artifactSHA256),
	)
	if validateSigstoreBundleSignatureErr := ValidateSigstoreBundleSignature(
		logger,
		sigstoreBundleMatchingVerification,
		artifactSHA256,
	); validateSigstoreBundleSignatureErr != nil {
		logger.Error(
			validateSigstoreBundleSignatureErr,
			"sigstore bundle failed signature verification",
			"opaOciRepositoryDigest", ociRepoAndDigest.Digest,
		)
		return errors.New("failed to verify sigstore bundle signature")
	}
	logger.Debug(
		"Sigstore bundle signature validated successfully",
		"bundleURL", bundleSource.URL,
		"sigstoreBundleDigest", "sha256:"+string(artifactSHA256),
	)
	return nil
}
