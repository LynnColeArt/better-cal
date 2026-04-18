package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LynnColeArt/better-cal/backend/internal/auth"
)

func TestStarterAPIContractSlice(t *testing.T) {
	handler := NewServer(testConfig())
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	assertStatus(t, server.URL, http.MethodGet, "/v2/me", "Bearer cal_test_valid_mock", nil, http.StatusOK)
	assertStatus(t, server.URL, http.MethodGet, "/v2/me", "Bearer invalid", nil, http.StatusUnauthorized)
	assertStatus(t, server.URL, http.MethodGet, "/v2/me", "cal_test_valid_mock", nil, http.StatusUnauthorized)

	platformReq, err := http.NewRequest(http.MethodGet, server.URL+"/v2/oauth-clients/mock-platform-client", nil)
	if err != nil {
		t.Fatal(err)
	}
	platformReq.Header.Set("x-cal-client-id", "mock-platform-client")
	platformReq.Header.Set("x-cal-secret-key", "mock-platform-secret")
	platformResp, body := do(t, platformReq)
	if platformResp.StatusCode != http.StatusOK {
		t.Fatalf("platform client status = %d, body = %s", platformResp.StatusCode, body)
	}
	if bytes.Contains(body, []byte("mock-platform-secret")) {
		t.Fatalf("platform client response leaked secret: %s", body)
	}

	createBody := map[string]any{
		"eventTypeId": 1001,
		"start":       "2026-05-01T15:00:00.000Z",
		"attendee": map[string]any{
			"name":     "Fixture Attendee",
			"email":    "fixture-attendee@example.test",
			"timeZone": "America/Chicago",
		},
		"responses": map[string]any{
			"name":  "Fixture Attendee",
			"email": "fixture-attendee@example.test",
		},
		"metadata": map[string]any{
			"fixture": "personal-basic",
		},
		"idempotencyKey": "fixture-booking-personal-basic",
	}
	assertStatus(t, server.URL, http.MethodPost, "/v2/bookings", "Bearer cal_test_valid_mock", createBody, http.StatusCreated)
	assertStatus(t, server.URL, http.MethodPost, "/v2/bookings", "Bearer cal_test_valid_mock", createBody, http.StatusOK)
	assertStatus(t, server.URL, http.MethodGet, "/v2/bookings/mock-booking-personal-basic", "Bearer cal_test_valid_mock", nil, http.StatusOK)
	assertStatus(t, server.URL, http.MethodPost, "/v2/bookings/mock-booking-personal-basic/cancel", "Bearer cal_test_valid_mock", map[string]any{"cancellationReason": "Fixture cancellation"}, http.StatusOK)
	assertStatus(t, server.URL, http.MethodPost, "/v2/bookings/mock-booking-personal-basic/reschedule", "Bearer cal_test_valid_mock", map[string]any{"start": "2026-05-02T15:00:00.000Z"}, http.StatusOK)
	assertStatus(t, server.URL, http.MethodPost, "/v2/bookings", "Bearer cal_test_unauthorized_mock", createBody, http.StatusForbidden)
}

func TestRequestIDPropagatesToResponse(t *testing.T) {
	handler := NewServer(testConfig())
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	req, err := http.NewRequest(http.MethodGet, server.URL+"/v2/me", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("authorization", "Bearer cal_test_valid_mock")
	req.Header.Set("x-request-id", "external-request-id")

	resp, body := do(t, req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
	}
	if got := resp.Header.Get("x-request-id"); got != "external-request-id" {
		t.Fatalf("x-request-id = %q", got)
	}
	if !bytes.Contains(body, []byte(`"requestId":"external-request-id"`)) {
		t.Fatalf("body did not contain propagated request id: %s", body)
	}
}

func TestAuthRepositoryErrorReturnsInternalError(t *testing.T) {
	service := auth.NewService(testConfig(), auth.WithAPIKeyPrincipalRepository(erroringAPIKeyPrincipals{}))
	handler := NewServer(testConfig(), WithAuthService(service))
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	assertStatus(t, server.URL, http.MethodGet, "/v2/me", "Bearer cal_test_valid_mock", nil, http.StatusInternalServerError)
}

func assertStatus(t *testing.T, baseURL string, method string, path string, authorization string, body any, expected int) {
	t.Helper()
	var requestBody *bytes.Reader
	if body == nil {
		requestBody = bytes.NewReader(nil)
	} else {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		requestBody = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, baseURL+path, requestBody)
	if err != nil {
		t.Fatal(err)
	}
	if authorization != "" {
		req.Header.Set("authorization", authorization)
	}
	if body != nil {
		req.Header.Set("content-type", "application/json")
	}
	resp, responseBody := do(t, req)
	if resp.StatusCode != expected {
		t.Fatalf("%s %s status = %d, want %d, body = %s", method, path, resp.StatusCode, expected, responseBody)
	}
}

func do(t *testing.T, req *http.Request) (*http.Response, []byte) {
	t.Helper()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body := new(bytes.Buffer)
	if _, err := body.ReadFrom(resp.Body); err != nil {
		t.Fatal(err)
	}
	return resp, body.Bytes()
}

type erroringAPIKeyPrincipals struct{}

func (erroringAPIKeyPrincipals) ReadAPIKeyPrincipal(context.Context, string) (auth.Principal, bool, error) {
	return auth.Principal{}, false, errors.New("repository unavailable")
}
