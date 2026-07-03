package validation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/kartverket/accesserator/api/v1alpha"
	"github.com/kartverket/accesserator/pkg/config"
	"github.com/kartverket/accesserator/pkg/log"
	"github.com/kartverket/accesserator/pkg/utilities"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	sigstorebundle "github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/sigstore/sigstore-go/pkg/verify"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/memory"
	"oras.land/oras-go/v2/registry/remote"
)

// ErrSourceMismatch is returned when an attestation's verification source does not match
// the specified verification source.
var ErrSourceMismatch = errors.New("source mismatch")

// ValidateSigstoreBundleSignature verifies that the bundle is signed via
// GitHub Actions' keyless flow by a workflow in one of the allowed
// organizations, and that the bundle's artifact digest matches the provided SHA256 digest.
func ValidateSigstoreBundleSignature(
	logger log.Logger,
	sigstoreBundle *sigstorebundle.Bundle,
	artifactSHA256 []byte,
) error {
	sanRegex, err := BuildGitHubSANRegex(config.Get().OpaAllowedBundleSignatureSourceOrgs)
	if err != nil {
		logger.Error(err, "Failed to build GitHub SAN regex")
		return fmt.Errorf("failed to build GitHub SAN regex: %w", err)
	}

	certID, err := verify.NewShortCertificateIdentity(
		githubActionsOIDCIssuer,
		"",
		"",
		sanRegex,
	)
	if err != nil {
		logger.Error(err, "Failed to build identity policy", "sanRegex", sanRegex)
		return fmt.Errorf("failed to CertificateIdentity from SAN regex %s", sanRegex)
	}

	verifier, getVerifierErr := utilities.GetBundleVerifier(sigstoreBundle)
	if getVerifierErr != nil {
		logger.Error(getVerifierErr, "Failed to get bundle verifier")
		return fmt.Errorf("failed to get bundle verifier")
	}

	result, err := verifier.Verify(sigstoreBundle, verify.NewPolicy(
		verify.WithArtifactDigest("sha256", artifactSHA256),
		verify.WithCertificateIdentity(certID),
	))
	if err != nil {
		logger.Error(err, "Failed to verify sigstore bundle signature")
		return errors.New("sigstore bundle signature verification failed")
	}

	if result.Signature == nil || result.Signature.Certificate == nil {
		return errors.New("verification result missing signing certificate")
	}
	return nil
}

// BuildGitHubSANRegex returns an anchored regex that matches the SAN of any
// keyless cert signed by a workflow in one of the given GitHub orgs.
func BuildGitHubSANRegex(orgs []string) (string, error) {
	if len(orgs) == 0 {
		return "", fmt.Errorf("at least one org is required")
	}
	escaped := make([]string, len(orgs))
	for i, o := range orgs {
		escaped[i] = regexp.QuoteMeta(o)
	}
	return fmt.Sprintf(
		`^https://github\.com/(?:%s)/[^/]+/\.github/workflows/[^@]+\.ya?ml@.+`,
		strings.Join(escaped, "|"),
	), nil
}

// pullSigstoreBundleBytes pulls the bytes of a Sigstore bundle from the given OCI repository and referrer descriptor.
func pullSigstoreBundleBytes(
	ctx context.Context,
	repo *remote.Repository,
	referrer ocispec.Descriptor,
) ([]byte, error) {
	store := memory.New()
	if _, err := oras.Copy(ctx, repo, referrer.Digest.String(), store, "", oras.DefaultCopyOptions); err != nil {
		return nil, fmt.Errorf("failed to pull Sigstore referrer %s: %w", referrer.Digest, err)
	}
	successors, err := content.Successors(ctx, store, referrer)
	if err != nil {
		return nil, fmt.Errorf("failed to list successors of Sigstore referrer %s: %w", referrer.Digest, err)
	}
	for _, successor := range successors {
		if successor.MediaType != SigstoreBundleMediaType {
			continue
		}
		bytes, fetchSuccessorErr := content.FetchAll(ctx, store, successor)
		if fetchSuccessorErr != nil {
			return nil, fmt.Errorf("failed to fetch Sigstore bundle %s: %w", successor.Digest, fetchSuccessorErr)
		}
		if len(bytes) > 0 {
			return bytes, nil
		}
	}
	return nil, fmt.Errorf("no Sigstore bundle layer in referrer %s", referrer.Digest)
}

