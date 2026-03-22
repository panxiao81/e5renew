package cookiepolicy

import (
	"net/http"
	"net/url"
	"strings"
)

func RequestUsesHTTPS(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}

	if strings.EqualFold(firstCommaSeparatedValue(r.Header.Values("X-Forwarded-Proto")), "https") {
		return true
	}

	if strings.EqualFold(firstForwardedProto(r.Header.Values("Forwarded")), "https") {
		return true
	}

	return false
}

func ShouldUseSecureCookieForRedirectURL(rawURL string) bool {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return false
	}

	return strings.EqualFold(u.Scheme, "https")
}
func firstCommaSeparatedValue(values []string) string {
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				return part
			}
		}
	}

	return ""
}

func firstForwardedProto(values []string) string {
	firstEntry := firstCommaSeparatedValue(values)
	for _, part := range strings.Split(firstEntry, ";") {
		key, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if ok && strings.EqualFold(key, "proto") {
			return strings.Trim(value, "\"")
		}
	}

	return ""
}
