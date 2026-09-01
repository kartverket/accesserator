# METADATA
# entrypoint: true
package request.authz

import rego.v1

default permit := true

permit := false if {
	input.parsed_path[0] == "api"
	input.parsed_path[1] == "admin"
}

allow := {
	"allowed": permit,
	"headers": {"x-touched-by-opa": "true"},
}
