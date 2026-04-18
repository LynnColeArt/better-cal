package booking

import (
	"context"
	"sync"
)

const (
	PrimaryFixtureUID     = "mock-booking-personal-basic"
	RescheduledFixtureUID = "mock-booking-rescheduled"
	FixtureEventTypeID    = 1001
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
	mu               sync.Mutex
	repo             Repository
	bookingValidator BookingValidator
	bookings         map[string]Booking
	idempotency      map[string]string
}

func NewStore() *Store {
	return &Store{
		bookingValidator: DefaultValidator{},
		bookings:         make(map[string]Booking),
		idempotency:      make(map[string]string),
	}
}

func NewStoreWithRepository(repo Repository) *Store {
	store := NewStore()
	store.repo = repo
	return store
}

func (s *Store) Create(ctx context.Context, requestID string, req CreateRequest) (Booking, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if req.IdempotencyKey != "" {
		uid, ok := s.idempotency[req.IdempotencyKey]
		if ok {
			bookingValue, ok := s.bookings[uid]
			if ok {
				return bookingValue, true, nil
			}
		}
		if s.repo != nil {
			bookingValue, ok, err := s.repo.ReadByIdempotencyKey(ctx, req.IdempotencyKey)
			if err != nil {
				return Booking{}, false, err
			}
			if ok {
				s.bookings[bookingValue.UID] = bookingValue
				s.idempotency[req.IdempotencyKey] = bookingValue.UID
				return bookingValue, true, nil
			}
		}
	}
	if err := s.validator().ValidateCreate(req); err != nil {
		return Booking{}, false, err
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
	if s.repo != nil {
		if err := s.repo.SaveCreated(ctx, created, req.IdempotencyKey); err != nil {
			return Booking{}, false, err
		}
	}

	s.bookings[created.UID] = created
	if req.IdempotencyKey != "" {
		s.idempotency[req.IdempotencyKey] = created.UID
	}

	return created, false, nil
}

func (s *Store) Read(ctx context.Context, requestID string, uid string) (Booking, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.findLocked(ctx, requestID, uid)
}

func (s *Store) Cancel(ctx context.Context, requestID string, uid string, req CancelRequest) (CancelResult, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok, err := s.findLocked(ctx, requestID, uid)
	if err != nil {
		return CancelResult{}, false, err
	}
	if !ok {
		return CancelResult{}, false, nil
	}
	if err := s.validator().ValidateCancel(req); err != nil {
		return CancelResult{}, false, err
	}

	cancelled := fixtureBooking(requestID, mergeBooking(existing, Booking{
		Status:    "cancelled",
		UpdatedAt: "2026-01-01T00:05:00.000Z",
	}))
	if s.repo != nil {
		if err := s.repo.Save(ctx, cancelled); err != nil {
			return CancelResult{}, false, err
		}
	}
	s.bookings[uid] = cancelled

	return CancelResult{
		Booking: cancelled,
		SideEffects: []string{
			"calendar.cancelled",
			"email.cancelled",
			"webhook.booking.cancelled",
		},
	}, true, nil
}

func (s *Store) Reschedule(ctx context.Context, requestID string, oldUID string, req RescheduleRequest) (RescheduleResult, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok, err := s.findLocked(ctx, requestID, oldUID)
	if err != nil {
		return RescheduleResult{}, false, err
	}
	if !ok {
		return RescheduleResult{}, false, nil
	}
	if err := s.validator().ValidateReschedule(req); err != nil {
		return RescheduleResult{}, false, err
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

	if s.repo != nil {
		if err := s.repo.Save(ctx, oldBooking, newBooking); err != nil {
			return RescheduleResult{}, false, err
		}
	}
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
	}, true, nil
}

func fixtureBooking(requestID string, overrides Booking) Booking {
	base := Booking{
		UID:         PrimaryFixtureUID,
		ID:          987,
		Title:       "Fixture Event",
		Status:      "accepted",
		Start:       "2026-05-01T15:00:00.000Z",
		End:         "2026-05-01T15:30:00.000Z",
		EventTypeID: FixtureEventTypeID,
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

func (s *Store) findLocked(ctx context.Context, requestID string, uid string) (Booking, bool, error) {
	existing, ok := s.bookings[uid]
	if ok {
		return existing, true, nil
	}

	if s.repo != nil {
		bookingValue, ok, err := s.repo.ReadByUID(ctx, uid)
		if err != nil {
			return Booking{}, false, err
		}
		if ok {
			s.bookings[bookingValue.UID] = bookingValue
			return bookingValue, true, nil
		}
	}

	if uid != PrimaryFixtureUID {
		return Booking{}, false, nil
	}

	created := fixtureBooking(requestID, Booking{})
	if s.repo != nil {
		if err := s.repo.SaveCreated(ctx, created, ""); err != nil {
			return Booking{}, false, err
		}
	}
	s.bookings[created.UID] = created
	return created, true, nil
}

func (s *Store) validator() BookingValidator {
	if s.bookingValidator != nil {
		return s.bookingValidator
	}
	return DefaultValidator{}
}
