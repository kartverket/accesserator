package opaenvoyfilter_test

import (
	"github.com/kartverket/accesserator/internal/state"
	"github.com/kartverket/accesserator/pkg/resourcegenerators/opa/opaenvoyfilter"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/protobuf/types/known/structpb"
	istioapiv1alpha3 "istio.io/api/networking/v1alpha3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("OPA EnvoyFilter GetDesired", func() {
	objectMeta := metav1.ObjectMeta{Name: "security-config-opa-abc123", Namespace: "team-a"}

	It("returns nil when OPA OR RequestPolicy is disabled", func() {
		Expect(opaenvoyfilter.GetDesired(objectMeta, state.OpaConfig{Enabled: false})).To(BeNil())
		Expect(opaenvoyfilter.GetDesired(objectMeta, state.OpaConfig{
			Enabled: true,
			RequestPolicy: state.RequestPolicyConfig{
				Enabled: false,
			},
		})).To(BeNil())
		Expect(opaenvoyfilter.GetDesired(objectMeta, state.OpaConfig{
			Enabled: false,
			RequestPolicy: state.RequestPolicyConfig{
				Enabled: true,
			},
		})).To(BeNil())
	})

	It("returns an EnvoyFilter with the correct workload selector, priority and two configPatches set", func() {
		envoyFilter := opaenvoyfilter.GetDesired(objectMeta, state.OpaConfig{
			Enabled: true,
			RequestPolicy: state.RequestPolicyConfig{
				Enabled: true,
				WorkloadLabels: map[string]string{
					"app": "app",
				},
				ClusterConfigPatchValue:               &structpb.Struct{},
				ExternalAuthorizationConfigPatchValue: &structpb.Struct{},
			},
		})
		Expect(envoyFilter).NotTo(BeNil())
		Expect(envoyFilter.Spec.WorkloadSelector.Labels).
			To(
				Equal(
					map[string]string{"app": "app"},
				),
			)
		Expect(envoyFilter.Spec.Priority).To(Equal(int32(1000)))
		Expect(envoyFilter.Spec.ConfigPatches).To(HaveLen(2))

		clusterConfigPatch := envoyFilter.Spec.ConfigPatches[0]
		Expect(clusterConfigPatch.ApplyTo).To(Equal(istioapiv1alpha3.EnvoyFilter_CLUSTER))
		Expect(clusterConfigPatch.Patch.Operation).To(Equal(istioapiv1alpha3.EnvoyFilter_Patch_ADD))

		externalAuthorizationConfigPatch := envoyFilter.Spec.ConfigPatches[1]
		Expect(externalAuthorizationConfigPatch.ApplyTo).To(Equal(istioapiv1alpha3.EnvoyFilter_HTTP_FILTER))
		Expect(externalAuthorizationConfigPatch.Patch.Operation).To(Equal(istioapiv1alpha3.EnvoyFilter_Patch_INSERT_BEFORE))
		listenerMatch := externalAuthorizationConfigPatch.Match.GetListener()
		Expect(listenerMatch).NotTo(BeNil())
		Expect(listenerMatch.FilterChain.Filter.Name).To(Equal("envoy.filters.network.http_connection_manager"))
		Expect(listenerMatch.FilterChain.Filter.SubFilter.Name).To(Equal("envoy.filters.http.router"))
	})
})
