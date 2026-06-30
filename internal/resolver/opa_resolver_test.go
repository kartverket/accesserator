package resolver_test

import (
	"context"
	"errors"
	"fmt"
	"os"

	accesseratorv1alpha "github.com/kartverket/accesserator/api/v1alpha"
	"github.com/kartverket/accesserator/internal/resolver"
	"github.com/kartverket/accesserator/pkg/config"
	"github.com/kartverket/accesserator/pkg/validation"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"oras.land/oras-go/v2/registry/remote/credentials"
)

// mockBundleFetcher satisfies resolver.BundleFetcher (which embeds
// validation.AttestationFetcher). Fields tune what each call returns; call
// counters and last-seen arguments are recorded for assertions.
type mockBundleFetcher struct {
	manifestDigest  string
	layerData       []byte
	attestationData []byte

	resolveErr error
	fetchErr   error
	attestErr  error

	resolveCalls int
	fetchCalls   int
	attestCalls  int

	lastCredStore credentials.Store
	lastReference string
}

func (m *mockBundleFetcher) Resolve(_ context.Context, credStore credentials.Store, reference string) (string, error) {
	m.resolveCalls++
	m.lastCredStore = credStore
	m.lastReference = reference
	if m.manifestDigest == "" {
		m.manifestDigest = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
	}
	return m.manifestDigest, m.resolveErr
}

func (m *mockBundleFetcher) FetchLayer(_ context.Context, _ credentials.Store, _ string, _ string) ([]byte, error) {
	m.fetchCalls++
	return m.layerData, m.fetchErr
}

func (m *mockBundleFetcher) FetchAttestation(_ context.Context, _ credentials.Store, _ string, _ string) ([]byte, error) {
	m.attestCalls++
	return m.attestationData, m.attestErr
}

// Compile-time check that mockBundleFetcher satisfies the interfaces.
var (
	_ resolver.BundleFetcher        = (*mockBundleFetcher)(nil)
	_ validation.AttestationFetcher = (*mockBundleFetcher)(nil)
)

