package booking

import "context"

type SlotAvailabilityPort interface {
	IsSlotAvailable(context.Context, SlotAvailabilityRequest) (bool, error)
}

type SlotAvailabilityRequest struct {
	EventTypeID int
	Start       string
	TimeZone    string
}

type FixtureSlotAvailabilityPort struct{}

func (FixtureSlotAvailabilityPort) IsSlotAvailable(_ context.Context, req SlotAvailabilityRequest) (bool, error) {
	return req.EventTypeID == FixtureEventTypeID &&
		req.Start == FixtureBookingStart &&
		req.TimeZone == FixtureTimeZone, nil
}
