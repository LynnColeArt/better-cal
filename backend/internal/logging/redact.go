package logging

import "net/http"

const Redacted = "<redacted>"

var secretHeaders = map[string]struct{}{
	"Authorization":        {},
	"Cookie":               {},
	"Set-Cookie":           {},
	"X-Cal-Secret-Key":     {},
	"X-Cal-Cron-Signature": {},
	"X-Vercel-Signature":   {},
}

func RedactHeaders(headers http.Header) map[string][]string {
	redacted := make(map[string][]string, len(headers))
	for key, values := range headers {
		canonical := http.CanonicalHeaderKey(key)
		if _, ok := secretHeaders[canonical]; ok {
			redacted[canonical] = []string{Redacted}
			continue
		}
		redacted[canonical] = append([]string(nil), values...)
	}
	return redacted
}
