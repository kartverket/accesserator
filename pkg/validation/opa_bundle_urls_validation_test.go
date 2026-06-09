package validation_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/kartverket/accesserator/api/v1alpha"
	"github.com/kartverket/accesserator/pkg/config"
	"github.com/kartverket/accesserator/pkg/validation"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/credentials"
)

// mockAttestationFetcher implements validation.AttestationFetcher with
// canned return values + call counters.
type mockAttestationFetcher struct {
	manifestDigest  string
	attestationData []byte
	resolveErr      error
	attestErr       error

	resolveCalls int
	attestCalls  int
}

func (m *mockAttestationFetcher) Resolve(_ context.Context, _ credentials.Store, _ string) (string, error) {
	m.resolveCalls++
	if m.manifestDigest == "" {
		m.manifestDigest = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
	}
	return m.manifestDigest, m.resolveErr
}

func (m *mockAttestationFetcher) FetchAttestation(
	_ context.Context,
	_ credentials.Store,
	_ string,
	_ string,
) ([]byte, error) {
	m.attestCalls++
	return m.attestationData, m.attestErr
}

var _ validation.AttestationFetcher = (*mockAttestationFetcher)(nil)

var _ = Describe("ValidateBundleUrls", func() {
	const allowedPrefix = "https://allowed/"

	BeforeEach(func() {
		Expect(os.Setenv("ACCESSERATOR_OPA_ALLOWED_BUNDLE_REGISTRY_URL_PREFIXES", allowedPrefix)).To(Succeed())
		Expect(config.Load()).To(Succeed())
	})

	It("returns an error when bundleURLs is nil", func() {
		err := validation.ValidateBundleUrls(nil)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("cannot be nil or empty"))
	})

	It("returns an error when bundleURLs is empty", func() {
		err := validation.ValidateBundleUrls([]v1alpha.BundleSource{})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("cannot be nil or empty"))
	})

	It("returns nil when every URL matches an allowed prefix", func() {
		bundles := []v1alpha.BundleSource{
			{Name: "a", URL: allowedPrefix + "foo:tag"},
			{Name: "b", URL: allowedPrefix + "bar:tag"},
		}
		Expect(validation.ValidateBundleUrls(bundles)).To(Succeed())
	})

	It("returns an error naming the disallowed URL", func() {
		bundles := []v1alpha.BundleSource{
			{Name: "a", URL: "https://forbidden/repo:tag"},
		}
		err := validation.ValidateBundleUrls(bundles)
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
		err := validation.ValidateBundleUrls(bundles)
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
		Expect(validation.ValidateBundleUrls(bundles)).To(Succeed())
	})
})

var _ = Describe("ValidateBundleSourceSignature", func() {
	const bundleURL = "https://allowed/repo:tag"

	withVerification := v1alpha.BundleSource{
		Name: "bundle",
		URL:  bundleURL,
		Verification: &v1alpha.BundleSourceVerification{
			Source: v1alpha.GitHubRepositorySource{Repository: "kartverket/accesserator"},
		},
	}

	It("returns an error when Verification is omitted", func() {
		fetcher := &mockAttestationFetcher{}
		bundleSource := v1alpha.BundleSource{
			Name: "bundle",
			URL:  bundleURL,
			// Verification omitted
		}

		err := validation.ValidateBundleSourceSignature(context.Background(), fetcher, nil, bundleSource)

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("bundle source cannot be nil and must have a repository"))
		Expect(fetcher.resolveCalls).To(BeZero())
		Expect(fetcher.attestCalls).To(BeZero())
	})

	It("returns an error when Verification.Source.Repository is empty", func() {
		fetcher := &mockAttestationFetcher{}
		bundleSource := v1alpha.BundleSource{
			Name: "bundle",
			URL:  bundleURL,
			Verification: &v1alpha.BundleSourceVerification{
				Source: v1alpha.GitHubRepositorySource{Repository: ""},
			},
		}

		err := validation.ValidateBundleSourceSignature(context.Background(), fetcher, nil, bundleSource)

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("bundle source cannot be nil and must have a repository"))
		Expect(fetcher.resolveCalls).To(BeZero())
		Expect(fetcher.attestCalls).To(BeZero())
	})

	It("returns an error when Resolve fails and skips the attestation fetch", func() {
		fetcher := &mockAttestationFetcher{resolveErr: errors.New("resolve-failed")}

		err := validation.ValidateBundleSourceSignature(context.Background(), fetcher, nil, withVerification)

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal(
			fmt.Sprintf("failed to resolve OCI bundle digest for %s", bundleURL),
		))
		Expect(fetcher.attestCalls).To(BeZero())
	})

	It("returns an error when FetchAttestation fails", func() {
		fetcher := &mockAttestationFetcher{attestErr: errors.New("attestation-failed")}

		err := validation.ValidateBundleSourceSignature(context.Background(), fetcher, nil, withVerification)

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal(
			fmt.Sprintf("failed to fetch cosign bundle for %s", bundleURL),
		))
		Expect(fetcher.resolveCalls).To(Equal(1))
		Expect(fetcher.attestCalls).To(Equal(1))
	})

	It("returns an error when the attestation bytes aren't a valid Sigstore bundle JSON", func() {
		fetcher := &mockAttestationFetcher{attestationData: []byte("not a sigstore bundle")}

		err := validation.ValidateBundleSourceSignature(context.Background(), fetcher, nil, withVerification)

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("parse cosign bundle"))
	})
})

