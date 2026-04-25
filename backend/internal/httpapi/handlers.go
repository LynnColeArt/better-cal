package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/LynnColeArt/better-cal/backend/internal/auth"
	"github.com/LynnColeArt/better-cal/backend/internal/authz"
	"github.com/LynnColeArt/better-cal/backend/internal/booking"
	"github.com/LynnColeArt/better-cal/backend/internal/calendars"
	"github.com/LynnColeArt/better-cal/backend/internal/slots"
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

func (s *Server) readCalendarConnections(w http.ResponseWriter, r *http.Request) {
	principal, ok, err := s.authenticateAPIKey(r)
	if err != nil {
		s.sendError(w, r, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", true)
		return
	}
	if !ok {
		s.sendError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid credentials", true)
		return
	}
	if !s.authorize(principal, authz.PolicyCalendarConnectionsRead) {
		s.sendError(w, r, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions", true)
		return
	}

	connections, err := s.calendars().ReadCalendarConnections(r.Context(), principal.ID)
	if err != nil {
		s.sendError(w, r, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", true)
		return
	}

	s.sendJSON(w, r, http.StatusOK, envelope{
		Status: "success",
		Data: map[string]any{
			"items":     connections,
			"requestId": s.requestID(r),
		},
	})
}

func (s *Server) readCalendarCatalog(w http.ResponseWriter, r *http.Request) {
	principal, ok, err := s.authenticateAPIKey(r)
	if err != nil {
		s.sendError(w, r, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", true)
		return
	}
	if !ok {
		s.sendError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid credentials", true)
		return
	}
	if !s.authorize(principal, authz.PolicyCalendarsRead) {
		s.sendError(w, r, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions", true)
		return
	}

	catalog, err := s.calendars().ReadCatalogCalendars(r.Context(), principal.ID)
	if err != nil {
		s.sendError(w, r, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", true)
		return
	}

	s.sendJSON(w, r, http.StatusOK, envelope{
		Status: "success",
		Data: map[string]any{
			"items":     catalog,
			"requestId": s.requestID(r),
		},
	})
}

func (s *Server) readCredentialMetadata(w http.ResponseWriter, r *http.Request) {
	principal, ok, err := s.authenticateAPIKey(r)
	if err != nil {
		s.sendError(w, r, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", true)
		return
	}
	if !ok {
		s.sendError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid credentials", true)
		return
	}
	if !s.authorize(principal, authz.PolicyCredentialsRead) {
		s.sendError(w, r, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions", true)
		return
	}

	credentials, err := s.credentials().ReadCredentialMetadata(r.Context(), principal.ID)
	if err != nil {
		s.sendError(w, r, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", true)
		return
	}

	s.sendJSON(w, r, http.StatusOK, envelope{
		Status: "success",
		Data: map[string]any{
			"items":     credentials,
			"requestId": s.requestID(r),
		},
	})
}

func (s *Server) readSelectedCalendars(w http.ResponseWriter, r *http.Request) {
	principal, ok, err := s.authenticateAPIKey(r)
	if err != nil {
		s.sendError(w, r, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", true)
		return
	}
	if !ok {
		s.sendError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid credentials", true)
		return
	}
	if !s.authorize(principal, authz.PolicySelectedCalendarsRead) {
		s.sendError(w, r, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions", true)
		return
	}

	selectedCalendars, err := s.calendars().ReadSelectedCalendars(r.Context(), principal.ID)
	if err != nil {
		s.sendError(w, r, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", true)
		return
	}

	s.sendJSON(w, r, http.StatusOK, envelope{
		Status: "success",
		Data: map[string]any{
			"items":     selectedCalendars,
			"requestId": s.requestID(r),
		},
	})
}

func (s *Server) saveSelectedCalendar(w http.ResponseWriter, r *http.Request) {
	principal, ok, err := s.authenticateAPIKey(r)
	if err != nil {
		s.sendError(w, r, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", true)
		return
	}
	if !ok {
		s.sendError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid credentials", true)
		return
	}
	if !s.authorize(principal, authz.PolicySelectedCalendarsWrite) {
		s.sendError(w, r, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions", true)
		return
	}

	var body calendars.SaveSelectedCalendarRequest
	if !decodeJSON(r, &body) {
		s.sendError(w, r, http.StatusBadRequest, "BAD_REQUEST", "Invalid JSON body", true)
		return
	}

	calendar, err := s.calendars().SaveSelectedCalendar(r.Context(), principal.ID, body)
	if err != nil {
		if errors.Is(err, calendars.ErrInvalidSelectedCalendar) {
			s.sendError(w, r, http.StatusBadRequest, "BAD_REQUEST", "Invalid selected calendar", true)
			return
		}
		if errors.Is(err, calendars.ErrCalendarCatalogEntryNotFound) {
			s.sendError(w, r, http.StatusNotFound, "NOT_FOUND", "Calendar not found", true)
			return
		}
		s.sendError(w, r, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", true)
		return
	}

	s.sendJSON(w, r, http.StatusOK, envelope{
		Status: "success",
		Data: map[string]any{
			"calendar":  calendar,
			"requestId": s.requestID(r),
		},
	})
}

func (s *Server) deleteSelectedCalendar(w http.ResponseWriter, r *http.Request) {
	principal, ok, err := s.authenticateAPIKey(r)
	if err != nil {
		s.sendError(w, r, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", true)
		return
	}
	if !ok {
		s.sendError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid credentials", true)
		return
	}
	if !s.authorize(principal, authz.PolicySelectedCalendarsWrite) {
		s.sendError(w, r, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions", true)
		return
	}

	calendarRef := r.PathValue("calendarRef")
	result, err := s.calendars().DeleteSelectedCalendar(r.Context(), principal.ID, calendarRef)
	if err != nil {
		if errors.Is(err, calendars.ErrInvalidSelectedCalendarRef) {
			s.sendError(w, r, http.StatusBadRequest, "BAD_REQUEST", "Invalid selected calendar", true)
			return
		}
		s.sendError(w, r, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", true)
		return
	}
	if !result.Removed {
		s.sendError(w, r, http.StatusNotFound, "NOT_FOUND", "Selected calendar not found", true)
		return
	}

	s.sendJSON(w, r, http.StatusOK, envelope{
		Status: "success",
		Data: map[string]any{
			"calendarRef":        calendarRef,
			"removed":            true,
			"destinationCleared": result.ClearedDestination,
			"requestId":          s.requestID(r),
		},
	})
}

func (s *Server) readDestinationCalendar(w http.ResponseWriter, r *http.Request) {
	principal, ok, err := s.authenticateAPIKey(r)
	if err != nil {
		s.sendError(w, r, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", true)
		return
	}
	if !ok {
		s.sendError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid credentials", true)
		return
	}
	if !s.authorize(principal, authz.PolicyDestinationCalendarsRead) {
		s.sendError(w, r, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions", true)
		return
	}

	calendar, found, err := s.calendars().ReadDestinationCalendar(r.Context(), principal.ID)
	if err != nil {
		s.sendError(w, r, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", true)
		return
	}

	var responseCalendar any
	if found {
		responseCalendar = calendar
	}
	s.sendJSON(w, r, http.StatusOK, envelope{
		Status: "success",
		Data: map[string]any{
			"calendar":  responseCalendar,
			"requestId": s.requestID(r),
		},
	})
}

type saveDestinationCalendarRequest struct {
	CalendarRef string `json:"calendarRef"`
}

func (s *Server) saveDestinationCalendar(w http.ResponseWriter, r *http.Request) {
	principal, ok, err := s.authenticateAPIKey(r)
	if err != nil {
		s.sendError(w, r, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", true)
		return
	}
	if !ok {
		s.sendError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid credentials", true)
		return
	}
	if !s.authorize(principal, authz.PolicyDestinationCalendarsWrite) {
		s.sendError(w, r, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions", true)
		return
	}

	var body saveDestinationCalendarRequest
	if !decodeJSON(r, &body) {
		s.sendError(w, r, http.StatusBadRequest, "BAD_REQUEST", "Invalid JSON body", true)
		return
	}

	calendar, found, err := s.calendars().SetDestinationCalendar(r.Context(), principal.ID, body.CalendarRef)
	if err != nil {
		if errors.Is(err, calendars.ErrInvalidDestinationCalendarRef) {
			s.sendError(w, r, http.StatusBadRequest, "BAD_REQUEST", "Invalid destination calendar", true)
			return
		}
		s.sendError(w, r, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", true)
		return
	}
	if !found {
		s.sendError(w, r, http.StatusNotFound, "NOT_FOUND", "Selected calendar not found", true)
		return
	}

	s.sendJSON(w, r, http.StatusOK, envelope{
		Status: "success",
		Data: map[string]any{
			"calendar":  calendar,
			"requestId": s.requestID(r),
		},
	})
}

func (s *Server) readSlots(w http.ResponseWriter, r *http.Request) {
	eventTypeID, err := parseOptionalInt(r.URL.Query().Get("eventTypeId"))
	if err != nil {
		s.sendError(w, r, http.StatusBadRequest, "INVALID_EVENT_TYPE", "Event type must be an integer", true)
		return
	}

	result, ok, err := s.slots().ReadAvailable(r.Context(), s.requestID(r), slots.Request{
		EventTypeID: eventTypeID,
		Start:       r.URL.Query().Get("start"),
		End:         r.URL.Query().Get("end"),
		TimeZone:    r.URL.Query().Get("timeZone"),
	})
	if err != nil {
		s.sendSlotServiceError(w, r, err)
		return
	}
	if !ok {
		s.sendError(w, r, http.StatusNotFound, "NOT_FOUND", "Slots not found", true)
		return
	}

	s.sendJSON(w, r, http.StatusOK, envelope{Status: "success", Data: result})
}

func (s *Server) createBooking(w http.ResponseWriter, r *http.Request) {
	principal, ok, err := s.authenticateAPIKeyOrOAuthAccessToken(r)
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
	if resource, ok := eventTypeBookingResource(body.EventTypeID); ok {
		if !s.authorizeBooking(principal, authz.PolicyBookingWrite, resource) {
			s.sendError(w, r, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions", true)
			return
		}
		body.OwnerUserID = resource.OwnerUserID
		body.HostUserIDs = resource.HostUserIDs
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
	principal, ok, err := s.authenticateAPIKeyOrOAuthAccessToken(r)
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
	if resource, ok := bookingResourceForUID(uid); ok && !s.authorizeBooking(principal, authz.PolicyBookingRead, resource) {
		s.sendError(w, r, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions", true)
		return
	}
	found, ok, err := s.bookings().Read(r.Context(), s.requestID(r), uid)
	if err != nil {
		s.sendError(w, r, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", true)
		return
	}
	if !ok {
		s.sendError(w, r, http.StatusNotFound, "NOT_FOUND", "Booking not found", true)
		return
	}
	if !s.authorizeBooking(principal, authz.PolicyBookingRead, bookingResource(found)) {
		s.sendError(w, r, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions", true)
		return
	}

	s.sendJSON(w, r, http.StatusOK, envelope{Status: "success", Data: found})
}

func (s *Server) cancelBooking(w http.ResponseWriter, r *http.Request) {
	principal, ok, err := s.authenticateAPIKeyOrOAuthAccessToken(r)
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

	uid := r.PathValue("bookingUid")
	if resource, ok := bookingResourceForUID(uid); ok && !s.authorizeBooking(principal, authz.PolicyBookingWrite, resource) {
		s.sendError(w, r, http.StatusForbidden, "FORBIDDEN", "", true)
		return
	}
	found, foundOK, err := s.bookings().Read(r.Context(), s.requestID(r), uid)
	if err != nil {
		s.sendError(w, r, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", true)
		return
	}
	if !foundOK {
		s.sendError(w, r, http.StatusNotFound, "NOT_FOUND", "", true)
		return
	}
	if !s.authorizeBooking(principal, authz.PolicyBookingWrite, bookingResource(found)) {
		s.sendError(w, r, http.StatusForbidden, "FORBIDDEN", "", true)
		return
	}

	var body booking.CancelRequest
	if !decodeJSON(r, &body) {
		s.sendError(w, r, http.StatusBadRequest, "BAD_REQUEST", "Invalid JSON body", true)
		return
	}

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
	principal, ok, err := s.authenticateAPIKeyOrOAuthAccessToken(r)
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

	oldUID := r.PathValue("bookingUid")
	if resource, ok := bookingResourceForUID(oldUID); ok && !s.authorizeBooking(principal, authz.PolicyBookingWrite, resource) {
		s.sendError(w, r, http.StatusForbidden, "FORBIDDEN", "", true)
		return
	}
	found, foundOK, err := s.bookings().Read(r.Context(), s.requestID(r), oldUID)
	if err != nil {
		s.sendError(w, r, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", true)
		return
	}
	if !foundOK {
		s.sendError(w, r, http.StatusNotFound, "NOT_FOUND", "", true)
		return
	}
	if !s.authorizeBooking(principal, authz.PolicyBookingWrite, bookingResource(found)) {
		s.sendError(w, r, http.StatusForbidden, "FORBIDDEN", "", true)
		return
	}

	var body booking.RescheduleRequest
	if !decodeJSON(r, &body) {
		s.sendError(w, r, http.StatusBadRequest, "BAD_REQUEST", "Invalid JSON body", true)
		return
	}

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

func (s *Server) confirmBooking(w http.ResponseWriter, r *http.Request) {
	principal, ok, err := s.authenticateAPIKey(r)
	if err != nil {
		s.sendError(w, r, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", true)
		return
	}
	if !ok {
		s.sendError(w, r, http.StatusForbidden, "FORBIDDEN", "", true)
		return
	}
	if !s.authorize(principal, authz.PolicyBookingHostAction) {
		s.sendError(w, r, http.StatusForbidden, "FORBIDDEN", "", true)
		return
	}

	uid := r.PathValue("bookingUid")
	if resource, ok := bookingResourceForUID(uid); ok && !s.authorizeBooking(principal, authz.PolicyBookingHostAction, resource) {
		s.sendError(w, r, http.StatusForbidden, "FORBIDDEN", "", true)
		return
	}
	found, foundOK, err := s.bookings().Read(r.Context(), s.requestID(r), uid)
	if err != nil {
		s.sendError(w, r, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", true)
		return
	}
	if !foundOK {
		s.sendError(w, r, http.StatusNotFound, "NOT_FOUND", "", true)
		return
	}
	if !s.authorizeBooking(principal, authz.PolicyBookingHostAction, bookingResource(found)) {
		s.sendError(w, r, http.StatusForbidden, "FORBIDDEN", "", true)
		return
	}

	var body booking.ConfirmRequest
	if !decodeJSON(r, &body) {
		s.sendError(w, r, http.StatusBadRequest, "BAD_REQUEST", "Invalid JSON body", true)
		return
	}

	result, ok, err := s.bookings().Confirm(r.Context(), s.requestID(r), uid, body)
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

func (s *Server) declineBooking(w http.ResponseWriter, r *http.Request) {
	principal, ok, err := s.authenticateAPIKey(r)
	if err != nil {
		s.sendError(w, r, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", true)
		return
	}
	if !ok {
		s.sendError(w, r, http.StatusForbidden, "FORBIDDEN", "", true)
		return
	}
	if !s.authorize(principal, authz.PolicyBookingHostAction) {
		s.sendError(w, r, http.StatusForbidden, "FORBIDDEN", "", true)
		return
	}

	uid := r.PathValue("bookingUid")
	if resource, ok := bookingResourceForUID(uid); ok && !s.authorizeBooking(principal, authz.PolicyBookingHostAction, resource) {
		s.sendError(w, r, http.StatusForbidden, "FORBIDDEN", "", true)
		return
	}
	found, foundOK, err := s.bookings().Read(r.Context(), s.requestID(r), uid)
	if err != nil {
		s.sendError(w, r, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", true)
		return
	}
	if !foundOK {
		s.sendError(w, r, http.StatusNotFound, "NOT_FOUND", "", true)
		return
	}
	if !s.authorizeBooking(principal, authz.PolicyBookingHostAction, bookingResource(found)) {
		s.sendError(w, r, http.StatusForbidden, "FORBIDDEN", "", true)
		return
	}

	var body booking.DeclineRequest
	if !decodeJSON(r, &body) {
		s.sendError(w, r, http.StatusBadRequest, "BAD_REQUEST", "Invalid JSON body", true)
		return
	}

	result, ok, err := s.bookings().Decline(r.Context(), s.requestID(r), uid, body)
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

type oauthTokenRequest struct {
	GrantType    string `json:"grant_type"`
	ClientID     string `json:"client_id"`
	Code         string `json:"code"`
	RedirectURI  string `json:"redirect_uri"`
	RefreshToken string `json:"refresh_token"`
}

func (s *Server) oauthToken(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeOAuthTokenRequest(r)
	if !ok {
		s.sendOAuthError(w, r, http.StatusBadRequest, "invalid_request", "Invalid token request")
		return
	}

	client, ok, err := s.authenticator().OAuthClientContext(r.Context(), req.ClientID)
	if err != nil {
		s.sendOAuthError(w, r, http.StatusInternalServerError, "server_error", "Internal server error")
		return
	}
	if !ok {
		s.sendOAuthError(w, r, http.StatusUnauthorized, "invalid_client", "Invalid OAuth client")
		return
	}
	if !s.authorize(client.Principal(), authz.PolicyOAuth2TokenExchange) {
		s.sendOAuthError(w, r, http.StatusUnauthorized, "invalid_client", "Invalid OAuth client")
		return
	}

	token, err := s.authenticator().ExchangeOAuthToken(r.Context(), auth.OAuthTokenExchangeRequest{
		GrantType:    req.GrantType,
		ClientID:     req.ClientID,
		Code:         req.Code,
		RedirectURI:  req.RedirectURI,
		RefreshToken: req.RefreshToken,
	})
	if err != nil {
		s.sendOAuthExchangeError(w, r, err)
		return
	}

	s.sendJSON(w, r, http.StatusOK, map[string]any{
		"access_token":  token.AccessToken,
		"token_type":    token.TokenType,
		"expires_in":    token.ExpiresIn,
		"refresh_token": token.RefreshToken,
		"scope":         token.Scope,
	})
}

func decodeOAuthTokenRequest(r *http.Request) (oauthTokenRequest, bool) {
	if r.Body == nil {
		return oauthTokenRequest{}, false
	}
	defer r.Body.Close()

	contentType := strings.ToLower(r.Header.Get("content-type"))
	if strings.HasPrefix(contentType, "application/x-www-form-urlencoded") {
		if err := r.ParseForm(); err != nil {
			return oauthTokenRequest{}, false
		}
		return oauthTokenRequest{
			GrantType:    r.PostForm.Get("grant_type"),
			ClientID:     r.PostForm.Get("client_id"),
			Code:         r.PostForm.Get("code"),
			RedirectURI:  r.PostForm.Get("redirect_uri"),
			RefreshToken: r.PostForm.Get("refresh_token"),
		}, true
	}

	var req oauthTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return oauthTokenRequest{}, false
	}
	return req, true
}

func (s *Server) sendOAuthExchangeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, auth.ErrInvalidOAuthTokenRequest):
		s.sendOAuthError(w, r, http.StatusBadRequest, "invalid_request", "Invalid token request")
	case errors.Is(err, auth.ErrUnsupportedOAuthGrantType):
		s.sendOAuthError(w, r, http.StatusBadRequest, "unsupported_grant_type", "Unsupported grant type")
	case errors.Is(err, auth.ErrInvalidOAuthClient):
		s.sendOAuthError(w, r, http.StatusUnauthorized, "invalid_client", "Invalid OAuth client")
	case errors.Is(err, auth.ErrInvalidOAuthRedirectURI),
		errors.Is(err, auth.ErrInvalidOAuthGrant),
		errors.Is(err, auth.ErrOAuthGrantConsumed),
		errors.Is(err, auth.ErrOAuthGrantExpired):
		s.sendOAuthError(w, r, http.StatusBadRequest, "invalid_grant", "Invalid OAuth grant")
	default:
		s.sendOAuthError(w, r, http.StatusInternalServerError, "server_error", "Internal server error")
	}
}

func (s *Server) sendOAuthError(w http.ResponseWriter, r *http.Request, status int, code string, description string) {
	s.sendJSON(w, r, status, map[string]any{
		"error":             code,
		"error_description": description,
		"requestId":         s.requestID(r),
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

func (s *Server) sendSlotServiceError(w http.ResponseWriter, r *http.Request, serviceErr error) {
	if validationErr, ok := slots.ValidationFromError(serviceErr); ok {
		s.sendError(w, r, http.StatusBadRequest, validationErr.Code, validationErr.Message, true)
		return
	}
	s.sendError(w, r, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", true)
}

func (s *Server) authorizeBooking(principal auth.Principal, policy authz.Policy, resource authz.BookingResource) bool {
	return s.policies().AuthorizeBooking(principal, policy, resource).Allowed
}

func eventTypeBookingResource(eventTypeID int) (authz.BookingResource, bool) {
	if eventTypeID != 0 && eventTypeID != booking.FixtureEventTypeID {
		return authz.BookingResource{}, false
	}
	return authz.BookingResource{
		OwnerUserID: booking.FixtureOwnerUserID,
		HostUserIDs: []int{booking.FixtureOwnerUserID},
	}, true
}

func bookingResource(bookingValue booking.Booking) authz.BookingResource {
	ownerUserID := bookingValue.OwnerUserID
	if ownerUserID == 0 {
		ownerUserID = booking.FixtureOwnerUserID
	}
	hostUserIDs := bookingValue.HostUserIDs
	if len(hostUserIDs) == 0 {
		hostUserIDs = []int{ownerUserID}
	}
	return authz.BookingResource{
		OwnerUserID: ownerUserID,
		HostUserIDs: hostUserIDs,
	}
}

func bookingResourceForUID(uid string) (authz.BookingResource, bool) {
	switch uid {
	case booking.PrimaryFixtureUID, booking.RescheduledFixtureUID, booking.PendingConfirmFixtureUID, booking.PendingDeclineFixtureUID:
		return authz.BookingResource{
			OwnerUserID: booking.FixtureOwnerUserID,
			HostUserIDs: []int{booking.FixtureOwnerUserID},
		}, true
	default:
		return authz.BookingResource{}, false
	}
}

func parseOptionalInt(value string) (int, error) {
	if value == "" {
		return 0, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, err
	}
	return parsed, nil
}
