package cookiepolicy

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestUsesHTTPS(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*http.Request)
		want  bool
	}{
		{
			name: "direct tls",
			setup: func(r *http.Request) {
				r.TLS = &tls.ConnectionState{}
			},
			want: true,
		},
		{
			name: "simple forwarded proto",
			setup: func(r *http.Request) {
				r.Header.Set("X-Forwarded-Proto", "https")
			},
			want: true,
		},
		{
			name: "multi proxy forwarded proto",
			setup: func(r *http.Request) {
				r.Header.Add("X-Forwarded-Proto", "https,http")
				r.Header.Add("X-Forwarded-Proto", "http")
			},
			want: true,
		},
		{
			name: "first forwarded proto wins for http",
			setup: func(r *http.Request) {
				r.Header.Set("X-Forwarded-Proto", "http,https")
			},
			want: false,
		},
		{
			name: "forwarded proto entry",
			setup: func(r *http.Request) {
				r.Header.Set("Forwarded", "for=1.2.3.4;proto=https, for=5.6.7.8;proto=http")
			},
			want: true,
		},
		{
			name: "first forwarded entry wins for http",
			setup: func(r *http.Request) {
				r.Header.Set("Forwarded", "for=1.2.3.4;proto=http, for=5.6.7.8;proto=https")
			},
			want: false,
		},
		{
			name:  "plain http",
			setup: func(*http.Request) {},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			tt.setup(req)

			if got := RequestUsesHTTPS(req); got != tt.want {
				t.Fatalf("RequestUsesHTTPS() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShouldUseSecureCookieForRedirectURL(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{name: "https localhost", input: "https://localhost:8080/oauth2/callback", want: true},
		{name: "https production", input: "https://example.com/oauth2/callback", want: true},
		{name: "http localhost", input: "http://localhost:8080/oauth2/callback", want: false},
		{name: "invalid", input: "://bad", want: false},
		{name: "empty", input: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ShouldUseSecureCookieForRedirectURL(tt.input); got != tt.want {
				t.Fatalf("ShouldUseSecureCookieForRedirectURL(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
