package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/LynnColeArt/better-cal/backend/internal/auth"
)

func TestStarterAPIContractSlice(t *testing.T) {
	handler := NewServer(testConfig())
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	assertStatus(t, server.URL, http.MethodGet, "/v2/me", "Bearer cal_test_valid_mock", nil, http.StatusOK)
	assertStatus(t, server.URL, http.MethodGet, "/v2/me", "Bearer invalid", nil, http.StatusUnauthorized)
	assertStatus(t, server.URL, http.MethodGet, "/v2/me", "cal_test_valid_mock", nil, http.StatusUnauthorized)
	assertStatus(t, server.URL, http.MethodGet, "/v2/calendar-connections", "Bearer cal_test_valid_mock", nil, http.StatusOK)
	assertStatus(t, server.URL, http.MethodGet, "/v2/calendars", "Bearer cal_test_valid_mock", nil, http.StatusOK)
	assertStatus(t, server.URL, http.MethodGet, "/v2/credentials", "Bearer cal_test_valid_mock", nil, http.StatusOK)
	assertStatus(t, server.URL, http.MethodGet, "/v2/selected-calendars", "Bearer cal_test_valid_mock", nil, http.StatusOK)
	assertStatus(t, server.URL, http.MethodGet, "/v2/destination-calendars", "Bearer cal_test_valid_mock", nil, http.StatusOK)
	assertStatus(t, server.URL, http.MethodPost, "/v2/selected-calendars", "Bearer cal_test_valid_mock", map[string]any{
		"calendarRef": "team-calendar-fixture",
	}, http.StatusOK)
	assertStatus(t, server.URL, http.MethodPost, "/v2/destination-calendars", "Bearer cal_test_valid_mock", map[string]any{
		"calendarRef": "team-calendar-fixture",
	}, http.StatusOK)
	assertStatus(t, server.URL, http.MethodDelete, "/v2/selected-calendars/team-calendar-fixture", "Bearer cal_test_valid_mock", nil, http.StatusOK)
	assertStatus(t, server.URL, http.MethodGet, "/v2/slots?eventTypeId=1001&start=2026-05-01T00:00:00.000Z&end=2026-05-02T00:00:00.000Z&timeZone=America%2FChicago", "", nil, http.StatusOK)

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
	assertStatus(t, server.URL, http.MethodPost, "/v2/bookings/mock-booking-pending-confirm/confirm", "Bearer cal_test_valid_mock", map[string]any{}, http.StatusOK)
	assertStatus(t, server.URL, http.MethodPost, "/v2/bookings/mock-booking-pending-decline/decline", "Bearer cal_test_valid_mock", map[string]any{"reason": "Fixture decline"}, http.StatusOK)
	assertStatus(t, server.URL, http.MethodPost, "/v2/bookings", "Bearer cal_test_unauthorized_mock", createBody, http.StatusForbidden)
	unavailableBody := map[string]any{
		"eventTypeId": 1001,
		"start":       "2026-05-01T16:00:00.000Z",
		"attendee": map[string]any{
			"name":     "Unavailable Slot Fixture",
			"email":    "unavailable-slot@example.test",
			"timeZone": "America/Chicago",
		},
	}
	assertStatus(t, server.URL, http.MethodPost, "/v2/bookings", "Bearer cal_test_valid_mock", unavailableBody, http.StatusBadRequest)
}

