package utilities_test

import (
	"github.com/kartverket/accesserator/internal/model"
	"github.com/kartverket/accesserator/pkg/config"
	"github.com/kartverket/accesserator/pkg/utilities"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	protobundle "github.com/sigstore/protobuf-specs/gen/pb-go/bundle/v1"
	protorekor "github.com/sigstore/protobuf-specs/gen/pb-go/rekor/v1"
	sigstorebundle "github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/sigstore/sigstore-go/pkg/fulcio/certificate"
)

var _ = Describe("GetRepositorySourceFromVerifiedSigstoreBundleCertificate", func() {
	It("returns an empty OpaBundleSource for a zero-value certificate summary", func() {
		Expect(utilities.GetRepositorySourceFromVerifiedSigstoreBundleCertificate(
			certificate.Summary{},
		)).To(Equal(model.OpaBundleSource{}))
	})

	It("extracts repository, workflow and ref from the certificate extensions", func() {
		summary := certificate.Summary{
			Extensions: certificate.Extensions{
				SourceRepositoryURI: "https://github.com/kartverket/accesserator",
				SourceRepositoryRef: "refs/heads/main",
				BuildConfigURI: "https://github.com/kartverket/accesserator" +
					"/.github/workflows/build.yml@refs/heads/main",
			},
		}

		source := utilities.GetRepositorySourceFromVerifiedSigstoreBundleCertificate(summary)

		Expect(source).To(Equal(model.OpaBundleSource{
			Repository: "kartverket/accesserator",
			Workflow:   ".github/workflows/build.yml",
			Ref:        "refs/heads/main",
		}))
	})

	It("leaves Workflow empty when BuildConfigURI has no '@<ref>' suffix", func() {
		summary := certificate.Summary{
			Extensions: certificate.Extensions{
				SourceRepositoryURI: "https://github.com/kartverket/accesserator",
				SourceRepositoryRef: "refs/tags/v1.2.3",
				BuildConfigURI:      "https://github.com/kartverket/accesserator/.github/workflows/build.yml",
			},
		}

		source := utilities.GetRepositorySourceFromVerifiedSigstoreBundleCertificate(summary)

		Expect(source.Repository).To(Equal("kartverket/accesserator"))
		Expect(source.Ref).To(Equal("refs/tags/v1.2.3"))
		Expect(source.Workflow).To(BeEmpty())
	})

	It("keeps the full BuildConfigURI path when the workflow lives in a different repo", func() {
		summary := certificate.Summary{
			Extensions: certificate.Extensions{
				SourceRepositoryURI: "https://github.com/kartverket/accesserator",
				SourceRepositoryRef: "refs/heads/main",
				// Reusable workflow lives in a different repo. The current
				// implementation only trims the SourceRepositoryURI prefix,
				// so the workflow path here does NOT match that prefix and
				// stays as the full workflow URI (minus the @<ref> suffix).
				BuildConfigURI: "https://github.com/kartverket/github-workflows" +
					"/.github/workflows/build-and-push-opa-rules.yml@refs/heads/main",
			},
		}

		source := utilities.GetRepositorySourceFromVerifiedSigstoreBundleCertificate(summary)

		Expect(source.Repository).To(Equal("kartverket/accesserator"))
		Expect(source.Ref).To(Equal("refs/heads/main"))
		Expect(source.Workflow).To(Equal(
			"https://github.com/kartverket/github-workflows/.github/workflows/build-and-push-opa-rules.yml",
		))
	})

	It("returns an empty Repository when SourceRepositoryURI is not a github.com URI", func() {
		summary := certificate.Summary{
			Extensions: certificate.Extensions{
				SourceRepositoryRef: "refs/heads/main",
			},
		}

		source := utilities.GetRepositorySourceFromVerifiedSigstoreBundleCertificate(summary)

		Expect(source.Repository).To(BeEmpty())
		Expect(source.Ref).To(Equal("refs/heads/main"))
		Expect(source.Workflow).To(BeEmpty())
	})
})

