package authz

import rego.v1

default allow := false

allow if {
    input.jwt.groups in linst of [abc, bdavs, tilgangsstyring]
}
