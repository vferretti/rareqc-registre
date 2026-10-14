package auth

import (
	"reflect"
	"strings"
	"testing"
)

func clearAuthEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"KEYCLOAK_HOST", "KEYCLOAK_INTERNAL_HOST", "KEYCLOAK_REALM",
		"KEYCLOAK_CLIENT", "KEYCLOAK_CLIENT_SECRET",
		"KEYCLOAK_PARTICIPANT_CLIENT", "KEYCLOAK_PARTICIPANT_CLIENT_SECRET",
		"PARTICIPANT_PORTAL_HOSTS", "SESSION_SECRET", "PORTAL_HOST", "COOKIE_SECURE",
	} {
		t.Setenv(k, "")
	}
}

func TestLoadConfigDevDefaults(t *testing.T) {
	clearAuthEnv(t)

	cfg, err := LoadConfig(false)
	if err != nil {
		t.Fatalf("LoadConfig(false) with dev defaults should pass: %v", err)
	}
	if cfg.ClientID != "registre-admin-bff" {
		t.Errorf("ClientID = %q", cfg.ClientID)
	}
	if cfg.ParticipantClientID != "portail-participant-bff" {
		t.Errorf("ParticipantClientID = %q", cfg.ParticipantClientID)
	}
	want := []string{"http://localhost:5174", "http://localhost:3002"}
	if !reflect.DeepEqual(cfg.ParticipantPortalHosts, want) {
		t.Errorf("ParticipantPortalHosts = %v, want %v", cfg.ParticipantPortalHosts, want)
	}
}

func TestLoadConfigProductionRejectsDevSecrets(t *testing.T) {
	clearAuthEnv(t)

	_, err := LoadConfig(true)
	if err == nil {
		t.Fatal("LoadConfig(true) with dev secrets should fail")
	}
	for _, want := range []string{"SESSION_SECRET", "KEYCLOAK_CLIENT_SECRET", "KEYCLOAK_PARTICIPANT_CLIENT_SECRET"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error should mention %s, got: %v", want, err)
		}
	}
}

// setProductionEnv provides a complete, valid production configuration;
// tests then break one setting at a time.
func setProductionEnv(t *testing.T) {
	t.Helper()
	clearAuthEnv(t)
	t.Setenv("SESSION_SECRET", "real-session-secret-with-enough-entropy-0123456789")
	t.Setenv("KEYCLOAK_CLIENT_SECRET", "real-client-secret")
	t.Setenv("KEYCLOAK_PARTICIPANT_CLIENT_SECRET", "real-participant-secret")
	t.Setenv("KEYCLOAK_HOST", "https://auth.rareqc.example")
	t.Setenv("PORTAL_HOST", "https://registre.rareqc.example")
	t.Setenv("PARTICIPANT_PORTAL_HOSTS", "https://portail.rareqc.example")
	t.Setenv("COOKIE_SECURE", "true")
}

func TestLoadConfigProductionRejectsInsecureTransport(t *testing.T) {
	cases := []struct {
		name  string
		key   string
		value string
		want  string
	}{
		{"cookie not secure", "COOKIE_SECURE", "false", "COOKIE_SECURE"},
		{"short session secret", "SESSION_SECRET", "short-but-not-the-dev-one", "SESSION_SECRET must be at least"},
		{"keycloak over http", "KEYCLOAK_HOST", "http://auth.rareqc.example", "KEYCLOAK_HOST"},
		{"portal over http", "PORTAL_HOST", "http://registre.rareqc.example", "PORTAL_HOST"},
		{"participant portal over http", "PARTICIPANT_PORTAL_HOSTS", "https://a.example,http://b.example", "PARTICIPANT_PORTAL_HOSTS"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setProductionEnv(t)
			t.Setenv(tc.key, tc.value)
			_, err := LoadConfig(true)
			if err == nil {
				t.Fatalf("LoadConfig(true) with %s=%q should fail", tc.key, tc.value)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error should mention %q, got: %v", tc.want, err)
			}
		})
	}
}

func TestLoadConfigDevAcceptsInsecureTransport(t *testing.T) {
	clearAuthEnv(t)
	t.Setenv("COOKIE_SECURE", "false")
	if _, err := LoadConfig(false); err != nil {
		t.Fatalf("dev mode must keep working over http: %v", err)
	}
}

func TestBrowserOrigins(t *testing.T) {
	clearAuthEnv(t)
	t.Setenv("PORTAL_HOST", "http://localhost:5173/")
	t.Setenv("PARTICIPANT_PORTAL_HOSTS", "http://localhost:5174, http://localhost:3002/")
	cfg, err := LoadConfig(false)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"http://localhost:5173", "http://localhost:5174", "http://localhost:3002"}
	if got := cfg.BrowserOrigins(); !reflect.DeepEqual(got, want) {
		t.Errorf("BrowserOrigins() = %v, want %v", got, want)
	}
}

func TestLoadConfigProductionAcceptsRealSecrets(t *testing.T) {
	setProductionEnv(t)

	if _, err := LoadConfig(true); err != nil {
		t.Fatalf("LoadConfig(true) with real secrets should pass: %v", err)
	}
}

func TestSplitHosts(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"http://a:1,http://b:2", []string{"http://a:1", "http://b:2"}},
		{" http://a:1 , http://b:2/ ", []string{"http://a:1", "http://b:2"}},
		{"http://a:1,,", []string{"http://a:1"}},
		{"", nil},
	}
	for _, tc := range cases {
		if got := splitHosts(tc.in); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("splitHosts(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
