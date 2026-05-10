package booking

import (
	"context"

	"github.com/LynnColeArt/better-cal/backend/internal/slots"
)

type SlotAvailabilityPort interface {
	IsSlotAvailable(context.Context, SlotAvailabilityRequest) (bool, error)
}

type SlotAvailabilityRequest struct {
	RequestID   string
	EventTypeID int
	Start       string
	TimeZone    string
}

type slotAvailabilityService interface {
	IsAvailable(context.Context, string, slots.AvailabilityRequest) (bool, error)
}

type SlotServiceAvailabilityPort struct {
	service slotAvailabilityService
}

func NewSlotServiceAvailabilityPort(service slotAvailabilityService) SlotServiceAvailabilityPort {
	return SlotServiceAvailabilityPort{service: service}
}

func (p SlotServiceAvailabilityPort) IsSlotAvailable(ctx context.Context, req SlotAvailabilityRequest) (bool, error) {
	service := p.service
	if service == nil {
		service = slots.NewService()
	}
	return service.IsAvailable(ctx, req.RequestID, slots.AvailabilityRequest{
		EventTypeID: req.EventTypeID,
		Start:       req.Start,
		TimeZone:    req.TimeZone,
	})
}
