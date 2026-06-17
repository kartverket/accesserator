package my_policy_test

import rego.v1

import data.my_policy

test_allow_with_correct_password if {
	my_policy.allow with input as {"password": "accesserator"}
}

test_deny_with_wrong_password if {
	not my_policy.allow with input as {"password": "wrong"}
}

test_deny_with_empty_password if {
	not my_policy.allow with input as {"password": ""}
}

test_deny_with_missing_password if {
	not my_policy.allow with input as {}
}

test_deny_with_null_password if {
	not my_policy.allow with input as {"password": null}
}
