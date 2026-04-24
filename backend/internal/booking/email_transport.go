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

const defaultEmailTransportTimeout = 5 * time.Second

type EmailDeliveryAttempt struct {
	ID           int64
	DeliveryID   int64
	SideEffectID int64
	TargetURL    string
	Action       string
	ContentType  string
	Body         string
}

type EmailDeliveryReceipt struct {
	StatusCode int
}

type EmailDeliveryTransport interface {
	DeliverEmailDelivery(context.Context, EmailDeliveryAttempt) (EmailDeliveryReceipt, error)
}

type HTTPEmailTransport struct {
	client *http.Client
}

func NewHTTPEmailTransport(client *http.Client) HTTPEmailTransport {
	if client == nil {
		client = &http.Client{Timeout: defaultEmailTransportTimeout}
	}
	return HTTPEmailTransport{client: client}
}

func (t HTTPEmailTransport) DeliverEmailDelivery(ctx context.Context, attempt EmailDeliveryAttempt) (EmailDeliveryReceipt, error) {
	if t.client == nil {
		return EmailDeliveryReceipt{}, EmailDeliveryError{}
	}
	if attempt.TargetURL == "" || attempt.Body == "" || attempt.ContentType == "" || attempt.Action == "" {
		return EmailDeliveryReceipt{}, EmailDeliveryError{}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, attempt.TargetURL, strings.NewReader(attempt.Body))
	if err != nil {
		return EmailDeliveryReceipt{}, EmailDeliveryError{}
	}
	req.Header.Set("Content-Type", attempt.ContentType)
	req.Header.Set("X-Cal-Email-Action", attempt.Action)

	resp, err := t.client.Do(req)
	if err != nil {
		return EmailDeliveryReceipt{}, EmailDeliveryError{}
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))

	receipt := EmailDeliveryReceipt{StatusCode: resp.StatusCode}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return receipt, EmailDeliveryError{StatusCode: resp.StatusCode}
	}
	return receipt, nil
}

type EmailDeliveryError struct {
	StatusCode int
}

func (e EmailDeliveryError) Error() string {
	if e.StatusCode > 0 {
		return fmt.Sprintf("email delivery returned status %d", e.StatusCode)
	}
	return "email delivery failed"
}

func safeEmailDeliveryError(err error) string {
	var deliveryErr EmailDeliveryError
	if errors.As(err, &deliveryErr) {
		return deliveryErr.Error()
	}
	return "email delivery failed"
}

func emailDeliveryStatusCode(receipt EmailDeliveryReceipt, err error) int {
	if receipt.StatusCode > 0 {
		return receipt.StatusCode
	}
	var deliveryErr EmailDeliveryError
	if errors.As(err, &deliveryErr) {
		return deliveryErr.StatusCode
	}
	return 0
}
