package booking

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/LynnColeArt/better-cal/backend/internal/calendar"
)

func (d PostgresSideEffectDispatcher) prepareCalendarProviderDispatch(ctx context.Context, attempt CalendarDispatchAttempt) (calendar.PreparedDispatch, error) {
	if d.calendarProvider == nil {
		return calendar.PreparedDispatch{}, errors.New("calendar dispatch requires a provider adapter")
	}

	var envelope CalendarDispatchEnvelope
	if err := json.Unmarshal([]byte(attempt.Body), &envelope); err != nil {
		return calendar.PreparedDispatch{}, fmt.Errorf("decode calendar dispatch envelope: %w", err)
	}

	prepared, err := d.calendarProvider.PrepareDispatch(ctx, calendar.DispatchInput{
		Action:        string(envelope.Action),
		CreatedAt:     envelope.CreatedAt,
		UID:           envelope.Payload.UID,
		Status:        envelope.Payload.Status,
		Start:         envelope.Payload.Start,
		End:           envelope.Payload.End,
		EventTypeID:   envelope.Payload.EventTypeID,
		RequestID:     envelope.Payload.RequestID,
		RescheduleUID: envelope.Payload.RescheduleUID,
	})
	if err != nil {
		return calendar.PreparedDispatch{}, fmt.Errorf("prepare calendar provider dispatch: %w", err)
	}

	prepared.TargetURL = attempt.TargetURL
	if prepared.ContentType == "" {
		prepared.ContentType = attempt.ContentType
	}
	if prepared.Headers == nil {
		prepared.Headers = map[string]string{}
	}
	if prepared.Headers["X-Cal-Calendar-Action"] == "" {
		prepared.Headers["X-Cal-Calendar-Action"] = attempt.Action
	}
	return prepared, nil
}