var _ = Describe("ValidateBundleSignature", func() {
	It("returns an error when src.repository is empty", func() {
		err := validation.ValidateBundleSignature(
			logger,
			nil, // bundle — never reached
			nil, // digest — never reached
			v1alpha.GitHubRepositorySource{Repository: ""},
			nil, // trustedRoot — never reached
		)

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("src.Repository is required"))
	})
})

var _ = Describe("DecodeManifestDigest", func() {
	It("decodes a valid sha256 digest", func() {
		got, err := validation.DecodeManifestDigest("sha256:0000000000000000000000000000000000000000000000000000000000000000")
		Expect(err).NotTo(HaveOccurred())
		Expect(got).To(HaveLen(32))
		for _, b := range got {
			Expect(b).To(BeEquivalentTo(0))
		}
	})

	It("decodes a non-zero sha256 digest into the expected bytes", func() {
		got, err := validation.DecodeManifestDigest("sha256:8f3a1b9c2d4e5f600102030405060708090a0b0c0d0e0f10111213141516170a")
		Expect(err).NotTo(HaveOccurred())
		Expect(got).To(HaveLen(32))
		Expect(got[0]).To(BeEquivalentTo(0x8f))
		Expect(got[31]).To(BeEquivalentTo(0x0a))
	})

	It("returns an error for a digest with the wrong algorithm prefix", func() {
		_, err := validation.DecodeManifestDigest("sha512:" + strings.Repeat("a", 128))
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("only sha256 supported"))
	})

	It("returns an error when the hex portion contains invalid characters", func() {
		_, err := validation.DecodeManifestDigest("sha256:zz" + strings.Repeat("0", 62))
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("error when decoding manifest digest hex"))
	})
})

