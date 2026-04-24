package main

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"
)

type capturedRequest struct {
	Path                   string `json:"path"`
	ContentType            string `json:"contentType"`
	SignatureHeaderPresent bool   `json:"signatureHeaderPresent"`
	Body                   string `json:"body"`
	ReceivedAt             string `json:"receivedAt"`
}

type webhookSink struct {
	mu       sync.Mutex
	requests []capturedRequest
}

func main() {
	host := env("HOST", "127.0.0.1")
	port := env("PORT", "8090")
	addr := env("ADDR", host+":"+port)

	sink := &webhookSink{}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/requests", sink.handleRequests)
	mux.HandleFunc("/caldiy/webhook", sink.handleWebhook)
	mux.HandleFunc("/caldiy/email-dispatch", sink.handleEmailDispatch)
	mux.HandleFunc("/caldiy/calendar-dispatch", sink.handleCalendarDispatch)

	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	slog.Info("starting webhook sink", "addr", addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("webhook sink failed", "error", err)
		os.Exit(1)
	}
}

func (s *webhookSink) handleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	s.captureRequest(w, r)
}

func (s *webhookSink) handleCalendarDispatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	s.captureRequest(w, r)
}

func (s *webhookSink) handleEmailDispatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	s.captureRequest(w, r)
}

func (s *webhookSink) captureRequest(w http.ResponseWriter, r *http.Request) {
	bodyRaw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "read failed", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	s.requests = append(s.requests, capturedRequest{
		Path:                   r.URL.Path,
		ContentType:            r.Header.Get("Content-Type"),
		SignatureHeaderPresent: r.Header.Get("X-Cal-Signature-256") != "",
		Body:                   string(bodyRaw),
		ReceivedAt:             time.Now().UTC().Format(time.RFC3339Nano),
	})
	s.mu.Unlock()

	w.WriteHeader(http.StatusAccepted)
}

func (s *webhookSink) handleRequests(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.writeRequests(w)
	case http.MethodDelete:
		s.clearRequests(w)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *webhookSink) writeRequests(w http.ResponseWriter) {
	s.mu.Lock()
	requests := append([]capturedRequest(nil), s.requests...)
	s.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"count":    len(requests),
		"requests": requests,
	})
}

func (s *webhookSink) clearRequests(w http.ResponseWriter) {
	s.mu.Lock()
	s.requests = nil
	s.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

func env(name string, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
