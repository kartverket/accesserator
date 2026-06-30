package config_test

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/kartverket/accesserator/pkg/config"
)

const (
	defaultClusterName                         = "my-cluster"
	defaultTokenxName                          = "tokendings"
	defaultTokenxNamespace                     = "ns"
	defaultTexasImageName                      = "ghcr.io/nais/texas"
	defaultTexasImageTag                       = "latest"
	defaultTexasImageSha                       = "abc123"
	defaultTexasPort                           = int32(3000)
	defaultTexasUrlEnvVarName                  = "TEXAS_URL"
	defaultEntraTenantId                       = "7f74c8a2-43ce-46b2-b0e8-b6306cba73a3"
	defaultOpaEnabled                          = "false"
	defaultOpaImageName                        = "openpolicyagent/opa"
	defaultOpaImageTag                         = "latest"
	defaultOpaImageSha                         = "def456"
	defaultOpaPort                             = int32(3010)
	defaultOpaUrlEnvVarName                    = "OPA_URL"
	defaultOpaAllowedBundleRegistryUrlPrefixes = "http://bundle-source,oci://bundle-source"
	defaultOpaAllowedBundleSignatureSourceOrgs = "kartverket,kartverket-skip"
	defaultRunsInProduction                    = "false"
)

var defaultEnvVars = map[string]string{
	"ACCESSERATOR_RUNS_IN_PRODUCTION":                       defaultRunsInProduction,
	"ACCESSERATOR_CLUSTER_NAME":                             defaultClusterName,
	"ACCESSERATOR_TOKENX_NAME":                              defaultTokenxName,
	"ACCESSERATOR_TOKENX_NAMESPACE":                         defaultTokenxNamespace,
	"ACCESSERATOR_TEXAS_IMAGE_NAME":                         defaultTexasImageName,
	"ACCESSERATOR_TEXAS_IMAGE_TAG":                          defaultTexasImageTag,
	"ACCESSERATOR_TEXAS_IMAGE_SHA":                          defaultTexasImageSha,
	"ACCESSERATOR_TEXAS_PORT":                               fmt.Sprintf("%d", defaultTexasPort),
	"ACCESSERATOR_TEXAS_URL_ENV_VAR_NAME":                   defaultTexasUrlEnvVarName,
	"ACCESSERATOR_ENTRA_TENANT_ID":                          defaultEntraTenantId,
	"ACCESSERATOR_OPA_ENABLED":                              defaultOpaEnabled,
	"ACCESSERATOR_OPA_IMAGE_NAME":                           defaultOpaImageName,
	"ACCESSERATOR_OPA_IMAGE_TAG":                            defaultOpaImageTag,
	"ACCESSERATOR_OPA_IMAGE_SHA":                            defaultOpaImageSha,
	"ACCESSERATOR_OPA_PORT":                                 fmt.Sprintf("%d", defaultOpaPort),
	"ACCESSERATOR_OPA_URL_ENV_VAR_NAME":                     defaultOpaUrlEnvVarName,
	"ACCESSERATOR_OPA_ALLOWED_BUNDLE_REGISTRY_URL_PREFIXES": defaultOpaAllowedBundleRegistryUrlPrefixes,
	"ACCESSERATOR_OPA_ALLOWED_BUNDLE_SIGNATURE_SOURCE_ORGS": defaultOpaAllowedBundleSignatureSourceOrgs,
}

func setAllEnvVars(t *testing.T) {
	t.Helper()
	setAllEnvVarsExcept(t)
}

func setAllEnvVarsExcept(t *testing.T, omittedKeys ...string) {
	t.Helper()

	omitted := make(map[string]struct{}, len(omittedKeys))
	for _, key := range omittedKeys {
		omitted[key] = struct{}{}
	}

	for key, value := range defaultEnvVars {
		if _, skip := omitted[key]; skip {
			continue
		}
		t.Setenv(key, value)
	}
}

