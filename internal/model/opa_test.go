package model_test

import (
	"github.com/kartverket/accesserator/api/v1alpha"
	"github.com/kartverket/accesserator/internal/model"
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

			result := model.ToOpaBundles(input)

			Expect(result).To(HaveLen(2))
			Expect(result).To(Equal([]model.OpaBundle{
				{
					Name: "bundle-a",
					URL:  "ghcr.io/kartverket/a:latest",
					BundleSource: model.OpaBundleSource{
						Repository: "kartverket/accesserator",
						Workflow:   ".github/workflows/release.yaml",
						Ref:        "refs/heads/main",
					},
				},
				{
					Name: "bundle-b",
					URL:  "ghcr.io/kartverket/b:latest",
					BundleSource: model.OpaBundleSource{
						Repository: "kartverket/accesserator",
						Workflow:   ".github/workflows/release.yaml",
						Ref:        "refs/tags/v1.0.0",
					},
				},
			}))
		})

		It("returns an empty result for empty input", func() {
			Expect(model.ToOpaBundles([]v1alpha.BundleSource{})).To(BeEmpty())
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

			result := model.ToOpaBundle(input)

			Expect(result).To(Equal(model.OpaBundle{
				Name: "bundle-a",
				URL:  "ghcr.io/kartverket/a:latest",
				BundleSource: model.OpaBundleSource{
					Repository: "kartverket/accesserator",
					Workflow:   ".github/workflows/release.yaml",
					Ref:        "refs/heads/main",
				},
			}))
		})
	})

	Describe("OpaBundle Decode", func() {
		It("decodes a valid JSON object", func() {
			var decoded model.OpaBundle

			err := decoded.Decode(`{"name":"bundle-a","url":"ghcr.io/kartverket/a:latest","verification":{"repository":"kartverket/accesserator","workflow":".github/workflows/release.yaml","ref":"refs/heads/main"}}`)

			Expect(err).NotTo(HaveOccurred())
			Expect(decoded).To(Equal(model.OpaBundle{
				Name: "bundle-a",
				URL:  "ghcr.io/kartverket/a:latest",
				BundleSource: model.OpaBundleSource{
					Repository: "kartverket/accesserator",
					Workflow:   ".github/workflows/release.yaml",
					Ref:        "refs/heads/main",
				},
			}))
		})

		It("returns an error for invalid JSON", func() {
			var decoded model.OpaBundle
			Expect(decoded.Decode(`{`)).To(HaveOccurred())
		})
	})

	Describe("OpaBundle Validate", func() {
		newValidBundle := func() model.OpaBundle {
			return model.OpaBundle{
				Name: "self-auth",
				URL:  "https://ghcr.io/kartverket/accesserator/self-auth:latest",
				BundleSource: model.OpaBundleSource{
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

	Describe("ToOpaEnvoyExtAuthzFilterConfig", func() {
		It("should apply the correct defaults when .requestBody is not specified", func() {
			result := model.ToOpaEnvoyExtAuthzFilterConfig(v1alpha.OpaRequestPolicy{
				Enabled:     true,
				Endpoint:    "/v1/data/envoy/authz/allow",
				FailureMode: "FORWARD",
			})

			Expect(result).To(Equal(model.OpaEnvoyExtAuthzFilterConfig{
				FailureMode: model.OpaRequestPolicyFailureModeForward,
				RequestBodyConfig: model.RequestBodyConfig{
					IncludeRequestBody: false,
				},
			}))
		})

		It("should set MaxRequestBodyBytes to the same as defined in the spec", func() {
			result := model.ToOpaEnvoyExtAuthzFilterConfig(v1alpha.OpaRequestPolicy{
				Enabled:     true,
				Endpoint:    "/v1/data/envoy/authz/allow",
				FailureMode: "DENY",
				RequestBody: &v1alpha.OpaRequestPolicyRequestBody{
					Include:             true,
					MaxRequestBodyBytes: 42,
				},
			})

			Expect(result).To(Equal(model.OpaEnvoyExtAuthzFilterConfig{
				FailureMode: model.OpaRequestPolicyFailureModeDeny,
				RequestBodyConfig: model.RequestBodyConfig{
					IncludeRequestBody:  true,
					MaxRequestBodyBytes: 42,
				},
			}))
		})

		It("defaults FailureMode to Deny when empty", func() {
			result := model.ToOpaEnvoyExtAuthzFilterConfig(v1alpha.OpaRequestPolicy{
				Enabled:  true,
				Endpoint: "/v1/data/envoy/authz/allow",
			})

			Expect(result.FailureMode).To(Equal(model.OpaRequestPolicyFailureModeDeny))
			Expect(result.RequestBodyConfig.IncludeRequestBody).To(BeFalse())
		})

		It("is case-insensitive for FailureMode", func() {
			for _, mode := range []string{"deny", "Deny", "DENY"} {
				result := model.ToOpaEnvoyExtAuthzFilterConfig(v1alpha.OpaRequestPolicy{
					Enabled:     true,
					Endpoint:    "/v1/data/envoy/authz/allow",
					FailureMode: mode,
				})
				Expect(result.FailureMode).To(Equal(model.OpaRequestPolicyFailureModeDeny), "input %q", mode)
			}

			for _, mode := range []string{"forward", "Forward", "FORWARD", "ForWaRd"} {
				result := model.ToOpaEnvoyExtAuthzFilterConfig(v1alpha.OpaRequestPolicy{
					Enabled:     true,
					Endpoint:    "/v1/data/envoy/authz/allow",
					FailureMode: mode,
				})
				Expect(result.FailureMode).To(Equal(model.OpaRequestPolicyFailureModeForward), "input %q", mode)
			}
		})

		It("falls back to Deny for unknown FailureMode values", func() {
			for _, mode := range []string{"allow", "reject", "nonsense"} {
				result := model.ToOpaEnvoyExtAuthzFilterConfig(v1alpha.OpaRequestPolicy{
					Enabled:     true,
					Endpoint:    "/v1/data/envoy/authz/allow",
					FailureMode: mode,
				})
				Expect(result.FailureMode).To(Equal(model.OpaRequestPolicyFailureModeDeny), "input %q", mode)
			}
		})
	})
})
