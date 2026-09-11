package utilities

import (
	"github.com/kartverket/accesserator/pkg/model"
)

func GetExternalAuthorizationFilterConfig(filterConfig model.OpaEnvoyExtAuthzFilterConfig) map[string]any {
	failureModeAllow := filterConfig.FailureMode == model.OpaRequestPolicyFailureModeForward
	externalAuthorizationFilterConfig := map[string]any{
		"name": "envoy.filters.http.ext_authz",
		"typed_config": map[string]any{
			"@type":                 "type.googleapis.com/envoy.extensions.filters.http.ext_authz.v3.ExtAuthz",
			"transport_api_version": "V3",
			"grpc_service": map[string]any{
				"envoy_grpc": map[string]any{
					"cluster_name": filterConfig.OpaClusterName,
				},
				"timeout": "1s",
			},
			// The field failure_mode_allow specifies whether the request should be denied (false), i.e. returning
			// 403 Access denied, or forwarded to the upstream application (true).
			"failure_mode_allow": failureModeAllow,
			// If failure_mode_allow AND field failure_mode_allow_header_add is set to true, the header
			// `x-envoy-auth-failure-mode-allowed: true` is added to the request if envoy failed to reach OPA or if OPA
			// returned a 5xx response.
			"failure_mode_allow_header_add": true,
		},
	}
	if filterConfig.RequestBodyConfig.IncludeRequestBody {
		externalAuthorizationFilterConfig["typed_config"].(map[string]any)["with_request_body"] = map[string]any{
			"max_request_bytes":     filterConfig.RequestBodyConfig.MaxRequestBodyBytes,
			"allow_partial_message": false,
		}
	}
	return externalAuthorizationFilterConfig
}

func GetLocalClusterConfig(
	clusterName string,
	port int32,
) map[string]any {
	return map[string]any{
		"name":            clusterName,
		"type":            "STRICT_DNS",
		"connect_timeout": "1s",
		"typed_extension_protocol_options": map[string]any{
			"envoy.extensions.upstreams.http.v3.HttpProtocolOptions": map[string]any{
				"@type": "type.googleapis.com/envoy.extensions.upstreams.http.v3.HttpProtocolOptions",
				"explicit_http_config": map[string]any{
					"http2_protocol_options": map[string]any{},
				},
			},
		},
		"load_assignment": map[string]any{
			"cluster_name": clusterName,
			"endpoints": []any{
				map[string]any{
					"lb_endpoints": []any{
						map[string]any{
							"endpoint": map[string]any{
								"address": map[string]any{
									"socket_address": map[string]any{
										"address":    "127.0.0.1",
										"port_value": port,
									},
								},
							},
						},
					},
				},
			},
		},
	}
}