func TestLoad_AllRequiredSet(t *testing.T) {
	setAllEnvVars(t)

	if err := config.Load(); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	c := config.Get()
	if c.RunsInProduction == nil || *c.RunsInProduction != false {
		t.Errorf("RunsInProduction = %v, want pointer to false", c.RunsInProduction)
	}
	if c.ClusterName != defaultClusterName {
		t.Errorf("ClusterName = %q, want %q", c.ClusterName, defaultClusterName)
	}
	if c.TokenxNamespace != defaultTokenxNamespace {
		t.Errorf("TokenxNamespace = %q, want %q", c.TokenxNamespace, defaultTokenxNamespace)
	}
	if c.TexasImageTag != defaultTexasImageTag {
		t.Errorf("TexasImageTag = %q, want %q", c.TexasImageTag, defaultTexasImageTag)
	}
	if c.TexasImageSha != defaultTexasImageSha {
		t.Errorf("TexasImageSha = %q, want %q", c.TexasImageSha, defaultTexasImageSha)
	}
	if c.EntraTenantId != defaultEntraTenantId {
		t.Errorf("EntraTenantId = %q, want %q", c.EntraTenantId, defaultEntraTenantId)
	}
	if c.OpaImageTag != defaultOpaImageTag {
		t.Errorf("OpaImageTag = %q, want %q", c.OpaImageTag, defaultOpaImageTag)
	}
	if c.OpaImageSha != defaultOpaImageSha {
		t.Errorf("OpaImageSha = %q, want %q", c.OpaImageSha, defaultOpaImageSha)
	}
	if !reflect.DeepEqual(c.OpaAllowedBundleRegistryUrlPrefixes, []string{"http://bundle-source", "oci://bundle-source"}) {
		t.Errorf(
			"OpaAllowedBundleRegistryUrlPrefixes = %v, want %v",
			c.OpaAllowedBundleRegistryUrlPrefixes,
			[]string{"http://bundle-source", "oci://bundle-source"},
		)
	}
	if !reflect.DeepEqual(c.OpaAllowedBundleSignatureSourceOrgs, []string{"kartverket", "kartverket-skip"}) {
		t.Errorf(
			"OpaAllowedBundleSignatureSourceOrgs = %v, want %v",
			c.OpaAllowedBundleSignatureSourceOrgs,
			[]string{"kartverket", "kartverket-skip"},
		)
	}
}

func TestLoad_Defaults(t *testing.T) {
	setAllEnvVarsExcept(
		t,
		"ACCESSERATOR_TOKENX_NAME",
		"ACCESSERATOR_TEXAS_IMAGE_NAME",
		"ACCESSERATOR_TEXAS_PORT",
		"ACCESSERATOR_TEXAS_URL_ENV_VAR_NAME",
		"ACCESSERATOR_OPA_IMAGE_NAME",
		"ACCESSERATOR_OPA_URL_ENV_VAR_NAME",
	)

	if err := config.Load(); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	c := config.Get()
	if c.RunsInProduction == nil || *c.RunsInProduction != false {
		t.Errorf("RunsInProduction = %v, want pointer to false", c.RunsInProduction)
	}
	if c.TokenxName != defaultTokenxName {
		t.Errorf("TokenxName = %q, want default %q", c.TokenxName, defaultTokenxName)
	}
	if !c.TokenxEnabled {
		t.Errorf("TokenxEnabled = %v, want default true", c.TokenxEnabled)
	}
	if c.TexasImageName != defaultTexasImageName {
		t.Errorf("TexasImageName = %q, want default %q", c.TexasImageName, defaultTexasImageName)
	}
	if c.TexasPort != defaultTexasPort {
		t.Errorf("TexasPort = %d, want default %d", c.TexasPort, defaultTexasPort)
	}
	if c.TexasUrlEnvVarName != defaultTexasUrlEnvVarName {
		t.Errorf("TexasUrlEnvVarName = %q, want default %q", c.TexasUrlEnvVarName, defaultTexasUrlEnvVarName)
	}
	if c.OpaImageName != defaultOpaImageName {
		t.Errorf("OpaImageName = %q, want default %q", c.OpaImageName, defaultOpaImageName)
	}
	if c.OpaUrlEnvVarName != defaultOpaUrlEnvVarName {
		t.Errorf("OpaUrlEnvVarName = %q, want default %q", c.OpaUrlEnvVarName, defaultOpaUrlEnvVarName)
	}
}

