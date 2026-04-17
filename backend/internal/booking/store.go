package booking

import "sync"

const (
	PrimaryFixtureUID     = "mock-booking-personal-basic"
	RescheduledFixtureUID = "mock-booking-rescheduled"
)

type Attendee struct {
	ID       int    `json:"id,omitempty"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	TimeZone string `json:"timeZone"`
}

type Booking struct {
	UID         string         `json:"uid"`
	ID          int            `json:"id"`
	Title       string         `json:"title"`
	Status      string         `json:"status"`
	Start       string         `json:"start"`
	End         string         `json:"end"`
	EventTypeID int            `json:"eventTypeId"`
	Attendees   []Attendee     `json:"attendees"`
	Responses   map[string]any `json:"responses"`
	Metadata    map[string]any `json:"metadata"`
	CreatedAt   string         `json:"createdAt"`
	UpdatedAt   string         `json:"updatedAt"`
	RequestID   string         `json:"requestId"`
}

type CreateRequest struct {
	EventTypeID    int            `json:"eventTypeId"`
	Start          string         `json:"start"`
	Attendee       Attendee       `json:"attendee"`
	Responses      map[string]any `json:"responses"`
	Metadata       map[string]any `json:"metadata"`
	IdempotencyKey string         `json:"idempotencyKey"`
}

type CancelRequest struct {
	CancellationReason string `json:"cancellationReason"`
}

type RescheduleRequest struct {
	Start              string `json:"start"`
	ReschedulingReason string `json:"reschedulingReason"`
}

type CancelResult struct {
	Booking     Booking
	SideEffects []string
}

type RescheduleResult struct {
	OldBooking  Booking
	NewBooking  Booking
	SideEffects []string
}

type Store struct {
	mu          sync.Mutex
	bookings    map[string]Booking
	idempotency map[string]string
}

func NewStore() *Store {
	return &Store{
		bookings:    make(map[string]Booking),
		idempotency: make(map[string]string),
	}
}

func (s *Store) Create(requestID string, req CreateRequest) (Booking, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if req.IdempotencyKey != "" {
		uid, ok := s.idempotency[req.IdempotencyKey]
		if ok {
			bookingValue, ok := s.bookings[uid]
			if ok {
				return bookingValue, true
			}
		}
	}

	attendeeValue := req.Attendee
	if attendeeValue.Name == "" {
		attendeeValue.Name = "Fixture Attendee"
	}
	if attendeeValue.Email == "" {
		attendeeValue.Email = "fixture-attendee@example.test"
	}
	if attendeeValue.TimeZone == "" {
		attendeeValue.TimeZone = "America/Chicago"
	}
	attendeeValue.ID = 321

	start := req.Start
	if start == "" {
		start = "2026-05-01T15:00:00.000Z"
	}
	responses := req.Responses
	if responses == nil {
		responses = map[string]any{}
	}
	metadata := req.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}

	created := fixtureBooking(requestID, Booking{
		Start: start,
		Attendees: []Attendee{
			attendeeValue,
		},
		Responses: responses,
		Metadata:  metadata,
	})
	s.bookings[created.UID] = created
	if req.IdempotencyKey != "" {
		s.idempotency[req.IdempotencyKey] = created.UID
	}

	return created, false
}

func (s *Store) Read(requestID string, uid string) (Booking, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if uid == PrimaryFixtureUID {
		return s.ensureBooking(requestID), true
	}
	bookingValue, ok := s.bookings[uid]
	return bookingValue, ok
}

func (s *Store) Cancel(requestID string, uid string, _ CancelRequest) (CancelResult, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.bookings[uid]
	if uid == PrimaryFixtureUID {
		existing = s.ensureBooking(requestID)
		ok = true
	}
	if !ok {
		return CancelResult{}, false
	}

	cancelled := fixtureBooking(requestID, mergeBooking(existing, Booking{
		Status:    "cancelled",
		UpdatedAt: "2026-01-01T00:05:00.000Z",
	}))
	s.bookings[uid] = cancelled

	return CancelResult{
		Booking: cancelled,
		SideEffects: []string{
			"calendar.cancelled",
			"email.cancelled",
			"webhook.booking.cancelled",
		},
	}, true
}

func (s *Store) Reschedule(requestID string, oldUID string, req RescheduleRequest) (RescheduleResult, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.bookings[oldUID]
	if oldUID == PrimaryFixtureUID {
		existing = s.ensureBooking(requestID)
		ok = true
	}
	if !ok {
		return RescheduleResult{}, false
	}

	oldBooking := fixtureBooking(requestID, mergeBooking(existing, Booking{
		Status:    "cancelled",
		UpdatedAt: "2026-01-01T00:10:00.000Z",
	}))
	start := req.Start
	if start == "" {
		start = "2026-05-02T15:00:00.000Z"
	}
	newBooking := fixtureBooking(requestID, mergeBooking(existing, Booking{
		UID:       RescheduledFixtureUID,
		Start:     start,
		End:       "2026-05-02T15:30:00.000Z",
		UpdatedAt: "2026-01-01T00:10:00.000Z",
	}))

	s.bookings[oldUID] = oldBooking
	s.bookings[newBooking.UID] = newBooking

	return RescheduleResult{
		OldBooking: oldBooking,
		NewBooking: newBooking,
		SideEffects: []string{
			"calendar.rescheduled",
			"email.rescheduled",
			"webhook.booking.rescheduled",
		},
	}, true
}

func fixtureBooking(requestID string, overrides Booking) Booking {
	base := Booking{
		UID:         PrimaryFixtureUID,
		ID:          987,
		Title:       "Fixture Event",
		Status:      "accepted",
		Start:       "2026-05-01T15:00:00.000Z",
		End:         "2026-05-01T15:30:00.000Z",
		EventTypeID: 1001,
		Attendees: []Attendee{
			{
				ID:       321,
				Name:     "Fixture Attendee",
				Email:    "fixture-attendee@example.test",
				TimeZone: "America/Chicago",
			},
		},
		Responses: map[string]any{
			"name":  "Fixture Attendee",
			"email": "fixture-attendee@example.test",
		},
		Metadata: map[string]any{
			"fixture": "personal-basic",
		},
		CreatedAt: "2026-01-01T00:00:00.000Z",
		UpdatedAt: "2026-01-01T00:00:00.000Z",
		RequestID: requestID,
	}
	return mergeBooking(base, overrides)
}

func mergeBooking(base Booking, overrides Booking) Booking {
	if overrides.UID != "" {
		base.UID = overrides.UID
	}
	if overrides.ID != 0 {
		base.ID = overrides.ID
	}
	if overrides.Title != "" {
		base.Title = overrides.Title
	}
	if overrides.Status != "" {
		base.Status = overrides.Status
	}
	if overrides.Start != "" {
		base.Start = overrides.Start
	}
	if overrides.End != "" {
		base.End = overrides.End
	}
	if overrides.EventTypeID != 0 {
		base.EventTypeID = overrides.EventTypeID
	}
	if overrides.Attendees != nil {
		base.Attendees = overrides.Attendees
	}
	if overrides.Responses != nil {
		base.Responses = overrides.Responses
	}
	if overrides.Metadata != nil {
		base.Metadata = overrides.Metadata
	}
	if overrides.CreatedAt != "" {
		base.CreatedAt = overrides.CreatedAt
	}
	if overrides.UpdatedAt != "" {
		base.UpdatedAt = overrides.UpdatedAt
	}
	if overrides.RequestID != "" {
		base.RequestID = overrides.RequestID
	}
	return base
}

func (s *Store) ensureBooking(requestID string) Booking {
	existing, ok := s.bookings[PrimaryFixtureUID]
	if ok {
		return existing
	}
	created := fixtureBooking(requestID, Booking{})
	s.bookings[created.UID] = created
	return created
}
