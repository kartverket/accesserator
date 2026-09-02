package request.authz_test

import rego.v1

import data.request.authz

test_allows_normal_path if {
	decision := authz.allow with input as {"parsed_path": ["api", "orders"]}
	decision.allowed == true
}

test_denies_admin_path if {
	decision := authz.allow with input as {"parsed_path": ["api", "admin"]}
	decision.allowed == false
}

test_denies_admin_subpath if {
	decision := authz.allow with input as {"parsed_path": ["api", "admin", "users"]}
	decision.allowed == false
}

test_allows_admin_outside_api if {
	decision := authz.allow with input as {"parsed_path": ["admin"]}
	decision.allowed == true
}

test_header_added_when_allowed if {
	decision := authz.allow with input as {"parsed_path": ["api", "orders"]}
	decision.headers["x-touched-by-opa"] == "true"
}

test_header_added_when_denied if {
	decision := authz.allow with input as {"parsed_path": ["api", "admin"]}
	decision.headers["x-touched-by-opa"] == "true"
}
