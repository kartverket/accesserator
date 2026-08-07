package utilities

import (
	_ "embed"
	"fmt"
	"strings"
	"sync"

	"github.com/kartverket/accesserator/internal/model"
	sigstorebundle "github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/sigstore/sigstore-go/pkg/fulcio/certificate"
	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/tuf"
	"github.com/sigstore/sigstore-go/pkg/verify"
)

const gitHubTufBaseUrl = "https://tuf-repo.github.com"

var (
	publicGoodTufOnce         sync.Once
	publicGoodTrustedMaterial root.TrustedMaterial

	githubTufOnce         sync.Once
	githubTrustedMaterial root.TrustedMaterial

	//go:embed github-tuf-root.json
	githubTUFRootAnchor []byte
)

// GetBundleVerifier returns a Sigstore verifier for the given Sigstore bundle, using the appropriate trusted
// root (GitHub's or public-good) based on the presence of TLog entries in the bundle.
func GetBundleVerifier(sigstoreBundle *sigstorebundle.Bundle, cachePath string) (*verify.Verifier, error) {
	if sigstoreBundle == nil || sigstoreBundle.Bundle == nil ||
		sigstoreBundle.VerificationMaterial == nil {
		return nil, fmt.Errorf("invalid sigstore bundle: missing verification material")
	}

	opts := make([]verify.VerifierOption, 0, 3)
	var trustedRoot root.TrustedMaterial
	var getTrustedRootErr error
	isSignedByPublicSigtoreInstance := len(sigstoreBundle.VerificationMaterial.TlogEntries) > 0

	if isSignedByPublicSigtoreInstance {
		opts = append(opts, verify.WithSignedCertificateTimestamps(1))
		opts = append(opts, verify.WithTransparencyLog(1))
		opts = append(opts, verify.WithObserverTimestamps(1))
		trustedRoot, getTrustedRootErr = PublicTrustedRoot(cachePath)
		if getTrustedRootErr != nil {
			return nil, fmt.Errorf("failed to initialize public good trusted root: %w", getTrustedRootErr)
		}
	} else {
		opts = append(opts, verify.WithSignedTimestamps(1))
		trustedRoot, getTrustedRootErr = GitHubTrustedRoot(cachePath)
		if getTrustedRootErr != nil {
			return nil, fmt.Errorf("failed to initialize GitHub's Sigstore trusted root: %w", getTrustedRootErr)
		}
	}
	return verify.NewVerifier(trustedRoot, opts...)
}

// PublicTrustedRoot returns trusted material for the Sigstore public-good
// instance (public repositories). Uses the defaults baked into sigstore-go:
// the public-good mirror (tuf-repo-cdn.sigstore.dev) and its embedded root.
func PublicTrustedRoot(cachePath string) (root.TrustedMaterial, error) {
	var err error
	publicGoodTufOnce.Do(func() {
		client, newTufErr := tuf.New(tuf.DefaultOptions().WithCachePath(cachePath))
		if newTufErr != nil {
			err = newTufErr
			return
		}
		publicGoodTrustedMaterial, err = root.GetTrustedRoot(client)
	})
	return publicGoodTrustedMaterial, err
}

// GitHubTrustedRoot returns trusted material for the GitHub Sigstore instance (private/internal repositories).
// Uses the GitHub mirror (tuf-repo.github.com) and its embedded root.
func GitHubTrustedRoot(cachePath string) (root.TrustedMaterial, error) {
	var err error
	githubTufOnce.Do(func() {
		client, newTufErr := tuf.New(tuf.DefaultOptions().
			WithRepositoryBaseURL(gitHubTufBaseUrl).
			WithRoot(githubTUFRootAnchor).
			WithCachePath(cachePath))
		if newTufErr != nil {
			err = newTufErr
			return
		}
		githubTrustedMaterial, err = root.GetTrustedRoot(client)
	})
	return githubTrustedMaterial, err
}

// GetRepositorySourceFromVerifiedSigstoreBundleCertificate extracts the GitHub repository source information from a
// Sigstore bundle's certificate. It performs no verification of the certificate.
func GetRepositorySourceFromVerifiedSigstoreBundleCertificate(
	certificateSummary certificate.Summary,
) model.OpaBundleSource {
	const gitHubUri = "https://github.com"
	source := model.OpaBundleSource{
		Repository: strings.TrimPrefix(certificateSummary.SourceRepositoryURI, gitHubUri+"/"),
		Ref:        certificateSummary.SourceRepositoryRef,
	}

	/*
		BuildConfigURI looks like: https://github.com/<owner>/<repo>/.github/workflows/<file>.yml@<ref>.
		Extract just the workflow file path (relative to the repo root).
	*/
	if workflowFileURI, _, ok := strings.Cut(certificateSummary.BuildConfigURI, "@"); ok {
		repoPrefix := certificateSummary.SourceRepositoryURI + "/"
		source.Workflow = strings.TrimPrefix(workflowFileURI, repoPrefix)
	}
	return source
}
