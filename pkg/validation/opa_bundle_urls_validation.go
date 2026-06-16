package validation

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/kartverket/accesserator/api/v1alpha"
	"github.com/kartverket/accesserator/pkg/config"
	"github.com/kartverket/accesserator/pkg/log"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	sigstorebundle "github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/sigstore/sigstore-go/pkg/fulcio/certificate"
	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/tuf"
	"github.com/sigstore/sigstore-go/pkg/verify"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/memory"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/credentials"
)

const (
	githubActionsOIDCIssuer = "https://token.actions.githubusercontent.com"
	sigstoreBundleMediaType = "application/vnd.dev.sigstore.bundle.v0.3+json"

	// GitHub's Sigstore TUF repository. Attestations created by
	// actions/attest-build-provenance are signed against this root, not the
	// public Sigstore TUF (tuf-repo.sigstore.dev).
	githubTUFRepositoryURL    = "https://tuf-repo.github.com"
	githubTUFInitialRootURL   = "https://tuf-repo.github.com/1.root.json"
	githubTUFInitialRootCache = "/tmp/github-tuf-1.root.json"
	githubTUFCacheDir         = "/tmp/tuf-github"
)

var ErrSourceMismatch = errors.New("source identity mismatch")

// AttestationFetcher fetches the inputs needed for cosign signature
// verification: an OCI manifest digest, and the cosign bundle attached as an
// OCI 1.1 referrer. Layer content is intentionally out of scope.
type AttestationFetcher interface {
	Resolve(ctx context.Context, credStore credentials.Store, reference string) (string, error)
	FetchAttestation(
		ctx context.Context,
		credStore credentials.Store,
		reference string,
		subjectDigest string,
	) ([]byte, error)
}

// DefaultAttestationFetcher uses oras-go to talk to OCI registries.
var DefaultAttestationFetcher AttestationFetcher = ociAttestationFetcher{}