func TestSlotsResponseContainsFixtureSlot(t *testing.T) {
	handler := NewServer(testConfig())
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	req, err := http.NewRequest(http.MethodGet, server.URL+"/v2/slots?eventTypeId=1001&start=2026-05-01T00:00:00.000Z&end=2026-05-02T00:00:00.000Z&timeZone=America%2FChicago", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("x-request-id", "slot-request-id")

	resp, body := do(t, req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
	}
	if !bytes.Contains(body, []byte(`"time":"2026-05-01T15:00:00.000Z"`)) {
		t.Fatalf("body did not contain fixture slot: %s", body)
	}
	if !bytes.Contains(body, []byte(`"requestId":"slot-request-id"`)) {
		t.Fatalf("body did not contain request id: %s", body)
	}
}

func TestBookingValidationErrorReturnsBadRequestWithoutEchoingSecret(t *testing.T) {
	handler := NewServer(testConfig())
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	req, err := http.NewRequest(http.MethodPost, server.URL+"/v2/bookings", bytes.NewReader([]byte(`{
		"eventTypeId": 1001,
		"start": "2026-05-01T15:00:00.000Z",
		"metadata": {
			"clientSecret": "super-secret-fixture"
		}
	}`)))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("authorization", "Bearer cal_test_valid_mock")
	req.Header.Set("content-type", "application/json")

	resp, body := do(t, req)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
	}
	if !bytes.Contains(body, []byte(`"code":"SECRET_FIELD_NOT_ALLOWED"`)) {
		t.Fatalf("body did not contain validation code: %s", body)
	}
	if bytes.Contains(body, []byte("super-secret-fixture")) {
		t.Fatalf("validation response echoed secret: %s", body)
	}
}

func TestBookingResourceAuthorizationDeniesWrongOwner(t *testing.T) {
	handler := NewServer(testConfig())
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	createBody := map[string]any{
		"eventTypeId": 1001,
		"start":       "2026-05-01T15:00:00.000Z",
		"attendee": map[string]any{
			"name":     "Fixture Attendee",
			"email":    "fixture-attendee@example.test",
			"timeZone": "America/Chicago",
		},
	}

	assertStatus(t, server.URL, http.MethodPost, "/v2/bookings", "Bearer "+auth.FixtureWrongOwnerAPIKey, createBody, http.StatusForbidden)
	assertStatus(t, server.URL, http.MethodGet, "/v2/bookings/mock-booking-personal-basic", "Bearer "+auth.FixtureWrongOwnerAPIKey, nil, http.StatusForbidden)
	assertStatus(t, server.URL, http.MethodPost, "/v2/bookings/mock-booking-personal-basic/cancel", "Bearer "+auth.FixtureWrongOwnerAPIKey, map[string]any{"cancellationReason": "Fixture cancellation"}, http.StatusForbidden)
	assertStatus(t, server.URL, http.MethodPost, "/v2/bookings/mock-booking-personal-basic/reschedule", "Bearer "+auth.FixtureWrongOwnerAPIKey, map[string]any{"start": "2026-05-02T15:00:00.000Z"}, http.StatusForbidden)
	assertStatus(t, server.URL, http.MethodPost, "/v2/bookings/mock-booking-pending-confirm/confirm", "Bearer "+auth.FixtureWrongOwnerAPIKey, map[string]any{}, http.StatusForbidden)
	assertStatus(t, server.URL, http.MethodPost, "/v2/bookings/mock-booking-pending-decline/decline", "Bearer "+auth.FixtureWrongOwnerAPIKey, map[string]any{"reason": "Fixture decline"}, http.StatusForbidden)
}

