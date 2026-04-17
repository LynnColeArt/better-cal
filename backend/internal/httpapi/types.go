package httpapi

type envelope struct {
	Status string `json:"status"`
	Data   any    `json:"data,omitempty"`
	Error  *err   `json:"error,omitempty"`
}

type err struct {
	Code      string `json:"code"`
	Message   string `json:"message,omitempty"`
	RequestID string `json:"requestId,omitempty"`
}

type attendee struct {
	ID       int    `json:"id,omitempty"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	TimeZone string `json:"timeZone"`
}

type booking struct {
	UID         string         `json:"uid"`
	ID          int            `json:"id"`
	Title       string         `json:"title"`
	Status      string         `json:"status"`
	Start       string         `json:"start"`
	End         string         `json:"end"`
	EventTypeID int            `json:"eventTypeId"`
	Attendees   []attendee     `json:"attendees"`
	Responses   map[string]any `json:"responses"`
	Metadata    map[string]any `json:"metadata"`
	CreatedAt   string         `json:"createdAt"`
	UpdatedAt   string         `json:"updatedAt"`
	RequestID   string         `json:"requestId"`
}

type createBookingRequest struct {
	EventTypeID    int            `json:"eventTypeId"`
	Start          string         `json:"start"`
	Attendee       attendee       `json:"attendee"`
	Responses      map[string]any `json:"responses"`
	Metadata       map[string]any `json:"metadata"`
	IdempotencyKey string         `json:"idempotencyKey"`
}

type cancelBookingRequest struct {
	CancellationReason string `json:"cancellationReason"`
}

type rescheduleBookingRequest struct {
	Start              string `json:"start"`
	ReschedulingReason string `json:"reschedulingReason"`
}
