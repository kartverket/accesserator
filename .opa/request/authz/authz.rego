# METADATA
# entrypoint: true
package request.authz

import rego.v1

default permit := true

permit := false if {
	input.parsed_path[0] == "api"
	input.parsed_path[1] == "admin"
}

# found_request_body is true when Envoy forwarded a non-empty request body to OPA
default found_request_body := false

found_request_body if {
	body := input.attributes.request.http.body
	body != ""
}

found_request_body if {
	raw := input.attributes.request.http.raw_body
	count(raw) > 0
}

allow := {
	"allowed": permit,
	"headers": {
		"x-touched-by-opa": "true",
		"x-opa-found-request-body": sprintf("%v", [found_request_body]),
	},
}
