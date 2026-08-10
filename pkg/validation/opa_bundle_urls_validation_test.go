package validation_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/kartverket/accesserator/internal/model"
	"github.com/kartverket/accesserator/pkg/config"
	"github.com/kartverket/accesserator/pkg/utilities"
	"github.com/kartverket/accesserator/pkg/validation"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/registry/remote/credentials"
)

// mockAttestationFetcher implements validation.AttestationFetcher with canned
// return values and call counters.
type mockAttestationFetcher struct {
	repoAndDigest *utilities.OciRepositoryAndDigest
	resolveErr    error

	referrers    []ocispec.Descriptor
	referrersErr error

	validateErr error

	resolveCalls   int
	referrersCalls int
	validateCalls  int
}

func (m *mockAttestationFetcher) ResolveOciRepositoryAndDigest(
	_ context.Context,
	_ credentials.Store,
	_ string,
) (*utilities.OciRepositoryAndDigest, error) {
	m.resolveCalls++
	if m.resolveErr != nil {
		return nil, m.resolveErr
	}
	if m.repoAndDigest == nil {
		m.repoAndDigest = &utilities.OciRepositoryAndDigest{
			Digest: "sha256:" + strings.Repeat("0", 64),
		}
	}
	return m.repoAndDigest, nil
}

func (m *mockAttestationFetcher) GetSLSAProvenanceReferrers(
	_ context.Context,
	_ utilities.OciRepositoryAndDigest,
) ([]ocispec.Descriptor, error) {
	m.referrersCalls++
	return m.referrers, m.referrersErr
}

func (m *mockAttestationFetcher) ValidateSigstoreBundlesMatchesExpectedSource(
	_ context.Context,
	_ utilities.OciRepositoryAndDigest,
	_ []ocispec.Descriptor,
	_ model.OpaBundleSource,
) error {
	m.validateCalls++
	return m.validateErr
}

var _ validation.AttestationFetcher = (*mockAttestationFetcher)(nil)

var _ = Describe("ValidateBundleUrls", func() {
	const allowedPrefix = "https://allowed/"

	BeforeEach(func() {
		Expect(os.Setenv("ACCESSERATOR_OPA_ALLOWED_BUNDLE_REGISTRY_URL_PREFIXES", allowedPrefix)).To(Succeed())
		Expect(config.Load()).To(Succeed())
	})

	It("returns an error when bundleURLs is nil", func() {
		err := validation.ValidateBundleUrlPrefixes(nil)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("cannot be nil or empty"))
	})

	It("returns an error when bundleURLs is empty", func() {
		err := validation.ValidateBundleUrlPrefixes([]model.OpaBundle{})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("cannot be nil or empty"))
	})

	It("returns nil when every URL matches an allowed prefix", func() {
		bundles := []model.OpaBundle{
			{Name: "a", URL: allowedPrefix + "foo:tag"},
			{Name: "b", URL: allowedPrefix + "bar:tag"},
		}
		Expect(validation.ValidateBundleUrlPrefixes(bundles)).To(Succeed())
	})

	It("returns an error naming the disallowed URL", func() {
		bundles := []model.OpaBundle{
			{Name: "a", URL: "https://forbidden/repo:tag"},
		}
		err := validation.ValidateBundleUrlPrefixes(bundles)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("https://forbidden/repo:tag"))
		Expect(err.Error()).To(ContainSubstring("must start with one of"))
	})

	It("collects every disallowed URL in the error message", func() {
		bundles := []model.OpaBundle{
			{Name: "a", URL: "https://forbidden/repo-a:tag"},
			{Name: "b", URL: allowedPrefix + "ok:tag"},
			{Name: "c", URL: "https://other-bad/repo-c:tag"},
		}
		err := validation.ValidateBundleUrlPrefixes(bundles)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("https://forbidden/repo-a:tag"))
		Expect(err.Error()).To(ContainSubstring("https://other-bad/repo-c:tag"))
		Expect(err.Error()).NotTo(ContainSubstring("https://allowed/ok:tag"))
	})

	It("accepts a URL that matches one of several configured prefixes", func() {
		Expect(os.Setenv(
			"ACCESSERATOR_OPA_ALLOWED_BUNDLE_REGISTRY_URL_PREFIXES",
			"https://allowed/,https://also-allowed/",
		)).To(Succeed())
		Expect(config.Load()).To(Succeed())
		DeferCleanup(func() {
			Expect(os.Setenv("ACCESSERATOR_OPA_ALLOWED_BUNDLE_REGISTRY_URL_PREFIXES", allowedPrefix)).To(Succeed())
			Expect(config.Load()).To(Succeed())
		})

		bundles := []model.OpaBundle{
			{Name: "a", URL: "https://also-allowed/repo:tag"},
		}
		Expect(validation.ValidateBundleUrlPrefixes(bundles)).To(Succeed())
	})
})

