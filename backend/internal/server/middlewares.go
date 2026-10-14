package server

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"net/url"
	"runtime/debug"
	"strings"

	"github.com/gin-gonic/gin"
)

// Context key and response header carrying the per-request correlation id.
const (
	requestIDKey    = "request_id"
	requestIDHeader = "X-Request-ID"
)

// Body size caps: JSON payloads stay small (id lists, forms), multipart
// uploads get the 10 MB document limit plus room for the form fields.
const (
	maxJSONBodyBytes      = 1 << 20
	maxUploadBytes        = 10 << 20
	maxMultipartBodyBytes = maxUploadBytes + (1 << 20)
)

// requestID stamps every request with a random id, echoed in the
// X-Request-ID response header and attached to log lines, so a 500 seen by
// a user can be matched with its server-side error.
func requestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := newRequestID()
		c.Set(requestIDKey, id)
		c.Header(requestIDHeader, id)
		c.Next()
	}
}

// newRequestID returns 16 random bytes, hex-encoded.
func newRequestID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "unavailable"
	}
	return hex.EncodeToString(b)
}

// RequestID returns the correlation id set by the requestID middleware.
func RequestID(c *gin.Context) string {
	return c.GetString(requestIDKey)
}

// recovery turns a panic into a generic 500 and a structured log line
// (gin.Recovery would print the stack as plain text in the JSON stream).
func recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("panic recovered",
					"panic", rec,
					"request_id", RequestID(c),
					"method", c.Request.Method,
					"path", c.FullPath(),
					"stack", string(debug.Stack()))
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
			}
		}()
		c.Next()
	}
}

// securityHeaders applies the browser hardening headers to every response.
// The API only serves JSON and file downloads containing health data, so
// nothing may be cached, framed or content-sniffed. HSTS is only meaningful
// over TLS, hence production only (the TLS proxy may also set it).
func securityHeaders(production bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Cache-Control", "no-store")
		h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		if production {
			h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
		}
		c.Next()
	}
}

// swaggerCSP relaxes the API-wide policy for the Swagger UI, which runs its
// own scripts and inline styles. Only mounted outside release, like the UI.
func swaggerCSP() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Content-Security-Policy",
			"default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; frame-ancestors 'none'")
		c.Next()
	}
}

// bodyLimit caps the request body before any handler reads it, so an
// oversized upload is cut off instead of being spooled to disk first.
func bodyLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		limit := int64(maxJSONBodyBytes)
		if strings.HasPrefix(c.ContentType(), "multipart/form-data") {
			limit = maxMultipartBodyBytes
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
		c.Next()
	}
}

// csrfGuard is the defence in depth behind SameSite=Lax: a state-changing
// request authenticated by session cookie must come from one of our own
// portals. Browsers always send Origin on cross-origin and on non-GET
// same-origin fetches; Referer is the fallback for the few that do not.
// Bearer-authenticated calls (scripts) carry no cookie and are exempt.
func csrfGuard(allowedOrigins []string) gin.HandlerFunc {
	allowed := make(map[string]bool, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allowed[normalizeOrigin(o)] = true
	}
	return func(c *gin.Context) {
		switch c.Request.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			c.Next()
			return
		}
		if strings.HasPrefix(c.GetHeader("Authorization"), "Bearer ") {
			c.Next()
			return
		}
		origin := c.GetHeader("Origin")
		if origin == "" {
			origin = refererOrigin(c.GetHeader("Referer"))
		}
		if origin == "" || !allowed[normalizeOrigin(origin)] {
			slog.Warn("request blocked by csrf guard",
				"origin", origin, "method", c.Request.Method, "path", c.FullPath(),
				"request_id", RequestID(c))
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Origin not allowed"})
			return
		}
		c.Next()
	}
}

// normalizeOrigin lower-cases and strips the trailing slash so configured
// hosts compare equal to the Origin header.
func normalizeOrigin(o string) string {
	return strings.ToLower(strings.TrimRight(strings.TrimSpace(o), "/"))
}

// refererOrigin reduces a Referer URL to its scheme://host origin.
func refererOrigin(referer string) string {
	if referer == "" {
		return ""
	}
	u, err := url.Parse(referer)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host
}
