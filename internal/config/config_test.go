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

// writeMinimalConfigFile writes the smallest YAML that LoadConfig accepts
// (balancer + at least one endpoint) and points CONFIG_FILE at it. It
// returns the chosen path so subtests can re-open it if they need to.
func writeMinimalConfigFile(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	body := "balancer_type: round-robin\n" +
		"dgraph_endpoints:\n  - localhost:9080\n" +
		"dev_mode: true\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CONFIG_FILE", path)
	return path
}

func TestLoadConfig_CORSAllowedOriginsEnv(t *testing.T) {
	t.Run("comma-separated list parsed and trimmed", func(t *testing.T) {
		writeMinimalConfigFile(t)
		t.Setenv("CORS_ALLOWED_ORIGINS", " https://app.example.com , *.dev.example.com ")

		cfg, err := LoadConfig()
		if err != nil {
			t.Fatalf("LoadConfig: %v", err)
		}
		want := []string{"https://app.example.com", "*.dev.example.com"}
		if len(cfg.CORSAllowedOrigins) != len(want) {
			t.Fatalf("CORSAllowedOrigins=%v, want %v", cfg.CORSAllowedOrigins, want)
		}
		for i, w := range want {
			if cfg.CORSAllowedOrigins[i] != w {
				t.Errorf("CORSAllowedOrigins[%d]=%q, want %q", i, cfg.CORSAllowedOrigins[i], w)
			}
		}
	})

	t.Run("empty entries skipped", func(t *testing.T) {
		writeMinimalConfigFile(t)
		t.Setenv("CORS_ALLOWED_ORIGINS", " ,, https://only.example , ,")

		cfg, err := LoadConfig()
		if err != nil {
			t.Fatalf("LoadConfig: %v", err)
		}
		if len(cfg.CORSAllowedOrigins) != 1 || cfg.CORSAllowedOrigins[0] != "https://only.example" {
			t.Fatalf("CORSAllowedOrigins=%v, want [https://only.example]", cfg.CORSAllowedOrigins)
		}
	})

	t.Run("unset leaves YAML untouched", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.yaml")
		body := "balancer_type: round-robin\n" +
			"dgraph_endpoints:\n  - localhost:9080\n" +
			"cors_allowed_origins:\n  - https://yaml.example.com\n"
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("CONFIG_FILE", path)
		t.Setenv("CORS_ALLOWED_ORIGINS", "")

		cfg, err := LoadConfig()
		if err != nil {
			t.Fatalf("LoadConfig: %v", err)
		}
		if len(cfg.CORSAllowedOrigins) != 1 || cfg.CORSAllowedOrigins[0] != "https://yaml.example.com" {
			t.Fatalf("YAML value lost; got %v", cfg.CORSAllowedOrigins)
		}
	})
}

func TestLoadConfig_RateLimitEnv(t *testing.T) {
	cases := []struct {
		name             string
		rps              string
		burst            string
		wantRPS          int
		wantBurst        int
		wantWarnContains string
	}{
		{"both unset disables limiter", "", "", 0, 0, ""},
		{"rps only defaults burst to rps", "5", "", 5, 5, ""},
		{"explicit burst overrides default", "5", "12", 5, 12, ""},
		{"invalid rps ignored", "abc", "", 0, 0, "invalid RATE_LIMIT_RPS"},
		{"negative rps ignored", "-3", "", 0, 0, "invalid RATE_LIMIT_RPS"},
		{"invalid burst ignored", "5", "xyz", 5, 5, "invalid RATE_LIMIT_BURST"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			writeMinimalConfigFile(t)
			t.Setenv("RATE_LIMIT_RPS", tc.rps)
			t.Setenv("RATE_LIMIT_BURST", tc.burst)

			var buf bytes.Buffer
			orig := log.Writer()
			log.SetOutput(&buf)
			defer log.SetOutput(orig)

			cfg, err := LoadConfig()
			if err != nil {
				t.Fatalf("LoadConfig: %v", err)
			}
			if cfg.RateLimitRPS != tc.wantRPS {
				t.Errorf("RateLimitRPS=%d, want %d", cfg.RateLimitRPS, tc.wantRPS)
			}
			if cfg.RateLimitBurst != tc.wantBurst {
				t.Errorf("RateLimitBurst=%d, want %d", cfg.RateLimitBurst, tc.wantBurst)
			}
			if tc.wantWarnContains != "" && !strings.Contains(buf.String(), tc.wantWarnContains) {
				t.Errorf("expected log to contain %q; got:\n%s", tc.wantWarnContains, buf.String())
			}
		})
	}
}