var _ = Describe("GetBundleVerifier", func() {
	const missingMaterialErr = "invalid sigstore bundle: missing verification material"

	// bundleWithVerificationMaterial builds a bundle whose verification material
	// carries the given transparency-log entries. No entries selects the GitHub
	// keyless (signed-timestamp) trusted root; one or more entries selects the
	// public-good trusted root.
	bundleWithVerificationMaterial := func(tlogEntries ...*protorekor.TransparencyLogEntry) *sigstorebundle.Bundle {
		return &sigstorebundle.Bundle{
			Bundle: &protobundle.Bundle{
				VerificationMaterial: &protobundle.VerificationMaterial{
					TlogEntries: tlogEntries,
				},
			},
		}
	}

	Context("guard clauses", func() {
		It("returns an error for a nil bundle", func() {
			_, err := utilities.GetBundleVerifier(nil, config.Get().SigstoreTufCachePath)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(missingMaterialErr))
		})

		It("returns an error when the embedded protobuf bundle is nil", func() {
			_, err := utilities.GetBundleVerifier(&sigstorebundle.Bundle{}, config.Get().SigstoreTufCachePath)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(missingMaterialErr))
		})

		It("returns an error when the bundle has no verification material", func() {
			_, err := utilities.GetBundleVerifier(
				&sigstorebundle.Bundle{Bundle: &protobundle.Bundle{}},
				config.Get().SigstoreTufCachePath,
			)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(missingMaterialErr))
		})
	})

	Context("building a verifier from trusted material (hitting network)", func() {
		It("builds a verifier via GitHub's trusted root when there are no transparency-log entries", func() {
			verifier, err := utilities.GetBundleVerifier(bundleWithVerificationMaterial(), config.Get().SigstoreTufCachePath)
			Expect(err).NotTo(HaveOccurred())
			Expect(verifier).NotTo(BeNil())
		})

		It("builds a verifier via the public-good trusted root when transparency-log entries are present", func() {
			verifier, err := utilities.GetBundleVerifier(
				bundleWithVerificationMaterial(&protorekor.TransparencyLogEntry{}),
				config.Get().SigstoreTufCachePath,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(verifier).NotTo(BeNil())
		})
	})
})

var _ = Describe("GitHubTrustedRoot (hitting network)", func() {
	It("returns trusted material fetched from GitHub's TUF repository", func() {
		trustedMaterial, err := utilities.GitHubTrustedRoot(config.Get().SigstoreTufCachePath)
		Expect(err).NotTo(HaveOccurred())
		Expect(trustedMaterial).NotTo(BeNil())
	})

	It("memoizes the trusted material across calls", func() {
		first, err := utilities.GitHubTrustedRoot(config.Get().SigstoreTufCachePath)
		Expect(err).NotTo(HaveOccurred())
		second, err := utilities.GitHubTrustedRoot(config.Get().SigstoreTufCachePath)
		Expect(err).NotTo(HaveOccurred())
		Expect(second).To(BeIdenticalTo(first))
	})
})

var _ = Describe("PublicGoodTrustedRoot (hitting network)", func() {
	It("returns trusted material fetched from the public-good TUF mirror", func() {
		trustedMaterial, err := utilities.PublicTrustedRoot(config.Get().SigstoreTufCachePath)
		Expect(err).NotTo(HaveOccurred())
		Expect(trustedMaterial).NotTo(BeNil())
	})

	It("memoizes the trusted material across calls", func() {
		first, err := utilities.PublicTrustedRoot(config.Get().SigstoreTufCachePath)
		Expect(err).NotTo(HaveOccurred())
		second, err := utilities.PublicTrustedRoot(config.Get().SigstoreTufCachePath)
		Expect(err).NotTo(HaveOccurred())
		Expect(second).To(BeIdenticalTo(first))
	})
})
