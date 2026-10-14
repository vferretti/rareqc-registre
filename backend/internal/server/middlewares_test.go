package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// newTestRouter builds a minimal engine with the hardening middlewares and
// one echo route per method, mirroring SetupRouter's ordering.
func newTestRouter(production bool, origins []string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(requestID(), recovery(), securityHeaders(production))
	api := r.Group("/api")
	api.Use(bodyLimit(), csrfGuard(origins))
	ok := func(c *gin.Context) { c.String(http.StatusOK, "ok") }
	api.GET("/ping", ok)
	api.POST("/ping", ok)
	api.PUT("/ping", ok)
	api.DELETE("/ping", ok)
	api.POST("/echo", func(c *gin.Context) {
		var body struct {
			Data string `json:"data"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.String(http.StatusBadRequest, err.Error())
			return
		}
		c.String(http.StatusOK, "ok")
	})
	api.GET("/panic", func(*gin.Context) { panic("boom") })
	return r
}

func do(r *gin.Engine, method, path string, headers map[string]string, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	r.ServeHTTP(w, req)
	return w
}

func TestSecurityHeaders(t *testing.T) {
	w := do(newTestRouter(false, nil), http.MethodGet, "/api/ping", nil, "")
	want := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "no-referrer",
		"Cache-Control":          "no-store",
	}
	for k, v := range want {
		if got := w.Header().Get(k); got != v {
			t.Errorf("%s = %q, want %q", k, got, v)
		}
	}
	if w.Header().Get("Strict-Transport-Security") != "" {
		t.Error("HSTS must not be set outside production")
	}
	if len(w.Header().Get("X-Request-ID")) != 32 {
		t.Errorf("X-Request-ID = %q, want 32 hex chars", w.Header().Get("X-Request-ID"))
	}

	w = do(newTestRouter(true, nil), http.MethodGet, "/api/ping", nil, "")
	if !strings.HasPrefix(w.Header().Get("Strict-Transport-Security"), "max-age=") {
		t.Error("HSTS must be set in production")
	}
}

func TestRecoveryReturnsGeneric500(t *testing.T) {
	w := do(newTestRouter(false, nil), http.MethodGet, "/api/panic", nil, "")
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
	if strings.Contains(w.Body.String(), "boom") {
		t.Errorf("panic value leaked to the client: %s", w.Body.String())
	}
}

func TestCSRFGuard(t *testing.T) {
	origins := []string{"http://localhost:5173", "https://portail.example/"}
	r := newTestRouter(false, origins)
	cases := []struct {
		name    string
		method  string
		headers map[string]string
		want    int
	}{
		{"GET without origin", http.MethodGet, nil, http.StatusOK},
		{"POST without origin", http.MethodPost, nil, http.StatusForbidden},
		{"PUT without origin", http.MethodPut, nil, http.StatusForbidden},
		{"DELETE without origin", http.MethodDelete, nil, http.StatusForbidden},
		{"POST from foreign origin", http.MethodPost, map[string]string{"Origin": "https://evil.example"}, http.StatusForbidden},
		{"POST from the admin portal", http.MethodPost, map[string]string{"Origin": "http://localhost:5173"}, http.StatusOK},
		{"POST from the participant portal (case/slash insensitive)", http.MethodPost, map[string]string{"Origin": "HTTPS://portail.example"}, http.StatusOK},
		{"POST with referer fallback", http.MethodPost, map[string]string{"Referer": "http://localhost:5173/participants/3"}, http.StatusOK},
		{"POST with foreign referer", http.MethodPost, map[string]string{"Referer": "https://evil.example/x"}, http.StatusForbidden},
		{"POST with bearer token (script)", http.MethodPost, map[string]string{"Authorization": "Bearer abc"}, http.StatusOK},
		{"POST with null origin", http.MethodPost, map[string]string{"Origin": "null"}, http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := do(r, tc.method, "/api/ping", tc.headers, "")
			if w.Code != tc.want {
				t.Errorf("status = %d, want %d", w.Code, tc.want)
			}
		})
	}
}

func TestBodyLimit(t *testing.T) {
	r := newTestRouter(false, []string{"http://localhost:5173"})
	headers := map[string]string{"Origin": "http://localhost:5173", "Content-Type": "application/json"}

	small := `{"data":"` + strings.Repeat("x", 1024) + `"}`
	if w := do(r, http.MethodPost, "/api/echo", headers, small); w.Code != http.StatusOK {
		t.Errorf("small body: status = %d, want 200", w.Code)
	}

	huge := `{"data":"` + strings.Repeat("x", maxJSONBodyBytes+1) + `"}`
	if w := do(r, http.MethodPost, "/api/echo", headers, huge); w.Code != http.StatusBadRequest {
		t.Errorf("oversized body: status = %d, want 400", w.Code)
	}
}

func TestRefererOrigin(t *testing.T) {
	cases := map[string]string{
		"https://portail.example/a/b?c=d": "https://portail.example",
		"http://localhost:5173":           "http://localhost:5173",
		"not a url":                       "",
		"":                                "",
	}
	for in, want := range cases {
		if got := refererOrigin(in); got != want {
			t.Errorf("refererOrigin(%q) = %q, want %q", in, got, want)
		}
	}
}