var _ = Describe("BuildGitHubSANRegex", func() {
	It("returns an error for a nil org list", func() {
		_, err := validation.BuildGitHubSANRegex(nil)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("at least one org is required"))
	})

	Context("with a single org", func() {
		var re *regexp.Regexp

		BeforeEach(func() {
			pattern, err := validation.BuildGitHubSANRegex([]string{"kartverket"})
			Expect(err).NotTo(HaveOccurred())
			re = regexp.MustCompile(pattern)
		})

		It("matches a workflow SAN for the org with a .yml extension", func() {
			Expect(re.MatchString(
				"https://github.com/kartverket/accesserator/.github/workflows/build.yml@refs/heads/main",
			)).To(BeTrue())
		})

		It("matches a workflow SAN with a .yaml extension", func() {
			Expect(re.MatchString(
				"https://github.com/kartverket/accesserator/.github/workflows/build.yaml@refs/heads/main",
			)).To(BeTrue())
		})

		It("matches a workflow SAN on a tag ref", func() {
			Expect(re.MatchString(
				"https://github.com/kartverket/accesserator/.github/workflows/build.yml@refs/tags/v1.2.3",
			)).To(BeTrue())
		})

		It("matches a reusable workflow SAN as long as the owning org is allowed", func() {
			Expect(re.MatchString(
				"https://github.com/kartverket/github-workflows/.github/workflows/build-and-push-opa-rules.yml@refs/heads/main",
			)).To(BeTrue())
		})

		It("rejects a SAN whose org is not in the allowlist", func() {
			Expect(re.MatchString(
				"https://github.com/attacker/accesserator/.github/workflows/build.yml@refs/heads/main",
			)).To(BeFalse())
		})

		It("rejects a SAN whose host has been spoofed with a path", func() {
			// Without the `^` anchor, this would match by accident.
			Expect(re.MatchString(
				"https://malicious.example/github.com/kartverket/accesserator/.github/workflows/build.yml@refs/heads/main",
			)).To(BeFalse())
		})

		It("rejects a SAN where the dot in 'github.com' is replaced", func() {
			// Catches unescaped `.` in the pattern.
			Expect(re.MatchString(
				"https://githubXcom/kartverket/accesserator/.github/workflows/build.yml@refs/heads/main",
			)).To(BeFalse())
		})

		It("rejects a SAN without a workflow path", func() {
			Expect(re.MatchString("https://github.com/kartverket/accesserator")).To(BeFalse())
		})

		It("rejects a SAN with an unsupported file extension", func() {
			Expect(re.MatchString(
				"https://github.com/kartverket/accesserator/.github/workflows/build.txt@refs/heads/main",
			)).To(BeFalse())
		})

		It("rejects a SAN without a ref portion", func() {
			Expect(re.MatchString(
				"https://github.com/kartverket/accesserator/.github/workflows/build.yml",
			)).To(BeFalse())
		})

		It("rejects a SAN with an empty repo", func() {
			Expect(re.MatchString(
				"https://github.com/kartverket//.github/workflows/build.yml@refs/heads/main",
			)).To(BeFalse())
		})
	})

	Context("with multiple orgs", func() {
		var re *regexp.Regexp

		BeforeEach(func() {
			pattern, err := validation.BuildGitHubSANRegex([]string{"kartverket", "accesserator"})
			Expect(err).NotTo(HaveOccurred())
			re = regexp.MustCompile(pattern)
		})

		It("matches a SAN from the first org", func() {
			Expect(re.MatchString(
				"https://github.com/kartverket/accesserator/.github/workflows/build.yml@refs/heads/main",
			)).To(BeTrue())
		})

		It("matches a SAN from the second org", func() {
			Expect(re.MatchString(
				"https://github.com/accesserator/foo/.github/workflows/x.yml@refs/tags/v1",
			)).To(BeTrue())
		})

		It("rejects a SAN from an org that is not in the allowlist", func() {
			Expect(re.MatchString(
				"https://github.com/attacker/repo/.github/workflows/build.yml@refs/heads/main",
			)).To(BeFalse())
		})

		It("does not match an org name that is a substring of an allowed org", func() {
			// "accesserator" should not also match "accesserator-evil"
			Expect(re.MatchString(
				"https://github.com/accesserator-evil/foo/.github/workflows/build.yml@refs/heads/main",
			)).To(BeFalse())
		})
	})

	It("escapes regex metacharacters in org names so they are matched literally", func() {
		// `.` is a regex wildcard. If it weren't escaped, "a.b" would match
		// "axb". With QuoteMeta it must match the literal "a.b".
		pattern, err := validation.BuildGitHubSANRegex([]string{"a.b"})
		Expect(err).NotTo(HaveOccurred())
		re := regexp.MustCompile(pattern)

		Expect(re.MatchString(
			"https://github.com/a.b/foo/.github/workflows/build.yml@refs/heads/main",
		)).To(BeTrue())
		Expect(re.MatchString(
			"https://github.com/axb/foo/.github/workflows/build.yml@refs/heads/main",
		)).To(BeFalse())
	})

	It("returns a regex that compiles via Go's regexp engine", func() {
		pattern, err := validation.BuildGitHubSANRegex([]string{"kartverket"})
		Expect(err).NotTo(HaveOccurred())
		_, compileErr := regexp.Compile(pattern)
		Expect(compileErr).NotTo(HaveOccurred())
	})
})

var _ = Describe("NewAuthedRepo", func() {
	It("returns a repo with an authed client for a well-formed reference", func() {
		credStore, err := credentials.NewStoreFromDocker(credentials.StoreOptions{})
		Expect(err).NotTo(HaveOccurred())

		repo, err := validation.NewAuthedRepo(credStore, "ghcr.io/kartverket/accesserator/opa-bundle:latest")

		Expect(err).NotTo(HaveOccurred())
		Expect(repo).NotTo(BeNil())
		Expect(repo.Client).NotTo(BeNil())

		authClient, ok := repo.Client.(*auth.Client)
		Expect(ok).To(BeTrue())
		Expect(authClient.Cache).NotTo(BeNil())
		Expect(authClient.Credential).NotTo(BeNil())
	})

	It("parses the reference's registry and repository correctly", func() {
		repo, err := validation.NewAuthedRepo(nil, "ghcr.io/kartverket/accesserator/opa-bundle:latest")
		Expect(err).NotTo(HaveOccurred())
		Expect(repo.Reference.Registry).To(Equal("ghcr.io"))
		Expect(repo.Reference.Repository).To(Equal("kartverket/accesserator/opa-bundle"))
		Expect(repo.Reference.Reference).To(Equal("latest"))
	})

	It("returns an error when the reference is empty", func() {
		_, err := validation.NewAuthedRepo(nil, "")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("failed parsing OCI reference"))
	})

	It("returns an error when the reference is malformed", func() {
		_, err := validation.NewAuthedRepo(nil, "not::a::valid::reference")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("failed parsing OCI reference"))
	})
})
