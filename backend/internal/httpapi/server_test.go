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

	"github.com/LynnColeArt/better-cal/backend/internal/apps"
	"github.com/LynnColeArt/better-cal/backend/internal/auth"
)

func TestStarterAPIContractSlice(t *testing.T) {
	handler := NewServer(testConfig())
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	assertStatus(t, server.URL, http.MethodGet, "/v2/me", "Bearer cal_test_valid_mock", nil, http.StatusOK)
	assertStatus(t, server.URL, http.MethodGet, "/v2/me", "Bearer invalid", nil, http.StatusUnauthorized)
	assertStatus(t, server.URL, http.MethodGet, "/v2/me", "cal_test_valid_mock", nil, http.StatusUnauthorized)
	assertStatus(t, server.URL, http.MethodGet, "/v2/apps", "Bearer cal_test_valid_mock", nil, http.StatusOK)
	assertStatus(t, server.URL, http.MethodPost, "/v2/app-install-intents", "Bearer cal_test_valid_mock", map[string]any{
		"appSlug": "google-calendar",
	}, http.StatusCreated)
	assertStatus(t, server.URL, http.MethodGet, "/v2/app-install-intents", "Bearer cal_test_valid_mock", nil, http.StatusOK)
	assertStatus(t, server.URL, http.MethodGet, "/v2/app-installations", "Bearer cal_test_valid_mock", nil, http.StatusOK)
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

func TestAppCatalogDoesNotExposeSecrets(t *testing.T) {
	handler := NewServer(testConfig())
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	req, err := http.NewRequest(http.MethodGet, server.URL+"/v2/apps", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("authorization", "Bearer cal_test_valid_mock")

	resp, body := do(t, req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
	}
	if !bytes.Contains(body, []byte(`"appSlug":"google-calendar"`)) {
		t.Fatalf("body did not contain fixture app catalog entry: %s", body)
	}
	for _, forbidden := range [][]byte{
		[]byte("secret"),
		[]byte("token"),
		[]byte("encrypted"),
		[]byte("refresh"),
		[]byte("access_token"),
		[]byte("refresh_token"),
		[]byte("credentialref"),
		[]byte("providerpayload"),
		[]byte("rawprovider"),
		[]byte("accountref"),
		[]byte("accountlabel"),
	} {
		if bytes.Contains(bytes.ToLower(body), bytes.ToLower(forbidden)) {
			t.Fatalf("app catalog response exposed forbidden term %q: %s", forbidden, body)
		}
	}
}

func TestAppCatalogRequiresAppsReadPermission(t *testing.T) {
	principal := auth.FixtureAPIKeyPrincipal()
	principal.Permissions = []string{"me:read"}
	service := auth.NewService(testConfig(), auth.WithAPIKeyPrincipalRepository(limitedAPIKeyPrincipals{principal: principal}))
	handler := NewServer(testConfig(), WithAuthService(service))
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	assertStatus(t, server.URL, http.MethodGet, "/v2/apps", "Bearer limited-token", nil, http.StatusForbidden)
}

func TestAppInstallIntentCreatesNonSecretPendingIntent(t *testing.T) {
	handler := NewServer(testConfig())
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	req, err := http.NewRequest(http.MethodPost, server.URL+"/v2/app-install-intents", bytes.NewReader([]byte(`{
		"appSlug": "google-calendar"
	}`)))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("authorization", "Bearer cal_test_valid_mock")
	req.Header.Set("content-type", "application/json")

	resp, body := do(t, req)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
	}
	if !bytes.Contains(body, []byte(`"appSlug":"google-calendar"`)) {
		t.Fatalf("body did not contain install intent app slug: %s", body)
	}
	if !bytes.Contains(body, []byte(`"status":"pending"`)) {
		t.Fatalf("body did not contain pending status: %s", body)
	}
	if !bytes.Contains(body, []byte(`"installIntentRef":"app-intent-`)) {
		t.Fatalf("body did not contain opaque install intent ref: %s", body)
	}
	assertNoAppInstallIntentSecretTerms(t, body)
}

func TestAppInstallIntentReadReturnsCurrentUserIntentsOnly(t *testing.T) {
	handler := NewServer(testConfig())
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	assertStatus(t, server.URL, http.MethodPost, "/v2/app-install-intents", "Bearer cal_test_valid_mock", map[string]any{
		"appSlug": "google-calendar",
	}, http.StatusCreated)
	assertStatus(t, server.URL, http.MethodPost, "/v2/app-install-intents", "Bearer "+auth.FixtureWrongOwnerAPIKey, map[string]any{
		"appSlug": "resend-email",
	}, http.StatusCreated)

	req, err := http.NewRequest(http.MethodGet, server.URL+"/v2/app-install-intents", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("authorization", "Bearer cal_test_valid_mock")

	resp, body := do(t, req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
	}
	if !bytes.Contains(body, []byte(`"appSlug":"google-calendar"`)) {
		t.Fatalf("body did not contain current user's install intent: %s", body)
	}
	if bytes.Contains(body, []byte(`"appSlug":"resend-email"`)) {
		t.Fatalf("body contained another user's install intent: %s", body)
	}
	assertNoAppInstallIntentSecretTerms(t, body)
}

func TestAppInstallIntentExternalAuthTransition(t *testing.T) {
	handler := NewServer(testConfig())
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	installIntentRef := createAppInstallIntent(t, server.URL, "Bearer cal_test_valid_mock", "google-calendar")
	assertStatus(t, server.URL, http.MethodPost, "/v2/app-install-intents/"+installIntentRef+"/external-auth", "Bearer "+auth.FixtureWrongOwnerAPIKey, nil, http.StatusNotFound)

	req, err := http.NewRequest(http.MethodPost, server.URL+"/v2/app-install-intents/"+installIntentRef+"/external-auth", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("authorization", "Bearer cal_test_valid_mock")

	resp, body := do(t, req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
	}
	if !bytes.Contains(body, []byte(`"status":"requires_external_auth"`)) {
		t.Fatalf("body did not contain external auth status: %s", body)
	}
	assertNoAppInstallIntentSecretTerms(t, body)

	readReq, err := http.NewRequest(http.MethodGet, server.URL+"/v2/app-install-intents", nil)
	if err != nil {
		t.Fatal(err)
	}
	readReq.Header.Set("authorization", "Bearer cal_test_valid_mock")
	resp, body = do(t, readReq)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("read status = %d, body = %s", resp.StatusCode, body)
	}
	if !bytes.Contains(body, []byte(`"status":"requires_external_auth"`)) {
		t.Fatalf("read body did not contain external auth status: %s", body)
	}
}

func TestAppInstallIntentExternalAuthDescriptor(t *testing.T) {
	handler := NewServer(testConfig())
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	installIntentRef := createAppInstallIntent(t, server.URL, "Bearer cal_test_valid_mock", "google-calendar")
	assertStatus(t, server.URL, http.MethodGet, "/v2/app-install-intents/"+installIntentRef+"/external-auth", "Bearer cal_test_valid_mock", nil, http.StatusConflict)
	assertStatus(t, server.URL, http.MethodPost, "/v2/app-install-intents/"+installIntentRef+"/external-auth", "Bearer cal_test_valid_mock", nil, http.StatusOK)
	assertStatus(t, server.URL, http.MethodGet, "/v2/app-install-intents/"+installIntentRef+"/external-auth", "Bearer "+auth.FixtureWrongOwnerAPIKey, nil, http.StatusNotFound)

	req, err := http.NewRequest(http.MethodGet, server.URL+"/v2/app-install-intents/"+installIntentRef+"/external-auth", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("authorization", "Bearer cal_test_valid_mock")

	resp, body := do(t, req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
	}
	descriptor := decodeExternalAuthDescriptor(t, body)
	for _, expected := range [][]byte{
		[]byte(`"externalAuth":`),
		[]byte(`"installIntentRef":"` + installIntentRef + `"`),
		[]byte(`"appSlug":"google-calendar"`),
		[]byte(`"provider":"google-calendar-fixture"`),
		[]byte(`"name":"Google Calendar"`),
		[]byte(`"authType":"oauth"`),
		[]byte(`"scopes":["calendar.read","calendar.write","booking.calendar-dispatch"]`),
		[]byte(`"status":"requires_external_auth"`),
		[]byte(`"handoffStateRef":"handoff-state-`),
		[]byte(`"expiresAt":`),
	} {
		if !bytes.Contains(body, expected) {
			t.Fatalf("body missing %s: %s", expected, body)
		}
	}
	assertNoAppInstallIntentSecretTerms(t, body)

	assertStatus(t, server.URL, http.MethodPost, "/v2/app-install-intents/"+installIntentRef+"/external-auth/consume", "Bearer "+auth.FixtureWrongOwnerAPIKey, map[string]any{
		"handoffStateRef": descriptor.HandoffStateRef,
	}, http.StatusNotFound)

	consumeReqBody, err := json.Marshal(map[string]any{"handoffStateRef": descriptor.HandoffStateRef})
	if err != nil {
		t.Fatal(err)
	}
	consumeReq, err := http.NewRequest(http.MethodPost, server.URL+"/v2/app-install-intents/"+installIntentRef+"/external-auth/consume", bytes.NewReader(consumeReqBody))
	if err != nil {
		t.Fatal(err)
	}
	consumeReq.Header.Set("authorization", "Bearer cal_test_valid_mock")
	consumeReq.Header.Set("content-type", "application/json")
	resp, body = do(t, consumeReq)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("consume status = %d, body = %s", resp.StatusCode, body)
	}
	if !bytes.Contains(body, []byte(`"handoffStateRef":"`+descriptor.HandoffStateRef+`"`)) || !bytes.Contains(body, []byte(`"consumedAt":`)) {
		t.Fatalf("consume body did not contain consumed session state: %s", body)
	}
	assertNoAppInstallIntentSecretTerms(t, body)
	assertStatus(t, server.URL, http.MethodPost, "/v2/app-install-intents/"+installIntentRef+"/external-auth/consume", "Bearer cal_test_valid_mock", map[string]any{
		"handoffStateRef": descriptor.HandoffStateRef,
	}, http.StatusConflict)
}

func TestAppInstallIntentExternalAuthAuthorizationPreview(t *testing.T) {
	handler := NewServer(testConfig())
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	installIntentRef := createAppInstallIntent(t, server.URL, "Bearer cal_test_valid_mock", "google-calendar")
	assertStatus(t, server.URL, http.MethodGet, "/v2/app-install-intents/"+installIntentRef+"/external-auth", "Bearer cal_test_valid_mock", nil, http.StatusConflict)
	assertStatus(t, server.URL, http.MethodPost, "/v2/app-install-intents/"+installIntentRef+"/external-auth", "Bearer cal_test_valid_mock", nil, http.StatusOK)

	req, err := http.NewRequest(http.MethodGet, server.URL+"/v2/app-install-intents/"+installIntentRef+"/external-auth", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("authorization", "Bearer cal_test_valid_mock")
	resp, body := do(t, req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("descriptor status = %d, body = %s", resp.StatusCode, body)
	}
	descriptor := decodeExternalAuthDescriptor(t, body)

	assertStatus(t, server.URL, http.MethodPost, "/v2/app-install-intents/"+installIntentRef+"/external-auth/authorization-preview", "Bearer "+auth.FixtureWrongOwnerAPIKey, map[string]any{
		"handoffStateRef": descriptor.HandoffStateRef,
	}, http.StatusNotFound)

	previewReqBody, err := json.Marshal(map[string]any{"handoffStateRef": descriptor.HandoffStateRef})
	if err != nil {
		t.Fatal(err)
	}
	previewReq, err := http.NewRequest(http.MethodPost, server.URL+"/v2/app-install-intents/"+installIntentRef+"/external-auth/authorization-preview", bytes.NewReader(previewReqBody))
	if err != nil {
		t.Fatal(err)
	}
	previewReq.Header.Set("authorization", "Bearer cal_test_valid_mock")
	previewReq.Header.Set("content-type", "application/json")
	resp, body = do(t, previewReq)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("preview status = %d, body = %s", resp.StatusCode, body)
	}
	preview := decodeExternalAuthAuthorizationPreview(t, body)
	if preview.InstallIntentRef != installIntentRef {
		t.Fatalf("preview install intent ref = %q", preview.InstallIntentRef)
	}
	for _, expected := range [][]byte{
		[]byte(`"externalAuthAuthorizationPreview":`),
		[]byte(`"appSlug":"google-calendar"`),
		[]byte(`"providerSlug":"google-calendar-fixture"`),
		[]byte(`"authType":"oauth"`),
		[]byte(`"requestedScopes":["calendar.read","calendar.write","booking.calendar-dispatch"]`),
		[]byte(`"stateRef":"` + descriptor.HandoffStateRef + `"`),
		[]byte(`"authorizationEndpointId":"google-calendar-fixture.authorization.preview"`),
		[]byte(`"status":"preview_only"`),
		[]byte(`"expiresAt":"` + descriptor.ExpiresAt + `"`),
	} {
		if !bytes.Contains(body, expected) {
			t.Fatalf("preview body missing %s: %s", expected, body)
		}
	}
	assertNoAppInstallIntentSecretTerms(t, body)
	if bytes.Contains(bytes.ToLower(body), []byte("url")) {
		t.Fatalf("preview response exposed URL-shaped field: %s", body)
	}

	assertStatus(t, server.URL, http.MethodPost, "/v2/app-install-intents/"+installIntentRef+"/external-auth/authorization-preview", "Bearer cal_test_valid_mock", map[string]any{
		"handoffStateRef": descriptor.HandoffStateRef,
	}, http.StatusConflict)
	assertStatus(t, server.URL, http.MethodPost, "/v2/app-install-intents/"+installIntentRef+"/external-auth/consume", "Bearer cal_test_valid_mock", map[string]any{
		"handoffStateRef": descriptor.HandoffStateRef,
	}, http.StatusConflict)
}

func TestAppInstallIntentExternalAuthCallbackPreflight(t *testing.T) {
	handler := NewServer(testConfig())
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	installIntentRef := createAppInstallIntent(t, server.URL, "Bearer cal_test_valid_mock", "google-calendar")
	assertStatus(t, server.URL, http.MethodPost, "/v2/app-install-intents/"+installIntentRef+"/external-auth", "Bearer cal_test_valid_mock", nil, http.StatusOK)

	req, err := http.NewRequest(http.MethodGet, server.URL+"/v2/app-install-intents/"+installIntentRef+"/external-auth", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("authorization", "Bearer cal_test_valid_mock")
	resp, body := do(t, req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("descriptor status = %d, body = %s", resp.StatusCode, body)
	}
	descriptor := decodeExternalAuthDescriptor(t, body)

	previewReqBody, err := json.Marshal(map[string]any{"handoffStateRef": descriptor.HandoffStateRef})
	if err != nil {
		t.Fatal(err)
	}
	previewReq, err := http.NewRequest(http.MethodPost, server.URL+"/v2/app-install-intents/"+installIntentRef+"/external-auth/authorization-preview", bytes.NewReader(previewReqBody))
	if err != nil {
		t.Fatal(err)
	}
	previewReq.Header.Set("authorization", "Bearer cal_test_valid_mock")
	previewReq.Header.Set("content-type", "application/json")
	resp, body = do(t, previewReq)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("preview status = %d, body = %s", resp.StatusCode, body)
	}
	preview := decodeExternalAuthAuthorizationPreview(t, body)

	strictReqBody, err := json.Marshal(map[string]any{
		"stateRef":          preview.StateRef,
		"callbackStatus":    apps.ExternalAuthCallbackStatusAuthorized,
		"authorizationCode": "provider-code-fixture",
	})
	if err != nil {
		t.Fatal(err)
	}
	strictReq, err := http.NewRequest(http.MethodPost, server.URL+"/v2/app-install-intents/"+installIntentRef+"/external-auth/callback-preflight", bytes.NewReader(strictReqBody))
	if err != nil {
		t.Fatal(err)
	}
	strictReq.Header.Set("authorization", "Bearer cal_test_valid_mock")
	strictReq.Header.Set("content-type", "application/json")
	resp, body = do(t, strictReq)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("strict callback preflight status = %d, body = %s", resp.StatusCode, body)
	}
	if bytes.Contains(body, []byte("provider-code-fixture")) || bytes.Contains(body, []byte("authorizationCode")) {
		t.Fatalf("strict rejection echoed provider code input: %s", body)
	}

	assertStatus(t, server.URL, http.MethodPost, "/v2/app-install-intents/"+installIntentRef+"/external-auth/callback-preflight", "Bearer "+auth.FixtureWrongOwnerAPIKey, map[string]any{
		"stateRef":       preview.StateRef,
		"callbackStatus": apps.ExternalAuthCallbackStatusAuthorized,
	}, http.StatusNotFound)

	callbackReqBody, err := json.Marshal(map[string]any{
		"stateRef":       preview.StateRef,
		"callbackStatus": apps.ExternalAuthCallbackStatusAuthorized,
	})
	if err != nil {
		t.Fatal(err)
	}
	callbackReq, err := http.NewRequest(http.MethodPost, server.URL+"/v2/app-install-intents/"+installIntentRef+"/external-auth/callback-preflight", bytes.NewReader(callbackReqBody))
	if err != nil {
		t.Fatal(err)
	}
	callbackReq.Header.Set("authorization", "Bearer cal_test_valid_mock")
	callbackReq.Header.Set("content-type", "application/json")
	resp, body = do(t, callbackReq)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("callback preflight status = %d, body = %s", resp.StatusCode, body)
	}
	callbackPreflight := decodeExternalAuthCallbackPreflight(t, body)
	if callbackPreflight.InstallIntentRef != installIntentRef {
		t.Fatalf("callback preflight install intent ref = %q", callbackPreflight.InstallIntentRef)
	}
	for _, expected := range [][]byte{
		[]byte(`"externalAuthCallbackPreflight":`),
		[]byte(`"callbackPreflightRef":"callback-preflight-`),
		[]byte(`"appSlug":"google-calendar"`),
		[]byte(`"providerSlug":"google-calendar-fixture"`),
		[]byte(`"stateRef":"` + preview.StateRef + `"`),
		[]byte(`"callbackStatus":"provider_authorized"`),
		[]byte(`"preflightStatus":"accepted"`),
		[]byte(`"receivedAt":`),
	} {
		if !bytes.Contains(body, expected) {
			t.Fatalf("callback preflight body missing %s: %s", expected, body)
		}
	}
	assertNoAppInstallIntentSecretTerms(t, body)
	for _, forbidden := range [][]byte{
		[]byte("authorizationCode"),
		[]byte("providerCode"),
		[]byte("rawQuery"),
		[]byte("queryString"),
		[]byte("providerResponse"),
	} {
		if bytes.Contains(bytes.ToLower(body), bytes.ToLower(forbidden)) {
			t.Fatalf("callback preflight response exposed forbidden term %q: %s", forbidden, body)
		}
	}

	assertStatus(t, server.URL, http.MethodPost, "/v2/app-install-intents/"+installIntentRef+"/external-auth/callback-preflight", "Bearer cal_test_valid_mock", map[string]any{
		"stateRef":       preview.StateRef,
		"callbackStatus": apps.ExternalAuthCallbackStatusAuthorized,
	}, http.StatusConflict)
}

