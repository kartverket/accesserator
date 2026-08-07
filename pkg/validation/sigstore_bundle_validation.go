package validation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/kartverket/accesserator/internal/model"
	"github.com/kartverket/accesserator/pkg/config"
	"github.com/kartverket/accesserator/pkg/log"
	"github.com/kartverket/accesserator/pkg/utilities"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	sigstorebundle "github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/sigstore/sigstore-go/pkg/fulcio/certificate"
	"github.com/sigstore/sigstore-go/pkg/verify"
	"golang.org/x/sync/errgroup"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/registry/remote"
)

// maxConcurrentSigstoreBundlePullAndVerifies is the maximum number of concurrent referrer pulls and verifications to
// perform.
const maxConcurrentSigstoreBundlePullAndVerifies = 10

// ErrSourceMismatch is returned when an attestation's verification source does not match
// the specified verification source.
var (
	ErrSourceMismatch                = errors.New("source mismatch")
	ErrNoMatchingSigstoreBundleFound = errors.New(
		"no Sigstore bundles with valid certificate matching verification source",
	)
	ErrWorkflowFileMismatch          = errors.New("workflow file mismatch")
	ErrNoMatchingCertificateIdentity *verify.ErrNoMatchingCertificateIdentity
)

// VerifySigstoreBundleCertificate verifies the Fulcio certificate attached to the provided Sigstore bundle
// against the provided certificate identity and returns the certificate summary.
func VerifySigstoreBundleCertificate(
	logger log.Logger,
	sigstoreBundle *sigstorebundle.Bundle,
	artifactSHA256 []byte,
	certID verify.CertificateIdentity,
) (*certificate.Summary, error) {
	verifier, getVerifierErr := utilities.GetBundleVerifier(sigstoreBundle, config.Get().SigstoreTufCachePath)
	if getVerifierErr != nil {
		logger.Error(getVerifierErr, "Failed to get bundle verifier")
		return nil, fmt.Errorf("failed to get bundle verifier")
	}

	result, err := verifier.Verify(sigstoreBundle, verify.NewPolicy(
		verify.WithArtifactDigest("sha256", artifactSHA256),
		verify.WithCertificateIdentity(certID),
	))
	if err != nil {
		logger.Error(err, "Failed to verify sigstore bundle signature")
		return nil, errors.New("sigstore bundle signature verification failed")
	}

	if result.Signature == nil || result.Signature.Certificate == nil {
		return nil, errors.New("verification result missing signing certificate")
	}

	return result.Signature.Certificate, nil
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

// pullSigstoreBundleBytes pulls the bytes of a Sigstore bundle from the given OCI repository and referrer descriptor.
func pullSigstoreBundleBytes(
	ctx context.Context,
	repo *remote.Repository,
	referrer ocispec.Descriptor,
) ([]byte, error) {
	successors, err := content.Successors(ctx, repo, referrer)
	if err != nil {
		return nil, fmt.Errorf("failed to pull Sigstore referrer %s: %w", referrer.Digest, err)
	}
	for _, successor := range successors {
		if successor.MediaType != SigstoreBundleMediaType {
			continue
		}
		bytes, fetchSuccessorErr := content.FetchAll(ctx, repo, successor)
		if fetchSuccessorErr != nil {
			return nil, fmt.Errorf("failed to fetch Sigstore bundle %s: %w", successor.Digest, fetchSuccessorErr)
		}
		if len(bytes) > 0 {
			return bytes, nil
		}
	}
	return nil, fmt.Errorf("no Sigstore bundle layer in referrer %s", referrer.Digest)
}

func fetchReferrerBundle(
	ctx context.Context,
	repo *remote.Repository,
	referrer ocispec.Descriptor,
) (*sigstorebundle.Bundle, error) {
	bundleBytes, err := pullSigstoreBundleBytes(ctx, repo, referrer)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Sigstore bundle bytes: %w", err)
	}
	bundle := &sigstorebundle.Bundle{}
	if jsonMarshalErr := json.Unmarshal(bundleBytes, bundle); jsonMarshalErr != nil {
		return nil, fmt.Errorf("failed to decode Sigstore bundle bytes: %w", jsonMarshalErr)
	}
	return bundle, nil
}

