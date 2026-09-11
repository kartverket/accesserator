package utilities_test

import (
	"github.com/kartverket/accesserator/pkg/model"
	"github.com/kartverket/accesserator/pkg/utilities"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("envoy.go unit tests", func() {
	Describe("GetExternalAuthorizationFilterConfig", func() {
		typedConfigOf := func(result map[string]any) map[string]any {
			GinkgoHelper()
			tc, ok := result["typed_config"].(map[string]any)
			Expect(ok).To(BeTrue(), "typed_config should be a map")
			return tc
		}

		It("sets failure_mode_allow=false when FailureMode is Deny", func() {
			result := utilities.GetExternalAuthorizationFilterConfig(
				model.OpaEnvoyExtAuthzFilterConfig{
					FailureMode: model.OpaRequestPolicyFailureModeDeny,
				},
			)

			Expect(typedConfigOf(result)).To(HaveKeyWithValue("failure_mode_allow", false))
		})

		It("sets failure_mode_allow=true when FailureMode is Forward", func() {
			result := utilities.GetExternalAuthorizationFilterConfig(
				model.OpaEnvoyExtAuthzFilterConfig{
					FailureMode: model.OpaRequestPolicyFailureModeForward,
				},
			)

			Expect(typedConfigOf(result)).To(HaveKeyWithValue("failure_mode_allow", true))
		})

		It("always sets failure_mode_allow_header_add=true", func() {
			for _, mode := range []model.OpaRequestPolicyFailureMode{
				model.OpaRequestPolicyFailureModeDeny,
				model.OpaRequestPolicyFailureModeForward,
			} {
				result := utilities.GetExternalAuthorizationFilterConfig(
					model.OpaEnvoyExtAuthzFilterConfig{FailureMode: mode},
				)
				Expect(typedConfigOf(result)).To(HaveKeyWithValue("failure_mode_allow_header_add", true))
			}
		})

		It("omits with_request_body when IncludeRequestBody is false", func() {
			result := utilities.GetExternalAuthorizationFilterConfig(
				model.OpaEnvoyExtAuthzFilterConfig{
					FailureMode: model.OpaRequestPolicyFailureModeDeny,
					RequestBodyConfig: model.RequestBodyConfig{
						IncludeRequestBody: false,
					},
				},
			)

			Expect(typedConfigOf(result)).NotTo(HaveKey("with_request_body"))
		})

		It("includes with_request_body with the given MaxRequestBodyBytes when IncludeRequestBody is true", func() {
			result := utilities.GetExternalAuthorizationFilterConfig(
				model.OpaEnvoyExtAuthzFilterConfig{
					FailureMode: model.OpaRequestPolicyFailureModeDeny,
					RequestBodyConfig: model.RequestBodyConfig{
						IncludeRequestBody:  true,
						MaxRequestBodyBytes: 16384,
					},
				},
			)

			tc := typedConfigOf(result)
			Expect(tc).To(HaveKey("with_request_body"))
			wrb, ok := tc["with_request_body"].(map[string]any)
			Expect(ok).To(BeTrue(), "with_request_body should be a map")

			Expect(wrb).To(HaveKeyWithValue("max_request_bytes", 16384))
			Expect(wrb).To(HaveKeyWithValue("allow_partial_message", false))
		})
	})
})
