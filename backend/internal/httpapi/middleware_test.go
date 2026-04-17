package httpapi

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/LynnColeArt/better-cal/backend/internal/config"
)

func TestRequestLoggingRedactsSecretHeaders(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	handler := NewServerWithLogger(testConfig(), logger)
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	req, err := http.NewRequest(http.MethodGet, server.URL+"/v2/oauth-clients/mock-platform-client", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("authorization", "Bearer cal_test_valid_mock")
	req.Header.Set("x-cal-client-id", "mock-platform-client")
	req.Header.Set("x-cal-secret-key", "mock-platform-secret")

	resp, body := do(t, req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
	}

	logText := logs.String()
	for _, leaked := range []string{"cal_test_valid_mock", "mock-platform-secret"} {
		if strings.Contains(logText, leaked) {
			t.Fatalf("log leaked %q: %s", leaked, logText)
		}
	}
	if !strings.Contains(logText, "<redacted>") {
		t.Fatalf("log did not contain redaction marker: %s", logText)
	}
}

func TestPanicRecoveryReturnsSafeError(t *testing.T) {
	var logs bytes.Buffer
	server := &Server{
		cfg:    testConfig(),
		logger: slog.New(slog.NewJSONHandler(&logs, nil)),
		mux:    http.NewServeMux(),
	}
	server.mux.HandleFunc("GET /panic", func(http.ResponseWriter, *http.Request) {
		panic("secret panic value")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "secret panic value") {
		t.Fatalf("panic value leaked in response: %s", recorder.Body.String())
	}
	if strings.Contains(logs.String(), "secret panic value") {
		t.Fatalf("panic value leaked in logs: %s", logs.String())
	}
}

func testConfig() config.Config {
	return config.Config{
		APIKey:               "cal_test_valid_mock",
		OAuthClientID:        "mock-oauth-client",
		PlatformClientID:     "mock-platform-client",
		PlatformClientSecret: "mock-platform-secret",
		RequestID:            "mock-request-id",
	}
}
