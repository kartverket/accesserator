# METADATA
# entrypoint: true
package authz

import rego.v1

default allow := false

allow if {
	input.password == "discovery works"
}
