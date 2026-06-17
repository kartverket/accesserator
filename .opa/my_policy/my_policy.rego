# METADATA
# entrypoint: true
package my_policy

import rego.v1

default allow := false

# METADATA
# entrypoint: true
# schemas:
#   - input: schema["my_policy_input"]
allow if {
	input.password == "accesserator"
}