func GetSigstoreBundleMatchingVerification(
	ctx context.Context,
	ociRepositoryAndDigest utilities.OciRepositoryAndDigest,
	sigstoreReferrers []ocispec.Descriptor,
	verificationSource v1alpha.GitHubRepositorySource,
) (*sigstorebundle.Bundle, error) {
	var mismatchedSources []v1alpha.GitHubRepositorySource
	var errList []error
	for _, sigstoreReferrer := range sigstoreReferrers {
		sigstoreBundleBytes, fetchSigstoreBundleErr := pullSigstoreBundleBytes(
			ctx,
			ociRepositoryAndDigest.Repository,
			sigstoreReferrer,
		)
		if fetchSigstoreBundleErr != nil {
			errList = append(errList, fmt.Errorf("failed to fetch Sigstore bundle bytes: %w", fetchSigstoreBundleErr))
			continue
		}

		sigstoreBundle := &sigstorebundle.Bundle{}
		if err := json.Unmarshal(sigstoreBundleBytes, sigstoreBundle); err != nil {
			errList = append(errList, fmt.Errorf("failed to decode Sigstore bundle bytes: %w", err))
			continue
		}

		githubRepositorySourceFromSigstoreBundle, err := utilities.GetRepositorySourceFromSigstoreBundle(sigstoreBundle)
		if err != nil {
			errList = append(
				errList,
				fmt.Errorf(
					"failed to fetch github repository source from Sigstore bundle fetched via refererrer %s: %w",
					sigstoreReferrer.Digest,
					err,
				),
			)
			continue
		}

		if SatisfiesVerificationSource(
			*githubRepositorySourceFromSigstoreBundle,
			verificationSource,
		) {
			return sigstoreBundle, nil
		}

		mismatchedSources = append(
			mismatchedSources,
			*githubRepositorySourceFromSigstoreBundle,
		)

	}

	if len(errList) > 0 {
		return nil, errors.Join(errList...)
	}

	return nil, fmt.Errorf(
		"%w: OPA bundle verification failed:\n  expected source:\n    - %s\n  found sources:\n%s",
		ErrSourceMismatch,
		sourceRepositoryToString(verificationSource, verificationSource),
		sourceRepositoriesToString(mismatchedSources, verificationSource),
	)
}

func SatisfiesVerificationSource(actual, expected v1alpha.GitHubRepositorySource) bool {
	return (actual.Repository == expected.Repository) &&
		(expected.Ref == "" || actual.Ref == expected.Ref) &&
		(expected.Workflow == "" || actual.Workflow == expected.Workflow)
}

func sourceRepositoryToString(sourceRepository, verificationSource v1alpha.GitHubRepositorySource) string {
	stringRepresentation := fmt.Sprintf("Repository: %s", sourceRepository.Repository)

	if verificationSource.Workflow != "" {
		stringRepresentation += fmt.Sprintf(", Workflow: %v", sourceRepository.Workflow)
	}

	if verificationSource.Ref != "" {
		stringRepresentation += fmt.Sprintf(", Ref: %v", sourceRepository.Ref)
	}

	return stringRepresentation
}

func sourceRepositoriesToString(
	sourceRepositories []v1alpha.GitHubRepositorySource,
	verificationSource v1alpha.GitHubRepositorySource,
) string {
	if len(sourceRepositories) == 0 {
		return "    - <none>"
	}

	sourceRepositoriesAsStrings := make([]string, 0, len(sourceRepositories))
	for _, source := range sourceRepositories {
		sourceRepositoriesAsStrings = append(
			sourceRepositoriesAsStrings,
			sourceRepositoryToString(source, verificationSource),
		)
	}
	slices.Sort(sourceRepositoriesAsStrings)

	compactSources := slices.Compact(sourceRepositoriesAsStrings)
	for i, source := range compactSources {
		compactSources[i] = "    - " + source
	}

	return strings.Join(compactSources, "\n")
}