func TestLoadConfig_DgraphHTTPEndpointsEnv(t *testing.T) {
	t.Run("env adds entries on top of YAML", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.yaml")
		body := "balancer_type: round-robin\n" +
			"dgraph_endpoints:\n  - localhost:9080\n" +
			"dgraph_http_endpoints:\n  \"localhost:9080\": \"localhost:8080\"\n"
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("CONFIG_FILE", path)
		t.Setenv("DGRAPH_HTTP_ENDPOINTS", "alpha:9091=alpha:7777")

		cfg, err := LoadConfig()
		if err != nil {
			t.Fatalf("LoadConfig: %v", err)
		}
		if got := cfg.DgraphHTTPEndpoints["localhost:9080"]; got != "localhost:8080" {
			t.Errorf("YAML entry lost: got %q", got)
		}
		if got := cfg.DgraphHTTPEndpoints["alpha:9091"]; got != "alpha:7777" {
			t.Errorf("env entry missing: got %q", got)
		}
	})

	t.Run("env overrides matching YAML entry", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.yaml")
		body := "balancer_type: round-robin\n" +
			"dgraph_endpoints:\n  - localhost:9080\n" +
			"dgraph_http_endpoints:\n  \"localhost:9080\": \"localhost:8080\"\n"
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("CONFIG_FILE", path)
		t.Setenv("DGRAPH_HTTP_ENDPOINTS", "localhost:9080=other:7777")

		cfg, err := LoadConfig()
		if err != nil {
			t.Fatalf("LoadConfig: %v", err)
		}
		if got := cfg.DgraphHTTPEndpoints["localhost:9080"]; got != "other:7777" {
			t.Fatalf("env should override YAML; got %q", got)
		}
	})

	t.Run("malformed pair is skipped with warning", func(t *testing.T) {
		writeMinimalConfigFile(t)
		t.Setenv("DGRAPH_HTTP_ENDPOINTS", "no-equals,grpc=,=http,valid:1=valid:2")

		var buf bytes.Buffer
		orig := log.Writer()
		log.SetOutput(&buf)
		defer log.SetOutput(orig)

		cfg, err := LoadConfig()
		if err != nil {
			t.Fatalf("LoadConfig: %v", err)
		}
		if got := cfg.DgraphHTTPEndpoints["valid:1"]; got != "valid:2" {
			t.Errorf("valid pair missing: %v", cfg.DgraphHTTPEndpoints)
		}
		if _, ok := cfg.DgraphHTTPEndpoints["no-equals"]; ok {
			t.Errorf("malformed entry leaked into map: %v", cfg.DgraphHTTPEndpoints)
		}
		if !strings.Contains(buf.String(), "invalid DGRAPH_HTTP_ENDPOINTS entry") {
			t.Errorf("expected warning log for malformed entry, got:\n%s", buf.String())
		}
	})
}

func TestLoadConfig_TrustedProxyCIDRsEnv(t *testing.T) {
	writeMinimalConfigFile(t)
	t.Setenv("TRUSTED_PROXY_CIDRS", " 10.0.0.0/24, , 192.168.1.0/24 ")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	want := []string{"10.0.0.0/24", "192.168.1.0/24"}
	if len(cfg.TrustedProxyCIDRs) != len(want) {
		t.Fatalf("TrustedProxyCIDRs=%v, want %v", cfg.TrustedProxyCIDRs, want)
	}
	for i, w := range want {
		if cfg.TrustedProxyCIDRs[i] != w {
			t.Errorf("[%d]=%q want %q", i, cfg.TrustedProxyCIDRs[i], w)
		}
	}
}

func TestLoadConfig_MaxBodyBytesInvalidIgnored(t *testing.T) {
	writeMinimalConfigFile(t)
	t.Setenv("MAX_BODY_BYTES", "not-a-number")

	var buf bytes.Buffer
	orig := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(orig)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.MaxBodyBytes != DefaultMaxBodyBytes {
		t.Fatalf("invalid env should have fallen back to default, got %d", cfg.MaxBodyBytes)
	}
	if !strings.Contains(buf.String(), "invalid MAX_BODY_BYTES") {
		t.Errorf("expected warning, got:\n%s", buf.String())
	}
}

// TestLoadConfig_ProdModeWebsocketDisabledSkipsValidation proves
// validateSecurity does not fail closed when the operator turns the
// WebSocket server off entirely. Without this exemption, every non-WS
// production deployment would have to pretend to set ws_token.
func TestLoadConfig_ProdModeWebsocketDisabledSkipsValidation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	body := "balancer_type: round-robin\n" +
		"dgraph_endpoints:\n  - localhost:9080\n" +
		"dev_mode: false\n" +
		"enable_websocket: false\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CONFIG_FILE", path)
	t.Setenv("WS_TOKEN", "")
	t.Setenv("WS_ALLOWED_ORIGINS", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig should accept dev_mode=false when WS is off: %v", err)
	}
	if cfg.EnableWebSocket == nil || *cfg.EnableWebSocket {
		t.Fatalf("EnableWebSocket should be false")
	}
}

// TestPtrBool keeps the tiny helper covered. ptrBool is currently used
// from a defensive branch in the env parser that other tests don't
// exercise; calling it directly is the cheapest way to keep the
// coverage signal honest.
func TestPtrBool(t *testing.T) {
	if got := ptrBool(true); got == nil || !*got {
		t.Fatalf("ptrBool(true) = %v", got)
	}
	if got := ptrBool(false); got == nil || *got {
		t.Fatalf("ptrBool(false) = %v", got)
	}
}

func TestRedactedYAML_NilConfigErrors(t *testing.T) {
	if _, err := redactedYAML(nil); err == nil {
		t.Fatal("expected error on nil config")
	}
}

func TestNormalizeLegacyKeys_NoLegacyKeys(t *testing.T) {
	in := []byte("balancer_type: round-robin\n")
	out := normalizeLegacyKeys(in)
	if string(out) != string(in) {
		t.Fatalf("non-legacy input mutated:\nin:  %s\nout: %s", in, out)
	}
}
