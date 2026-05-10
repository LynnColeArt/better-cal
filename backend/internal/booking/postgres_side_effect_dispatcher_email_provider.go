package booking

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	emailprovider "github.com/LynnColeArt/better-cal/backend/internal/email"
)

func (d PostgresSideEffectDispatcher) prepareEmailProviderDispatch(ctx context.Context, attempt EmailDeliveryAttempt) (emailprovider.PreparedDispatch, error) {
	if d.emailProvider == nil {
		return emailprovider.PreparedDispatch{}, errors.New("email delivery requires a provider adapter")
	}

	var envelope EmailDeliveryEnvelope
	if err := json.Unmarshal([]byte(attempt.Body), &envelope); err != nil {
		return emailprovider.PreparedDispatch{}, fmt.Errorf("decode email delivery envelope: %w", err)
	}

	prepared, err := d.emailProvider.PrepareDispatch(ctx, emailprovider.DispatchInput{
		Action:             string(envelope.Action),
		CreatedAt:          envelope.CreatedAt,
		UID:                envelope.Payload.UID,
		Title:              envelope.Payload.Title,
		Status:             envelope.Payload.Status,
		Start:              envelope.Payload.Start,
		End:                envelope.Payload.End,
		EventTypeID:        envelope.Payload.EventTypeID,
		RequestID:          envelope.Payload.RequestID,
		Recipients:         emailProviderRecipients(envelope.Payload.Recipients),
		CancellationReason: envelope.Payload.CancellationReason,
		RescheduleUID:      envelope.Payload.RescheduleUID,
		ReschedulingReason: envelope.Payload.ReschedulingReason,
		RejectionReason:    envelope.Payload.RejectionReason,
	})
	if err != nil {
		return emailprovider.PreparedDispatch{}, fmt.Errorf("prepare email provider dispatch: %w", err)
	}

	prepared.TargetURL = attempt.TargetURL
	if prepared.ContentType == "" {
		prepared.ContentType = attempt.ContentType
	}
	if prepared.Headers == nil {
		prepared.Headers = map[string]string{}
	}
	if prepared.Headers["X-Cal-Email-Action"] == "" {
		prepared.Headers["X-Cal-Email-Action"] = attempt.Action
	}
	return prepared, nil
}

func emailProviderRecipients(recipients []EmailRecipient) []emailprovider.Recipient {
	if len(recipients) == 0 {
		return nil
	}
	result := make([]emailprovider.Recipient, 0, len(recipients))
	for _, recipient := range recipients {
		result = append(result, emailprovider.Recipient{
			Name:     recipient.Name,
			Email:    recipient.Email,
			TimeZone: recipient.TimeZone,
		})
	}
	return result
}
