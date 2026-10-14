package config

import (
	"reflect"
	"strings"
	"testing"
)

// clearEnv blanks every variable Load reads, so tests are hermetic to the
// developer's shell (t.Setenv also restores the previous values).
func clearEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"GIN_MODE", "PORT", "CORS_ALLOWED_ORIGINS",
		"POSTGRES_HOST", "POSTGRES_PORT", "POSTGRES_USER",
		"POSTGRES_PASSWORD", "POSTGRES_DB", "POSTGRES_SSLMODE",
	} {
		t.Setenv(k, "")
	}
}

func TestLoadDevDefaults(t *testing.T) {
	clearEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() in dev mode returned error: %v", err)
	}
	if cfg.Production {
		t.Error("Production should be false without GIN_MODE=release")
	}
	if cfg.Port != "8080" {
		t.Errorf("Port = %q, want 8080", cfg.Port)
	}
	if !reflect.DeepEqual(cfg.CORSAllowedOrigins, []string{"*"}) {
		t.Errorf("CORSAllowedOrigins = %v, want [*]", cfg.CORSAllowedOrigins)
	}
	if cfg.DB.SSLMode != "disable" {
		t.Errorf("DB.SSLMode = %q, want disable", cfg.DB.SSLMode)
	}
}

func TestLoadReleaseRefusesDevDefaults(t *testing.T) {
	clearEnv(t)
	t.Setenv("GIN_MODE", "release")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() in release mode with dev defaults should fail")
	}
	for _, want := range []string{"POSTGRES_PASSWORD", "POSTGRES_SSLMODE", "CORS_ALLOWED_ORIGINS"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error should mention %s, got: %v", want, err)
		}
	}
}

func TestLoadReleaseRejectsWildcardCORS(t *testing.T) {
	clearEnv(t)
	t.Setenv("GIN_MODE", "release")
	t.Setenv("POSTGRES_PASSWORD", "s3cret")
	t.Setenv("POSTGRES_SSLMODE", "require")
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://registre.example.org,*")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "CORS_ALLOWED_ORIGINS") {
		t.Fatalf("wildcard CORS in release should fail, got: %v", err)
	}
}

func TestLoadReleaseAcceptsExplicitValues(t *testing.T) {
	clearEnv(t)
	t.Setenv("GIN_MODE", "release")
	t.Setenv("POSTGRES_PASSWORD", "s3cret")
	t.Setenv("POSTGRES_SSLMODE", "require")
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://registre.example.org")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() with explicit values should pass: %v", err)
	}
	if !cfg.Production {
		t.Error("Production should be true with GIN_MODE=release")
	}
	if cfg.DB.Password != "s3cret" || cfg.DB.SSLMode != "require" {
		t.Errorf("DB config not picked up: %+v", cfg.DB)
	}
}

func TestSplitAndTrim(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"*", []string{"*"}},
		{"a,b", []string{"a", "b"}},
		{" a , b ", []string{"a", "b"}},
		{"a,,b,", []string{"a", "b"}},
		{"", nil},
	}
	for _, tc := range cases {
		if got := SplitAndTrim(tc.in); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("SplitAndTrim(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestLoadValidatesTrustedProxies(t *testing.T) {
	t.Setenv("GIN_MODE", "")
	t.Setenv("TRUSTED_PROXIES", "10.0.0.1, 10.1.0.0/16")
	if _, err := Load(); err != nil {
		t.Fatalf("valid proxies rejected: %v", err)
	}
	t.Setenv("TRUSTED_PROXIES", "10.0.0.1, nginx")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), `"nginx"`) {
		t.Fatalf("malformed proxy accepted or badly reported: %v", err)
	}
}
