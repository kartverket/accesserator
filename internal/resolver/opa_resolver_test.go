package resolver_test

import (
	"context"
	"errors"
	"os"

	accesseratorv1alpha "github.com/kartverket/accesserator/api/v1alpha"
	"github.com/kartverket/accesserator/internal/resolver"
	"github.com/kartverket/accesserator/pkg/config"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"oras.land/oras-go/v2/registry/remote/credentials"
)

type mockBundleFetcher struct {
	data      []byte
	err       error
	credStore credentials.Store
	reference string
}

func (m *mockBundleFetcher) Fetch(_ context.Context, credStore credentials.Store, reference string) ([]byte, error) {
	m.credStore = credStore
	m.reference = reference
	return m.data, m.err
}

var _ = Describe("OPA Resolver", func() {
	const allowedPrefix = "https://allowed/"

	BeforeEach(func() {
		Expect(os.Setenv("ACCESSERATOR_OPA_ALLOWED_BUNDLE_REGISTRY_URL_PREFIXES", allowedPrefix)).To(Succeed())
		Expect(config.Load()).To(Succeed())
	})

	Describe("ValidateBundleURLs", func() {
		It("returns nil when all bundle URLs have allowed prefixes", func() {
			bundles := []accesseratorv1alpha.BundleSource{
				{Name: "bundle", URL: allowedPrefix + "repo:tag"},
			}

			Expect(resolver.ValidateBundleURLs(bundles)).To(Succeed())
		})

		It("returns error when any bundle URL has a disallowed prefix", func() {
			bundles := []accesseratorv1alpha.BundleSource{
				{Name: "bundle", URL: "https://forbidden/repo:tag"},
			}

			err := resolver.ValidateBundleURLs(bundles)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("bundle URLs are not allowed"))
			Expect(err.Error()).To(ContainSubstring("https://forbidden/repo:tag"))
		})
	})

	Describe("ResolveOpaConfigWithFetcher", func() {
		It("returns bundle data from the injected fetcher", func() {
			fetcher := &mockBundleFetcher{data: []byte("bundle-bytes")}
			sc := accesseratorv1alpha.SecurityConfig{
				Spec: accesseratorv1alpha.SecurityConfigSpec{
					Opa: &accesseratorv1alpha.OpenPolicyAgentSpec{
						Enabled: true,
						BundleURLs: []accesseratorv1alpha.BundleSource{
							{Name: "bundle", URL: allowedPrefix + "repo:tag"},
						},
					},
				},
			}

			result, err := resolver.ResolveOpaConfigWithFetcher(fetcher, sc)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Enabled).To(BeTrue())
			Expect(result.BundleBinaryData).To(HaveKeyWithValue("bundle", []byte("bundle-bytes")))
			Expect(fetcher.credStore).NotTo(BeNil())
			Expect(fetcher.reference).To(Equal(allowedPrefix + "repo:tag"))
		})

		It("propagates errors from the injected fetcher", func() {
			fetcher := &mockBundleFetcher{err: errors.New("fetch-failed")}
			sc := accesseratorv1alpha.SecurityConfig{
				Spec: accesseratorv1alpha.SecurityConfigSpec{
					Opa: &accesseratorv1alpha.OpenPolicyAgentSpec{
						Enabled: true,
						BundleURLs: []accesseratorv1alpha.BundleSource{
							{Name: "bundle", URL: allowedPrefix + "repo:tag"},
						},
					},
				},
			}

			result, err := resolver.ResolveOpaConfigWithFetcher(fetcher, sc)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("fetch-failed"))
			Expect(result).To(BeNil())
			Expect(fetcher.credStore).NotTo(BeNil())
			Expect(fetcher.reference).To(Equal(allowedPrefix + "repo:tag"))
		})
	})

	It("throws an error when ACCESSERATOR_OPA_ENABLED is set to 'false' and a SecurityConfig with 'spec.opa' is provided", func() {
		Expect(os.Setenv("ACCESSERATOR_OPA_ENABLED", "false")).To(Succeed())
		Expect(config.Load()).To(Succeed())
		DeferCleanup(func() {
			Expect(os.Setenv("ACCESSERATOR_OPA_ENABLED", "true")).To(Succeed())
			Expect(config.Load()).To(Succeed())
		})

		sc := accesseratorv1alpha.SecurityConfig{
			Spec: accesseratorv1alpha.SecurityConfigSpec{
				Opa: &accesseratorv1alpha.OpenPolicyAgentSpec{
					Enabled: false,
					BundleURLs: []accesseratorv1alpha.BundleSource{
						{Name: "bundle", URL: allowedPrefix + "repo:tag"},
					},
				},
			},
		}
		_, err := resolver.ResolveOpaConfig(sc)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring(
			"OPA is not enabled on this cluster and 'spec.opa' can therefore not be set",
		))
	})
})
