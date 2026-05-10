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
	FixtureReschedule  = "2026-05-02T15:00:00.000Z"
	FixtureDuration    = 30
)

type Repository interface {
	ReadAvailable(context.Context, string, Request) (Response, bool, error)
	SaveEventType(context.Context, EventType) error
	SaveAvailabilitySlot(context.Context, AvailabilitySlot) error
}

type BusyTimeProvider interface {
	BusyTimes(context.Context, Request) ([]BusyTime, error)
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

type BusyTime struct {
	Start string
	End   string
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
	repo      Repository
	busyTimes BusyTimeProvider
}

type ServiceOption func(*Service)

func WithRepository(repo Repository) ServiceOption {
	return func(s *Service) {
		s.repo = repo
	}
}

func WithBusyTimeProvider(provider BusyTimeProvider) ServiceOption {
	return func(s *Service) {
		s.busyTimes = provider
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
		response, ok, err := s.repo.ReadAvailable(ctx, requestID, req)
		if err != nil || !ok {
			return response, ok, err
		}
		return s.filterBusySlots(ctx, req, response)
	}
	if req.EventTypeID != FixtureEventTypeID {
		return Response{}, false, nil
	}
	response := Response{
		EventTypeID: req.EventTypeID,
		TimeZone:    req.TimeZone,
		Start:       req.Start,
		End:         req.End,
		Slots:       fixtureSlotsInRange(req),
		RequestID:   requestID,
	}
	return s.filterBusySlots(ctx, req, response)
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

func (s *Service) filterBusySlots(ctx context.Context, req Request, response Response) (Response, bool, error) {
	if s.busyTimes == nil {
		return response, true, nil
	}
	busyTimes, err := s.busyTimes.BusyTimes(ctx, req)
	if err != nil {
		return Response{}, false, err
	}
	if len(busyTimes) == 0 {
		return response, true, nil
	}
	filtered := map[string][]Slot{}
	for day, daySlots := range response.Slots {
		for _, slot := range daySlots {
			blocked, err := slotOverlapsBusyTime(slot, busyTimes)
			if err != nil {
				return Response{}, false, err
			}
			if blocked {
				continue
			}
			filtered[day] = append(filtered[day], slot)
		}
	}
	response.Slots = filtered
	return response, true, nil
}

func slotOverlapsBusyTime(slot Slot, busyTimes []BusyTime) (bool, error) {
	slotStart, err := time.Parse(time.RFC3339Nano, slot.Time)
	if err != nil {
		return false, fmt.Errorf("parse slot time: %w", err)
	}
	slotEnd := slotStart.Add(time.Duration(slot.Duration) * time.Minute)
	for _, busyTime := range busyTimes {
		busyStart, err := time.Parse(time.RFC3339Nano, busyTime.Start)
		if err != nil {
			return false, fmt.Errorf("parse busy start: %w", err)
		}
		busyEnd, err := time.Parse(time.RFC3339Nano, busyTime.End)
		if err != nil {
			return false, fmt.Errorf("parse busy end: %w", err)
		}
		if slotStart.Before(busyEnd) && busyStart.Before(slotEnd) {
			return true, nil
		}
	}
	return false, nil
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

func FixtureAvailabilitySlots() []AvailabilitySlot {
	return []AvailabilitySlot{
		FixtureAvailabilitySlot(),
		{
			EventTypeID: FixtureEventTypeID,
			Time:        FixtureReschedule,
			Duration:    FixtureDuration,
			TimeZone:    FixtureTimeZone,
		},
	}
}

func fixtureSlotsInRange(req Request) map[string][]Slot {
	startAt, err := time.Parse(time.RFC3339Nano, req.Start)
	if err != nil {
		return map[string][]Slot{}
	}
	endAt, err := time.Parse(time.RFC3339Nano, req.End)
	if err != nil {
		return map[string][]Slot{}
	}
	slotsByDay := map[string][]Slot{}
	for _, fixtureSlot := range FixtureAvailabilitySlots() {
		slotAt, err := time.Parse(time.RFC3339Nano, fixtureSlot.Time)
		if err != nil {
			continue
		}
		if slotAt.Before(startAt) || !slotAt.Before(endAt) {
			continue
		}
		day := slotAt.Format("2006-01-02")
		slotsByDay[day] = append(slotsByDay[day], Slot{
			Time:     fixtureSlot.Time,
			Duration: fixtureSlot.Duration,
		})
	}
	return slotsByDay
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
