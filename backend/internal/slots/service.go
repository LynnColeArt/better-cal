package slots

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const (
	FixtureEventTypeID = 1001
	FixtureStart       = "2026-05-01T00:00:00.000Z"
	FixtureEnd         = "2026-05-02T00:00:00.000Z"
	FixtureTimeZone    = "America/Chicago"
	FixtureSlotTime    = "2026-05-01T15:00:00.000Z"
)

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

type Service struct{}

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

func NewService() *Service {
	return &Service{}
}

func (s *Service) IsAvailable(ctx context.Context, requestID string, req AvailabilityRequest) (bool, error) {
	if req.Start == "" {
		return false, validationError("INVALID_START_TIME", "Start time is required")
	}
	response, ok, err := s.ReadAvailable(ctx, requestID, Request{
		EventTypeID: req.EventTypeID,
		Start:       req.Start,
		TimeZone:    req.TimeZone,
	})
	if err != nil || !ok {
		return false, err
	}
	return response.HasSlot(req.Start), nil
}

func (s *Service) ReadAvailable(_ context.Context, requestID string, req Request) (Response, bool, error) {
	if err := validateRequest(req); err != nil {
		return Response{}, false, err
	}
	if req.EventTypeID != FixtureEventTypeID {
		return Response{}, false, nil
	}
	timeZone := req.TimeZone
	if timeZone == "" {
		timeZone = FixtureTimeZone
	}
	start := req.Start
	if start == "" {
		start = FixtureStart
	}
	end := req.End
	if end == "" {
		end = FixtureEnd
	}
	return Response{
		EventTypeID: req.EventTypeID,
		TimeZone:    timeZone,
		Start:       start,
		End:         end,
		Slots: map[string][]Slot{
			"2026-05-01": {
				{Time: FixtureSlotTime, Duration: 30},
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