// ValidateBundleUrls validates that each bundle URL has an allowed registry
// prefix as configured via ACCESSERATOR_OPA_ALLOWED_BUNDLE_REGISTRY_URL_PREFIXES.
// It performs no network I/O and is safe to call from an admission webhook.
func ValidateBundleUrls(bundleURLs []v1alpha.BundleSource) error {
	if len(bundleURLs) == 0 {
		return fmt.Errorf("bundle URLs cannot be nil or empty")
	}
	allowedPrefixes := config.Get().OpaAllowedBundleRegistryUrlPrefixes
	invalidURLs := make([]string, 0)

	for _, bundleSource := range bundleURLs {
		if !slices.ContainsFunc(allowedPrefixes, func(prefix string) bool {
			return strings.HasPrefix(bundleSource.URL, prefix)
		}) {
			invalidURLs = append(invalidURLs, bundleSource.URL)
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

// ValidateBundleSourceSignature resolves the manifest digest for bundleSource.URL,
// fetches the cosign attestation attached as an OCI referrer, and verifies it
// against bundleSource.Verification.Source. Returns nil if Verification is nil.
//
// Requires network access to the OCI registry, Sigstore TUF, and Rekor.
// Suitable for admission webhooks if their timeout budget can absorb that.
func ValidateBundleSourceSignature(
	ctx context.Context,
	fetcher AttestationFetcher,
	credStore credentials.Store,
	bundleSource v1alpha.BundleSource,
) error {
	logger := log.GetLogger(ctx)
	if bundleSource.Verification == nil || bundleSource.Verification.Source.Repository == "" {
		return fmt.Errorf("bundle source cannot be nil and must have a repository")
	}

	logger.Info(
		"Bundle verification configured. Validating cosign signature",
		"bundleURL", bundleSource.URL,
		"repository", bundleSource.Verification.Source.Repository,
	)
	logger.Debug("Resolving OCI bundle digest", "url", bundleSource.URL)
	manifestDigest, err := fetcher.Resolve(ctx, credStore, bundleSource.URL)
	if err != nil {
		logger.Error(err, "Failed to resolve OCI bundle digest", "url", bundleSource.URL)
		return fmt.Errorf("failed to resolve OCI bundle digest for %s", bundleSource.URL)
	}
	logger.Debug("Resolved OCI bundle digest", "url", bundleSource.URL, "digest", manifestDigest)

	logger.Debug("Fetching cosign bundle", "url", bundleSource.URL)
	cosignBundleBytes, err := fetcher.FetchAttestation(ctx, credStore, bundleSource.URL, manifestDigest)
	if err != nil {
		logger.Error(err, "Failed to fetch cosign bundle", "url", bundleSource.URL)
		return fmt.Errorf("failed to fetch cosign bundle for %s", bundleSource.URL)
	}
	logger.Debug("Fetched cosign bundle", "url", bundleSource.URL)

	sigstoreBundle := &sigstorebundle.Bundle{}
	if unmarshalErr := sigstoreBundle.UnmarshalJSON(cosignBundleBytes); unmarshalErr != nil {
		logger.Error(unmarshalErr, "Failed to parse cosign bundle", "url", bundleSource.URL)
		return fmt.Errorf("failed to parse cosign bundle for %s", bundleSource.URL)
	}

	digestBytes, err := DecodeManifestDigest(manifestDigest)
	if err != nil {
		logger.Error(err, "Failed to decode OCI bundle digest", "url", bundleSource.URL)
		return fmt.Errorf("failed to decode OCI bundle digest for %s", bundleSource.URL)
	}

	trustedMaterial, err := getTrustedRoot()
	if err != nil {
		logger.Error(err, "Failed to load sigstore trusted root")
		return errors.New("failed to load sigstore trusted root")
	}

	if validateBundleSignatureErr := ValidateBundleSignature(
		logger,
		sigstoreBundle,
		digestBytes,
		bundleSource.Verification.Source,
		trustedMaterial,
	); validateBundleSignatureErr != nil {
		if errors.Is(validateBundleSignatureErr, ErrSourceMismatch) {
			logger.Warning(
				fmt.Sprintf("Bundle verification failed: %s", validateBundleSignatureErr.Error()),
				"bundleURL", bundleSource.URL,
			)
		} else {
			logger.Error(validateBundleSignatureErr, "Bundle verification failed", "bundleURL", bundleSource.URL)
		}
		return fmt.Errorf("cosign bundle verification failed for %s: %w", bundleSource.URL, validateBundleSignatureErr)
	}
	logger.Info("Bundle verification succeeded", "bundleURL", bundleSource.URL)
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

// ValidateBundleSignature verifies that b is a Sigstore bundle signed via
// GitHub Actions keyless flow, is bound to artifactSHA256, and was built
// from the source described by src.
func ValidateBundleSignature(
	logger log.Logger,
	b *sigstorebundle.Bundle,
	artifactSHA256 []byte,
	src v1alpha.GitHubRepositorySource,
	trustedRoot root.TrustedMaterial,
) error {
	if src.Repository == "" {
		return fmt.Errorf("src.Repository is required")
	}

	if len(config.Get().OpaAllowedBundleSignatureSourceOrgs) == 0 {
		return fmt.Errorf(
			"at least one OpaAllowedBundleSignatureSourceOrg is required, " +
				"configure it via ACCESSERATOR_OPA_ALLOWED_BUNDLE_SIGNATURE_SOURCE_ORGS",
		)
	}

	sanRegex, err := BuildGitHubSANRegex(config.Get().OpaAllowedBundleSignatureSourceOrgs)
	if err != nil {
		logger.Error(err, "Failed to build GitHub SAN regex")
		return fmt.Errorf("failed to build GitHub SAN regex: %w", err)
	}

	// Identity policy: must be a GitHub Actions keyless cert. The SAN reflects
	// the SIGNER, not the source, so we keep this permissive and bind to the
	// source via the cert extensions below.
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

	v, err := verify.NewVerifier(trustedRoot,
		// GitHub attestation bundles use GitHub's own TSA for timestamping.
		// They do not carry SCT or Rekor tlog entries, so only the TSA
		// observer timestamp is required — matching what gh attestation verify
		// enforces.
		verify.WithObserverTimestamps(1),
	)
	if err != nil {
		logger.Error(err, "Failed to create new verifier")
		return errors.New("failed to setup certificate verifier")
	}

	result, err := v.Verify(b, verify.NewPolicy(
		verify.WithArtifactDigest("sha256", artifactSHA256),
		verify.WithCertificateIdentity(certID),
	))
	if err != nil {
		logger.Error(err, "Failed to verify bundle signature")
		return errors.New("bundle signature verification failed")
	}

	if result.Signature == nil || result.Signature.Certificate == nil {
		return errors.New("verification result missing signing certificate")
	}
	return checkSourceExtensions(result.Signature.Certificate, src)
}

func checkSourceExtensions(cert *certificate.Summary, src v1alpha.GitHubRepositorySource) error {
	wantSourceURI := "https://github.com/" + src.Repository
	ext := cert.Extensions

	if ext.SourceRepositoryURI != wantSourceURI {
		return fmt.Errorf("%w: GitHub repository mismatch: got %q, want %q",
			ErrSourceMismatch, ext.SourceRepositoryURI, wantSourceURI)
	}

	if src.Ref != "" && ext.SourceRepositoryRef != src.Ref {
		return fmt.Errorf("%w: git ref mismatch: got %q, want %q",
			ErrSourceMismatch, ext.SourceRepositoryRef, src.Ref)
	}

	if src.Workflow != "" {
		// BuildConfigURI looks like "<repoURI>/<workflow-path>@<ref>". We only
		// want to assert the workflow file part — the ref is checked separately
		// via SourceRepositoryRef above.
		wantBase := fmt.Sprintf("%s/%s", wantSourceURI, strings.TrimPrefix(src.Workflow, "/"))
		gotBase, _, _ := strings.Cut(ext.BuildConfigURI, "@")
		if gotBase != wantBase {
			return fmt.Errorf("%w: workflow file mismatch: got %q, want %q",
				ErrSourceMismatch, gotBase, wantBase)
		}
	}

	return nil
}

// DecodeManifestDigest turns "sha256:<hex>" into the raw 32-byte hash.
func DecodeManifestDigest(d string) ([]byte, error) {
	algo, hexPart, ok := strings.Cut(d, ":")
	if !ok || algo != "sha256" {
		return nil, fmt.Errorf("unsupported manifest digest %q (only sha256 supported)", d)
	}
	raw, err := hex.DecodeString(hexPart)
	if err != nil {
		return nil, fmt.Errorf("error when decoding manifest digest hex: %w", err)
	}
	return raw, nil
}

// trustedRoot caches GitHub's Sigstore trusted root, fetched lazily via TUF
// on the first verification call.
var (
	trustedRootOnce sync.Once
	trustedRoot     root.TrustedMaterial
	trustedRootErr  error
)

func getTrustedRoot() (root.TrustedMaterial, error) {
	trustedRootOnce.Do(func() {
		initialRoot, err := fetchGitHubTUFInitialRoot()
		if err != nil {
			trustedRootErr = fmt.Errorf("fetch GitHub TUF initial root: %w", err)
			return
		}
		opts := tuf.DefaultOptions()
		opts.RepositoryBaseURL = githubTUFRepositoryURL
		opts.CachePath = githubTUFCacheDir
		opts.Root = initialRoot
		tufClient, err := tuf.New(opts)
		if err != nil {
			trustedRootErr = fmt.Errorf("init GitHub TUF client: %w", err)
			return
		}
		trustedRoot, trustedRootErr = root.GetTrustedRoot(tufClient)
	})
	return trustedRoot, trustedRootErr
}

// fetchGitHubTUFInitialRoot returns GitHub's TUF 1.root.json bytes, using a
// local cache to avoid re-fetching on every pod restart.
func fetchGitHubTUFInitialRoot() ([]byte, error) {
	if data, err := os.ReadFile(githubTUFInitialRootCache); err == nil {
		return data, nil
	}

	resp, err := http.Get(githubTUFInitialRootURL)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", githubTUFInitialRootURL, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: unexpected HTTP %d", githubTUFInitialRootURL, resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	_ = os.WriteFile(githubTUFInitialRootCache, data, 0600)
	return data, nil
}

// ociAttestationFetcher implements AttestationFetcher using oras-go.
type ociAttestationFetcher struct{}

func (ociAttestationFetcher) Resolve(
	ctx context.Context,
	credStore credentials.Store,
	reference string,
) (string, error) {
	repo, err := NewAuthedRepo(credStore, reference)
	if err != nil {
		return "", err
	}

	desc, err := repo.Resolve(ctx, repo.Reference.Reference)
	if err != nil {
		return "", fmt.Errorf("resolve OCI artifact %s: %w", reference, err)
	}
	return desc.Digest.String(), nil
}

func (ociAttestationFetcher) FetchAttestation(
	ctx context.Context,
	credStore credentials.Store,
	reference string,
	subjectDigest string,
) ([]byte, error) {
	repo, err := NewAuthedRepo(credStore, reference)
	if err != nil {
		return nil, err
	}

	subjectDesc := ocispec.Descriptor{Digest: digest.Digest(subjectDigest)}

	var sigstoreReferrer ocispec.Descriptor
	found := false
	err = repo.Referrers(ctx, subjectDesc, sigstoreBundleMediaType, func(referrers []ocispec.Descriptor) error {
		for _, r := range referrers {
			if r.ArtifactType == sigstoreBundleMediaType {
				sigstoreReferrer = r
				found = true
				return nil
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list referrers for %s: %w", subjectDigest, err)
	}
	if !found {
		return nil, fmt.Errorf("no sigstore bundle referrer found for %s", subjectDigest)
	}

	memStore := memory.New()
	if _, pullErr := oras.Copy(
		ctx,
		repo,
		sigstoreReferrer.Digest.String(),
		memStore,
		"",
		oras.DefaultCopyOptions,
	); pullErr != nil {
		return nil, fmt.Errorf("error pulling sigstore bundle %s: %w", sigstoreReferrer.Digest, pullErr)
	}

	successors, err := content.Successors(ctx, memStore, sigstoreReferrer)
	if err != nil {
		return nil, fmt.Errorf("get sigstore bundle layers: %w", err)
	}
	for _, desc := range successors {
		if desc.MediaType == sigstoreBundleMediaType {
			return content.FetchAll(ctx, memStore, desc)
		}
	}
	return nil, fmt.Errorf("sigstore bundle layer not found in referrer %s", sigstoreReferrer.Digest)
}

func NewAuthedRepo(credStore credentials.Store, reference string) (*remote.Repository, error) {
	repo, err := remote.NewRepository(reference)
	if err != nil {
		return nil, fmt.Errorf("failed parsing OCI reference %s: %w", reference, err)
	}
	repo.Client = &auth.Client{
		Cache:      auth.NewCache(),
		Credential: credentials.Credential(credStore),
	}
	return repo, nil
}
