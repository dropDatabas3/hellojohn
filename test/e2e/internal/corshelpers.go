package internal

import (
	"net/http"
	"strings"
)

// GetHeaderLower returns the header value in lowercase for case-insensitive comparisons.
func GetHeaderLower(resp *http.Response, key string) string {
	return strings.ToLower(resp.Header.Get(key))
}

// HasTokenCI checks if a comma-separated header list contains the token (case-insensitive, trimmed).
func HasTokenCI(list, token string) bool {
	if list == "" || token == "" {
		return false
	}
	token = strings.ToLower(strings.TrimSpace(token))
	for _, part := range strings.Split(list, ",") {
		if strings.ToLower(strings.TrimSpace(part)) == token {
			return true
		}
	}
	return false
}

// VaryContainsOrigin reports whether Vary header includes Origin (case-insensitive token match).
func VaryContainsOrigin(resp *http.Response) bool {
	v := resp.Header.Values("Vary")
	// Values aggregates multiple Vary headers; each may have comma-separated tokens.
	for _, one := range v {
		if HasTokenCI(one, "origin") {
			return true
		}
	}
	// Some servers collapse Vary into single header
	if len(v) == 0 {
		return HasTokenCI(resp.Header.Get("Vary"), "origin")
	}
	return false
}

// ACAOReflects checks if Access-Control-Allow-Origin echoes the provided origin exactly.
func ACAOReflects(resp *http.Response, origin string) bool {
	return resp.Header.Get("Access-Control-Allow-Origin") == origin
}