func TestAppInstallIntentExternalAuthProviderExchange(t *testing.T) {
	handler := NewServer(testConfig())
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	installIntentRef := createAppInstallIntent(t, server.URL, "Bearer cal_test_valid_mock", "google-calendar")
	assertStatus(t, server.URL, http.MethodPost, "/v2/app-install-intents/"+installIntentRef+"/external-auth", "Bearer cal_test_valid_mock", nil, http.StatusOK)

	req, err := http.NewRequest(http.MethodGet, server.URL+"/v2/app-install-intents/"+installIntentRef+"/external-auth", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("authorization", "Bearer cal_test_valid_mock")
	resp, body := do(t, req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("descriptor status = %d, body = %s", resp.StatusCode, body)
	}
	descriptor := decodeExternalAuthDescriptor(t, body)

	previewReqBody, err := json.Marshal(map[string]any{"handoffStateRef": descriptor.HandoffStateRef})
	if err != nil {
		t.Fatal(err)
	}
	previewReq, err := http.NewRequest(http.MethodPost, server.URL+"/v2/app-install-intents/"+installIntentRef+"/external-auth/authorization-preview", bytes.NewReader(previewReqBody))
	if err != nil {
		t.Fatal(err)
	}
	previewReq.Header.Set("authorization", "Bearer cal_test_valid_mock")
	previewReq.Header.Set("content-type", "application/json")
	resp, body = do(t, previewReq)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("preview status = %d, body = %s", resp.StatusCode, body)
	}
	preview := decodeExternalAuthAuthorizationPreview(t, body)

	callbackReqBody, err := json.Marshal(map[string]any{
		"stateRef":       preview.StateRef,
		"callbackStatus": apps.ExternalAuthCallbackStatusAuthorized,
	})
	if err != nil {
		t.Fatal(err)
	}
	callbackReq, err := http.NewRequest(http.MethodPost, server.URL+"/v2/app-install-intents/"+installIntentRef+"/external-auth/callback-preflight", bytes.NewReader(callbackReqBody))
	if err != nil {
		t.Fatal(err)
	}
	callbackReq.Header.Set("authorization", "Bearer cal_test_valid_mock")
	callbackReq.Header.Set("content-type", "application/json")
	resp, body = do(t, callbackReq)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("callback preflight status = %d, body = %s", resp.StatusCode, body)
	}
	preflight := decodeExternalAuthCallbackPreflight(t, body)

	strictReqBody, err := json.Marshal(map[string]any{
		"stateRef":             preview.StateRef,
		"callbackPreflightRef": preflight.CallbackPreflightRef,
		"providerCode":         "provider-code-fixture",
	})
	if err != nil {
		t.Fatal(err)
	}
	strictReq, err := http.NewRequest(http.MethodPost, server.URL+"/v2/app-install-intents/"+installIntentRef+"/external-auth/provider-exchange", bytes.NewReader(strictReqBody))
	if err != nil {
		t.Fatal(err)
	}
	strictReq.Header.Set("authorization", "Bearer cal_test_valid_mock")
	strictReq.Header.Set("content-type", "application/json")
	resp, body = do(t, strictReq)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("strict provider exchange status = %d, body = %s", resp.StatusCode, body)
	}
	if bytes.Contains(body, []byte("provider-code-fixture")) || bytes.Contains(body, []byte("providerCode")) {
		t.Fatalf("strict provider exchange rejection echoed provider input: %s", body)
	}

	assertStatus(t, server.URL, http.MethodPost, "/v2/app-install-intents/"+installIntentRef+"/external-auth/provider-exchange", "Bearer "+auth.FixtureWrongOwnerAPIKey, map[string]any{
		"stateRef":             preview.StateRef,
		"callbackPreflightRef": preflight.CallbackPreflightRef,
	}, http.StatusNotFound)

	exchangeReqBody, err := json.Marshal(map[string]any{
		"stateRef":             preview.StateRef,
		"callbackPreflightRef": preflight.CallbackPreflightRef,
	})
	if err != nil {
		t.Fatal(err)
	}
	exchangeReq, err := http.NewRequest(http.MethodPost, server.URL+"/v2/app-install-intents/"+installIntentRef+"/external-auth/provider-exchange", bytes.NewReader(exchangeReqBody))
	if err != nil {
		t.Fatal(err)
	}
	exchangeReq.Header.Set("authorization", "Bearer cal_test_valid_mock")
	exchangeReq.Header.Set("content-type", "application/json")
	resp, body = do(t, exchangeReq)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("provider exchange status = %d, body = %s", resp.StatusCode, body)
	}
	exchange := decodeExternalAuthProviderExchange(t, body)
	if exchange.InstallIntentRef != installIntentRef {
		t.Fatalf("provider exchange install intent ref = %q", exchange.InstallIntentRef)
	}
	for _, expected := range [][]byte{
		[]byte(`"externalAuthProviderExchange":`),
		[]byte(`"providerExchangeRef":"provider-exchange-`),
		[]byte(`"appSlug":"google-calendar"`),
		[]byte(`"providerSlug":"google-calendar-fixture"`),
		[]byte(`"stateRef":"` + preview.StateRef + `"`),
		[]byte(`"callbackPreflightRef":"` + preflight.CallbackPreflightRef + `"`),
		[]byte(`"exchangeStatus":"queued"`),
		[]byte(`"exchangeMode":"simulated"`),
		[]byte(`"recordedAt":`),
	} {
		if !bytes.Contains(body, expected) {
			t.Fatalf("provider exchange body missing %s: %s", expected, body)
		}
	}
	assertNoAppInstallIntentSecretTerms(t, body)
	for _, forbidden := range [][]byte{
		[]byte("authorizationCode"),
		[]byte("providerCode"),
		[]byte("rawQuery"),
		[]byte("queryString"),
		[]byte("providerResponse"),
	} {
		if bytes.Contains(bytes.ToLower(body), bytes.ToLower(forbidden)) {
			t.Fatalf("provider exchange response exposed forbidden term %q: %s", forbidden, body)
		}
	}

	progressBody := readAppInstallProgressBody(t, server.URL, "Bearer cal_test_valid_mock", installIntentRef, http.StatusOK)
	progress := decodeAppInstallProgress(t, progressBody)
	if progress.ProgressStatus != apps.InstallProgressStatusProviderExchangeQueued {
		t.Fatalf("provider exchange progress status = %q, body = %s", progress.ProgressStatus, progressBody)
	}
	if !progress.ProviderExchangeRecorded || progress.ProviderExchangeRef != exchange.ProviderExchangeRef {
		t.Fatalf("provider exchange progress = %#v, exchange = %#v", progress, exchange)
	}
	strictCompletionReqBody, err := json.Marshal(map[string]any{
		"providerExchangeRef": exchange.ProviderExchangeRef,
		"providerCode":        "provider-code-fixture",
	})
	if err != nil {
		t.Fatal(err)
	}
	strictCompletionReq, err := http.NewRequest(http.MethodPost, server.URL+"/v2/app-install-intents/"+installIntentRef+"/complete", bytes.NewReader(strictCompletionReqBody))
	if err != nil {
		t.Fatal(err)
	}
	strictCompletionReq.Header.Set("authorization", "Bearer cal_test_valid_mock")
	strictCompletionReq.Header.Set("content-type", "application/json")
	resp, body = do(t, strictCompletionReq)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("strict install completion status = %d, body = %s", resp.StatusCode, body)
	}
	if bytes.Contains(body, []byte("provider-code-fixture")) || bytes.Contains(body, []byte("providerCode")) {
		t.Fatalf("strict install completion rejection echoed provider input: %s", body)
	}

	assertStatus(t, server.URL, http.MethodPost, "/v2/app-install-intents/"+installIntentRef+"/complete", "Bearer "+auth.FixtureWrongOwnerAPIKey, map[string]any{
		"providerExchangeRef": exchange.ProviderExchangeRef,
	}, http.StatusNotFound)

	completionReqBody, err := json.Marshal(map[string]any{
		"providerExchangeRef": exchange.ProviderExchangeRef,
	})
	if err != nil {
		t.Fatal(err)
	}
	completionReq, err := http.NewRequest(http.MethodPost, server.URL+"/v2/app-install-intents/"+installIntentRef+"/complete", bytes.NewReader(completionReqBody))
	if err != nil {
		t.Fatal(err)
	}
	completionReq.Header.Set("authorization", "Bearer cal_test_valid_mock")
	completionReq.Header.Set("content-type", "application/json")
	resp, body = do(t, completionReq)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("install completion status = %d, body = %s", resp.StatusCode, body)
	}
	completion := decodeAppInstallCompletion(t, body)
	if completion.InstallIntentRef != installIntentRef {
		t.Fatalf("install completion install intent ref = %q", completion.InstallIntentRef)
	}
	for _, expected := range [][]byte{
		[]byte(`"appInstallCompletion":`),
		[]byte(`"installCompletionRef":"install-completion-`),
		[]byte(`"appSlug":"google-calendar"`),
		[]byte(`"providerSlug":"google-calendar-fixture"`),
		[]byte(`"providerExchangeRef":"` + exchange.ProviderExchangeRef + `"`),
		[]byte(`"completionStatus":"installed"`),
		[]byte(`"completionMode":"simulated"`),
		[]byte(`"completedAt":`),
	} {
		if !bytes.Contains(body, expected) {
			t.Fatalf("install completion body missing %s: %s", expected, body)
		}
	}
	assertNoAppInstallIntentSecretTerms(t, body)
	for _, forbidden := range [][]byte{
		[]byte("authorizationCode"),
		[]byte("providerCode"),
		[]byte("rawQuery"),
		[]byte("queryString"),
		[]byte("providerResponse"),
	} {
		if bytes.Contains(bytes.ToLower(body), bytes.ToLower(forbidden)) {
			t.Fatalf("install completion response exposed forbidden term %q: %s", forbidden, body)
		}
	}

	progressBody = readAppInstallProgressBody(t, server.URL, "Bearer cal_test_valid_mock", installIntentRef, http.StatusOK)
	progress = decodeAppInstallProgress(t, progressBody)
	if progress.InstallStatus != apps.InstallIntentStatusInstalled {
		t.Fatalf("completed install status = %q, body = %s", progress.InstallStatus, progressBody)
	}
	if progress.ProgressStatus != apps.InstallProgressStatusInstalled {
		t.Fatalf("completed progress status = %q, body = %s", progress.ProgressStatus, progressBody)
	}
	if !progress.InstallCompletionRecorded || progress.InstallCompletionRef != completion.InstallCompletionRef {
		t.Fatalf("install completion progress = %#v, completion = %#v", progress, completion)
	}
	if progress.NextAction != apps.InstallProgressActionNone {
		t.Fatalf("completed progress next action = %q", progress.NextAction)
	}
	strictActivationReqBody, err := json.Marshal(map[string]any{
		"installCompletionRef": completion.InstallCompletionRef,
		"providerCode":         "provider-code-fixture",
	})
	if err != nil {
		t.Fatal(err)
	}
	strictActivationReq, err := http.NewRequest(http.MethodPost, server.URL+"/v2/app-install-intents/"+installIntentRef+"/activate", bytes.NewReader(strictActivationReqBody))
	if err != nil {
		t.Fatal(err)
	}
	strictActivationReq.Header.Set("authorization", "Bearer cal_test_valid_mock")
	strictActivationReq.Header.Set("content-type", "application/json")
	resp, body = do(t, strictActivationReq)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("strict app installation activation status = %d, body = %s", resp.StatusCode, body)
	}
	if bytes.Contains(body, []byte("provider-code-fixture")) || bytes.Contains(body, []byte("providerCode")) {
		t.Fatalf("strict app installation activation rejection echoed provider input: %s", body)
	}

	assertStatus(t, server.URL, http.MethodPost, "/v2/app-install-intents/"+installIntentRef+"/activate", "Bearer "+auth.FixtureWrongOwnerAPIKey, map[string]any{
		"installCompletionRef": completion.InstallCompletionRef,
	}, http.StatusNotFound)

	activationReqBody, err := json.Marshal(map[string]any{
		"installCompletionRef": completion.InstallCompletionRef,
	})
	if err != nil {
		t.Fatal(err)
	}
	activationReq, err := http.NewRequest(http.MethodPost, server.URL+"/v2/app-install-intents/"+installIntentRef+"/activate", bytes.NewReader(activationReqBody))
	if err != nil {
		t.Fatal(err)
	}
	activationReq.Header.Set("authorization", "Bearer cal_test_valid_mock")
	activationReq.Header.Set("content-type", "application/json")
	resp, body = do(t, activationReq)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("app installation activation status = %d, body = %s", resp.StatusCode, body)
	}
	installation := decodeAppInstallation(t, body)
	if installation.InstallIntentRef != installIntentRef {
		t.Fatalf("app installation install intent ref = %q", installation.InstallIntentRef)
	}
	if installation.InstallCompletionRef != completion.InstallCompletionRef {
		t.Fatalf("app installation completion ref = %q", installation.InstallCompletionRef)
	}
	for _, expected := range [][]byte{
		[]byte(`"appInstallation":`),
		[]byte(`"appInstallationRef":"app-installation-`),
		[]byte(`"installCompletionRef":"` + completion.InstallCompletionRef + `"`),
		[]byte(`"appSlug":"google-calendar"`),
		[]byte(`"appName":"Google Calendar"`),
		[]byte(`"providerSlug":"google-calendar-fixture"`),
		[]byte(`"activationStatus":"active"`),
		[]byte(`"activationMode":"simulated"`),
		[]byte(`"activatedAt":`),
	} {
		if !bytes.Contains(body, expected) {
			t.Fatalf("app installation body missing %s: %s", expected, body)
		}
	}
	assertNoAppInstallIntentSecretTerms(t, body)
	for _, forbidden := range [][]byte{
		[]byte("authorizationCode"),
		[]byte("providerCode"),
		[]byte("rawQuery"),
		[]byte("queryString"),
		[]byte("providerResponse"),
	} {
		if bytes.Contains(bytes.ToLower(body), bytes.ToLower(forbidden)) {
			t.Fatalf("app installation response exposed forbidden term %q: %s", forbidden, body)
		}
	}

	installationsReq, err := http.NewRequest(http.MethodGet, server.URL+"/v2/app-installations", nil)
	if err != nil {
		t.Fatal(err)
	}
	installationsReq.Header.Set("authorization", "Bearer cal_test_valid_mock")
	resp, body = do(t, installationsReq)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("read app installations status = %d, body = %s", resp.StatusCode, body)
	}
	installations := decodeAppInstallations(t, body)
	if len(installations) != 1 || installations[0].AppInstallationRef != installation.AppInstallationRef {
		t.Fatalf("app installations = %#v, installation = %#v", installations, installation)
	}
	assertNoAppInstallIntentSecretTerms(t, body)

	progressBody = readAppInstallProgressBody(t, server.URL, "Bearer cal_test_valid_mock", installIntentRef, http.StatusOK)
	progress = decodeAppInstallProgress(t, progressBody)
	if progress.ProgressStatus != apps.InstallProgressStatusActive {
		t.Fatalf("active progress status = %q, body = %s", progress.ProgressStatus, progressBody)
	}
	if !progress.AppInstallationRecorded || progress.AppInstallationRef != installation.AppInstallationRef {
		t.Fatalf("app installation progress = %#v, installation = %#v", progress, installation)
	}
	if progress.AppInstallationStatus != apps.AppInstallationStatusActive {
		t.Fatalf("app installation progress status = %q", progress.AppInstallationStatus)
	}
	if progress.NextAction != apps.InstallProgressActionNone {
		t.Fatalf("active progress next action = %q", progress.NextAction)
	}
	assertStatus(t, server.URL, http.MethodPost, "/v2/app-install-intents/"+installIntentRef+"/activate", "Bearer cal_test_valid_mock", map[string]any{
		"installCompletionRef": completion.InstallCompletionRef,
	}, http.StatusConflict)
	assertStatus(t, server.URL, http.MethodPost, "/v2/app-install-intents/"+installIntentRef+"/complete", "Bearer cal_test_valid_mock", map[string]any{
		"providerExchangeRef": exchange.ProviderExchangeRef,
	}, http.StatusConflict)
	assertStatus(t, server.URL, http.MethodPost, "/v2/app-install-intents/"+installIntentRef+"/external-auth/provider-exchange", "Bearer cal_test_valid_mock", map[string]any{
		"stateRef":             preview.StateRef,
		"callbackPreflightRef": preflight.CallbackPreflightRef,
	}, http.StatusConflict)
}

