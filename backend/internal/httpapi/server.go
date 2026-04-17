package httpapi

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/LynnColeArt/better-cal/backend/internal/config"
	"github.com/LynnColeArt/better-cal/backend/internal/logging"
)

type contextKey string

const requestIDKey contextKey = "request-id"

type Server struct {
	cfg         config.Config
	logger      *slog.Logger
	mux         *http.ServeMux
	mu          sync.Mutex
	bookings    map[string]booking
	idempotency map[string]string
}

func NewServer(cfg config.Config) http.Handler {
	return NewServerWithLogger(cfg, slog.Default())
}

func NewServerWithLogger(cfg config.Config, logger *slog.Logger) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	server := &Server{
		cfg:         cfg,
		logger:      logger,
		mux:         http.NewServeMux(),
		bookings:    make(map[string]booking),
		idempotency: make(map[string]string),
	}
	server.routes()
	return server
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("x-request-id")
	if requestID == "" {
		requestID = s.cfg.RequestID
	}
	r = r.WithContext(context.WithValue(r.Context(), requestIDKey, requestID))
	recorder := &responseRecorder{ResponseWriter: w, status: http.StatusOK}
	startedAt := time.Now()

	defer func() {
		if recovered := recover(); recovered != nil {
			s.logger.ErrorContext(
				r.Context(),
				"http request panic",
				"request_id", requestID,
				"method", r.Method,
				"path", r.URL.Path,
			)
			if !recorder.wrote {
				s.sendError(recorder, r, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", true)
			}
		}
		s.logger.InfoContext(
			r.Context(),
			"http request",
			"request_id", requestID,
			"method", r.Method,
			"path", r.URL.Path,
			"status", recorder.status,
			"duration_ms", time.Since(startedAt).Milliseconds(),
			"headers", logging.RedactHeaders(r.Header),
		)
	}()

	s.mux.ServeHTTP(recorder, r)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /health", s.health)
	s.mux.HandleFunc("GET /v2/me", s.me)
	s.mux.HandleFunc("POST /v2/bookings", s.createBooking)
	s.mux.HandleFunc("GET /v2/bookings/{bookingUid}", s.readBooking)
	s.mux.HandleFunc("POST /v2/bookings/{bookingUid}/cancel", s.cancelBooking)
	s.mux.HandleFunc("POST /v2/bookings/{bookingUid}/reschedule", s.rescheduleBooking)
	s.mux.HandleFunc("GET /v2/auth/oauth2/clients/{clientId}", s.oauthClientMetadata)
	s.mux.HandleFunc("GET /v2/oauth-clients/{clientId}", s.platformClient)
}

func (s *Server) sendJSON(w http.ResponseWriter, r *http.Request, status int, body any) {
	w.Header().Set("content-type", "application/json")
	w.Header().Set("x-request-id", s.requestID(r))
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func (s *Server) sendError(w http.ResponseWriter, r *http.Request, status int, code string, message string, includeRequestID bool) {
	apiErr := &err{Code: code, Message: message}
	if includeRequestID {
		apiErr.RequestID = s.requestID(r)
	}
	s.sendJSON(w, r, status, envelope{Status: "error", Error: apiErr})
}

func decodeJSON(r *http.Request, target any) bool {
	if r.Body == nil {
		return true
	}
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(target) == nil
}

func bearerToken(r *http.Request) string {
	header := r.Header.Get("authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(header, "Bearer ")
}

func (s *Server) authorized(r *http.Request) bool {
	return secureEqual(bearerToken(r), s.cfg.APIKey)
}

func secureEqual(left string, right string) bool {
	leftHash := sha256.Sum256([]byte(left))
	rightHash := sha256.Sum256([]byte(right))
	return subtle.ConstantTimeCompare(leftHash[:], rightHash[:]) == 1
}

func (s *Server) requestID(r *http.Request) string {
	requestID, _ := r.Context().Value(requestIDKey).(string)
	if requestID == "" {
		return s.cfg.RequestID
	}
	return requestID
}

func (s *Server) fixtureBooking(r *http.Request, overrides booking) booking {
	base := booking{
		UID:         "mock-booking-personal-basic",
		ID:          987,
		Title:       "Fixture Event",
		Status:      "accepted",
		Start:       "2026-05-01T15:00:00.000Z",
		End:         "2026-05-01T15:30:00.000Z",
		EventTypeID: 1001,
		Attendees: []attendee{
			{
				ID:       321,
				Name:     "Fixture Attendee",
				Email:    "fixture-attendee@example.test",
				TimeZone: "America/Chicago",
			},
		},
		Responses: map[string]any{
			"name":  "Fixture Attendee",
			"email": "fixture-attendee@example.test",
		},
		Metadata: map[string]any{
			"fixture": "personal-basic",
		},
		CreatedAt: "2026-01-01T00:00:00.000Z",
		UpdatedAt: "2026-01-01T00:00:00.000Z",
		RequestID: s.requestID(r),
	}
	return mergeBooking(base, overrides)
}

func mergeBooking(base booking, overrides booking) booking {
	if overrides.UID != "" {
		base.UID = overrides.UID
	}
	if overrides.ID != 0 {
		base.ID = overrides.ID
	}
	if overrides.Title != "" {
		base.Title = overrides.Title
	}
	if overrides.Status != "" {
		base.Status = overrides.Status
	}
	if overrides.Start != "" {
		base.Start = overrides.Start
	}
	if overrides.End != "" {
		base.End = overrides.End
	}
	if overrides.EventTypeID != 0 {
		base.EventTypeID = overrides.EventTypeID
	}
	if overrides.Attendees != nil {
		base.Attendees = overrides.Attendees
	}
	if overrides.Responses != nil {
		base.Responses = overrides.Responses
	}
	if overrides.Metadata != nil {
		base.Metadata = overrides.Metadata
	}
	if overrides.CreatedAt != "" {
		base.CreatedAt = overrides.CreatedAt
	}
	if overrides.UpdatedAt != "" {
		base.UpdatedAt = overrides.UpdatedAt
	}
	if overrides.RequestID != "" {
		base.RequestID = overrides.RequestID
	}
	return base
}

func (s *Server) ensureBooking(r *http.Request) booking {
	existing, ok := s.bookings["mock-booking-personal-basic"]
	if ok {
		return existing
	}
	created := s.fixtureBooking(r, booking{})
	s.bookings[created.UID] = created
	return created
}

type responseRecorder struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func (r *responseRecorder) WriteHeader(status int) {
	if r.wrote {
		return
	}
	r.status = status
	r.wrote = true
	r.ResponseWriter.WriteHeader(status)
}

func (r *responseRecorder) Write(body []byte) (int, error) {
	if !r.wrote {
		r.WriteHeader(http.StatusOK)
	}
	return r.ResponseWriter.Write(body)
}