func TestCalendarManagementRoundTrip(t *testing.T) {
	handler := NewServer(testConfig())
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	connectionsReq, err := http.NewRequest(http.MethodGet, server.URL+"/v2/calendar-connections", nil)
	if err != nil {
		t.Fatal(err)
	}
	connectionsReq.Header.Set("authorization", "Bearer cal_test_valid_mock")

	resp, body := do(t, connectionsReq)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("connection status = %d, body = %s", resp.StatusCode, body)
	}
	if !bytes.Contains(body, []byte(`"connectionRef":"google-calendar-connection-fixture"`)) {
		t.Fatalf("body did not contain fixture connection: %s", body)
	}

	catalogReq, err := http.NewRequest(http.MethodGet, server.URL+"/v2/calendars", nil)
	if err != nil {
		t.Fatal(err)
	}
	catalogReq.Header.Set("authorization", "Bearer cal_test_valid_mock")

	resp, body = do(t, catalogReq)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("catalog status = %d, body = %s", resp.StatusCode, body)
	}
	if !bytes.Contains(body, []byte(`"calendarRef":"team-calendar-fixture"`)) {
		t.Fatalf("body did not contain fixture team calendar: %s", body)
	}

	req, err := http.NewRequest(http.MethodGet, server.URL+"/v2/selected-calendars", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("authorization", "Bearer cal_test_valid_mock")

	resp, body = do(t, req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
	}
	if !bytes.Contains(body, []byte(`"calendarRef":"selected-calendar-fixture"`)) {
		t.Fatalf("body did not contain fixture selected calendar: %s", body)
	}
	if !bytes.Contains(body, []byte(`"calendarRef":"destination-calendar-fixture"`)) {
		t.Fatalf("body did not contain fixture destination calendar: %s", body)
	}

	saveReq, err := http.NewRequest(http.MethodPost, server.URL+"/v2/selected-calendars", bytes.NewReader([]byte(`{
		"calendarRef": "team-calendar-fixture"
	}`)))
	if err != nil {
		t.Fatal(err)
	}
	saveReq.Header.Set("authorization", "Bearer cal_test_valid_mock")
	saveReq.Header.Set("content-type", "application/json")

	resp, body = do(t, saveReq)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("save status = %d, body = %s", resp.StatusCode, body)
	}
	if !bytes.Contains(body, []byte(`"calendarRef":"team-calendar-fixture"`)) {
		t.Fatalf("save body did not contain team fixture calendar: %s", body)
	}

	destinationReq, err := http.NewRequest(http.MethodPost, server.URL+"/v2/destination-calendars", bytes.NewReader([]byte(`{
		"calendarRef": "team-calendar-fixture"
	}`)))
	if err != nil {
		t.Fatal(err)
	}
	destinationReq.Header.Set("authorization", "Bearer cal_test_valid_mock")
	destinationReq.Header.Set("content-type", "application/json")

	resp, body = do(t, destinationReq)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("destination status = %d, body = %s", resp.StatusCode, body)
	}
	if !bytes.Contains(body, []byte(`"calendarRef":"team-calendar-fixture"`)) {
		t.Fatalf("destination body did not contain team fixture calendar: %s", body)
	}

	readDestinationReq, err := http.NewRequest(http.MethodGet, server.URL+"/v2/destination-calendars", nil)
	if err != nil {
		t.Fatal(err)
	}
	readDestinationReq.Header.Set("authorization", "Bearer cal_test_valid_mock")

	resp, body = do(t, readDestinationReq)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("read destination status = %d, body = %s", resp.StatusCode, body)
	}
	if !bytes.Contains(body, []byte(`"calendarRef":"team-calendar-fixture"`)) {
		t.Fatalf("read destination body did not contain team fixture calendar: %s", body)
	}

	deleteReq, err := http.NewRequest(http.MethodDelete, server.URL+"/v2/selected-calendars/team-calendar-fixture", nil)
	if err != nil {
		t.Fatal(err)
	}
	deleteReq.Header.Set("authorization", "Bearer cal_test_valid_mock")

	resp, body = do(t, deleteReq)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("delete status = %d, body = %s", resp.StatusCode, body)
	}
	if !bytes.Contains(body, []byte(`"destinationCleared":true`)) {
		t.Fatalf("delete body did not clear destination: %s", body)
	}

	readDestinationReq, err = http.NewRequest(http.MethodGet, server.URL+"/v2/destination-calendars", nil)
	if err != nil {
		t.Fatal(err)
	}
	readDestinationReq.Header.Set("authorization", "Bearer cal_test_valid_mock")

	resp, body = do(t, readDestinationReq)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("read destination status = %d, body = %s", resp.StatusCode, body)
	}
	if !bytes.Contains(body, []byte(`"calendar":null`)) {
		t.Fatalf("read destination body did not clear calendar: %s", body)
	}
}