func TestAppInstallIntentProgressReadModel(t *testing.T) {
	handler := NewServer(testConfig())
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	installIntentRef := createAppInstallIntent(t, server.URL, "Bearer cal_test_valid_mock", "google-calendar")
	progressBody := readAppInstallProgressBody(t, server.URL, "Bearer cal_test_valid_mock", installIntentRef, http.StatusOK)
	progress := decodeAppInstallProgress(t, progressBody)
	if progress.ProgressStatus != apps.InstallProgressStatusPending {
		t.Fatalf("pending progress status = %q, body = %s", progress.ProgressStatus, progressBody)
	}
	if progress.ExternalAuthDescriptorAvailable {
		t.Fatalf("pending descriptor availability = true: %#v", progress)
	}
	assertNoAppInstallIntentSecretTerms(t, progressBody)

	assertStatus(t, server.URL, http.MethodPost, "/v2/app-install-intents/"+installIntentRef+"/external-auth", "Bearer cal_test_valid_mock", nil, http.StatusOK)
	progressBody = readAppInstallProgressBody(t, server.URL, "Bearer cal_test_valid_mock", installIntentRef, http.StatusOK)
	progress = decodeAppInstallProgress(t, progressBody)
	if progress.ProgressStatus != apps.InstallProgressStatusExternalAuthReady {
		t.Fatalf("ready progress status = %q, body = %s", progress.ProgressStatus, progressBody)
	}
	if !progress.ExternalAuthDescriptorAvailable {
		t.Fatalf("descriptor availability = false: %#v", progress)
	}
	if progress.NextAction != apps.InstallProgressActionReadDescriptor {
		t.Fatalf("ready next action = %q", progress.NextAction)
	}

	descriptorReq, err := http.NewRequest(http.MethodGet, server.URL+"/v2/app-install-intents/"+installIntentRef+"/external-auth", nil)
	if err != nil {
		t.Fatal(err)
	}
	descriptorReq.Header.Set("authorization", "Bearer cal_test_valid_mock")
	resp, body := do(t, descriptorReq)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("descriptor status = %d, body = %s", resp.StatusCode, body)
	}
	descriptor := decodeExternalAuthDescriptor(t, body)
	progressBody = readAppInstallProgressBody(t, server.URL, "Bearer cal_test_valid_mock", installIntentRef, http.StatusOK)
	progress = decodeAppInstallProgress(t, progressBody)
	if progress.ProgressStatus != apps.InstallProgressStatusHandoffReady {
		t.Fatalf("handoff progress status = %q, body = %s", progress.ProgressStatus, progressBody)
	}
	if progress.LatestHandoffStateRef != descriptor.HandoffStateRef {
		t.Fatalf("latest handoff state ref = %q", progress.LatestHandoffStateRef)
	}
	if progress.LatestHandoffStateExpiresAt != descriptor.ExpiresAt {
		t.Fatalf("latest handoff expires at = %q", progress.LatestHandoffStateExpiresAt)
	}

	previewReqBody, err := json.Marshal(map[string]any{"handoffStateRef": descriptor.HandoffStateRef})
	if err != nil {
		t.Fatal(err)
	}
	previewReq, err := http.NewRequest(http.MethodPost, server.URL+"/v2/app-install-intents/"+installIntentRef+"/external-auth/authorization-preview", bytes.NewReader(previewReqBody))
	if err != nil {
		t.Fatal(err)
	}
	previewReq.Header.Set("authorization", "Bearer cal_test_valid_mock")
	previewReq.Header.Set("content-type", "application/json")
	resp, body = do(t, previewReq)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("preview status = %d, body = %s", resp.StatusCode, body)
	}
	preview := decodeExternalAuthAuthorizationPreview(t, body)
	progressBody = readAppInstallProgressBody(t, server.URL, "Bearer cal_test_valid_mock", installIntentRef, http.StatusOK)
	progress = decodeAppInstallProgress(t, progressBody)
	if progress.ProgressStatus != apps.InstallProgressStatusPreviewRecorded {
		t.Fatalf("preview progress status = %q, body = %s", progress.ProgressStatus, progressBody)
	}
	if !progress.AuthorizationPreviewRecorded || progress.AuthorizationPreviewRef != preview.AuthorizationPreviewRef {
		t.Fatalf("preview progress = %#v, preview = %#v", progress, preview)
	}
	if progress.NextAction != apps.InstallProgressActionRecordCallback {
		t.Fatalf("preview next action = %q", progress.NextAction)
	}

	callbackReqBody, err := json.Marshal(map[string]any{
		"stateRef":       preview.StateRef,
		"callbackStatus": apps.ExternalAuthCallbackStatusAuthorized,
	})
	if err != nil {
		t.Fatal(err)
	}
	callbackReq, err := http.NewRequest(http.MethodPost, server.URL+"/v2/app-install-intents/"+installIntentRef+"/external-auth/callback-preflight", bytes.NewReader(callbackReqBody))
	if err != nil {
		t.Fatal(err)
	}
	callbackReq.Header.Set("authorization", "Bearer cal_test_valid_mock")
	callbackReq.Header.Set("content-type", "application/json")
	resp, body = do(t, callbackReq)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("callback preflight status = %d, body = %s", resp.StatusCode, body)
	}
	preflight := decodeExternalAuthCallbackPreflight(t, body)
	progressBody = readAppInstallProgressBody(t, server.URL, "Bearer cal_test_valid_mock", installIntentRef, http.StatusOK)
	progress = decodeAppInstallProgress(t, progressBody)
	if progress.ProgressStatus != apps.InstallProgressStatusCallbackPreflight {
		t.Fatalf("callback progress status = %q, body = %s", progress.ProgressStatus, progressBody)
	}
	if !progress.CallbackPreflightRecorded || progress.CallbackPreflightRef != preflight.CallbackPreflightRef {
		t.Fatalf("callback progress = %#v, preflight = %#v", progress, preflight)
	}
	if progress.NextAction != apps.InstallProgressActionAwaitExchange {
		t.Fatalf("callback next action = %q", progress.NextAction)
	}
	assertNoAppInstallIntentSecretTerms(t, progressBody)
	for _, forbidden := range [][]byte{
		[]byte("authorizationCode"),
		[]byte("providerCode"),
		[]byte("rawQuery"),
		[]byte("queryString"),
		[]byte("providerResponse"),
		[]byte("url"),
	} {
		if bytes.Contains(bytes.ToLower(progressBody), bytes.ToLower(forbidden)) {
			t.Fatalf("progress response exposed forbidden term %q: %s", forbidden, progressBody)
		}
	}

	readAppInstallProgressBody(t, server.URL, "Bearer "+auth.FixtureWrongOwnerAPIKey, installIntentRef, http.StatusNotFound)
}