func TestLoad_MissingRunsInProduction(t *testing.T) {
	setAllEnvVarsExcept(t, "ACCESSERATOR_RUNS_IN_PRODUCTION")

	err := config.Load()
	if err == nil {
		t.Fatal("expected error for missing ACCESSERATOR_RUNS_IN_PRODUCTION, got nil")
	}
	if got := err.Error(); !contains(got, "ACCESSERATOR_RUNS_IN_PRODUCTION") {
		t.Errorf("error = %q, want it to mention ACCESSERATOR_RUNS_IN_PRODUCTION", got)
	}
}

func TestLoad_MissingClusterName(t *testing.T) {
	setAllEnvVarsExcept(t, "ACCESSERATOR_CLUSTER_NAME")

	err := config.Load()
	if err == nil {
		t.Fatal("expected error for missing CLUSTER_NAME, got nil")
	}
	if got := err.Error(); !contains(got, "ACCESSERATOR_CLUSTER_NAME") {
		t.Errorf("error = %q, want it to mention ACCESSERATOR_CLUSTER_NAME", got)
	}
}

func TestLoad_MissingTokenxNamespace(t *testing.T) {
	setAllEnvVarsExcept(t, "ACCESSERATOR_TOKENX_NAMESPACE")

	err := config.Load()
	if err == nil {
		t.Fatal("expected error for missing TOKENX_NAMESPACE, got nil")
	}
	if got := err.Error(); !contains(got, "ACCESSERATOR_TOKENX_NAMESPACE") {
		t.Errorf("error = %q, want it to mention ACCESSERATOR_TOKENX_NAMESPACE", got)
	}
}

func TestLoad_MissingTexasImageTag(t *testing.T) {
	setAllEnvVarsExcept(t, "ACCESSERATOR_TEXAS_IMAGE_TAG")

	err := config.Load()
	if err == nil {
		t.Fatal("expected error for missing TEXAS_IMAGE_TAG, got nil")
	}
	if got := err.Error(); !contains(got, "ACCESSERATOR_TEXAS_IMAGE_TAG") {
		t.Errorf("error = %q, want it to mention ACCESSERATOR_TEXAS_IMAGE_TAG", got)
	}
}

func TestLoad_MissingEntraTenantId(t *testing.T) {
	setAllEnvVarsExcept(t, "ACCESSERATOR_ENTRA_TENANT_ID")

	err := config.Load()
	if err == nil {
		t.Fatal("expected error for missing ENTRA_TENANT_ID, got nil")
	}
	if got := err.Error(); !contains(got, "ACCESSERATOR_ENTRA_TENANT_ID") {
		t.Errorf("error = %q, want it to mention ACCESSERATOR_ENTRA_TENANT_ID", got)
	}
}

func TestLoad_MissingOpaImageTag(t *testing.T) {
	setAllEnvVarsExcept(t, "ACCESSERATOR_OPA_IMAGE_TAG")

	err := config.Load()
	if err == nil {
		t.Fatal("expected error for missing OPA_IMAGE_TAG, got nil")
	}
	if got := err.Error(); !contains(got, "ACCESSERATOR_OPA_IMAGE_TAG") {
		t.Errorf("error = %q, want it to mention ACCESSERATOR_OPA_IMAGE_TAG", got)
	}
}

func TestLoad_MissingOpaImageSha(t *testing.T) {
	setAllEnvVarsExcept(t, "ACCESSERATOR_OPA_IMAGE_SHA")

	err := config.Load()
	if err == nil {
		t.Fatal("expected error for missing OPA_IMAGE_SHA, got nil")
	}
	if got := err.Error(); !contains(got, "ACCESSERATOR_OPA_IMAGE_SHA") {
		t.Errorf("error = %q, want it to mention ACCESSERATOR_OPA_IMAGE_SHA", got)
	}
}

func TestLoad_MissingOpaAllowedBundleRegistryUrlPrefixes(t *testing.T) {
	setAllEnvVarsExcept(t, "ACCESSERATOR_OPA_ALLOWED_BUNDLE_REGISTRY_URL_PREFIXES")

	err := config.Load()
	if err == nil {
		t.Fatal("expected error for missing OPA_ALLOWED_BUNDLE_REGISTRY_URL_PREFIXES, got nil")
	}
	if got := err.Error(); !contains(got, "ACCESSERATOR_OPA_ALLOWED_BUNDLE_REGISTRY_URL_PREFIXES") {
		t.Errorf("error = %q, want it to mention ACCESSERATOR_OPA_ALLOWED_BUNDLE_REGISTRY_URL_PREFIXES", got)
	}
}

