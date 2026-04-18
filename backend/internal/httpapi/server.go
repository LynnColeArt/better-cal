package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/LynnColeArt/better-cal/backend/internal/auth"
	"github.com/LynnColeArt/better-cal/backend/internal/authz"
	"github.com/LynnColeArt/better-cal/backend/internal/booking"
	"github.com/LynnColeArt/better-cal/backend/internal/config"
	"github.com/LynnColeArt/better-cal/backend/internal/logging"
)

type contextKey string

const requestIDKey contextKey = "request-id"

type Server struct {
	cfg          config.Config
	authService  *auth.Service
	authorizer   *authz.Authorizer
	bookingStore *booking.Store
	logger       *slog.Logger
	mux          *http.ServeMux
}

type Option func(*Server)

func WithBookingStore(store *booking.Store) Option {
	return func(s *Server) {
		if store != nil {
			s.bookingStore = store
		}
	}
}

func NewServer(cfg config.Config, opts ...Option) http.Handler {
	return NewServerWithLogger(cfg, slog.Default(), opts...)
}

func NewServerWithLogger(cfg config.Config, logger *slog.Logger, opts ...Option) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	server := &Server{
		cfg:          cfg,
		authService:  auth.NewService(cfg),
		authorizer:   authz.NewAuthorizer(),
		bookingStore: booking.NewStore(),
		logger:       logger,
		mux:          http.NewServeMux(),
	}
	for _, opt := range opts {
		opt(server)
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

func (s *Server) authenticateAPIKey(r *http.Request) (auth.Principal, bool) {
	return s.authenticator().AuthenticateAPIKey(r.Header.Get("authorization"))
}

func (s *Server) authenticator() *auth.Service {
	if s.authService != nil {
		return s.authService
	}
	return auth.NewService(s.cfg)
}

func (s *Server) authorize(principal auth.Principal, policy authz.Policy) bool {
	return s.policies().Authorize(principal, policy).Allowed
}

func (s *Server) policies() *authz.Authorizer {
	if s.authorizer != nil {
		return s.authorizer
	}
	s.authorizer = authz.NewAuthorizer()
	return s.authorizer
}

func (s *Server) bookings() *booking.Store {
	if s.bookingStore != nil {
		return s.bookingStore
	}
	s.bookingStore = booking.NewStore()
	return s.bookingStore
}

func (s *Server) requestID(r *http.Request) string {
	requestID, _ := r.Context().Value(requestIDKey).(string)
	if requestID == "" {
		return s.cfg.RequestID
	}
	return requestID
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