func TestCredentialMetadataDoesNotExposeSecrets(t *testing.T) {
	handler := NewServer(testConfig())
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	req, err := http.NewRequest(http.MethodGet, server.URL+"/v2/credentials", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("authorization", "Bearer cal_test_valid_mock")

	resp, body := do(t, req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
	}
	if !bytes.Contains(body, []byte(`"credentialRef":"google-calendar-credential-fixture"`)) {
		t.Fatalf("body did not contain fixture credential metadata: %s", body)
	}
	for _, forbidden := range [][]byte{
		[]byte("secret"),
		[]byte("token"),
		[]byte("encrypted"),
		[]byte("refresh"),
		[]byte("providerPayload"),
		[]byte("rawProvider"),
	} {
		if bytes.Contains(bytes.ToLower(body), bytes.ToLower(forbidden)) {
			t.Fatalf("credential metadata response exposed forbidden term %q: %s", forbidden, body)
		}
	}
}

func TestOAuthTokenExchangeConsumesAuthorizationCode(t *testing.T) {
	handler := NewServer(testConfig())
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	body := []byte(`{
		"grant_type": "authorization_code",
		"client_id": "mock-oauth-client",
		"code": "mock-oauth-authorization-code",
		"redirect_uri": "https://fixture.example.test/callback"
	}`)
	req, err := http.NewRequest(http.MethodPost, server.URL+"/v2/auth/oauth2/token", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("content-type", "application/json")

	resp, responseBody := do(t, req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, responseBody)
	}
	if !bytes.Contains(responseBody, []byte(`"token_type":"Bearer"`)) {
		t.Fatalf("body did not contain bearer token type: %s", responseBody)
	}
	if !bytes.Contains(responseBody, []byte(`"access_token"`)) || !bytes.Contains(responseBody, []byte(`"refresh_token"`)) {
		t.Fatalf("body did not contain token fields: %s", responseBody)
	}
	if bytes.Contains(responseBody, []byte("mock-oauth-authorization-code")) {
		t.Fatalf("response echoed authorization code: %s", responseBody)
	}
	var tokenResponse map[string]any
	if err := json.Unmarshal(responseBody, &tokenResponse); err != nil {
		t.Fatal(err)
	}
	accessToken, _ := tokenResponse["access_token"].(string)
	refreshToken, _ := tokenResponse["refresh_token"].(string)
	if accessToken == "" {
		t.Fatalf("missing access token in response: %s", responseBody)
	}
	if refreshToken == "" {
		t.Fatalf("missing refresh token in response: %s", responseBody)
	}

	readReq, err := http.NewRequest(http.MethodGet, server.URL+"/v2/bookings/mock-booking-personal-basic", nil)
	if err != nil {
		t.Fatal(err)
	}
	readReq.Header.Set("authorization", "Bearer "+accessToken)
	resp, responseBody = do(t, readReq)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("oauth access-token booking read status = %d, body = %s", resp.StatusCode, responseBody)
	}

	refreshBody := []byte(`{
		"grant_type": "refresh_token",
		"client_id": "mock-oauth-client",
		"refresh_token": "` + refreshToken + `"
	}`)
	req, err = http.NewRequest(http.MethodPost, server.URL+"/v2/auth/oauth2/token", bytes.NewReader(refreshBody))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("content-type", "application/json")

	resp, responseBody = do(t, req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("refresh status = %d, body = %s", resp.StatusCode, responseBody)
	}
	var rotatedResponse map[string]any
	if err := json.Unmarshal(responseBody, &rotatedResponse); err != nil {
		t.Fatal(err)
	}
	rotatedAccessToken, _ := rotatedResponse["access_token"].(string)
	if rotatedAccessToken == "" || rotatedAccessToken == accessToken {
		t.Fatalf("invalid rotated access token in response: %s", responseBody)
	}

	oldReadReq, err := http.NewRequest(http.MethodGet, server.URL+"/v2/bookings/mock-booking-personal-basic", nil)
	if err != nil {
		t.Fatal(err)
	}
	oldReadReq.Header.Set("authorization", "Bearer "+accessToken)
	resp, responseBody = do(t, oldReadReq)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("old oauth access-token read status = %d, body = %s", resp.StatusCode, responseBody)
	}

	readReq, err = http.NewRequest(http.MethodGet, server.URL+"/v2/bookings/mock-booking-personal-basic", nil)
	if err != nil {
		t.Fatal(err)
	}
	readReq.Header.Set("authorization", "Bearer "+rotatedAccessToken)
	resp, responseBody = do(t, readReq)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("rotated oauth access-token booking read status = %d, body = %s", resp.StatusCode, responseBody)
	}

	req, err = http.NewRequest(http.MethodPost, server.URL+"/v2/auth/oauth2/token", bytes.NewReader(refreshBody))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("content-type", "application/json")

	resp, responseBody = do(t, req)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("refresh replay status = %d, body = %s", resp.StatusCode, responseBody)
	}
	if !bytes.Contains(responseBody, []byte(`"error":"invalid_grant"`)) {
		t.Fatalf("refresh replay body did not contain invalid_grant: %s", responseBody)
	}

	req, err = http.NewRequest(http.MethodPost, server.URL+"/v2/auth/oauth2/token", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("content-type", "application/json")

	resp, responseBody = do(t, req)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("replay status = %d, body = %s", resp.StatusCode, responseBody)
	}
	if !bytes.Contains(responseBody, []byte(`"error":"invalid_grant"`)) {
		t.Fatalf("replay body did not contain invalid_grant: %s", responseBody)
	}
	if bytes.Contains(responseBody, []byte("mock-oauth-authorization-code")) {
		t.Fatalf("replay response echoed authorization code: %s", responseBody)
	}
}

