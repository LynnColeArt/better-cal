package httpapi

import (
	"net/http"
)

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("content-type", "text/plain")
	w.Header().Set("x-request-id", s.requestID(r))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		s.sendError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid credentials", true)
		return
	}

	s.sendJSON(w, r, http.StatusOK, envelope{
		Status: "success",
		Data: map[string]any{
			"id":        123,
			"uuid":      "00000000-0000-4000-8000-000000000123",
			"username":  "fixture-user",
			"email":     "fixture-user@example.test",
			"createdAt": "2026-01-01T00:00:00.000Z",
			"updatedAt": "2026-01-01T00:00:00.000Z",
			"requestId": s.requestID(r),
		},
	})
}

func (s *Server) createBooking(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		s.sendError(w, r, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions", true)
		return
	}

	var body createBookingRequest
	if !decodeJSON(r, &body) {
		s.sendError(w, r, http.StatusBadRequest, "BAD_REQUEST", "Invalid JSON body", true)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if body.IdempotencyKey != "" {
		if uid, ok := s.idempotency[body.IdempotencyKey]; ok {
			s.sendJSON(w, r, http.StatusOK, envelope{Status: "success", Data: s.bookings[uid]})
			return
		}
	}

	attendeeValue := body.Attendee
	if attendeeValue.Name == "" {
		attendeeValue.Name = "Fixture Attendee"
	}
	if attendeeValue.Email == "" {
		attendeeValue.Email = "fixture-attendee@example.test"
	}
	if attendeeValue.TimeZone == "" {
		attendeeValue.TimeZone = "America/Chicago"
	}
	attendeeValue.ID = 321

	start := body.Start
	if start == "" {
		start = "2026-05-01T15:00:00.000Z"
	}
	responses := body.Responses
	if responses == nil {
		responses = map[string]any{}
	}
	metadata := body.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}

	created := s.fixtureBooking(r, booking{
		Start: start,
		Attendees: []attendee{
			attendeeValue,
		},
		Responses: responses,
		Metadata:  metadata,
	})
	s.bookings[created.UID] = created
	if body.IdempotencyKey != "" {
		s.idempotency[body.IdempotencyKey] = created.UID
	}

	s.sendJSON(w, r, http.StatusCreated, envelope{Status: "success", Data: created})
}

func (s *Server) readBooking(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		s.sendError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "", true)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	uid := r.PathValue("bookingUid")
	var found booking
	var ok bool
	if uid == "mock-booking-personal-basic" {
		found = s.ensureBooking(r)
		ok = true
	} else {
		found, ok = s.bookings[uid]
	}
	if !ok {
		s.sendError(w, r, http.StatusNotFound, "NOT_FOUND", "Booking not found", true)
		return
	}

	s.sendJSON(w, r, http.StatusOK, envelope{Status: "success", Data: found})
}

func (s *Server) cancelBooking(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		s.sendError(w, r, http.StatusForbidden, "FORBIDDEN", "", true)
		return
	}

	var body cancelBookingRequest
	if !decodeJSON(r, &body) {
		s.sendError(w, r, http.StatusBadRequest, "BAD_REQUEST", "Invalid JSON body", true)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	uid := r.PathValue("bookingUid")
	existing, ok := s.bookings[uid]
	if uid == "mock-booking-personal-basic" {
		existing = s.ensureBooking(r)
		ok = true
	}
	if !ok {
		s.sendError(w, r, http.StatusNotFound, "NOT_FOUND", "", true)
		return
	}

	cancelled := s.fixtureBooking(r, mergeBooking(existing, booking{
		Status:    "cancelled",
		UpdatedAt: "2026-01-01T00:05:00.000Z",
	}))
	s.bookings[uid] = cancelled

	s.sendJSON(w, r, http.StatusOK, map[string]any{
		"status": "success",
		"data":   cancelled,
		"sideEffects": []string{
			"calendar.cancelled",
			"email.cancelled",
			"webhook.booking.cancelled",
		},
	})
}

func (s *Server) rescheduleBooking(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		s.sendError(w, r, http.StatusForbidden, "FORBIDDEN", "", true)
		return
	}

	var body rescheduleBookingRequest
	if !decodeJSON(r, &body) {
		s.sendError(w, r, http.StatusBadRequest, "BAD_REQUEST", "Invalid JSON body", true)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	oldUID := r.PathValue("bookingUid")
	existing, ok := s.bookings[oldUID]
	if oldUID == "mock-booking-personal-basic" {
		existing = s.ensureBooking(r)
		ok = true
	}
	if !ok {
		s.sendError(w, r, http.StatusNotFound, "NOT_FOUND", "", true)
		return
	}

	oldBooking := s.fixtureBooking(r, mergeBooking(existing, booking{
		Status:    "cancelled",
		UpdatedAt: "2026-01-01T00:10:00.000Z",
	}))
	start := body.Start
	if start == "" {
		start = "2026-05-02T15:00:00.000Z"
	}
	newBooking := s.fixtureBooking(r, mergeBooking(existing, booking{
		UID:       "mock-booking-rescheduled",
		Start:     start,
		End:       "2026-05-02T15:30:00.000Z",
		UpdatedAt: "2026-01-01T00:10:00.000Z",
	}))

	s.bookings[oldUID] = oldBooking
	s.bookings[newBooking.UID] = newBooking

	s.sendJSON(w, r, http.StatusOK, map[string]any{
		"status": "success",
		"data": map[string]any{
			"oldBooking":    oldBooking,
			"newBooking":    newBooking,
			"oldBookingUid": oldBooking.UID,
			"newBookingUid": newBooking.UID,
		},
		"sideEffects": []string{
			"calendar.rescheduled",
			"email.rescheduled",
			"webhook.booking.rescheduled",
		},
	})
}

func (s *Server) oauthClientMetadata(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		s.sendError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "", false)
		return
	}

	clientID := r.PathValue("clientId")
	if clientID != s.cfg.OAuthClientID {
		s.sendError(w, r, http.StatusNotFound, "NOT_FOUND", "OAuth client not found", true)
		return
	}

	s.sendJSON(w, r, http.StatusOK, envelope{
		Status: "success",
		Data: map[string]any{
			"clientId":     clientID,
			"name":         "Fixture OAuth Client",
			"redirectUris": []string{"https://fixture.example.test/callback"},
			"createdAt":    "2026-01-01T00:00:00.000Z",
			"updatedAt":    "2026-01-01T00:00:00.000Z",
			"requestId":    s.requestID(r),
		},
	})
}

func (s *Server) platformClient(w http.ResponseWriter, r *http.Request) {
	clientID := r.PathValue("clientId")
	if clientID != s.cfg.PlatformClientID ||
		r.Header.Get("x-cal-client-id") != s.cfg.PlatformClientID ||
		!secureEqual(r.Header.Get("x-cal-secret-key"), s.cfg.PlatformClientSecret) {
		s.sendError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid platform client credentials", true)
		return
	}

	s.sendJSON(w, r, http.StatusOK, envelope{
		Status: "success",
		Data: map[string]any{
			"id":             s.cfg.PlatformClientID,
			"name":           "Fixture Platform Client",
			"organizationId": 456,
			"permissions":    []string{"booking:read", "booking:write"},
			"createdAt":      "2026-01-01T00:00:00.000Z",
			"updatedAt":      "2026-01-01T00:00:00.000Z",
			"requestId":      s.requestID(r),
		},
	})
}