var _ = Describe("ValidateCollisionWithAdditionalBundleNames", func() {
	const selfAuthorizationBundleJSON = `
		{
			"name": "opa-common-self-auth",
			"url": "https://allowed/self-auth:tag",
			"verification": {
				"repository": "kartverket/opa-common",
				"workflow": ".github/workflows/build-opa-api-self-authentication.yml",
				"ref": "refs/tags/opa-api-self-authentication/v0.0.1"
			}
		}`

	AfterEach(func() {
		Expect(os.Unsetenv("ACCESSERATOR_OPA_SELF_AUTHORIZATION_BUNDLE")).To(Succeed())
		Expect(config.Load()).To(Succeed())
	})

	It("returns nil when no additional self-authorization bundle is configured", func() {
		bundles := []model.OpaBundle{{Name: "bundle-a", URL: "https://allowed/repo-a:tag"}}

		err := validation.ValidateCollisionWithConfiguredSelfAuthBundle(bundles)

		Expect(err).NotTo(HaveOccurred())
	})

	It("returns nil when bundle names do not collide", func() {
		Expect(os.Setenv("ACCESSERATOR_OPA_SELF_AUTHORIZATION_BUNDLE", selfAuthorizationBundleJSON)).To(Succeed())
		Expect(config.Load()).To(Succeed())

		bundles := []model.OpaBundle{{Name: "bundle-a", URL: "https://allowed/repo-a:tag"}}

		err := validation.ValidateCollisionWithConfiguredSelfAuthBundle(bundles)

		Expect(err).NotTo(HaveOccurred())
	})

	It("returns an error when a bundle name collides with the pre-configured self-authorization bundle", func() {
		Expect(os.Setenv("ACCESSERATOR_OPA_SELF_AUTHORIZATION_BUNDLE", selfAuthorizationBundleJSON)).To(Succeed())
		Expect(config.Load()).To(Succeed())

		bundles := []model.OpaBundle{{
			Name: "opa-common-self-auth",
			URL:  "https://allowed/repo-a:tag",
		}}

		err := validation.ValidateCollisionWithConfiguredSelfAuthBundle(bundles)

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("collision"))
		Expect(err.Error()).To(ContainSubstring("opa-common-self-auth"))
	})
})

