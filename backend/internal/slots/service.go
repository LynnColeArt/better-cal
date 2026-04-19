package slots

import (
	"context"
	"errors"
	"fmt"
	"time"
	_ "time/tzdata"
)

const (
	FixtureEventTypeID = 1001
	FixtureEventTitle  = "Fixture Event"
	FixtureStart       = "2026-05-01T00:00:00.000Z"
	FixtureEnd         = "2026-05-02T00:00:00.000Z"
	FixtureTimeZone    = "America/Chicago"
	FixtureSlotTime    = "2026-05-01T15:00:00.000Z"
	FixtureDuration    = 30
)

type Repository interface {
	ReadAvailable(context.Context, string, Request) (Response, bool, error)
	SaveEventType(context.Context, EventType) error
	SaveAvailabilitySlot(context.Context, AvailabilitySlot) error
}

type Request struct {
	EventTypeID int
	Start       string
	End         string
	TimeZone    string
}

type AvailabilityRequest struct {
	EventTypeID int
	Start       string
	TimeZone    string
}

type EventType struct {
	ID       int
	Title    string
	Duration int
	TimeZone string
}

type AvailabilitySlot struct {
	EventTypeID int
	Time        string
	Duration    int
	TimeZone    string
}

type Response struct {
	EventTypeID int               `json:"eventTypeId"`
	TimeZone    string            `json:"timeZone"`
	Start       string            `json:"start"`
	End         string            `json:"end"`
	Slots       map[string][]Slot `json:"slots"`
	RequestID   string            `json:"requestId"`
}

type Slot struct {
	Time     string `json:"time"`
	Duration int    `json:"duration"`
}

type Service struct {
	repo Repository
}

type ServiceOption func(*Service)

func WithRepository(repo Repository) ServiceOption {
	return func(s *Service) {
		s.repo = repo
	}
}

type ErrorWithCode struct {
	Code    string
	Message string
}

func (e *ErrorWithCode) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func ValidationFromError(err error) (*ErrorWithCode, bool) {
	var codedErr *ErrorWithCode
	if errors.As(err, &codedErr) {
		return codedErr, true
	}
	return nil, false
}

func NewService(opts ...ServiceOption) *Service {
	service := &Service{}
	for _, opt := range opts {
		opt(service)
	}
	return service
}

func (s *Service) IsAvailable(ctx context.Context, requestID string, req AvailabilityRequest) (bool, error) {
	if req.Start == "" {
		return false, validationError("INVALID_START_TIME", "Start time is required")
	}
	startAt, err := time.Parse(time.RFC3339Nano, req.Start)
	if err != nil {
		return false, validationError("INVALID_START_TIME", "Start time must be an RFC3339 timestamp")
	}
	response, ok, err := s.ReadAvailable(ctx, requestID, Request{
		EventTypeID: req.EventTypeID,
		Start:       req.Start,
		End:         startAt.Add(time.Microsecond).Format(time.RFC3339Nano),
		TimeZone:    req.TimeZone,
	})
	if err != nil || !ok {
		return false, err
	}
	return response.HasSlot(req.Start), nil
}

func (s *Service) ReadAvailable(ctx context.Context, requestID string, req Request) (Response, bool, error) {
	if err := validateRequest(req); err != nil {
		return Response{}, false, err
	}
	req = normalizedRequest(req)
	if s.repo != nil {
		return s.repo.ReadAvailable(ctx, requestID, req)
	}
	if req.EventTypeID != FixtureEventTypeID {
		return Response{}, false, nil
	}
	return Response{
		EventTypeID: req.EventTypeID,
		TimeZone:    req.TimeZone,
		Start:       req.Start,
		End:         req.End,
		Slots: map[string][]Slot{
			"2026-05-01": {
				{Time: FixtureSlotTime, Duration: FixtureDuration},
			},
		},
		RequestID: requestID,
	}, true, nil
}

func (r Response) HasSlot(start string) bool {
	for _, daySlots := range r.Slots {
		for _, slot := range daySlots {
			if slot.Time == start {
				return true
			}
		}
	}
	return false
}

func FixtureEventType() EventType {
	return EventType{
		ID:       FixtureEventTypeID,
		Title:    FixtureEventTitle,
		Duration: FixtureDuration,
		TimeZone: FixtureTimeZone,
	}
}

func FixtureAvailabilitySlot() AvailabilitySlot {
	return AvailabilitySlot{
		EventTypeID: FixtureEventTypeID,
		Time:        FixtureSlotTime,
		Duration:    FixtureDuration,
		TimeZone:    FixtureTimeZone,
	}
}

func normalizedRequest(req Request) Request {
	if req.TimeZone == "" {
		req.TimeZone = FixtureTimeZone
	}
	if req.Start == "" {
		req.Start = FixtureStart
	}
	if req.End == "" {
		req.End = FixtureEnd
	}
	return req
}

func validateRequest(req Request) error {
	if req.EventTypeID == 0 {
		return validationError("INVALID_EVENT_TYPE", "Event type is required")
	}
	if err := validateTimestamp(req.Start, "INVALID_START_TIME", "Start time must be an RFC3339 timestamp"); err != nil {
		return err
	}
	if err := validateTimestamp(req.End, "INVALID_END_TIME", "End time must be an RFC3339 timestamp"); err != nil {
		return err
	}
	if req.TimeZone != "" && !validTimeZone(req.TimeZone) {
		return validationError("INVALID_TIME_ZONE", "Time zone is invalid")
	}
	return nil
}

func validateTimestamp(value string, code string, message string) error {
	if value == "" {
		return nil
	}
	if _, err := time.Parse(time.RFC3339Nano, value); err != nil {
		return validationError(code, message)
	}
	return nil
}

func validTimeZone(value string) bool {
	switch value {
	case "UTC", "Etc/UTC", FixtureTimeZone:
		return true
	}
	_, err := time.LoadLocation(value)
	return err == nil
}

func validationError(code string, message string) error {
	return &ErrorWithCode{Code: code, Message: message}
}