func TestAppInstallIntentExternalAuthConsumeRejectsExpiredState(t *testing.T) {
	current := time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC)
	appStore := apps.NewStore(apps.WithClock(func() time.Time { return current }))
	handler := NewServer(testConfig(), WithAppStore(appStore))
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	installIntentRef := createAppInstallIntent(t, server.URL, "Bearer cal_test_valid_mock", "google-calendar")
	assertStatus(t, server.URL, http.MethodPost, "/v2/app-install-intents/"+installIntentRef+"/external-auth", "Bearer cal_test_valid_mock", nil, http.StatusOK)

	req, err := http.NewRequest(http.MethodGet, server.URL+"/v2/app-install-intents/"+installIntentRef+"/external-auth", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("authorization", "Bearer cal_test_valid_mock")
	resp, body := do(t, req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("descriptor status = %d, body = %s", resp.StatusCode, body)
	}
	descriptor := decodeExternalAuthDescriptor(t, body)

	current = current.Add(11 * time.Minute)
	assertStatus(t, server.URL, http.MethodPost, "/v2/app-install-intents/"+installIntentRef+"/external-auth/consume", "Bearer cal_test_valid_mock", map[string]any{
		"handoffStateRef": descriptor.HandoffStateRef,
	}, http.StatusGone)
	assertStatus(t, server.URL, http.MethodPost, "/v2/app-install-intents/"+installIntentRef+"/external-auth/authorization-preview", "Bearer cal_test_valid_mock", map[string]any{
		"handoffStateRef": descriptor.HandoffStateRef,
	}, http.StatusGone)
	assertStatus(t, server.URL, http.MethodPost, "/v2/app-install-intents/"+installIntentRef+"/external-auth/callback-preflight", "Bearer cal_test_valid_mock", map[string]any{
		"stateRef":       descriptor.HandoffStateRef,
		"callbackStatus": apps.ExternalAuthCallbackStatusAuthorized,
	}, http.StatusGone)
}