var _ = Describe("OPA Resolver", func() {
	const allowedPrefix = "https://allowed/"
	const skiperatorApp = "app"

	BeforeEach(func() {
		Expect(os.Setenv("ACCESSERATOR_OPA_ALLOWED_BUNDLE_REGISTRY_URL_PREFIXES", allowedPrefix)).To(Succeed())
		Expect(os.Setenv("ACCESSERATOR_OPA_ENABLED", "true")).To(Succeed())
		Expect(config.Load()).To(Succeed())
	})

	// makeSecurityConfig builds a SecurityConfig for tests. Pass nil for opaSpec to leave
	// spec.opa unset.
	makeSecurityConfig := func(opaSpec *accesseratorv1alpha.OpenPolicyAgentSpec) accesseratorv1alpha.SecurityConfig {
		return accesseratorv1alpha.SecurityConfig{
			Spec: accesseratorv1alpha.SecurityConfigSpec{
				Opa:            opaSpec,
				ApplicationRef: skiperatorApp,
			},
		}
	}

	Describe("ResolveOpaConfigWithFetcher", func() {
		Context("when spec.opa is not set", func() {
			It("returns Enabled=false and does not touch the fetcher", func() {
				fetcher := &mockBundleFetcher{}

				result, err := resolver.ResolveOpaConfigWithFetcher(logger, fetcher, makeSecurityConfig(nil))

				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.Enabled).To(BeFalse())
				Expect(result.BundleBinaryData).To(BeEmpty())
				Expect(fetcher.resolveCalls).To(BeZero())
				Expect(fetcher.fetchCalls).To(BeZero())
			})
		})

		Context("when spec.opa.enabled is false", func() {
			It("returns Enabled=false and does not touch the fetcher", func() {
				fetcher := &mockBundleFetcher{}
				sc := makeSecurityConfig(&accesseratorv1alpha.OpenPolicyAgentSpec{
					Enabled: false,
					BundleURLs: []accesseratorv1alpha.BundleSource{
						{Name: "bundle", URL: allowedPrefix + "repo:tag"},
					},
				})

				result, err := resolver.ResolveOpaConfigWithFetcher(logger, fetcher, sc)

				Expect(err).NotTo(HaveOccurred())
				Expect(result.Enabled).To(BeFalse())
				Expect(fetcher.resolveCalls).To(BeZero())
				Expect(fetcher.fetchCalls).To(BeZero())
			})
		})

		Context("when spec.opa.enabled is true", func() {
			It("returns the layer content keyed by bundle name", func() {
				fetcher := &mockBundleFetcher{layerData: []byte("bundle-bytes")}
				sc := makeSecurityConfig(&accesseratorv1alpha.OpenPolicyAgentSpec{
					Enabled: true,
					BundleURLs: []accesseratorv1alpha.BundleSource{
						{Name: "bundle", URL: allowedPrefix + "repo:tag"},
					},
				})

				result, err := resolver.ResolveOpaConfigWithFetcher(logger, fetcher, sc)

				Expect(err).NotTo(HaveOccurred())
				Expect(result.Enabled).To(BeTrue())
				Expect(result.BundleBinaryData).To(HaveKeyWithValue("bundle", []byte("bundle-bytes")))
			})

			It("passes credStore and reference through to the fetcher", func() {
				fetcher := &mockBundleFetcher{layerData: []byte("x")}
				sc := makeSecurityConfig(&accesseratorv1alpha.OpenPolicyAgentSpec{
					Enabled: true,
					BundleURLs: []accesseratorv1alpha.BundleSource{
						{Name: "bundle", URL: allowedPrefix + "repo:tag"},
					},
				})

				_, err := resolver.ResolveOpaConfigWithFetcher(logger, fetcher, sc)

				Expect(err).NotTo(HaveOccurred())
				Expect(fetcher.lastCredStore).NotTo(BeNil())
				Expect(fetcher.lastReference).To(Equal(allowedPrefix + "repo:tag"))
			})

			It("resolves and fetches every bundle in the spec", func() {
				fetcher := &mockBundleFetcher{layerData: []byte("payload")}
				sc := makeSecurityConfig(&accesseratorv1alpha.OpenPolicyAgentSpec{
					Enabled: true,
					BundleURLs: []accesseratorv1alpha.BundleSource{
						{Name: "a", URL: allowedPrefix + "repo-a:tag"},
						{Name: "b", URL: allowedPrefix + "repo-b:tag"},
						{Name: "c", URL: allowedPrefix + "repo-c:tag"},
					},
				})

				result, err := resolver.ResolveOpaConfigWithFetcher(logger, fetcher, sc)

				Expect(err).NotTo(HaveOccurred())
				Expect(result.BundleBinaryData).To(HaveLen(3))
				Expect(result.BundleBinaryData).To(HaveKey("a"))
				Expect(result.BundleBinaryData).To(HaveKey("b"))
				Expect(result.BundleBinaryData).To(HaveKey("c"))
				Expect(fetcher.resolveCalls).To(Equal(3))
				Expect(fetcher.fetchCalls).To(Equal(3))
			})

			It("rejects a bundle URL that doesn't match the allowed prefix", func() {
				fetcher := &mockBundleFetcher{}
				sc := makeSecurityConfig(&accesseratorv1alpha.OpenPolicyAgentSpec{
					Enabled: true,
					BundleURLs: []accesseratorv1alpha.BundleSource{
						{Name: "bundle", URL: "https://forbidden/repo:tag"},
					},
				})

				result, err := resolver.ResolveOpaConfigWithFetcher(logger, fetcher, sc)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid OPA bundle URLs"))
				Expect(result).To(BeNil())
				Expect(fetcher.resolveCalls).To(BeZero())
			})

			It("propagates an error from Resolve and does not pull the layer", func() {
				fetcher := &mockBundleFetcher{resolveErr: errors.New("resolve-failed")}
				sc := makeSecurityConfig(&accesseratorv1alpha.OpenPolicyAgentSpec{
					Enabled: true,
					BundleURLs: []accesseratorv1alpha.BundleSource{
						{Name: "bundle", URL: allowedPrefix + "repo:tag"},
					},
				})

				result, err := resolver.ResolveOpaConfigWithFetcher(logger, fetcher, sc)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("resolve-failed"))
				Expect(err.Error()).To(ContainSubstring(allowedPrefix + "repo:tag"))
				Expect(result).To(BeNil())
				Expect(fetcher.fetchCalls).To(BeZero())
			})

			It("propagates an error from FetchLayer", func() {
				fetcher := &mockBundleFetcher{fetchErr: errors.New("fetch-failed")}
				sc := makeSecurityConfig(&accesseratorv1alpha.OpenPolicyAgentSpec{
					Enabled: true,
					BundleURLs: []accesseratorv1alpha.BundleSource{
						{Name: "bundle", URL: allowedPrefix + "repo:tag"},
					},
				})

				result, err := resolver.ResolveOpaConfigWithFetcher(logger, fetcher, sc)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fetch-failed"))
				Expect(result).To(BeNil())
			})

			It("stops at the first failing bundle and doesn't process later ones", func() {
				fetcher := &mockBundleFetcher{resolveErr: errors.New("resolve-failed")}
				sc := makeSecurityConfig(&accesseratorv1alpha.OpenPolicyAgentSpec{
					Enabled: true,
					BundleURLs: []accesseratorv1alpha.BundleSource{
						{Name: "a", URL: allowedPrefix + "repo-a:tag"},
						{Name: "b", URL: allowedPrefix + "repo-b:tag"},
					},
				})

				_, err := resolver.ResolveOpaConfigWithFetcher(logger, fetcher, sc)

				Expect(err).To(HaveOccurred())
				Expect(fetcher.resolveCalls).To(Equal(1))
			})
		})

		Context("when verification is set on a bundle", func() {
			// We can't end-to-end test a successful verification in a unit
			// test (it would need a real Sigstore bundle + reachable TUF +
			// reachable Rekor). These tests cover that the verification step
			// is attempted and that failures bubble up correctly.

			validSourceConfig := func() *accesseratorv1alpha.OpenPolicyAgentSpec {
				return &accesseratorv1alpha.OpenPolicyAgentSpec{
					Enabled: true,
					BundleURLs: []accesseratorv1alpha.BundleSource{{
						Name: "bundle",
						URL:  allowedPrefix + "repo:tag",
						Verification: &accesseratorv1alpha.BundleSourceVerification{
							Source: accesseratorv1alpha.GitHubRepositorySource{
								Repository: "kartverket/accesserator",
							},
						},
					}},
				}
			}

			It("calls FetchAttestation before pulling the layer", func() {
				fetcher := &mockBundleFetcher{
					attestErr: errors.New("attestation-fetch-failed"),
				}

				sc := makeSecurityConfig(validSourceConfig())
				_, err := resolver.ResolveOpaConfigWithFetcher(logger, fetcher, sc)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal(
					fmt.Sprintf(
						"failed to verify OCI bundle from %s: "+
							"failed to fetch cosign bundle for %s",
						sc.Spec.Opa.BundleURLs[0].URL,
						sc.Spec.Opa.BundleURLs[0].URL,
					),
				))
				Expect(fetcher.attestCalls).To(Equal(1))
				Expect(fetcher.fetchCalls).To(BeZero())
			})

			It("propagates the failure when the attestation bytes aren't a valid sigstore bundle", func() {
				fetcher := &mockBundleFetcher{
					attestationData: []byte("not-a-sigstore-bundle"),
				}

				_, err := resolver.ResolveOpaConfigWithFetcher(logger, fetcher, makeSecurityConfig(validSourceConfig()))

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to verify OCI bundle"))
				Expect(fetcher.fetchCalls).To(BeZero())
			})
		})
	})

	Describe("ResolveOpaConfig", func() {
		Context("when ACCESSERATOR_OPA_ENABLED is false", func() {
			BeforeEach(func() {
				Expect(os.Setenv("ACCESSERATOR_OPA_ENABLED", "false")).To(Succeed())
				Expect(config.Load()).To(Succeed())
			})

			AfterEach(func() {
				Expect(os.Setenv("ACCESSERATOR_OPA_ENABLED", "true")).To(Succeed())
				Expect(config.Load()).To(Succeed())
			})

			It("returns an error when spec.opa is set", func() {
				sc := makeSecurityConfig(&accesseratorv1alpha.OpenPolicyAgentSpec{
					Enabled: false,
					BundleURLs: []accesseratorv1alpha.BundleSource{
						{Name: "bundle", URL: allowedPrefix + "repo:tag"},
					},
				})

				_, err := resolver.ResolveOpaConfig(logger, sc)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(
					"OPA is not enabled on this cluster and 'spec.opa' can therefore not be set",
				))
			})

			It("returns Enabled=false when spec.opa is omitted", func() {
				result, err := resolver.ResolveOpaConfig(logger, makeSecurityConfig(nil))

				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.Enabled).To(BeFalse())
			})
		})

		Context("when ACCESSERATOR_OPA_ENABLED is true", func() {
			It("returns Enabled=false when spec.opa is omitted", func() {
				result, err := resolver.ResolveOpaConfig(logger, makeSecurityConfig(nil))

				Expect(err).NotTo(HaveOccurred())
				Expect(result.Enabled).To(BeFalse())
			})

			It("returns Enabled=false when spec.opa.enabled is false", func() {
				sc := makeSecurityConfig(&accesseratorv1alpha.OpenPolicyAgentSpec{
					Enabled: false,
				})

				result, err := resolver.ResolveOpaConfig(logger, sc)

				Expect(err).NotTo(HaveOccurred())
				Expect(result.Enabled).To(BeFalse())
			})

			// The "spec.opa.enabled=true" branch of ResolveOpaConfig delegates
			// to ResolveOpaConfigWithFetcher with the default OCI-backed
			// fetcher, which would hit the network. Coverage of that branch
			// lives in the ResolveOpaConfigWithFetcher tests above.
		})
	})
})
