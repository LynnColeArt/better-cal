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

const defaultCalendarTransportTimeout = 5 * time.Second

type CalendarDispatchAttempt struct {
	ID           int64
	DispatchID   int64
	SideEffectID int64
	TargetURL    string
	Action       string
	ContentType  string
	Body         string
}

type CalendarDispatchReceipt struct {
	StatusCode int
}

type CalendarDispatchTransport interface {
	DeliverCalendarDispatch(context.Context, CalendarDispatchAttempt) (CalendarDispatchReceipt, error)
}

type HTTPCalendarTransport struct {
	client *http.Client
}

func NewHTTPCalendarTransport(client *http.Client) HTTPCalendarTransport {
	if client == nil {
		client = &http.Client{Timeout: defaultCalendarTransportTimeout}
	}
	return HTTPCalendarTransport{client: client}
}

func (t HTTPCalendarTransport) DeliverCalendarDispatch(ctx context.Context, attempt CalendarDispatchAttempt) (CalendarDispatchReceipt, error) {
	if t.client == nil {
		return CalendarDispatchReceipt{}, CalendarDispatchError{}
	}
	if attempt.TargetURL == "" || attempt.Body == "" || attempt.ContentType == "" || attempt.Action == "" {
		return CalendarDispatchReceipt{}, CalendarDispatchError{}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, attempt.TargetURL, strings.NewReader(attempt.Body))
	if err != nil {
		return CalendarDispatchReceipt{}, CalendarDispatchError{}
	}
	req.Header.Set("Content-Type", attempt.ContentType)
	req.Header.Set("X-Cal-Calendar-Action", attempt.Action)

	resp, err := t.client.Do(req)
	if err != nil {
		return CalendarDispatchReceipt{}, CalendarDispatchError{}
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))

	receipt := CalendarDispatchReceipt{StatusCode: resp.StatusCode}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return receipt, CalendarDispatchError{StatusCode: resp.StatusCode}
	}
	return receipt, nil
}

type CalendarDispatchError struct {
	StatusCode int
}

func (e CalendarDispatchError) Error() string {
	if e.StatusCode > 0 {
		return fmt.Sprintf("calendar dispatch returned status %d", e.StatusCode)
	}
	return "calendar dispatch failed"
}

func safeCalendarDispatchError(err error) string {
	var dispatchErr CalendarDispatchError
	if errors.As(err, &dispatchErr) {
		return dispatchErr.Error()
	}
	return "calendar dispatch failed"
}

func calendarDispatchStatusCode(receipt CalendarDispatchReceipt, err error) int {
	if receipt.StatusCode > 0 {
		return receipt.StatusCode
	}
	var dispatchErr CalendarDispatchError
	if errors.As(err, &dispatchErr) {
		return dispatchErr.StatusCode
	}
	return 0
}