func TestOAuthAccessTokenRequiresBookingReadScope(t *testing.T) {
	principal := auth.FixtureAPIKeyPrincipal()
	principal.Permissions = []string{"booking:write"}
	service := auth.NewService(testConfig(), auth.WithOAuthTokenExchangeRepository(staticOAuthTokens{
		byToken: map[string]auth.Principal{
			"write-only-token": principal,
		},
	}))
	handler := NewServer(testConfig(), WithAuthService(service))
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	assertStatus(t, server.URL, http.MethodGet, "/v2/bookings/mock-booking-personal-basic", "Bearer write-only-token", nil, http.StatusForbidden)
}

func TestOAuthAccessTokenAuthorizesBookingWriteRoutes(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		body   map[string]any
		status int
	}{
		{
			name:   "create",
			method: http.MethodPost,
			path:   "/v2/bookings",
			body: map[string]any{
				"eventTypeId": 1001,
				"start":       "2026-05-01T15:00:00.000Z",
				"attendee": map[string]any{
					"name":     "OAuth Attendee",
					"email":    "oauth-attendee@example.test",
					"timeZone": "America/Chicago",
				},
				"idempotencyKey": "oauth-booking-create-fixture",
			},
			status: http.StatusCreated,
		},
		{
			name:   "cancel",
			method: http.MethodPost,
			path:   "/v2/bookings/mock-booking-personal-basic/cancel",
			body: map[string]any{
				"cancellationReason": "OAuth cancellation",
			},
			status: http.StatusOK,
		},
		{
			name:   "reschedule",
			method: http.MethodPost,
			path:   "/v2/bookings/mock-booking-personal-basic/reschedule",
			body: map[string]any{
				"start": "2026-05-02T15:00:00.000Z",
			},
			status: http.StatusOK,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			handler := NewServer(testConfig())
			server := httptest.NewServer(handler)
			t.Cleanup(server.Close)

			accessToken := issueOAuthAccessToken(t, server.URL)
			assertStatus(t, server.URL, testCase.method, testCase.path, "Bearer "+accessToken, testCase.body, testCase.status)
		})
	}
}

