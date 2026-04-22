package booking

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultWebhookTransportTimeout = 5 * time.Second

type WebhookDeliveryAttempt struct {
	ID                   int64
	DeliveryID           int64
	SideEffectID         int64
	SubscriberID         int64
	SubscriberURL        string
	TriggerEvent         string
	ContentType          string
	SignatureHeaderName  string
	SignatureHeaderValue string
	Body                 string
}

type WebhookAttemptReceipt struct {
	StatusCode int
}

type WebhookAttemptTransport interface {
	DeliverWebhookAttempt(context.Context, WebhookDeliveryAttempt) (WebhookAttemptReceipt, error)
}

type HTTPWebhookTransport struct {
	client *http.Client
}

func NewHTTPWebhookTransport(client *http.Client) HTTPWebhookTransport {
	if client == nil {
		client = &http.Client{Timeout: defaultWebhookTransportTimeout}
	}
	return HTTPWebhookTransport{client: client}
}

func (t HTTPWebhookTransport) DeliverWebhookAttempt(ctx context.Context, attempt WebhookDeliveryAttempt) (WebhookAttemptReceipt, error) {
	if t.client == nil {
		return WebhookAttemptReceipt{}, WebhookAttemptError{}
	}
	if attempt.SubscriberURL == "" || attempt.Body == "" || attempt.ContentType == "" || attempt.SignatureHeaderName == "" || attempt.SignatureHeaderValue == "" {
		return WebhookAttemptReceipt{}, WebhookAttemptError{}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, attempt.SubscriberURL, strings.NewReader(attempt.Body))
	if err != nil {
		return WebhookAttemptReceipt{}, WebhookAttemptError{}
	}
	req.Header.Set("Content-Type", attempt.ContentType)
	req.Header.Set(attempt.SignatureHeaderName, attempt.SignatureHeaderValue)

	resp, err := t.client.Do(req)
	if err != nil {
		return WebhookAttemptReceipt{}, WebhookAttemptError{}
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))

	receipt := WebhookAttemptReceipt{StatusCode: resp.StatusCode}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return receipt, WebhookAttemptError{StatusCode: resp.StatusCode}
	}
	return receipt, nil
}

type WebhookAttemptError struct {
	StatusCode int
}

func (e WebhookAttemptError) Error() string {
	if e.StatusCode > 0 {
		return fmt.Sprintf("webhook delivery returned status %d", e.StatusCode)
	}
	return "webhook delivery failed"
}

func safeWebhookAttemptError(err error) string {
	var attemptErr WebhookAttemptError
	if errors.As(err, &attemptErr) {
		return attemptErr.Error()
	}
	return "webhook delivery failed"
}

func webhookAttemptStatusCode(receipt WebhookAttemptReceipt, err error) int {
	if receipt.StatusCode > 0 {
		return receipt.StatusCode
	}
	var attemptErr WebhookAttemptError
	if errors.As(err, &attemptErr) {
		return attemptErr.StatusCode
	}
	return 0
}