func TestLoad_MissingOpaAllowedBundleSignatureSourceOrgs(t *testing.T) {
	setAllEnvVarsExcept(t, "ACCESSERATOR_OPA_ALLOWED_BUNDLE_SIGNATURE_SOURCE_ORGS")

	err := config.Load()
	if err == nil {
		t.Fatal("expected error for missing OPA_ALLOWED_BUNDLE_SIGNATURE_SOURCE_ORGS, got nil")
	}
	if got := err.Error(); !contains(got, "ACCESSERATOR_OPA_ALLOWED_BUNDLE_SIGNATURE_SOURCE_ORGS") {
		t.Errorf("error = %q, want it to mention ACCESSERATOR_OPA_ALLOWED_BUNDLE_SIGNATURE_SOURCE_ORGS", got)
	}
}

func TestLoad_AllRequiredMissing(t *testing.T) {
	err := config.Load()
	if err == nil {
		t.Fatal("expected error when all required vars missing, got nil")
	}
	got := err.Error()
	for _, key := range []string{
		"ACCESSERATOR_CLUSTER_NAME",
		"ACCESSERATOR_TOKENX_NAMESPACE",
		"ACCESSERATOR_TEXAS_IMAGE_TAG",
		"ACCESSERATOR_TEXAS_IMAGE_SHA",
		"ACCESSERATOR_ENTRA_TENANT_ID",
		"ACCESSERATOR_OPA_IMAGE_TAG",
		"ACCESSERATOR_OPA_IMAGE_SHA",
		"ACCESSERATOR_OPA_ALLOWED_BUNDLE_REGISTRY_URL_PREFIXES",
		"ACCESSERATOR_OPA_ALLOWED_BUNDLE_SIGNATURE_SOURCE_ORGS",
		"ACCESSERATOR_RUNS_IN_PRODUCTION",
	} {
		if !contains(got, key) {
			t.Errorf("error = %q, want it to mention %s", got, key)
		}
	}
}

func TestLoad_InvalidPort(t *testing.T) {
	setAllEnvVars(t)
	t.Setenv("ACCESSERATOR_TEXAS_PORT", "not-a-number")

	err := config.Load()
	if err == nil {
		t.Fatal("expected error for invalid port, got nil")
	}
}