func TestAppInstallIntentRejectsUnknownApp(t *testing.T) {
	handler := NewServer(testConfig())
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	assertStatus(t, server.URL, http.MethodPost, "/v2/app-install-intents", "Bearer cal_test_valid_mock", map[string]any{
		"appSlug": "unknown-app",
	}, http.StatusNotFound)
}

func TestAppInstallIntentRequiresAppsInstallPermission(t *testing.T) {
	principal := auth.FixtureAPIKeyPrincipal()
	principal.Permissions = []string{"apps:read"}
	service := auth.NewService(testConfig(), auth.WithAPIKeyPrincipalRepository(limitedAPIKeyPrincipals{principal: principal}))
	handler := NewServer(testConfig(), WithAuthService(service))
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	assertStatus(t, server.URL, http.MethodPost, "/v2/app-install-intents", "Bearer limited-token", map[string]any{
		"appSlug": "google-calendar",
	}, http.StatusForbidden)
}

func TestAppInstallIntentReadRequiresAppsReadPermission(t *testing.T) {
	principal := auth.FixtureAPIKeyPrincipal()
	principal.Permissions = []string{"apps:install"}
	service := auth.NewService(testConfig(), auth.WithAPIKeyPrincipalRepository(limitedAPIKeyPrincipals{principal: principal}))
	handler := NewServer(testConfig(), WithAuthService(service))
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	assertStatus(t, server.URL, http.MethodGet, "/v2/app-install-intents", "Bearer limited-token", nil, http.StatusForbidden)
	assertStatus(t, server.URL, http.MethodGet, "/v2/app-installations", "Bearer limited-token", nil, http.StatusForbidden)
}

