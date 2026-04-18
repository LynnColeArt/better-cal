package httpapi

import (
	"net/http"

	"github.com/LynnColeArt/better-cal/backend/internal/authz"
	"github.com/LynnColeArt/better-cal/backend/internal/booking"
)

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("content-type", "text/plain")
	w.Header().Set("x-request-id", s.requestID(r))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	principal, ok, err := s.authenticateAPIKey(r)
	if err != nil {
		s.sendError(w, r, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", true)
		return
	}
	if !ok {
		s.sendError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid credentials", true)
		return
	}
	if !s.authorize(principal, authz.PolicyMeRead) {
		s.sendError(w, r, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions", true)
		return
	}

	s.sendJSON(w, r, http.StatusOK, envelope{
		Status: "success",
		Data: map[string]any{
			"id":        principal.ID,
			"uuid":      principal.UUID,
			"username":  principal.Username,
			"email":     principal.Email,
			"createdAt": principal.CreatedAt,
			"updatedAt": principal.UpdatedAt,
			"requestId": s.requestID(r),
		},
	})
}

func (s *Server) createBooking(w http.ResponseWriter, r *http.Request) {
	principal, ok, err := s.authenticateAPIKey(r)
	if err != nil {
		s.sendError(w, r, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", true)
		return
	}
	if !ok {
		s.sendError(w, r, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions", true)
		return
	}
	if !s.authorize(principal, authz.PolicyBookingWrite) {
		s.sendError(w, r, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions", true)
		return
	}

	var body booking.CreateRequest
	if !decodeJSON(r, &body) {
		s.sendError(w, r, http.StatusBadRequest, "BAD_REQUEST", "Invalid JSON body", true)
		return
	}

	created, duplicate, err := s.bookings().Create(r.Context(), s.requestID(r), body)
	if err != nil {
		s.sendBookingServiceError(w, r, err)
		return
	}
	status := http.StatusCreated
	if duplicate {
		status = http.StatusOK
	}
	s.sendJSON(w, r, status, envelope{Status: "success", Data: created})
}

func (s *Server) readBooking(w http.ResponseWriter, r *http.Request) {
	principal, ok, err := s.authenticateAPIKey(r)
	if err != nil {
		s.sendError(w, r, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", true)
		return
	}
	if !ok {
		s.sendError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "", true)
		return
	}
	if !s.authorize(principal, authz.PolicyBookingRead) {
		s.sendError(w, r, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions", true)
		return
	}

	uid := r.PathValue("bookingUid")
	found, ok, err := s.bookings().Read(r.Context(), s.requestID(r), uid)
	if err != nil {
		s.sendError(w, r, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", true)
		return
	}
	if !ok {
		s.sendError(w, r, http.StatusNotFound, "NOT_FOUND", "Booking not found", true)
		return
	}

	s.sendJSON(w, r, http.StatusOK, envelope{Status: "success", Data: found})
}

func (s *Server) cancelBooking(w http.ResponseWriter, r *http.Request) {
	principal, ok, err := s.authenticateAPIKey(r)
	if err != nil {
		s.sendError(w, r, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", true)
		return
	}
	if !ok {
		s.sendError(w, r, http.StatusForbidden, "FORBIDDEN", "", true)
		return
	}
	if !s.authorize(principal, authz.PolicyBookingWrite) {
		s.sendError(w, r, http.StatusForbidden, "FORBIDDEN", "", true)
		return
	}

	var body booking.CancelRequest
	if !decodeJSON(r, &body) {
		s.sendError(w, r, http.StatusBadRequest, "BAD_REQUEST", "Invalid JSON body", true)
		return
	}

	uid := r.PathValue("bookingUid")
	result, ok, err := s.bookings().Cancel(r.Context(), s.requestID(r), uid, body)
	if err != nil {
		s.sendBookingServiceError(w, r, err)
		return
	}
	if !ok {
		s.sendError(w, r, http.StatusNotFound, "NOT_FOUND", "", true)
		return
	}

	s.sendJSON(w, r, http.StatusOK, map[string]any{
		"status":      "success",
		"data":        result.Booking,
		"sideEffects": result.SideEffects,
	})
}

func (s *Server) rescheduleBooking(w http.ResponseWriter, r *http.Request) {
	principal, ok, err := s.authenticateAPIKey(r)
	if err != nil {
		s.sendError(w, r, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", true)
		return
	}
	if !ok {
		s.sendError(w, r, http.StatusForbidden, "FORBIDDEN", "", true)
		return
	}
	if !s.authorize(principal, authz.PolicyBookingWrite) {
		s.sendError(w, r, http.StatusForbidden, "FORBIDDEN", "", true)
		return
	}

	var body booking.RescheduleRequest
	if !decodeJSON(r, &body) {
		s.sendError(w, r, http.StatusBadRequest, "BAD_REQUEST", "Invalid JSON body", true)
		return
	}

	oldUID := r.PathValue("bookingUid")
	result, ok, err := s.bookings().Reschedule(r.Context(), s.requestID(r), oldUID, body)
	if err != nil {
		s.sendBookingServiceError(w, r, err)
		return
	}
	if !ok {
		s.sendError(w, r, http.StatusNotFound, "NOT_FOUND", "", true)
		return
	}

	s.sendJSON(w, r, http.StatusOK, map[string]any{
		"status": "success",
		"data": map[string]any{
			"oldBooking":    result.OldBooking,
			"newBooking":    result.NewBooking,
			"oldBookingUid": result.OldBooking.UID,
			"newBookingUid": result.NewBooking.UID,
		},
		"sideEffects": result.SideEffects,
	})
}

func (s *Server) oauthClientMetadata(w http.ResponseWriter, r *http.Request) {
	principal, ok, err := s.authenticateAPIKey(r)
	if err != nil {
		s.sendError(w, r, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", false)
		return
	}
	if !ok {
		s.sendError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "", false)
		return
	}
	if !s.authorize(principal, authz.PolicyOAuth2Read) {
		s.sendError(w, r, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions", true)
		return
	}

	clientID := r.PathValue("clientId")
	client, ok, err := s.authenticator().OAuthClientContext(r.Context(), clientID)
	if err != nil {
		s.sendError(w, r, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", true)
		return
	}
	if !ok {
		s.sendError(w, r, http.StatusNotFound, "NOT_FOUND", "OAuth client not found", true)
		return
	}

	s.sendJSON(w, r, http.StatusOK, envelope{
		Status: "success",
		Data: map[string]any{
			"clientId":     client.ClientID,
			"name":         client.Name,
			"redirectUris": client.RedirectURIs,
			"createdAt":    client.CreatedAt,
			"updatedAt":    client.UpdatedAt,
			"requestId":    s.requestID(r),
		},
	})
}

func (s *Server) platformClient(w http.ResponseWriter, r *http.Request) {
	clientID := r.PathValue("clientId")
	client, ok, err := s.authenticator().VerifyPlatformClientContext(
		r.Context(),
		clientID,
		r.Header.Get("x-cal-client-id"),
		r.Header.Get("x-cal-secret-key"),
	)
	if err != nil {
		s.sendError(w, r, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", true)
		return
	}
	if !ok {
		s.sendError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid platform client credentials", true)
		return
	}
	if !s.authorize(client.Principal(), authz.PolicyPlatformClientRead) {
		s.sendError(w, r, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions", true)
		return
	}

	s.sendJSON(w, r, http.StatusOK, envelope{
		Status: "success",
		Data: map[string]any{
			"id":             client.ID,
			"name":           client.Name,
			"organizationId": client.OrganizationID,
			"permissions":    client.Permissions,
			"createdAt":      client.CreatedAt,
			"updatedAt":      client.UpdatedAt,
			"requestId":      s.requestID(r),
		},
	})
}

func (s *Server) sendBookingServiceError(w http.ResponseWriter, r *http.Request, serviceErr error) {
	if validationErr, ok := booking.ValidationFromError(serviceErr); ok {
		s.sendError(w, r, http.StatusBadRequest, validationErr.Code, validationErr.Message, true)
		return
	}
	s.sendError(w, r, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", true)
}