func ValidateSigstoreBundlesMatchesExpectedSource(
	ctx context.Context,
	ociRepositoryAndDigest utilities.OciRepositoryAndDigest,
	sigstoreReferrers []ocispec.Descriptor,
	expectedSource model.OpaBundleSource,
) error {
	logger := log.GetLogger(ctx)
	artifactSHA256, err := utilities.StripAlgPrefix(ociRepositoryAndDigest.Digest)
	if err != nil {
		return fmt.Errorf(
			"failed to strip alg prefix artifact SHA256 for for OPA bundle %s/%s@%s",
			ociRepositoryAndDigest.Repository.Reference.Registry,
			ociRepositoryAndDigest.Repository.Reference.Repository,
			ociRepositoryAndDigest.Digest,
		)
	}
	sanRegex, err := BuildGitHubSANRegex(config.Get().OpaAllowedBundleSignatureSourceOrgs)
	if err != nil {
		return fmt.Errorf("failed to build GitHub SAN regex: %w", err)
	}
	certificateIdentity, err := GetCertificateIdentity(*sanRegex)
	if err != nil {
		return fmt.Errorf("failed to create CertificateIdentity: %w", err)
	}

	// Pull the referrer bundles that have a valid certificate concurrently.
	gCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	group, gCtx := errgroup.WithContext(gCtx)
	group.SetLimit(maxConcurrentSigstoreBundlePullAndVerifies)

	var (
		mu                         sync.Mutex
		hasValid                   atomic.Bool
		verifiedSourcesNotMatching []model.OpaBundleSource
		errList                    []error
	)

	for _, referrer := range sigstoreReferrers {
		if hasValid.Load() {
			break
		}
		group.Go(func() error {
			if gCtx.Err() != nil {
				return nil
			}
			bundle, fetchErr := fetchReferrerBundle(gCtx, ociRepositoryAndDigest.Repository, referrer)
			if fetchErr != nil {
				mu.Lock()
				errList = append(errList, fetchErr)
				mu.Unlock()
				return nil
			}
			certificateSummary, verifyErr := VerifySigstoreBundleCertificate(
				logger, bundle, artifactSHA256, *certificateIdentity,
			)
			if verifyErr != nil {
				mu.Lock()
				errList = append(errList, verifyErr)
				mu.Unlock()
				return nil
			}
			verifiedSource := utilities.GetRepositorySourceFromVerifiedSigstoreBundleCertificate(*certificateSummary)
			if SatisfiesVerificationSource(verifiedSource, expectedSource) {
				hasValid.Store(true)
				cancel()
				return nil
			}
			mu.Lock()
			verifiedSourcesNotMatching = append(verifiedSourcesNotMatching, verifiedSource)
			mu.Unlock()
			return nil
		})
	}
	_ = group.Wait()

	if hasValid.Load() {
		return nil
	}

	if len(verifiedSourcesNotMatching) > 0 {
		return fmt.Errorf(
			"%w: OPA bundle verification failed:\n  expected source:\n    - %s\n  found sources:\n%s",
			ErrSourceMismatch,
			sourceRepositoryToString(expectedSource, expectedSource),
			sourceRepositoriesToString(verifiedSourcesNotMatching, expectedSource),
		)
	}

	if len(errList) > 0 {
		return errors.Join(errList...)
	}

	return fmt.Errorf(
		"verification failed: %w",
		ErrNoMatchingSigstoreBundleFound,
	)
}

func SatisfiesVerificationSource(actual, expected model.OpaBundleSource) bool {
	return (actual.Repository == expected.Repository) &&
		(expected.Ref == "" || actual.Ref == expected.Ref) &&
		(expected.Workflow == "" || actual.Workflow == expected.Workflow)
}

func sourceRepositoryToString(sourceRepository, verificationSource model.OpaBundleSource) string {
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
	sourceRepositories []model.OpaBundleSource,
	verificationSource model.OpaBundleSource,
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

func GetCertificateIdentity(sanRegex regexp.Regexp) (*verify.CertificateIdentity, error) {
	certificateIdentity, err := verify.NewCertificateIdentity(
		verify.SubjectAlternativeNameMatcher{
			Regexp: sanRegex,
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