func TestAppInstallIntentExternalAuthRequiresAppsInstallPermission(t *testing.T) {
	principal := auth.FixtureAPIKeyPrincipal()
	principal.Permissions = []string{"apps:read"}
	service := auth.NewService(testConfig(), auth.WithAPIKeyPrincipalRepository(limitedAPIKeyPrincipals{principal: principal}))
	handler := NewServer(testConfig(), WithAuthService(service))
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	assertStatus(t, server.URL, http.MethodGet, "/v2/app-install-intents/app-intent-fixture/external-auth", "Bearer limited-token", nil, http.StatusForbidden)
	assertStatus(t, server.URL, http.MethodPost, "/v2/app-install-intents/app-intent-fixture/external-auth", "Bearer limited-token", nil, http.StatusForbidden)
	assertStatus(t, server.URL, http.MethodPost, "/v2/app-install-intents/app-intent-fixture/external-auth/consume", "Bearer limited-token", map[string]any{
		"handoffStateRef": "handoff-state-fixture",
	}, http.StatusForbidden)
	assertStatus(t, server.URL, http.MethodPost, "/v2/app-install-intents/app-intent-fixture/external-auth/authorization-preview", "Bearer limited-token", map[string]any{
		"handoffStateRef": "handoff-state-fixture",
	}, http.StatusForbidden)
	assertStatus(t, server.URL, http.MethodPost, "/v2/app-install-intents/app-intent-fixture/external-auth/callback-preflight", "Bearer limited-token", map[string]any{
		"stateRef":       "handoff-state-fixture",
		"callbackStatus": apps.ExternalAuthCallbackStatusAuthorized,
	}, http.StatusForbidden)
	assertStatus(t, server.URL, http.MethodPost, "/v2/app-install-intents/app-intent-fixture/external-auth/provider-exchange", "Bearer limited-token", map[string]any{
		"stateRef":             "handoff-state-fixture",
		"callbackPreflightRef": "callback-preflight-fixture",
	}, http.StatusForbidden)
	assertStatus(t, server.URL, http.MethodPost, "/v2/app-install-intents/app-intent-fixture/complete", "Bearer limited-token", map[string]any{
		"providerExchangeRef": "provider-exchange-fixture",
	}, http.StatusForbidden)
	assertStatus(t, server.URL, http.MethodPost, "/v2/app-install-intents/app-intent-fixture/activate", "Bearer limited-token", map[string]any{
		"installCompletionRef": "install-completion-fixture",
	}, http.StatusForbidden)
	assertStatus(t, server.URL, http.MethodGet, "/v2/app-install-intents/app-intent-fixture/progress", "Bearer limited-token", nil, http.StatusForbidden)
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

func createAppInstallIntent(t *testing.T, baseURL string, authorization string, appSlug string) string {
	t.Helper()
	requestBody, err := json.Marshal(map[string]any{"appSlug": appSlug})
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, baseURL+"/v2/app-install-intents", bytes.NewReader(requestBody))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("authorization", authorization)
	req.Header.Set("content-type", "application/json")

	resp, body := do(t, req)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create app install intent status = %d, body = %s", resp.StatusCode, body)
	}
	var decoded struct {
		Data struct {
			InstallIntent struct {
				InstallIntentRef string `json:"installIntentRef"`
			} `json:"installIntent"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Data.InstallIntent.InstallIntentRef == "" {
		t.Fatalf("missing install intent ref in body: %s", body)
	}
	return decoded.Data.InstallIntent.InstallIntentRef
}

func decodeExternalAuthDescriptor(t *testing.T, body []byte) apps.ExternalAuthDescriptor {
	t.Helper()
	var decoded struct {
		Data struct {
			ExternalAuth apps.ExternalAuthDescriptor `json:"externalAuth"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Data.ExternalAuth.HandoffStateRef == "" {
		t.Fatalf("missing external auth descriptor in body: %s", body)
	}
	return decoded.Data.ExternalAuth
}

func decodeExternalAuthAuthorizationPreview(t *testing.T, body []byte) apps.ExternalAuthAuthorizationPreview {
	t.Helper()
	var decoded struct {
		Data struct {
			Preview apps.ExternalAuthAuthorizationPreview `json:"externalAuthAuthorizationPreview"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Data.Preview.StateRef == "" {
		t.Fatalf("missing external auth authorization preview in body: %s", body)
	}
	return decoded.Data.Preview
}

func decodeExternalAuthCallbackPreflight(t *testing.T, body []byte) apps.ExternalAuthCallbackPreflight {
	t.Helper()
	var decoded struct {
		Data struct {
			Preflight apps.ExternalAuthCallbackPreflight `json:"externalAuthCallbackPreflight"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Data.Preflight.CallbackPreflightRef == "" {
		t.Fatalf("missing external auth callback preflight in body: %s", body)
	}
	return decoded.Data.Preflight
}

func decodeExternalAuthProviderExchange(t *testing.T, body []byte) apps.ExternalAuthProviderExchange {
	t.Helper()
	var decoded struct {
		Data struct {
			Exchange apps.ExternalAuthProviderExchange `json:"externalAuthProviderExchange"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Data.Exchange.ProviderExchangeRef == "" {
		t.Fatalf("missing external auth provider exchange in body: %s", body)
	}
	return decoded.Data.Exchange
}

func decodeAppInstallCompletion(t *testing.T, body []byte) apps.AppInstallCompletion {
	t.Helper()
	var decoded struct {
		Data struct {
			Completion apps.AppInstallCompletion `json:"appInstallCompletion"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Data.Completion.InstallCompletionRef == "" {
		t.Fatalf("missing app install completion in body: %s", body)
	}
	return decoded.Data.Completion
}

func decodeAppInstallation(t *testing.T, body []byte) apps.AppInstallation {
	t.Helper()
	var decoded struct {
		Data struct {
			Installation apps.AppInstallation `json:"appInstallation"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Data.Installation.AppInstallationRef == "" {
		t.Fatalf("missing app installation in body: %s", body)
	}
	return decoded.Data.Installation
}

func decodeAppInstallations(t *testing.T, body []byte) []apps.AppInstallation {
	t.Helper()
	var decoded struct {
		Data struct {
			Items []apps.AppInstallation `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatal(err)
	}
	return decoded.Data.Items
}

func readAppInstallProgressBody(t *testing.T, baseURL string, authorization string, installIntentRef string, expectedStatus int) []byte {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, baseURL+"/v2/app-install-intents/"+installIntentRef+"/progress", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("authorization", authorization)
	resp, body := do(t, req)
	if resp.StatusCode != expectedStatus {
		t.Fatalf("install progress status = %d, want %d, body = %s", resp.StatusCode, expectedStatus, body)
	}
	return body
}

func decodeAppInstallProgress(t *testing.T, body []byte) apps.AppInstallProgress {
	t.Helper()
	var decoded struct {
		Data struct {
			Progress apps.AppInstallProgress `json:"installProgress"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Data.Progress.InstallIntentRef == "" {
		t.Fatalf("missing app install progress in body: %s", body)
	}
	return decoded.Data.Progress
}

func assertNoAppInstallIntentSecretTerms(t *testing.T, body []byte) {
	t.Helper()
	for _, forbidden := range [][]byte{
		[]byte("secret"),
		[]byte("token"),
		[]byte("encrypted"),
		[]byte("refresh"),
		[]byte("access_token"),
		[]byte("refresh_token"),
		[]byte("credentialref"),
		[]byte("providerpayload"),
		[]byte("rawprovider"),
		[]byte("accountref"),
		[]byte("accountlabel"),
		[]byte("userid"),
		[]byte("user_id"),
		[]byte("redirecturl"),
		[]byte("callbackurl"),
	} {
		if bytes.Contains(bytes.ToLower(body), bytes.ToLower(forbidden)) {
			t.Fatalf("app install intent response exposed forbidden term %q: %s", forbidden, body)
		}
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

type limitedAPIKeyPrincipals struct {
	principal auth.Principal
}

func (p limitedAPIKeyPrincipals) ReadAPIKeyPrincipal(context.Context, string) (auth.Principal, bool, error) {
	return p.principal, true, nil
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
