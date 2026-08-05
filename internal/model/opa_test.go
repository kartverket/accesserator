package model

import (
	"github.com/kartverket/accesserator/api/v1alpha"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("opa.go unit tests", func() {
	Describe("ToOpaBundles", func() {
		It("converts bundle sources to model bundles in order", func() {
			input := []v1alpha.BundleSource{
				{
					Name: "bundle-a",
					URL:  "ghcr.io/kartverket/a:latest",
					Verification: &v1alpha.BundleSourceVerification{
						Source: v1alpha.GitHubRepositorySource{
							Repository: "kartverket/accesserator",
							Workflow:   ".github/workflows/release.yaml",
							Ref:        "refs/heads/main",
						},
					},
				},
				{
					Name: "bundle-b",
					URL:  "ghcr.io/kartverket/b:latest",
					Verification: &v1alpha.BundleSourceVerification{
						Source: v1alpha.GitHubRepositorySource{
							Repository: "kartverket/accesserator",
							Workflow:   ".github/workflows/release.yaml",
							Ref:        "refs/tags/v1.0.0",
						},
					},
				},
			}

			result := ToOpaBundles(input)

			Expect(result).To(HaveLen(2))
			Expect(result).To(Equal([]OpaBundle{
				{
					Name: "bundle-a",
					URL:  "ghcr.io/kartverket/a:latest",
					BundleSource: OpaBundleSource{
						Repository: "kartverket/accesserator",
						Workflow:   ".github/workflows/release.yaml",
						Ref:        "refs/heads/main",
					},
				},
				{
					Name: "bundle-b",
					URL:  "ghcr.io/kartverket/b:latest",
					BundleSource: OpaBundleSource{
						Repository: "kartverket/accesserator",
						Workflow:   ".github/workflows/release.yaml",
						Ref:        "refs/tags/v1.0.0",
					},
				},
			}))
		})

		It("returns an empty result for empty input", func() {
			Expect(ToOpaBundles([]v1alpha.BundleSource{})).To(BeEmpty())
		})
	})

	Describe("ToOpaBundle", func() {
		It("converts one bundle source", func() {
			input := v1alpha.BundleSource{
				Name: "bundle-a",
				URL:  "ghcr.io/kartverket/a:latest",
				Verification: &v1alpha.BundleSourceVerification{
					Source: v1alpha.GitHubRepositorySource{
						Repository: "kartverket/accesserator",
						Workflow:   ".github/workflows/release.yaml",
						Ref:        "refs/heads/main",
					},
				},
			}

			result := ToOpaBundle(input)

			Expect(result).To(Equal(OpaBundle{
				Name: "bundle-a",
				URL:  "ghcr.io/kartverket/a:latest",
				BundleSource: OpaBundleSource{
					Repository: "kartverket/accesserator",
					Workflow:   ".github/workflows/release.yaml",
					Ref:        "refs/heads/main",
				},
			}))
		})
	})

	Describe("OpaBundle Decode", func() {
		It("decodes a valid JSON object", func() {
			var decoded OpaBundle

			err := decoded.Decode(`{"name":"bundle-a","url":"ghcr.io/kartverket/a:latest","verification":{"repository":"kartverket/accesserator","workflow":".github/workflows/release.yaml","ref":"refs/heads/main"}}`)

			Expect(err).NotTo(HaveOccurred())
			Expect(decoded).To(Equal(OpaBundle{
				Name: "bundle-a",
				URL:  "ghcr.io/kartverket/a:latest",
				BundleSource: OpaBundleSource{
					Repository: "kartverket/accesserator",
					Workflow:   ".github/workflows/release.yaml",
					Ref:        "refs/heads/main",
				},
			}))
		})

		It("returns an error for invalid JSON", func() {
			var decoded OpaBundle
			Expect(decoded.Decode(`{`)).To(HaveOccurred())
		})
	})

	Describe("OpaBundle Validate", func() {
		newValidBundle := func() OpaBundle {
			return OpaBundle{
				Name: "self-auth",
				URL:  "https://ghcr.io/kartverket/accesserator/self-auth:latest",
				BundleSource: OpaBundleSource{
					Repository: "kartverket/accesserator",
					Workflow:   ".github/workflows/release.yaml",
					Ref:        "refs/heads/main",
				},
			}
		}

		It("succeeds for a valid bundle", func() {
			Expect(newValidBundle().ValidateOpaBundle()).To(Succeed())
		})

		It("fails when name is not a valid configmap key", func() {
			bundle := newValidBundle()
			bundle.Name = "invalid/name"
			Expect(bundle.ValidateOpaBundle()).To(HaveOccurred())
		})

		It("fails when repository is empty", func() {
			bundle := newValidBundle()
			bundle.BundleSource.Repository = ""
			Expect(bundle.ValidateOpaBundle()).To(HaveOccurred())
		})

		It("fails when repository does not match pattern", func() {
			bundle := newValidBundle()
			bundle.BundleSource.Repository = "not-valid"
			Expect(bundle.ValidateOpaBundle()).To(HaveOccurred())
		})

		It("fails when workflow does not match pattern", func() {
			bundle := newValidBundle()
			bundle.BundleSource.Workflow = "workflow.yml"
			Expect(bundle.ValidateOpaBundle()).To(HaveOccurred())
		})

		It("fails when ref does not match pattern", func() {
			bundle := newValidBundle()
			bundle.BundleSource.Ref = "main"
			Expect(bundle.ValidateOpaBundle()).To(HaveOccurred())
		})
	})
})
