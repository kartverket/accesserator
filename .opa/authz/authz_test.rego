package authz_test

import rego.v1

import data.authz

test_allow_with_correct_password if {
	authz.allow with input as {"password": "accesserator"}
}

test_deny_with_wrong_password if {
	not authz.allow with input as {"password": "wrong"}
}

test_deny_with_empty_password if {
	not authz.allow with input as {"password": ""}
}

test_deny_with_missing_password if {
	not authz.allow with input as {}
}

test_deny_with_null_password if {
	not authz.allow with input as {"password": null}
}