func TestOAuthAccessTokenRequiresBookingWriteScope(t *testing.T) {
	principal := auth.FixtureAPIKeyPrincipal()
	principal.Permissions = []string{"booking:read"}
	service := auth.NewService(testConfig(), auth.WithOAuthTokenExchangeRepository(staticOAuthTokens{
		byToken: map[string]auth.Principal{
			"read-only-token": principal,
		},
	}))
	handler := NewServer(testConfig(), WithAuthService(service))
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	assertStatus(t, server.URL, http.MethodPost, "/v2/bookings/mock-booking-personal-basic/cancel", "Bearer read-only-token", map[string]any{
		"cancellationReason": "OAuth cancellation",
	}, http.StatusForbidden)
}

func TestOAuthAccessTokenBookingWriteDeniesWrongOwner(t *testing.T) {
	service := auth.NewService(testConfig(), auth.WithOAuthTokenExchangeRepository(staticOAuthTokens{
		byToken: map[string]auth.Principal{
			"wrong-owner-token": auth.FixtureWrongOwnerAPIKeyPrincipal(),
		},
	}))
	handler := NewServer(testConfig(), WithAuthService(service))
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	assertStatus(t, server.URL, http.MethodPost, "/v2/bookings/mock-booking-personal-basic/cancel", "Bearer wrong-owner-token", map[string]any{
		"cancellationReason": "OAuth cancellation",
	}, http.StatusForbidden)
}

func TestOAuthAccessTokenAuthorizesBookingHostActionRoutes(t *testing.T) {
	tests := []struct {
		name string
		path string
		body map[string]any
	}{
		{
			name: "confirm",
			path: "/v2/bookings/mock-booking-pending-confirm/confirm",
			body: map[string]any{},
		},
		{
			name: "decline",
			path: "/v2/bookings/mock-booking-pending-decline/decline",
			body: map[string]any{"reason": "OAuth decline"},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			handler := NewServer(testConfig())
			server := httptest.NewServer(handler)
			t.Cleanup(server.Close)

			accessToken := issueOAuthAccessToken(t, server.URL)
			assertStatus(t, server.URL, http.MethodPost, testCase.path, "Bearer "+accessToken, testCase.body, http.StatusOK)
		})
	}
}

func TestOAuthAccessTokenRequiresBookingHostActionScope(t *testing.T) {
	principal := auth.FixtureAPIKeyPrincipal()
	principal.Permissions = []string{"booking:read", "booking:write"}
	service := auth.NewService(testConfig(), auth.WithOAuthTokenExchangeRepository(staticOAuthTokens{
		byToken: map[string]auth.Principal{
			"write-only-token": principal,
		},
	}))
	handler := NewServer(testConfig(), WithAuthService(service))
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	assertStatus(t, server.URL, http.MethodPost, "/v2/bookings/mock-booking-pending-confirm/confirm", "Bearer write-only-token", map[string]any{}, http.StatusForbidden)
}

