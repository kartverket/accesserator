package authz

import rego.v1

import data.authz

# Test that correct password allows access
test_allow_with_correct_password if {
    authz.allow with input as {"password": "password123"}
}

# Test that wrong password denies access
test_deny_with_wrong_password if {
    not authz.allow with input as {"password": "wrong"}
}

# Test that empty password denies access
test_deny_with_empty_password if {
    not authz.allow with input as {"password": ""}
}

# Test that missing password denies access
test_deny_with_missing_password if {
    not authz.allow with input as {}
}

# Test that null password denies access
test_deny_with_null_password if {
    not authz.allow with input as {"password": null}
}

