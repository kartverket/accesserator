package config_test

import (
	"fmt"
	"testing"

	"github.com/kartverket/accesserator/pkg/config"
)

const (
	defaultClusterName        = "my-cluster"
	defaultTokenxName         = "tokendings"
	defaultTokenxNamespace    = "ns"
	defaultTexasImageName     = "ghcr.io/nais/texas"
	defaultTexasImageTag      = "latest"
	defaultTexasPort          = int32(3000)
	defaultTexasUrlEnvVarName = "TEXAS_URL"
)

func setAllEnvVars(t *testing.T) {
	t.Helper()
	t.Setenv("ACCESSERATOR_CLUSTER_NAME", defaultClusterName)
	t.Setenv("ACCESSERATOR_TOKENX_NAME", defaultTokenxName)
	t.Setenv("ACCESSERATOR_TOKENX_NAMESPACE", defaultTokenxNamespace)
	t.Setenv("ACCESSERATOR_TEXAS_IMAGE_NAME", defaultTexasImageName)
	t.Setenv("ACCESSERATOR_TEXAS_IMAGE_TAG", defaultTexasImageTag)
	t.Setenv("ACCESSERATOR_TEXAS_PORT", fmt.Sprintf("%d", defaultTexasPort))
	t.Setenv("ACCESSERATOR_TEXAS_URL_ENV_VAR_NAME", defaultTexasUrlEnvVarName)
}

func TestLoad_AllRequiredSet(t *testing.T) {
	setAllEnvVars(t)

	if err := config.Load(); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	c := config.Get()
	if c.ClusterName != defaultClusterName {
		t.Errorf("ClusterName = %q, want %q", c.ClusterName, defaultClusterName)
	}
	if c.TokenxNamespace != defaultTokenxNamespace {
		t.Errorf("TokenxNamespace = %q, want %q", c.TokenxNamespace, defaultTokenxNamespace)
	}
	if c.TexasImageTag != defaultTexasImageTag {
		t.Errorf("TexasImageTag = %q, want %q", c.TexasImageTag, defaultTexasImageTag)
	}
}

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("ACCESSERATOR_CLUSTER_NAME", defaultClusterName)
	t.Setenv("ACCESSERATOR_TOKENX_NAMESPACE", defaultTokenxNamespace)
	t.Setenv("ACCESSERATOR_TEXAS_IMAGE_TAG", defaultTexasImageTag)

	if err := config.Load(); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	c := config.Get()
	if c.TokenxName != defaultTokenxName {
		t.Errorf("TokenxName = %q, want default %q", c.TokenxName, defaultTokenxName)
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
}

func TestLoad_MissingClusterName(t *testing.T) {
	t.Setenv("ACCESSERATOR_TOKENX_NAMESPACE", defaultTokenxNamespace)
	t.Setenv("ACCESSERATOR_TEXAS_IMAGE_TAG", defaultTexasImageTag)

	err := config.Load()
	if err == nil {
		t.Fatal("expected error for missing CLUSTER_NAME, got nil")
	}
	if got := err.Error(); !contains(got, "ACCESSERATOR_CLUSTER_NAME") {
		t.Errorf("error = %q, want it to mention ACCESSERATOR_CLUSTER_NAME", got)
	}
}

func TestLoad_MissingTokenxNamespace(t *testing.T) {
	t.Setenv("ACCESSERATOR_CLUSTER_NAME", defaultClusterName)
	t.Setenv("ACCESSERATOR_TEXAS_IMAGE_TAG", defaultTexasImageTag)

	err := config.Load()
	if err == nil {
		t.Fatal("expected error for missing TOKENX_NAMESPACE, got nil")
	}
	if got := err.Error(); !contains(got, "ACCESSERATOR_TOKENX_NAMESPACE") {
		t.Errorf("error = %q, want it to mention ACCESSERATOR_TOKENX_NAMESPACE", got)
	}
}

func TestLoad_MissingTexasImageTag(t *testing.T) {
	t.Setenv("ACCESSERATOR_CLUSTER_NAME", defaultClusterName)
	t.Setenv("ACCESSERATOR_TOKENX_NAMESPACE", defaultTokenxNamespace)

	err := config.Load()
	if err == nil {
		t.Fatal("expected error for missing TEXAS_IMAGE_TAG, got nil")
	}
	if got := err.Error(); !contains(got, "ACCESSERATOR_TEXAS_IMAGE_TAG") {
		t.Errorf("error = %q, want it to mention ACCESSERATOR_TEXAS_IMAGE_TAG", got)
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
	t.Setenv("ACCESSERATOR_CLUSTER_NAME", "prod-cluster")
	t.Setenv("ACCESSERATOR_TOKENX_NAME", "custom-tokendings")
	t.Setenv("ACCESSERATOR_TOKENX_NAMESPACE", "custom-ns")
	t.Setenv("ACCESSERATOR_TEXAS_IMAGE_NAME", "custom-image")
	t.Setenv("ACCESSERATOR_TEXAS_IMAGE_TAG", "v2.0.0")
	t.Setenv("ACCESSERATOR_TEXAS_PORT", "8080")
	t.Setenv("ACCESSERATOR_TEXAS_URL_ENV_VAR_NAME", "CUSTOM_URL")

	if err := config.Load(); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	c := config.Get()
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
	if c.TexasPort != 8080 {
		t.Errorf("TexasPort = %d, want %d", c.TexasPort, 8080)
	}
	if c.TexasUrlEnvVarName != "CUSTOM_URL" {
		t.Errorf("TexasUrlEnvVarName = %q, want %q", c.TexasUrlEnvVarName, "CUSTOM_URL")
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
	if before != after {
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
	if c1 != c2 {
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