func TestOAuthAccessTokenBookingHostActionDeniesNonHost(t *testing.T) {
	service := auth.NewService(testConfig(), auth.WithOAuthTokenExchangeRepository(staticOAuthTokens{
		byToken: map[string]auth.Principal{
			"non-host-token": auth.FixtureWrongOwnerAPIKeyPrincipal(),
		},
	}))
	handler := NewServer(testConfig(), WithAuthService(service))
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	assertStatus(t, server.URL, http.MethodPost, "/v2/bookings/mock-booking-pending-confirm/confirm", "Bearer non-host-token", map[string]any{}, http.StatusForbidden)
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

func TestOAuthClientRepositoryErrorReturnsInternalError(t *testing.T) {
	service := auth.NewService(
		testConfig(),
		auth.WithAPIKeyPrincipalRepository(validAPIKeyPrincipals{}),
		auth.WithOAuthClientRepository(erroringOAuthClients{}),
	)
	handler := NewServer(testConfig(), WithAuthService(service))
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	assertStatus(t, server.URL, http.MethodGet, "/v2/auth/oauth2/clients/mock-oauth-client", "Bearer cal_test_valid_mock", nil, http.StatusInternalServerError)
}

func TestPlatformClientRepositoryErrorReturnsInternalError(t *testing.T) {
	service := auth.NewService(testConfig(), auth.WithPlatformClientRepository(erroringPlatformClients{}))
	handler := NewServer(testConfig(), WithAuthService(service))
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	req, err := http.NewRequest(http.MethodGet, server.URL+"/v2/oauth-clients/mock-platform-client", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("x-cal-client-id", "mock-platform-client")
	req.Header.Set("x-cal-secret-key", "mock-platform-secret")
	resp, body := do(t, req)
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
	}
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

func issueOAuthAccessToken(t *testing.T, baseURL string) string {
	t.Helper()
	body := []byte(`{
		"grant_type": "authorization_code",
		"client_id": "mock-oauth-client",
		"code": "mock-oauth-authorization-code",
		"redirect_uri": "https://fixture.example.test/callback"
	}`)
	req, err := http.NewRequest(http.MethodPost, baseURL+"/v2/auth/oauth2/token", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("content-type", "application/json")

	resp, responseBody := do(t, req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("oauth token status = %d, body = %s", resp.StatusCode, responseBody)
	}
	var tokenResponse map[string]any
	if err := json.Unmarshal(responseBody, &tokenResponse); err != nil {
		t.Fatal(err)
	}
	accessToken, _ := tokenResponse["access_token"].(string)
	if accessToken == "" {
		t.Fatalf("missing access token in response: %s", responseBody)
	}
	return accessToken
}

type erroringAPIKeyPrincipals struct{}

func (erroringAPIKeyPrincipals) ReadAPIKeyPrincipal(context.Context, string) (auth.Principal, bool, error) {
	return auth.Principal{}, false, errors.New("repository unavailable")
}

type validAPIKeyPrincipals struct{}

func (validAPIKeyPrincipals) ReadAPIKeyPrincipal(context.Context, string) (auth.Principal, bool, error) {
	return auth.FixtureAPIKeyPrincipal(), true, nil
}

type erroringOAuthClients struct{}

func (erroringOAuthClients) ReadOAuthClient(context.Context, string) (auth.OAuthClient, bool, error) {
	return auth.OAuthClient{}, false, errors.New("oauth repository unavailable")
}

type staticOAuthTokens struct {
	byToken map[string]auth.Principal
	err     error
}

func (t staticOAuthTokens) ExchangeOAuthAuthorizationCode(context.Context, auth.OAuthTokenExchangeRequest, time.Time) (auth.OAuthTokenResponse, error) {
	return auth.OAuthTokenResponse{}, errors.New("unused exchange path")
}

func (t staticOAuthTokens) ExchangeOAuthRefreshToken(context.Context, auth.OAuthTokenExchangeRequest, time.Time) (auth.OAuthTokenResponse, error) {
	return auth.OAuthTokenResponse{}, errors.New("unused refresh path")
}

func (t staticOAuthTokens) ReadOAuthAccessTokenPrincipal(_ context.Context, token string, _ time.Time) (auth.Principal, bool, error) {
	if t.err != nil {
		return auth.Principal{}, false, t.err
	}
	principal, ok := t.byToken[token]
	return principal, ok, nil
}

type erroringPlatformClients struct{}

func (erroringPlatformClients) ReadPlatformClient(context.Context, string) (auth.PlatformClientRecord, bool, error) {
	return auth.PlatformClientRecord{}, false, errors.New("platform repository unavailable")
}
