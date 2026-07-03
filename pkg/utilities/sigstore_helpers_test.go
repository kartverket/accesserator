package utilities_test

import (
	"github.com/kartverket/accesserator/pkg/utilities"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	protobundle "github.com/sigstore/protobuf-specs/gen/pb-go/bundle/v1"
	protodsse "github.com/sigstore/protobuf-specs/gen/pb-go/dsse"
	protorekor "github.com/sigstore/protobuf-specs/gen/pb-go/rekor/v1"
	sigstorebundle "github.com/sigstore/sigstore-go/pkg/bundle"
)

// bundleWithDsseEnvelope builds a Sigstore bundle carrying a DSSE envelope
// whose payload is the given in-toto statement bytes.
func bundleWithDsseEnvelope(payload []byte) *sigstorebundle.Bundle {
	return &sigstorebundle.Bundle{
		Bundle: &protobundle.Bundle{
			Content: &protobundle.Bundle_DsseEnvelope{
				DsseEnvelope: &protodsse.Envelope{
					Payload:     payload,
					PayloadType: "application/vnd.in-toto+json",
				},
			},
		},
	}
}

var _ = Describe("GetRepositorySourceFromSigstoreBundle", func() {
	It("returns an error when the bundle is nil", func() {
		_, err := utilities.GetRepositorySourceFromSigstoreBundle(nil)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("no DsseEnvelope in Sigstore bundle"))
	})

	It("returns an error when the bundle has no DSSE envelope", func() {
		_, err := utilities.GetRepositorySourceFromSigstoreBundle(&sigstorebundle.Bundle{
			Bundle: &protobundle.Bundle{},
		})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("no DsseEnvelope in Sigstore bundle"))
	})

	It("returns an error when the payload is not valid JSON", func() {
		_, err := utilities.GetRepositorySourceFromSigstoreBundle(
			bundleWithDsseEnvelope([]byte("not-json")),
		)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("failed to unmarshal in-toto payload"))
	})

	It("extracts repository, workflow and ref from the in-toto payload", func() {
		payload := []byte(`{
			"predicate": {
				"buildDefinition": {
					"externalParameters": {
						"workflow": {
							"ref": "refs/heads/main",
							"repository": "https://github.com/kartverket/accesserator",
							"path": ".github/workflows/build.yml"
						}
					}
				}
			}
		}`)

		source, err := utilities.GetRepositorySourceFromSigstoreBundle(bundleWithDsseEnvelope(payload))
		Expect(err).NotTo(HaveOccurred())
		Expect(source.Repository).To(Equal("kartverket/accesserator"))
		Expect(source.Workflow).To(Equal(".github/workflows/build.yml"))
		Expect(source.Ref).To(Equal("refs/heads/main"))
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
			_, err := utilities.GetBundleVerifier(nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(missingMaterialErr))
		})

		It("returns an error when the embedded protobuf bundle is nil", func() {
			_, err := utilities.GetBundleVerifier(&sigstorebundle.Bundle{})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(missingMaterialErr))
		})

		It("returns an error when the bundle has no verification material", func() {
			_, err := utilities.GetBundleVerifier(&sigstorebundle.Bundle{Bundle: &protobundle.Bundle{}})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(missingMaterialErr))
		})
	})

	Context("building a verifier from trusted material (hitting network)", func() {
		It("builds a verifier via GitHub's trusted root when there are no transparency-log entries", func() {
			verifier, err := utilities.GetBundleVerifier(bundleWithVerificationMaterial())
			Expect(err).NotTo(HaveOccurred())
			Expect(verifier).NotTo(BeNil())
		})

		It("builds a verifier via the public-good trusted root when transparency-log entries are present", func() {
			verifier, err := utilities.GetBundleVerifier(
				bundleWithVerificationMaterial(&protorekor.TransparencyLogEntry{}),
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(verifier).NotTo(BeNil())
		})
	})
})

var _ = Describe("GitHubTrustedRoot (hitting network)", func() {
	It("returns trusted material fetched from GitHub's TUF repository", func() {
		trustedMaterial, err := utilities.GitHubTrustedRoot()
		Expect(err).NotTo(HaveOccurred())
		Expect(trustedMaterial).NotTo(BeNil())
	})

	It("memoizes the trusted material across calls", func() {
		first, err := utilities.GitHubTrustedRoot()
		Expect(err).NotTo(HaveOccurred())
		second, err := utilities.GitHubTrustedRoot()
		Expect(err).NotTo(HaveOccurred())
		Expect(second).To(BeIdenticalTo(first))
	})
})

var _ = Describe("PublicGoodTrustedRoot (hitting network)", func() {
	It("returns trusted material fetched from the public-good TUF mirror", func() {
		trustedMaterial, err := utilities.PublicTrustedRoot()
		Expect(err).NotTo(HaveOccurred())
		Expect(trustedMaterial).NotTo(BeNil())
	})

	It("memoizes the trusted material across calls", func() {
		first, err := utilities.PublicTrustedRoot()
		Expect(err).NotTo(HaveOccurred())
		second, err := utilities.PublicTrustedRoot()
		Expect(err).NotTo(HaveOccurred())
		Expect(second).To(BeIdenticalTo(first))
	})
})