var _ = Describe("VerifyBundleSource", func() {
	const bundleURL = "https://allowed/repo:tag"

	withVerification := model.OpaBundle{
		Name: "bundle",
		URL:  bundleURL,
		BundleSource: model.OpaBundleSource{
			Repository: "kartverket/accesserator",
		},
	}

	It("returns an error when Verification.Source.Repository is empty", func() {
		fetcher := &mockAttestationFetcher{}
		bundleSource := model.OpaBundle{
			Name: "bundle",
			URL:  bundleURL,
			BundleSource: model.OpaBundleSource{
				Repository: "",
			},
		}

		err := validation.VerifyBundleSource(context.Background(), fetcher, nil, bundleSource)

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("bundle source must have a repository"))
		Expect(fetcher.resolveCalls).To(BeZero())
	})

	It("returns an error when resolving the digest fails and skips later steps", func() {
		fetcher := &mockAttestationFetcher{resolveErr: errors.New("resolve-failed")}

		err := validation.VerifyBundleSource(context.Background(), fetcher, nil, withVerification)

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal(fmt.Sprintf("failed to resolve digest for %s", bundleURL)))
		Expect(fetcher.resolveCalls).To(Equal(1))
		Expect(fetcher.referrersCalls).To(BeZero())
	})

	It("returns an error when fetching provenance referrers fails and skips the bundle match", func() {
		fetcher := &mockAttestationFetcher{referrersErr: errors.New("referrers-failed")}

		err := validation.VerifyBundleSource(context.Background(), fetcher, nil, withVerification)

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal(
			fmt.Sprintf("failed to fetch SLSA referrers for %s", bundleURL),
		))
		Expect(fetcher.referrersCalls).To(Equal(1))
		Expect(fetcher.validateCalls).To(BeZero())
	})

	It("wraps ErrSourceMismatch with the bundle URL when the fetcher signals a source mismatch", func() {
		mismatch := fmt.Errorf("%w: the found source did not match", validation.ErrSourceMismatch)
		fetcher := &mockAttestationFetcher{validateErr: mismatch}

		err := validation.VerifyBundleSource(context.Background(), fetcher, nil, withVerification)

		Expect(err).To(HaveOccurred())
		Expect(errors.Is(err, validation.ErrSourceMismatch)).To(BeTrue())
		Expect(err.Error()).To(ContainSubstring("OPA bundle verification failed for"))
		Expect(err.Error()).To(ContainSubstring(bundleURL))
		Expect(fetcher.validateCalls).To(Equal(1))
	})

	It("wraps ErrNoMatchingSigstoreBundleFound with the bundle URL when no matching bundle is found", func() {
		noMatch := fmt.Errorf("verification failed: %w", validation.ErrNoMatchingSigstoreBundleFound)
		fetcher := &mockAttestationFetcher{validateErr: noMatch}

		err := validation.VerifyBundleSource(context.Background(), fetcher, nil, withVerification)

		Expect(err).To(HaveOccurred())
		Expect(errors.Is(err, validation.ErrNoMatchingSigstoreBundleFound)).To(BeTrue())
		Expect(err.Error()).To(ContainSubstring("OPA bundle verification failed for"))
		Expect(err.Error()).To(ContainSubstring(bundleURL))
		Expect(fetcher.validateCalls).To(Equal(1))
	})

	It("returns a generic error when the bundle match fails for a non-mismatch reason", func() {
		fetcher := &mockAttestationFetcher{validateErr: errors.New("network boom")}

		err := validation.VerifyBundleSource(context.Background(), fetcher, nil, withVerification)

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal(fmt.Sprintf("OPA bundle verification failed for %s", bundleURL)))
		Expect(errors.Is(err, validation.ErrSourceMismatch)).To(BeFalse())
		Expect(errors.Is(err, validation.ErrNoMatchingSigstoreBundleFound)).To(BeFalse())
		Expect(fetcher.validateCalls).To(Equal(1))
	})

	It("succeeds when the fetcher validates the Sigstore bundle successfully", func() {
		fetcher := &mockAttestationFetcher{}

		err := validation.VerifyBundleSource(context.Background(), fetcher, nil, withVerification)

		Expect(err).NotTo(HaveOccurred())
		Expect(fetcher.resolveCalls).To(Equal(1))
		Expect(fetcher.referrersCalls).To(Equal(1))
		Expect(fetcher.validateCalls).To(Equal(1))
	})
})

var _ = Describe("DefaultAttestationFetcher", func() {
	It("returns an error from ResolveOciRepositoryAndDigest for a malformed reference", func() {
		_, err := validation.DefaultAttestationFetcher{}.ResolveOciRepositoryAndDigest(
			context.Background(),
			nil,
			"not::a::valid::reference",
		)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("failed parsing OCI reference"))
	})
})
