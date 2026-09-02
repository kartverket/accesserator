package opaenvoyfilter

import (
	"github.com/kartverket/accesserator/internal/state"
	"google.golang.org/protobuf/types/known/structpb"
	istioapiv1alpha3 "istio.io/api/networking/v1alpha3"
	istioclientgov1alpha3 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Priority controls the order in which this EnvoyFilter is applied relative to other
// EnvoyFilters targeting the same workload. A filter with a higher priority is patched in later.
// This ensures the OPA ext_authz check runs after any other HTTP filters that other
// teams or platform components may have inserted (e.g. OAuth login, RBAC), so it sees the fully
// processed request just before it is forwarded to the application.
const Priority = 1000

func GetDesired(objectMeta metav1.ObjectMeta, opaConfig state.OpaConfig) *istioclientgov1alpha3.EnvoyFilter {
	if !opaConfig.Enabled || !opaConfig.RequestPolicy.Enabled {
		return nil
	}
	clusterConfigPatch := GetClusterConfigPatch(opaConfig.RequestPolicy.ClusterConfigPatchValue)
	externalAuthorizationConfigPatch := GetExternalAuthorizationConfigPatch(
		opaConfig.RequestPolicy.ExternalAuthorizationConfigPatchValue,
	)
	return &istioclientgov1alpha3.EnvoyFilter{
		ObjectMeta: objectMeta,
		Spec: istioapiv1alpha3.EnvoyFilter{
			Priority: Priority,
			WorkloadSelector: &istioapiv1alpha3.WorkloadSelector{
				Labels: opaConfig.RequestPolicy.WorkloadLabels,
			},
			ConfigPatches: []*istioapiv1alpha3.EnvoyFilter_EnvoyConfigObjectPatch{
				clusterConfigPatch,
				externalAuthorizationConfigPatch,
			},
		},
	}
}

func GetClusterConfigPatch(
	clusterConfigPatchValue *structpb.Struct,
) *istioapiv1alpha3.EnvoyFilter_EnvoyConfigObjectPatch {
	return &istioapiv1alpha3.EnvoyFilter_EnvoyConfigObjectPatch{
		ApplyTo: istioapiv1alpha3.EnvoyFilter_CLUSTER,
		Match: &istioapiv1alpha3.EnvoyFilter_EnvoyConfigObjectMatch{
			Context: istioapiv1alpha3.EnvoyFilter_SIDECAR_INBOUND,
		},
		Patch: &istioapiv1alpha3.EnvoyFilter_Patch{
			Operation: istioapiv1alpha3.EnvoyFilter_Patch_ADD,
			Value:     clusterConfigPatchValue,
		},
	}
}

func GetExternalAuthorizationConfigPatch(
	externalAuthorizationConfigPatchValue *structpb.Struct,
) *istioapiv1alpha3.EnvoyFilter_EnvoyConfigObjectPatch {
	return &istioapiv1alpha3.EnvoyFilter_EnvoyConfigObjectPatch{
		ApplyTo: istioapiv1alpha3.EnvoyFilter_HTTP_FILTER,
		Match: &istioapiv1alpha3.EnvoyFilter_EnvoyConfigObjectMatch{
			Context: istioapiv1alpha3.EnvoyFilter_SIDECAR_INBOUND,
			ObjectTypes: &istioapiv1alpha3.EnvoyFilter_EnvoyConfigObjectMatch_Listener{
				Listener: &istioapiv1alpha3.EnvoyFilter_ListenerMatch{
					FilterChain: &istioapiv1alpha3.EnvoyFilter_ListenerMatch_FilterChainMatch{
						Filter: &istioapiv1alpha3.EnvoyFilter_ListenerMatch_FilterMatch{
							Name: "envoy.filters.network.http_connection_manager",
							SubFilter: &istioapiv1alpha3.EnvoyFilter_ListenerMatch_SubFilterMatch{
								Name: "envoy.filters.http.router",
							},
						},
					},
				},
			},
		},
		Patch: &istioapiv1alpha3.EnvoyFilter_Patch{
			// EnvoyFilter_Patch_INSERT_BEFORE the subFilter envoy.filters.http.router places the filter immediately
			// before the router, i.e. at the very end of the filter chain.
			Operation: istioapiv1alpha3.EnvoyFilter_Patch_INSERT_BEFORE,
			Value:     externalAuthorizationConfigPatchValue,
		},
	}
}