func TestLoad_CustomValues(t *testing.T) {
	t.Setenv("ACCESSERATOR_RUNS_IN_PRODUCTION", "true")
	t.Setenv("ACCESSERATOR_CLUSTER_NAME", "prod-cluster")
	t.Setenv("ACCESSERATOR_TOKENX_NAME", "custom-tokendings")
	t.Setenv("ACCESSERATOR_TOKENX_NAMESPACE", "custom-ns")
	t.Setenv("ACCESSERATOR_TEXAS_IMAGE_NAME", "custom-image")
	t.Setenv("ACCESSERATOR_TEXAS_IMAGE_TAG", "v2.0.0")
	t.Setenv("ACCESSERATOR_TEXAS_IMAGE_SHA", "123abc")
	t.Setenv("ACCESSERATOR_TEXAS_PORT", "8080")
	t.Setenv("ACCESSERATOR_TEXAS_URL_ENV_VAR_NAME", "CUSTOM_URL")
	t.Setenv("ACCESSERATOR_ENTRA_TENANT_ID", "custom-entra-tenant-id")
	t.Setenv("ACCESSERATOR_OPA_IMAGE_NAME", "custom-opa-image")
	t.Setenv("ACCESSERATOR_OPA_IMAGE_TAG", "v1.2.3")
	t.Setenv("ACCESSERATOR_OPA_IMAGE_SHA", "456def")
	t.Setenv("ACCESSERATOR_OPA_URL_ENV_VAR_NAME", "CUSTOM_OPA_URL")
	t.Setenv("ACCESSERATOR_OPA_ALLOWED_BUNDLE_REGISTRY_URL_PREFIXES", "https://allowed.example,oci://allowed.example")
	t.Setenv("ACCESSERATOR_OPA_ALLOWED_BUNDLE_SIGNATURE_SOURCE_ORGS", "custom-org,another-org")

	if err := config.Load(); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	c := config.Get()
	if c.RunsInProduction == nil || *c.RunsInProduction != true {
		t.Errorf("RunsInProduction = %v, want %v", *c.RunsInProduction, true)
	}
	if c.ClusterName != "prod-cluster" {
		t.Errorf("ClusterName = %q, want %q", c.ClusterName, "prod-cluster")
	}
	if c.TokenxName != "custom-tokendings" {
		t.Errorf("TokenxName = %q, want %q", c.TokenxName, "custom-tokendings")
	}
	if c.TokenxNamespace != "custom-ns" {
		t.Errorf("TokenxNamespace = %q, want %q", c.TokenxNamespace, "custom-ns")
	}
	if c.TexasImageName != "custom-image" {
		t.Errorf("TexasImageName = %q, want %q", c.TexasImageName, "custom-image")
	}
	if c.TexasImageTag != "v2.0.0" {
		t.Errorf("TexasImageTag = %q, want %q", c.TexasImageTag, "v2.0.0")
	}
	if c.TexasImageSha != "123abc" {
		t.Errorf("TexasImageSha = %q, want %q", c.TexasImageSha, "123abc")
	}
	if c.TexasPort != 8080 {
		t.Errorf("TexasPort = %d, want %d", c.TexasPort, 8080)
	}
	if c.TexasUrlEnvVarName != "CUSTOM_URL" {
		t.Errorf("TexasUrlEnvVarName = %q, want %q", c.TexasUrlEnvVarName, "CUSTOM_URL")
	}
	if c.EntraTenantId != "custom-entra-tenant-id" {
		t.Errorf("EntraTenantId = %q, want %q", c.EntraTenantId, "custom-entra-tenant-id")
	}
	if c.OpaImageName != "custom-opa-image" {
		t.Errorf("OpaImageName = %q, want %q", c.OpaImageName, "custom-opa-image")
	}
	if c.OpaImageTag != "v1.2.3" {
		t.Errorf("OpaImageTag = %q, want %q", c.OpaImageTag, "v1.2.3")
	}
	if c.OpaImageSha != "456def" {
		t.Errorf("OpaImageSha = %q, want %q", c.OpaImageSha, "456def")
	}
	if c.OpaUrlEnvVarName != "CUSTOM_OPA_URL" {
		t.Errorf("OpaUrlEnvVarName = %q, want %q", c.OpaUrlEnvVarName, "CUSTOM_OPA_URL")
	}
	if !reflect.DeepEqual(
		c.OpaAllowedBundleRegistryUrlPrefixes,
		[]string{"https://allowed.example", "oci://allowed.example"},
	) {
		t.Errorf(
			"OpaAllowedBundleRegistryUrlPrefixes = %v, want %v",
			c.OpaAllowedBundleRegistryUrlPrefixes,
			[]string{"https://allowed.example", "oci://allowed.example"},
		)
	}
	if !reflect.DeepEqual(
		c.OpaAllowedBundleSignatureSourceOrgs,
		[]string{"custom-org", "another-org"},
	) {
		t.Errorf(
			"OpaAllowedBundleSignatureSourceOrgs = %v, want %v",
			c.OpaAllowedBundleSignatureSourceOrgs,
			[]string{"custom-org", "another-org"},
		)
	}
}

func TestLoad_DoesNotUpdateGlobalOnError(t *testing.T) {
	// First, load a valid config
	setAllEnvVars(t)
	if err := config.Load(); err != nil {
		t.Fatalf("initial Load failed: %v", err)
	}
	before := config.Get()

	// Now attempt a Load that will fail (invalid port)
	t.Setenv("ACCESSERATOR_TEXAS_PORT", "not-a-number")
	err := config.Load()
	if err == nil {
		t.Fatal("expected error for invalid port, got nil")
	}

	// appCfg should still hold the previous valid config
	after := config.Get()
	if !reflect.DeepEqual(before, after) {
		t.Errorf("Get() changed after failed Load: before=%+v, after=%+v", before, after)
	}
}

func TestGet_ReturnsConsistentConfig(t *testing.T) {
	setAllEnvVars(t)

	if err := config.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	c1 := config.Get()
	c2 := config.Get()
	if !reflect.DeepEqual(c1, c2) {
		t.Errorf("Get() returned different values on successive calls")
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
