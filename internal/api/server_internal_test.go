package api

import (
	"net/http/httptest"
	"testing"

	"kala/internal/config"
)

func TestClientIP(t *testing.T) {
	cases := []struct {
		name  string
		trust bool
		xff   string
		want  string
	}{
		{"direct", false, "", "192.0.2.1"},
		{"header ignored without a trusted proxy", false, "203.0.113.9", "192.0.2.1"},
		{"single hop", true, "203.0.113.9", "203.0.113.9"},
		// The proxy appends the peer it saw; anything before is client-supplied.
		{"spoofed prefix", true, "10.0.0.1, 203.0.113.9", "203.0.113.9"},
		{"trusted but no header", true, "", "192.0.2.1"},
	}
	for _, tc := range cases {
		s := &Server{cfg: config.Config{TrustProxy: tc.trust}}
		r := httptest.NewRequest("POST", "/api/v1/auth/login", nil)
		r.RemoteAddr = "192.0.2.1:4711"
		if tc.xff != "" {
			r.Header.Set("X-Forwarded-For", tc.xff)
		}
		if got := s.clientIP(r); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}
