package utilities

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kartverket/accesserator/api/v1alpha"
	"github.com/kartverket/accesserator/pkg/config"
	sigstorebundle "github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/tuf"
	"github.com/sigstore/sigstore-go/pkg/verify"
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

func GetBundleVerifier(sigstoreBundle *sigstorebundle.Bundle) (*verify.Verifier, error) {
	opts := make([]verify.VerifierOption, 0, 3)
	var trustedRoot root.TrustedMaterial
	var getTrustedRootErr error
	if sigstoreBundle != nil && sigstoreBundle.Bundle != nil &&
		sigstoreBundle.Bundle.VerificationMaterial != nil {
		if len(sigstoreBundle.Bundle.VerificationMaterial.TlogEntries) == 0 {
			opts = append(opts, verify.WithSignedTimestamps(1))
			trustedRoot, getTrustedRootErr = GitHubTrustedRoot()
			if getTrustedRootErr != nil {
				return nil, fmt.Errorf("failed to initialize GitHub's Sigstore trusted root: %w", getTrustedRootErr)
			}
		} else {
			opts = append(opts, verify.WithSignedCertificateTimestamps(1))
			opts = append(opts, verify.WithTransparencyLog(1))
			opts = append(opts, verify.WithObserverTimestamps(1))
			trustedRoot, getTrustedRootErr = PublicGoodTrustedRoot()
			if getTrustedRootErr != nil {
				return nil, fmt.Errorf("failed to initialize public good trusted root: %w", getTrustedRootErr)
			}
		}
	}
	return verify.NewVerifier(trustedRoot, opts...)
}

// PublicGoodTrustedRoot returns trusted material for the Sigstore public-good
// instance (public repositories). Uses the defaults baked into sigstore-go:
// the public-good mirror (tuf-repo-cdn.sigstore.dev) and its embedded root.
func PublicGoodTrustedRoot() (root.TrustedMaterial, error) {
	client, err := tuf.New(tuf.DefaultOptions().WithCachePath(config.Get().SigstoreTufCachePath))
	if err != nil {
		return nil, err
	}
	return root.GetTrustedRoot(client)
}

// GitHubTrustedRoot returns trusted material for GitHub's internal Sigstore
// instance, loaded directly from the embedded trusted_root.json (Fulcio CAs
// and TSA chains for fulcio.githubapp.com / timestamp.githubapp.com).
func GitHubTrustedRoot() (root.TrustedMaterial, error) {
	trustedMaterial, err := root.NewTrustedRootFromPath(config.Get().SigstoreGithubTrustedRootPath)
	if err != nil {
		return nil, fmt.Errorf(
			"unable to setup GitHub Trusted Root %q: %w",
			config.Get().SigstoreGithubTrustedRootPath,
			err,
		)
	}
	return trustedMaterial, nil
}

func GetRepositorySourceFromSigstoreBundle(
	sigstoreBundle *sigstorebundle.Bundle,
) (*v1alpha.GitHubRepositorySource, error) {
	if sigstoreBundle == nil || sigstoreBundle.GetDsseEnvelope() == nil {
		return nil, fmt.Errorf("no DsseEnvelope in Sigstore bundle")
	}

	var inTotoStatement InTotoStatement
	if err := json.Unmarshal(sigstoreBundle.GetDsseEnvelope().GetPayload(), &inTotoStatement); err != nil {
		return nil, fmt.Errorf("failed to unmarshal in-toto payload: %w", err)
	}

	return &v1alpha.GitHubRepositorySource{
		Repository: strings.Split(
			inTotoStatement.Predicate.BuildDefinition.ExternalParameters.Workflow.Repository,
			"://github.com/",
		)[1],
		Workflow: inTotoStatement.Predicate.BuildDefinition.ExternalParameters.Workflow.Path,
		Ref:      inTotoStatement.Predicate.BuildDefinition.ExternalParameters.Workflow.Ref,
	}, nil
}
