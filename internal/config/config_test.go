package config

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRedactedYAML_HidesPassword(t *testing.T) {
	cfg := &Config{
		DgraphUser:      "admin",
		DgraphPassword:  "super-secret",
		DgraphEndpoints: []string{"localhost:9080"},
		BalancerType:    "round-robin",
	}

	out, err := redactedYAML(cfg)
	if err != nil {
		t.Fatalf("redactedYAML returned error: %v", err)
	}
	s := string(out)
	if strings.Contains(s, "super-secret") {
		t.Fatalf("redactedYAML leaked DgraphPassword:\n%s", s)
	}
	if !strings.Contains(s, "***REDACTED***") {
		t.Fatalf("redactedYAML missing redaction marker:\n%s", s)
	}
	// DgraphUser is not a secret and should remain readable.
	if !strings.Contains(s, "admin") {
		t.Fatalf("redactedYAML unexpectedly removed DgraphUser:\n%s", s)
	}
}

func TestRedactedYAML_EmptyPasswordPreserved(t *testing.T) {
	cfg := &Config{DgraphPassword: ""}
	out, err := redactedYAML(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(string(out), "***REDACTED***") {
		t.Fatalf("empty password must not be redacted:\n%s", out)
	}
}

func TestNormalizeLegacyKeys_RatelGraphQL(t *testing.T) {
	in := []byte("ratel-graphql: true\nratel: localhost:8000\n")
	out := normalizeLegacyKeys(in)
	if !strings.Contains(string(out), "ratel_graphql:") {
		t.Fatalf("expected key rewrite, got:\n%s", out)
	}
	if strings.Contains(string(out), "ratel-graphql:") {
		t.Fatalf("legacy key should be rewritten, got:\n%s", out)
	}
}

func TestLoadConfig_AcceptsLegacyRatelGraphQLKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	body := "balancer_type: round-robin\n" +
		"dgraph_endpoints:\n  - localhost:9080\n" +
		"ratel-graphql: true\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("CONFIG_FILE", path)
	// keep unrelated envs empty so defaults apply
	t.Setenv("DGRAPH_ENDPOINTS", "")
	t.Setenv("RATEL_GRAPHQL", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if cfg.RatelGraphQL == nil || !*cfg.RatelGraphQL {
		t.Fatalf("expected RatelGraphQL=true from legacy key, got %+v", cfg.RatelGraphQL)
	}
}

func TestLoadConfig_DevModeGeneratesWSToken(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	body := "balancer_type: round-robin\n" +
		"dgraph_endpoints:\n  - localhost:9080\n" +
		"dev_mode: true\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CONFIG_FILE", path)
	t.Setenv("WS_TOKEN", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if cfg.WSToken == "" {
		t.Fatal("dev-mode should auto-generate a WS token when unset")
	}
	if len(cfg.WSToken) < 16 {
		t.Fatalf("generated token looks too short: %q", cfg.WSToken)
	}
}

func TestLoadConfig_ProdModeMissingSecretsFails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	body := "balancer_type: round-robin\n" +
		"dgraph_endpoints:\n  - localhost:9080\n" +
		"dev_mode: false\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CONFIG_FILE", path)
	t.Setenv("WS_TOKEN", "")
	t.Setenv("WS_ALLOWED_ORIGINS", "")

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected LoadConfig to fail when dev_mode=false and secrets unset")
	}
	if !strings.Contains(err.Error(), "ws_token") {
		t.Fatalf("error should mention the missing fields: %v", err)
	}
}

func TestLoadConfig_ProdModeWithAllSecretsPasses(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	body := "balancer_type: round-robin\n" +
		"dgraph_endpoints:\n  - localhost:9080\n" +
		"dev_mode: false\n" +
		"ws_token: \"not-a-banana\"\n" +
		"ws_allowed_origins:\n  - https://app.example.com\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CONFIG_FILE", path)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if cfg.WSToken != "not-a-banana" {
		t.Fatalf("expected configured token, got %q", cfg.WSToken)
	}
	if len(cfg.WSAllowedOrigins) != 1 {
		t.Fatalf("expected one allowed origin, got %v", cfg.WSAllowedOrigins)
	}
}

func TestRedactedYAML_HidesWSToken(t *testing.T) {
	cfg := &Config{WSToken: "real-secret-token"}
	out, err := redactedYAML(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "real-secret-token") {
		t.Fatalf("redactedYAML leaked WSToken:\n%s", out)
	}
}

func TestLoadConfig_DoesNotLogPassword(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	body := "balancer_type: round-robin\n" +
		"dgraph_endpoints:\n  - localhost:9080\n" +
		"dgraph_password: super-secret-123\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("CONFIG_FILE", path)
	t.Setenv("DGRAPH_PASSWORD", "")

	var buf bytes.Buffer
	orig := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(orig)

	if _, err := LoadConfig(); err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if strings.Contains(buf.String(), "super-secret-123") {
		t.Fatalf("password leaked into logs:\n%s", buf.String())
	}
}

func TestLoadConfig_DefaultsWSMaxMessageBytes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	body := "balancer_type: round-robin\n" +
		"dgraph_endpoints:\n  - localhost:9080\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("CONFIG_FILE", path)
	t.Setenv("WS_MAX_MESSAGE_BYTES", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if cfg.WSMaxMessageBytes != DefaultWSMaxMessageBytes {
		t.Fatalf("WSMaxMessageBytes = %d, want %d", cfg.WSMaxMessageBytes, DefaultWSMaxMessageBytes)
	}
}

func TestLoadConfig_WSMaxMessageBytesEnvOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	body := "balancer_type: round-robin\n" +
		"dgraph_endpoints:\n  - localhost:9080\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("CONFIG_FILE", path)
	t.Setenv("WS_MAX_MESSAGE_BYTES", "2048")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if cfg.WSMaxMessageBytes != 2048 {
		t.Fatalf("WSMaxMessageBytes = %d, want 2048", cfg.WSMaxMessageBytes)
	}
}
