package utilities

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"github.com/kartverket/accesserator/internal/model"
	sigstorebundle "github.com/sigstore/sigstore-go/pkg/bundle"
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

type InTotoStatement struct {
	Predicate struct {
		BuildDefinition struct {
			ExternalParameters struct {
				Workflow struct {
					Ref        string `json:"ref"`
					Repository string `json:"repository"`
					Path       string `json:"path"`
				} `json:"workflow"`
			} `json:"externalParameters"`
		} `json:"buildDefinition"`
	} `json:"predicate"`
}

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

// GetRepositorySourceFromSigstoreBundle extracts the GitHub repository source information from a Sigstore bundle's
// in-toto statement payload (the DSSE-envelope).
func GetRepositorySourceFromSigstoreBundle(
	sigstoreBundle *sigstorebundle.Bundle,
) (*model.OpaBundleSource, error) {
	if sigstoreBundle == nil || sigstoreBundle.GetDsseEnvelope() == nil {
		return nil, fmt.Errorf("no DsseEnvelope in Sigstore bundle")
	}

	var inTotoStatement InTotoStatement
	if err := json.Unmarshal(sigstoreBundle.GetDsseEnvelope().GetPayload(), &inTotoStatement); err != nil {
		return nil, fmt.Errorf("failed to unmarshal in-toto payload: %w", err)
	}

	rawRepo := inTotoStatement.Predicate.BuildDefinition.ExternalParameters.Workflow.Repository
	repoName, err := extractGitHubRepoName(rawRepo)
	if err != nil {
		return nil, fmt.Errorf("failed to extract repository name from %q: %w", rawRepo, err)
	}

	return &model.OpaBundleSource{
		Repository: repoName,
		Workflow:   inTotoStatement.Predicate.BuildDefinition.ExternalParameters.Workflow.Path,
		Ref:        inTotoStatement.Predicate.BuildDefinition.ExternalParameters.Workflow.Ref,
	}, nil
}

// extractGitHubRepoName extracts the "<org>/<repo>" path from a GitHub repository URL or a bare
// "<org>/<repo>" string. Returns an error if the input cannot be resolved to a valid two-segment path.
func extractGitHubRepoName(raw string) (string, error) {
	if raw == "" {
		return "", fmt.Errorf("repository field is empty")
	}

	// If it looks like a URL (has a scheme), parse it properly.
	if strings.Contains(raw, "://") {
		u, err := url.Parse(raw)
		if err != nil {
			return "", fmt.Errorf("invalid URL: %w", err)
		}
		if !strings.EqualFold(u.Host, "github.com") {
			return "", fmt.Errorf("unexpected host %q, expected github.com", u.Host)
		}
		// u.Path starts with "/", trim it and any trailing slash.
		path := strings.Trim(u.Path, "/")
		if err := validateOrgRepo(path); err != nil {
			return "", err
		}
		return path, nil
	}

	// Otherwise assume it is already in "<org>/<repo>" form.
	if err := validateOrgRepo(raw); err != nil {
		return "", err
	}
	return raw, nil
}

// validateOrgRepo checks that s is exactly "<org>/<repo>" with no extra segments.
func validateOrgRepo(s string) error {
	parts := strings.Split(s, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return fmt.Errorf("expected \"<org>/<repo>\", got %q", s)
	}
	return nil
}
