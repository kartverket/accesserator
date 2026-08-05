package validation_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/kartverket/accesserator/api/v1alpha"
	"github.com/kartverket/accesserator/internal/model"
	"github.com/kartverket/accesserator/pkg/config"
	"github.com/kartverket/accesserator/pkg/utilities"
	"github.com/kartverket/accesserator/pkg/validation"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	sigstorebundle "github.com/sigstore/sigstore-go/pkg/bundle"
	"oras.land/oras-go/v2/registry/remote/credentials"
)

// mockAttestationFetcher implements validation.AttestationFetcher with canned
// return values and call counters.
type mockAttestationFetcher struct {
	repoAndDigest *utilities.OciRepositoryAndDigest
	resolveErr    error

	referrers    []ocispec.Descriptor
	referrersErr error

	bundle    *sigstorebundle.Bundle
	bundleErr error

	resolveCalls   int
	referrersCalls int
	bundleCalls    int
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

func (m *mockAttestationFetcher) GetSigstoreProvenanceReferrers(
	_ context.Context,
	_ utilities.OciRepositoryAndDigest,
) ([]ocispec.Descriptor, error) {
	m.referrersCalls++
	return m.referrers, m.referrersErr
}

func (m *mockAttestationFetcher) GetSigstoreBundleMatchingVerificationSource(
	_ context.Context,
	_ utilities.OciRepositoryAndDigest,
	_ []ocispec.Descriptor,
	_ model.OpaBundleSource,
) (*sigstorebundle.Bundle, error) {
	m.bundleCalls++
	return m.bundle, m.bundleErr
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
		err := validation.ValidateBundleUrlPrefixes(v1alpha.GetURLs([]v1alpha.BundleSource{}))
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("cannot be nil or empty"))
	})

	It("returns nil when every URL matches an allowed prefix", func() {
		bundles := []v1alpha.BundleSource{
			{Name: "a", URL: allowedPrefix + "foo:tag"},
			{Name: "b", URL: allowedPrefix + "bar:tag"},
		}
		Expect(validation.ValidateBundleUrlPrefixes(v1alpha.GetURLs(bundles))).To(Succeed())
	})

	It("returns an error naming the disallowed URL", func() {
		bundles := []v1alpha.BundleSource{
			{Name: "a", URL: "https://forbidden/repo:tag"},
		}
		err := validation.ValidateBundleUrlPrefixes(v1alpha.GetURLs(bundles))
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("https://forbidden/repo:tag"))
		Expect(err.Error()).To(ContainSubstring("must start with one of"))
	})

	It("collects every disallowed URL in the error message", func() {
		bundles := []v1alpha.BundleSource{
			{Name: "a", URL: "https://forbidden/repo-a:tag"},
			{Name: "b", URL: allowedPrefix + "ok:tag"},
			{Name: "c", URL: "https://other-bad/repo-c:tag"},
		}
		err := validation.ValidateBundleUrlPrefixes(v1alpha.GetURLs(bundles))
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

		bundles := []v1alpha.BundleSource{
			{Name: "a", URL: "https://also-allowed/repo:tag"},
		}
		Expect(validation.ValidateBundleUrlPrefixes(v1alpha.GetURLs(bundles))).To(Succeed())
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
		bundles := []v1alpha.BundleSource{{Name: "bundle-a", URL: "https://allowed/repo-a:tag"}}

		err := validation.ValidateCollisionWithConfiguredSelfAuthBundle(bundles)

		Expect(err).NotTo(HaveOccurred())
	})

	It("returns nil when bundle names do not collide", func() {
		Expect(os.Setenv("ACCESSERATOR_OPA_SELF_AUTHORIZATION_BUNDLE", selfAuthorizationBundleJSON)).To(Succeed())
		Expect(config.Load()).To(Succeed())

		bundles := []v1alpha.BundleSource{{Name: "bundle-a", URL: "https://allowed/repo-a:tag"}}

		err := validation.ValidateCollisionWithConfiguredSelfAuthBundle(bundles)

		Expect(err).NotTo(HaveOccurred())
	})

	It("returns an error when a bundle name collides with the pre-configured self-authorization bundle", func() {
		Expect(os.Setenv("ACCESSERATOR_OPA_SELF_AUTHORIZATION_BUNDLE", selfAuthorizationBundleJSON)).To(Succeed())
		Expect(config.Load()).To(Succeed())

		bundles := []v1alpha.BundleSource{{
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
			fmt.Sprintf("failed to fetch Sigstore provenance referrers for %s", bundleURL),
		))
		Expect(fetcher.referrersCalls).To(Equal(1))
		Expect(fetcher.bundleCalls).To(BeZero())
	})

	It("propagates ErrSourceMismatch unchanged", func() {
		mismatch := fmt.Errorf("%w: the found source did not match", validation.ErrSourceMismatch)
		fetcher := &mockAttestationFetcher{bundleErr: mismatch}

		err := validation.VerifyBundleSource(context.Background(), fetcher, nil, withVerification)

		Expect(err).To(HaveOccurred())
		Expect(errors.Is(err, validation.ErrSourceMismatch)).To(BeTrue())
		Expect(fetcher.bundleCalls).To(Equal(1))
	})

	It("returns a generic error when the bundle match fails for a non-mismatch reason", func() {
		fetcher := &mockAttestationFetcher{bundleErr: errors.New("network boom")}

		err := validation.VerifyBundleSource(context.Background(), fetcher, nil, withVerification)

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("failed to fetch Sigstore bundle matching verification"))
		Expect(errors.Is(err, validation.ErrSourceMismatch)).To(BeFalse())
	})

	It("returns an error when the resolved digest cannot be decoded", func() {
		fetcher := &mockAttestationFetcher{
			repoAndDigest: &utilities.OciRepositoryAndDigest{Digest: "sha512:deadbeef"},
			bundle:        &sigstorebundle.Bundle{},
		}

		err := validation.VerifyBundleSource(context.Background(), fetcher, nil, withVerification)

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("failed to strip alg prefix artifact SHA256 for digest"))
	})

	It("proceeds to signature validation and fails when the bundle cannot be verified", func() {
		// A valid digest and a matching (but empty) Sigstore bundle exercise the
		// full happy path up to signature verification, which fails because the
		// empty bundle cannot produce a verifier.
		fetcher := &mockAttestationFetcher{
			repoAndDigest: &utilities.OciRepositoryAndDigest{
				Digest: "sha256:" + strings.Repeat("0", 64),
			},
			bundle: &sigstorebundle.Bundle{},
		}

		err := validation.VerifyBundleSource(context.Background(), fetcher, nil, withVerification)

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("failed to verify sigstore bundle signature"))
		Expect(fetcher.resolveCalls).To(Equal(1))
		Expect(fetcher.referrersCalls).To(Equal(1))
		Expect(fetcher.bundleCalls).To(Equal(1))
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
