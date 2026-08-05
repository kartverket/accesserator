package validation_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"

	"github.com/kartverket/accesserator/internal/model"
	"github.com/kartverket/accesserator/pkg/utilities"
	"github.com/kartverket/accesserator/pkg/validation"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/registry/remote"
)

var _ = Describe("ValidateSigstoreBundleSignature", func() {
	It("returns an error when the bundle cannot produce a verifier", func() {
		// A nil bundle produces no verifier options, so GetBundleVerifier fails
		// before any signature verification is attempted.
		err := validation.ValidateSigstoreBundleSignature(logger, nil, nil)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("failed to get bundle verifier"))
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

var _ = Describe("SatisfiesVerificationSource", func() {
	full := model.OpaBundleSource{
		Repository: "kartverket/accesserator",
		Workflow:   ".github/workflows/build.yml",
		Ref:        "refs/heads/main",
	}

	It("matches when only the repository is expected and it is equal", func() {
		expected := model.OpaBundleSource{Repository: "kartverket/accesserator"}
		Expect(validation.SatisfiesVerificationSource(full, expected)).To(BeTrue())
	})

	It("does not match when the repository differs", func() {
		expected := model.OpaBundleSource{Repository: "kartverket/other"}
		Expect(validation.SatisfiesVerificationSource(full, expected)).To(BeFalse())
	})

	It("ignores workflow and ref when they are not specified in the expected source", func() {
		actual := model.OpaBundleSource{
			Repository: "kartverket/accesserator",
			Workflow:   ".github/workflows/anything.yml",
			Ref:        "refs/tags/v9",
		}
		expected := model.OpaBundleSource{Repository: "kartverket/accesserator"}
		Expect(validation.SatisfiesVerificationSource(actual, expected)).To(BeTrue())
	})

	It("requires the ref to match when the expected source specifies one", func() {
		expected := model.OpaBundleSource{
			Repository: "kartverket/accesserator",
			Ref:        "refs/heads/main",
		}
		Expect(validation.SatisfiesVerificationSource(full, expected)).To(BeTrue())

		expected.Ref = "refs/heads/other"
		Expect(validation.SatisfiesVerificationSource(full, expected)).To(BeFalse())
	})

	It("requires the workflow to match when the expected source specifies one", func() {
		expected := model.OpaBundleSource{
			Repository: "kartverket/accesserator",
			Workflow:   ".github/workflows/build.yml",
		}
		Expect(validation.SatisfiesVerificationSource(full, expected)).To(BeTrue())

		expected.Workflow = ".github/workflows/other.yml"
		Expect(validation.SatisfiesVerificationSource(full, expected)).To(BeFalse())
	})

	It("matches when repository, workflow and ref are all specified and equal", func() {
		Expect(validation.SatisfiesVerificationSource(full, full)).To(BeTrue())
	})
})

var _ = Describe("DefaultAttestationFetcher.GetSigstoreProvenanceReferrers", func() {
	const referrersRepo = "test/repo"
	subjectDigest := "sha256:" + strings.Repeat("0", 64)

	// referrersServer serves only the OCI Referrers API, returning the given
	// descriptors as an image index for the subject digest.
	referrersServer := func(referrers []ocispec.Descriptor) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.URL.Path == "/v2/":
				w.WriteHeader(http.StatusOK)
			case strings.HasPrefix(r.URL.Path, "/v2/"+referrersRepo+"/referrers/"):
				index := ocispec.Index{MediaType: ocispec.MediaTypeImageIndex, Manifests: referrers}
				raw, err := json.Marshal(index)
				Expect(err).NotTo(HaveOccurred())
				w.Header().Set("Content-Type", ocispec.MediaTypeImageIndex)
				_, _ = w.Write(raw)
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
	}

	repoAndDigestFor := func(server *httptest.Server) utilities.OciRepositoryAndDigest {
		u, err := url.Parse(server.URL)
		Expect(err).NotTo(HaveOccurred())
		repo, err := remote.NewRepository(u.Host + "/" + referrersRepo)
		Expect(err).NotTo(HaveOccurred())
		repo.PlainHTTP = true
		return utilities.OciRepositoryAndDigest{Repository: repo, Digest: subjectDigest}
	}

	provenanceReferrer := ocispec.Descriptor{
		MediaType:    validation.SigstoreBundleMediaType,
		ArtifactType: validation.SigstoreBundleMediaType,
		Digest:       digest.FromString("provenance"),
		Size:         1,
		Annotations: map[string]string{
			validation.SigstoreBundleContentAnnotationKey:       validation.SigstoreBundleContentAnnotationValue,
			validation.SigstoreBundlePredicateTypeAnnotationKey: validation.SigstoreSlsaProvenanceV1,
		},
	}

	It("returns referrers whose artifact type and annotations identify SLSA provenance", func() {
		// A second referrer with the right artifact type but wrong predicate
		// annotation must be filtered out.
		otherReferrer := ocispec.Descriptor{
			MediaType:    validation.SigstoreBundleMediaType,
			ArtifactType: validation.SigstoreBundleMediaType,
			Digest:       digest.FromString("other"),
			Size:         1,
			Annotations: map[string]string{
				validation.SigstoreBundleContentAnnotationKey:       validation.SigstoreBundleContentAnnotationValue,
				validation.SigstoreBundlePredicateTypeAnnotationKey: "https://slsa.dev/something-else",
			},
		}
		server := referrersServer([]ocispec.Descriptor{provenanceReferrer, otherReferrer})
		DeferCleanup(server.Close)

		got, err := validation.DefaultAttestationFetcher{}.GetSigstoreProvenanceReferrers(
			context.Background(),
			repoAndDigestFor(server),
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(got).To(HaveLen(1))
		Expect(got[0].Digest).To(Equal(provenanceReferrer.Digest))
	})

	It("returns an error when no referrer matches the provenance predicate", func() {
		server := referrersServer(nil)
		DeferCleanup(server.Close)

		_, err := validation.DefaultAttestationFetcher{}.GetSigstoreProvenanceReferrers(
			context.Background(),
			repoAndDigestFor(server),
		)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("no OCI 1.1 referrer found"))
	})
})

var _ = Describe("GetSigstoreBundleMatchingVerification", func() {
	verificationSource := model.OpaBundleSource{Repository: "kartverket/accesserator"}

	It("returns a source-mismatch error and no bundle when there are no referrers", func() {
		repoAndDig := utilities.OciRepositoryAndDigest{
			Digest: "sha256:" + "0000000000000000000000000000000000000000000000000000000000000000",
		}
		bundle, err := validation.GetSigstoreBundleMatchingVerification(
			context.Background(),
			repoAndDig,
			nil,
			verificationSource,
		)
		Expect(bundle).To(BeNil())
		Expect(err).To(HaveOccurred())
		Expect(errors.Is(err, validation.ErrSourceMismatch)).To(BeTrue())
	})

	It("aggregates fetch errors when a referrer's bundle cannot be pulled", func() {
		// The repository points at a server that has already been closed, so
		// pulling the referrer's bundle bytes fails and the error is aggregated.
		server := httptest.NewServer(nil)
		u, err := url.Parse(server.URL)
		Expect(err).NotTo(HaveOccurred())
		server.Close()

		repo, err := remote.NewRepository(u.Host + "/test/repo")
		Expect(err).NotTo(HaveOccurred())
		repo.PlainHTTP = true

		repoAndDig := utilities.OciRepositoryAndDigest{
			Repository: repo,
			Digest:     "sha256:" + "0000000000000000000000000000000000000000000000000000000000000000",
		}
		referrers := []ocispec.Descriptor{
			{
				MediaType: validation.SigstoreBundleMediaType,
				Digest:    digest.FromString("referrer"),
				Size:      1,
			},
		}

		bundle, err := validation.GetSigstoreBundleMatchingVerification(
			context.Background(),
			repoAndDig,
			referrers,
			verificationSource,
		)
		Expect(bundle).To(BeNil())
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("failed to fetch Sigstore bundle bytes"))
	})
})
