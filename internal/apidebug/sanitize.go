package apidebug

import (
	"net/http"
	"strings"
)

var sensitiveHeaderKeys = map[string]struct{}{
	"authorization":   {},
	"api-key":         {},
	"x-api-key":       {},
	"chatgpt-account-id": {},
}

func SanitizeHeaders(h http.Header) map[string]string {
	if h == nil {
		return nil
	}
	out := make(map[string]string, len(h))
	for key, vals := range h {
		lk := strings.ToLower(strings.TrimSpace(key))
		val := strings.Join(vals, ", ")
		if _, ok := sensitiveHeaderKeys[lk]; ok {
			out[key] = maskSecret(val)
			continue
		}
		if lk == "authorization" || strings.Contains(lk, "token") {
			out[key] = maskSecret(val)
			continue
		}
		out[key] = val
	}
	return out
}

func SanitizeHeaderMap(h map[string]string) map[string]string {
	if len(h) == 0 {
		return nil
	}
	out := make(map[string]string, len(h))
	for key, val := range h {
		lk := strings.ToLower(strings.TrimSpace(key))
		if _, ok := sensitiveHeaderKeys[lk]; ok {
			out[key] = maskSecret(val)
			continue
		}
		if strings.Contains(lk, "token") || strings.Contains(lk, "authorization") {
			out[key] = maskSecret(val)
			continue
		}
		out[key] = val
	}
	return out
}

func maskSecret(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(s), "bearer ") {
		return "Bearer ***"
	}
	if len(s) <= 4 {
		return "***"
	}
	return s[:2] + "***" + s[len(s)-2:]
}

func TruncateBody(body []byte) (text string, truncated bool) {
	if len(body) == 0 {
		return "", false
	}
	if len(body) <= MaxBodyBytes {
		return string(body), false
	}
	return string(body[:MaxBodyBytes]), true
}

func TruncateString(body string) (text string, truncated bool) {
	if len(body) == 0 {
		return "", false
	}
	if len(body) <= MaxBodyBytes {
		return body, false
	}
	return body[:MaxBodyBytes], true
}
