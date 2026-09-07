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

test_found_request_body_false_when_attributes_missing if {
	decision := authz.allow with input as {"parsed_path": ["api", "orders"]}
	decision.headers["x-opa-found-request-body"] == "false"
}

test_found_request_body_false_when_body_empty if {
	decision := authz.allow with input as {
		"parsed_path": ["api", "orders"],
		"attributes": {"request": {"http": {"body": ""}}},
	}
	decision.headers["x-opa-found-request-body"] == "false"
}

test_found_request_body_true_when_body_present if {
	decision := authz.allow with input as {
		"parsed_path": ["api", "orders"],
		"attributes": {"request": {"http": {"body": "{\"hello\":\"world\"}"}}},
	}
	decision.headers["x-opa-found-request-body"] == "true"
}

test_found_request_body_false_when_raw_body_empty if {
	decision := authz.allow with input as {
		"parsed_path": ["api", "orders"],
		"attributes": {"request": {"http": {"raw_body": []}}},
	}
	decision.headers["x-opa-found-request-body"] == "false"
}

test_found_request_body_true_when_raw_body_present if {
	decision := authz.allow with input as {
		"parsed_path": ["api", "orders"],
		"attributes": {"request": {"http": {"raw_body": [123, 125]}}},
	}
	decision.headers["x-opa-found-request-body"] == "true"
}

test_found_request_body_true_on_denied_path_with_body if {
	decision := authz.allow with input as {
		"parsed_path": ["api", "admin"],
		"attributes": {"request": {"http": {"body": "payload"}}},
	}
	decision.allowed == false
	decision.headers["x-opa-found-request-body"] == "true"
}
