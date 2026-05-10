package email

import "context"

type Recipient struct {
	Name     string
	Email    string
	TimeZone string
}

type DispatchInput struct {
	Action             string
	CreatedAt          string
	UID                string
	Title              string
	Status             string
	Start              string
	End                string
	EventTypeID        int
	RequestID          string
	Recipients         []Recipient
	CancellationReason string
	RescheduleUID      string
	ReschedulingReason string
	RejectionReason    string
}

type PreparedDispatch struct {
	TargetURL   string
	ContentType string
	Headers     map[string]string
	Body        string
}

type ProviderAdapter interface {
	PrepareDispatch(context.Context, DispatchInput) (PreparedDispatch, error)
}
