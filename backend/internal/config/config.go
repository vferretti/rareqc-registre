// Package config centralizes the runtime configuration read from the
// environment. Dev defaults keep `go run ./cmd/api/` working out of the box
// against the rareqc-infra platform; in production (GIN_MODE=release) the
// critical values must be provided explicitly or the server refuses to start.
package config

import (
	"fmt"
	"net"
	"os"
	"strings"
)

// DB holds the PostgreSQL connection settings.
type DB struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
	// SSLMode is passed to the driver (disable, require, verify-full...).
	SSLMode string
}

// Config is the application-level configuration (HTTP + database).
type Config struct {
	// Production is true when GIN_MODE=release.
	Production bool
	// Port the HTTP server listens on (PORT).
	Port string
	// CORSAllowedOrigins is the browser origin allowlist
	// (CORS_ALLOWED_ORIGINS, comma-separated). Dev default: "*".
	CORSAllowedOrigins []string
	// TrustedProxies lists the proxies (IPs or CIDRs, TRUSTED_PROXIES,
	// comma-separated) allowed to set X-Forwarded-* headers. Empty = none:
	// the client IP is then always the TCP peer.
	TrustedProxies []string
	DB             DB
}

const devDBPassword = "rareqc"

// Load reads the configuration from the environment. In production it
// returns an error listing every critical setting left to its dev default,
// so the failure is actionable in one pass.
func Load() (Config, error) {
	cfg := Config{
		Production:         os.Getenv("GIN_MODE") == "release",
		Port:               getEnvOrDefault("PORT", "8080"),
		CORSAllowedOrigins: SplitAndTrim(getEnvOrDefault("CORS_ALLOWED_ORIGINS", "*")),
		TrustedProxies:     SplitAndTrim(os.Getenv("TRUSTED_PROXIES")),
		DB: DB{
			Host: getEnvOrDefault("POSTGRES_HOST", "localhost"),
			// 5440 = port hôte du postgres de la plateforme rareqc-infra,
			// pour que `go run ./cmd/api/` fonctionne sans .env en dev.
			Port:     getEnvOrDefault("POSTGRES_PORT", "5440"),
			User:     getEnvOrDefault("POSTGRES_USER", "rareqc"),
			Password: getEnvOrDefault("POSTGRES_PASSWORD", devDBPassword),
			Name:     getEnvOrDefault("POSTGRES_DB", "rareqc_registre"),
			SSLMode:  getEnvOrDefault("POSTGRES_SSLMODE", "disable"),
		},
	}

	if err := validateProxies(cfg.TrustedProxies); err != nil {
		return cfg, err
	}
	if !cfg.Production {
		return cfg, nil
	}

	var problems []string
	if os.Getenv("POSTGRES_PASSWORD") == "" || cfg.DB.Password == devDBPassword {
		problems = append(problems, "POSTGRES_PASSWORD must be set to a non-dev value")
	}
	if os.Getenv("POSTGRES_SSLMODE") == "" {
		problems = append(problems, "POSTGRES_SSLMODE must be set explicitly (e.g. require)")
	}
	if os.Getenv("CORS_ALLOWED_ORIGINS") == "" || contains(cfg.CORSAllowedOrigins, "*") {
		problems = append(problems, "CORS_ALLOWED_ORIGINS must list explicit origins (no *)")
	}
	if len(problems) > 0 {
		return cfg, fmt.Errorf("refusing to start in production (GIN_MODE=release):\n  - %s",
			strings.Join(problems, "\n  - "))
	}
	return cfg, nil
}

// SplitAndTrim parses a comma-separated list, trimming whitespace and
// dropping empty entries.
func SplitAndTrim(raw string) []string {
	var out []string
	for _, s := range strings.Split(raw, ",") {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func contains(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

func getEnvOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// validateProxies checks that every TRUSTED_PROXIES entry is an IP or a
// CIDR, so a typo is reported at startup rather than silently trusting no
// one or panicking in the router.
func validateProxies(proxies []string) error {
	for _, p := range proxies {
		if net.ParseIP(p) != nil {
			continue
		}
		if _, _, err := net.ParseCIDR(p); err != nil {
			return fmt.Errorf("TRUSTED_PROXIES: %q is neither an IP nor a CIDR", p)
		}
	}
	return nil
}
