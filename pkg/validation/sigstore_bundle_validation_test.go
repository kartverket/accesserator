package validation_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"

	"github.com/kartverket/accesserator/internal/model"
	"github.com/kartverket/accesserator/pkg/utilities"
	"github.com/kartverket/accesserator/pkg/validation"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sigstore/sigstore-go/pkg/verify"
	"oras.land/oras-go/v2/registry/remote"
)

var _ = Describe("VerifySigstoreBundleCertificate", func() {
	It("returns an error when the bundle cannot produce a verifier", func() {
		// A nil bundle produces no verifier options, so GetBundleVerifier fails
		// before any signature verification is attempted.
		summary, verifyErr := validation.VerifySigstoreBundleCertificate(
			logger, nil, nil, verify.CertificateIdentity{},
		)
		Expect(verifyErr).To(HaveOccurred())
		Expect(verifyErr.Error()).To(ContainSubstring("failed to get bundle verifier"))
		Expect(summary).To(BeNil())
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

		got, err := validation.DefaultAttestationFetcher{}.GetSLSAProvenanceReferrers(
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

		_, err := validation.DefaultAttestationFetcher{}.GetSLSAProvenanceReferrers(
			context.Background(),
			repoAndDigestFor(server),
		)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("no OCI 1.1 referrer found"))
	})
})

var _ = Describe("ValidateSigstoreBundlesMatchesExpectedSource", func() {
	verificationSource := model.OpaBundleSource{Repository: "kartverket/accesserator"}

	It("returns ErrNoMatchingSigstoreBundleFound when there are no referrers", func() {
		repoAndDig := utilities.OciRepositoryAndDigest{
			Digest: "sha256:" + "0000000000000000000000000000000000000000000000000000000000000000",
		}
		err := validation.ValidateSigstoreBundlesMatchesExpectedSource(
			context.Background(),
			repoAndDig,
			nil,
			verificationSource,
		)
		Expect(err).To(HaveOccurred())
		Expect(errors.Is(err, validation.ErrNoMatchingSigstoreBundleFound)).To(BeTrue())
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

		err = validation.ValidateSigstoreBundlesMatchesExpectedSource(
			context.Background(),
			repoAndDig,
			referrers,
			verificationSource,
		)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("failed to fetch Sigstore bundle bytes"))
	})
})
