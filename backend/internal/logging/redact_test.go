package logging

import (
	"net/http"
	"testing"
)

func TestRedactHeaders(t *testing.T) {
	headers := http.Header{}
	headers.Set("authorization", "Bearer secret")
	headers.Set("x-cal-secret-key", "platform-secret")
	headers.Set("cal-api-version", "2024-08-13")

	redacted := RedactHeaders(headers)
	if got := redacted["Authorization"]; len(got) != 1 || got[0] != Redacted {
		t.Fatalf("Authorization = %v", got)
	}
	if got := redacted["X-Cal-Secret-Key"]; len(got) != 1 || got[0] != Redacted {
		t.Fatalf("X-Cal-Secret-Key = %v", got)
	}
	if got := redacted["Cal-Api-Version"]; len(got) != 1 || got[0] != "2024-08-13" {
		t.Fatalf("Cal-Api-Version = %v", got)
	}
}
