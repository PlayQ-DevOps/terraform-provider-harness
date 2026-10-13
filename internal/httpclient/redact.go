package httpclient

import (
	"net/http"
	"regexp"
	"strings"
)

const redacted = "***"

var (
	// JSON string members whose key suggests a secret value. The key is captured so the
	// exclusions below (references, identifiers, names) can be applied.
	secretMemberRe       = regexp.MustCompile(`(?i)"([^"\\]*(?:password|passphrase|token|secret|api_?key|private_?key|credential|access_?key|secret_?key|bearer)[^"\\]*)"(\s*:\s*)"(?:[^"\\]|\\.)*"`)
	secretKeyExclusionRe = regexp.MustCompile(`(?i)(ref|refs|id|ids|identifier|identifiers|name|names|type|types|path|url|status|version|enabled|scope|ttl|length)$`)
	credentialHeaders    = map[string]bool{"X-Api-Key": true, "Authorization": true, "Cookie": true, "Set-Cookie": true, "Proxy-Authorization": true}
	queryKeyRe           = regexp.MustCompile(`(?i)([?&](?:[^=&]*(?:token|key|secret|password)[^=&]*)=)[^&]*`)
)

// Redact masks secret-looking JSON members and every literal secret in text.
func Redact(text string, secrets []string) string {
	out := secretMemberRe.ReplaceAllStringFunc(text, func(m string) string {
		sub := secretMemberRe.FindStringSubmatch(m)
		if len(sub) < 3 || secretKeyExclusionRe.MatchString(sub[1]) {
			return m
		}
		return `"` + sub[1] + `"` + sub[2] + `"` + redacted + `"`
	})
	for _, s := range secrets {
		if s != "" {
			out = strings.ReplaceAll(out, s, redacted)
		}
	}
	return out
}

// RedactHeaders returns a copy of h with credential headers masked and literal secrets removed.
func RedactHeaders(h http.Header, secrets []string) map[string]string {
	out := make(map[string]string, len(h))
	for k, vs := range h {
		if credentialHeaders[http.CanonicalHeaderKey(k)] {
			out[k] = redacted
			continue
		}
		out[k] = Redact(strings.Join(vs, ", "), secrets)
	}
	return out
}

// RedactURL masks secret-looking query parameters and literal secrets in a URL.
func RedactURL(u string, secrets []string) string {
	out := queryKeyRe.ReplaceAllString(u, "${1}"+redacted)
	for _, s := range secrets {
		if s != "" {
			out = strings.ReplaceAll(out, s, redacted)
		}
	}
	return out
}
