package booking

import (
	"context"
	"sync"
	"time"

	"github.com/LynnColeArt/better-cal/backend/internal/slots"
)

const (
	PrimaryFixtureUID             = "mock-booking-personal-basic"
	RescheduledFixtureUID         = "mock-booking-rescheduled"
	PendingConfirmFixtureUID      = "mock-booking-pending-confirm"
	PendingDeclineFixtureUID      = "mock-booking-pending-decline"
	FixtureOwnerUserID            = 123
	FixtureEventTypeID            = slots.FixtureEventTypeID
	FixtureBookingStart           = slots.FixtureSlotTime
	FixtureBookingEnd             = "2026-05-01T15:30:00.000Z"
	FixtureTimeZone               = slots.FixtureTimeZone
	FixtureSelectedCalendarRef    = "selected-calendar-fixture"
	FixtureDestinationCalendarRef = "destination-calendar-fixture"
)

type Attendee struct {
	ID       int    `json:"id,omitempty"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	TimeZone string `json:"timeZone"`
}

type Booking struct {
	UID                     string         `json:"uid"`
	ID                      int            `json:"id"`
	Title                   string         `json:"title"`
	Status                  string         `json:"status"`
	Start                   string         `json:"start"`
	End                     string         `json:"end"`
	EventTypeID             int            `json:"eventTypeId"`
	Attendees               []Attendee     `json:"attendees"`
	Responses               map[string]any `json:"responses"`
	Metadata                map[string]any `json:"metadata"`
	CreatedAt               string         `json:"createdAt"`
	UpdatedAt               string         `json:"updatedAt"`
	RequestID               string         `json:"requestId"`
	SelectedCalendarRef     string         `json:"-"`
	DestinationCalendarRef  string         `json:"-"`
	ExternalCalendarEventID string         `json:"-"`
	OwnerUserID             int            `json:"-"`
	HostUserIDs             []int          `json:"-"`
}

type CreateRequest struct {
	EventTypeID    int            `json:"eventTypeId"`
	Start          string         `json:"start"`
	Attendee       Attendee       `json:"attendee"`
	Responses      map[string]any `json:"responses"`
	Metadata       map[string]any `json:"metadata"`
	IdempotencyKey string         `json:"idempotencyKey"`
	OwnerUserID    int            `json:"-"`
	HostUserIDs    []int          `json:"-"`
}

type CancelRequest struct {
	CancellationReason string `json:"cancellationReason"`
}

type RescheduleRequest struct {
	Start              string `json:"start"`
	ReschedulingReason string `json:"reschedulingReason"`
}

type ConfirmRequest struct{}

type DeclineRequest struct {
	Reason string `json:"reason"`
}

type CancelResult struct {
	Booking     Booking
	SideEffects []string
}

type LifecycleResult struct {
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
	slotAvailability SlotAvailabilityPort
	sideEffects      SideEffectPort
	bookings         map[string]Booking
	idempotency      map[string]string
}

type StoreOption func(*Store)

func WithRepository(repo Repository) StoreOption {
	return func(s *Store) {
		s.repo = repo
	}
}

func WithSideEffectPort(port SideEffectPort) StoreOption {
	return func(s *Store) {
		if port != nil {
			s.sideEffects = port
		}
	}
}

func WithSlotAvailabilityPort(port SlotAvailabilityPort) StoreOption {
	return func(s *Store) {
		if port != nil {
			s.slotAvailability = port
		}
	}
}

func NewStore(opts ...StoreOption) *Store {
	store := &Store{
		bookingValidator: DefaultValidator{},
		slotAvailability: NewSlotServiceAvailabilityPort(slots.NewService()),
		sideEffects:      FixtureSideEffectPort{},
		bookings:         make(map[string]Booking),
		idempotency:      make(map[string]string),
	}
	for _, opt := range opts {
		opt(store)
	}
	return store
}

func NewStoreWithRepository(repo Repository, opts ...StoreOption) *Store {
	return NewStore(append([]StoreOption{WithRepository(repo)}, opts...)...)
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
		attendeeValue.TimeZone = FixtureTimeZone
	}
	attendeeValue.ID = 321

	start := req.Start
	if start == "" {
		start = FixtureBookingStart
	}
	eventTypeID := req.EventTypeID
	if eventTypeID == 0 {
		eventTypeID = FixtureEventTypeID
	}
	available, err := s.availabilityPort().IsSlotAvailable(ctx, SlotAvailabilityRequest{
		RequestID:   requestID,
		EventTypeID: eventTypeID,
		Start:       start,
		TimeZone:    attendeeValue.TimeZone,
	})
	if err != nil {
		return Booking{}, false, err
	}
	if !available {
		return Booking{}, false, validationError(errCodeSlotUnavailable, "Requested slot is unavailable")
	}
	end, err := endForStart(start)
	if err != nil {
		return Booking{}, false, err
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
		Start:       start,
		End:         end,
		OwnerUserID: req.OwnerUserID,
		HostUserIDs: req.HostUserIDs,
		Attendees: []Attendee{
			attendeeValue,
		},
		Responses: responses,
		Metadata:  metadata,
	})
	if s.repo != nil {
		persisted, duplicate, err := s.repo.SaveCreated(ctx, created, req.IdempotencyKey, nil)
		if err != nil {
			return Booking{}, false, err
		}
		if duplicate {
			s.bookings[persisted.UID] = persisted
			if req.IdempotencyKey != "" {
				s.idempotency[req.IdempotencyKey] = persisted.UID
			}
			return persisted, true, nil
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
	plannedSideEffects, err := s.sideEffectPort().PlanBookingCancelled(ctx, BookingCancelledSideEffect{
		Booking:            sideEffectSnapshot(cancelled),
		CancellationReason: req.CancellationReason,
	})
	if err != nil {
		return CancelResult{}, false, err
	}
	if s.repo != nil {
		if err := s.repo.Save(ctx, plannedSideEffects, cancelled); err != nil {
			return CancelResult{}, false, err
		}
	}
	s.bookings[uid] = cancelled

	return CancelResult{
		Booking:     cancelled,
		SideEffects: sideEffectNames(plannedSideEffects),
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
	eventTypeID := existing.EventTypeID
	if eventTypeID == 0 {
		eventTypeID = FixtureEventTypeID
	}
	timeZone := bookingTimeZone(existing)
	available, err := s.availabilityPort().IsSlotAvailable(ctx, SlotAvailabilityRequest{
		RequestID:   requestID,
		EventTypeID: eventTypeID,
		Start:       start,
		TimeZone:    timeZone,
	})
	if err != nil {
		return RescheduleResult{}, false, err
	}
	if !available {
		return RescheduleResult{}, false, validationError(errCodeSlotUnavailable, "Requested slot is unavailable")
	}
	end, err := endForStart(start)
	if err != nil {
		return RescheduleResult{}, false, err
	}
	newBooking := fixtureBooking(requestID, mergeBooking(existing, Booking{
		UID:                     RescheduledFixtureUID,
		Status:                  "accepted",
		Start:                   start,
		End:                     end,
		UpdatedAt:               "2026-01-01T00:10:00.000Z",
		ExternalCalendarEventID: fixtureExternalCalendarEventID(RescheduledFixtureUID),
	}))
	plannedSideEffects, err := s.sideEffectPort().PlanBookingRescheduled(ctx, BookingRescheduledSideEffect{
		OldBooking:         sideEffectSnapshot(oldBooking),
		NewBooking:         sideEffectSnapshot(newBooking),
		ReschedulingReason: req.ReschedulingReason,
	})
	if err != nil {
		return RescheduleResult{}, false, err
	}

	if s.repo != nil {
		if err := s.repo.Save(ctx, plannedSideEffects, oldBooking, newBooking); err != nil {
			return RescheduleResult{}, false, err
		}
	}
	s.bookings[oldUID] = oldBooking
	s.bookings[newBooking.UID] = newBooking

	return RescheduleResult{
		OldBooking:  oldBooking,
		NewBooking:  newBooking,
		SideEffects: sideEffectNames(plannedSideEffects),
	}, true, nil
}

func (s *Store) Confirm(ctx context.Context, requestID string, uid string, req ConfirmRequest) (LifecycleResult, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok, err := s.findLocked(ctx, requestID, uid)
	if err != nil {
		return LifecycleResult{}, false, err
	}
	if !ok {
		return LifecycleResult{}, false, nil
	}
	if err := s.validator().ValidateConfirm(req); err != nil {
		return LifecycleResult{}, false, err
	}
	if !confirmableStatus(existing.Status) {
		return LifecycleResult{}, false, validationError(errCodeInvalidBookingStatus, "Booking is not confirmable")
	}

	confirmed := fixtureBooking(requestID, mergeBooking(existing, Booking{
		Status:    "accepted",
		UpdatedAt: "2026-01-01T00:15:00.000Z",
	}))
	plannedSideEffects, err := s.sideEffectPort().PlanBookingConfirmed(ctx, BookingConfirmedSideEffect{
		Booking: sideEffectSnapshot(confirmed),
	})
	if err != nil {
		return LifecycleResult{}, false, err
	}
	if s.repo != nil {
		if err := s.repo.Save(ctx, plannedSideEffects, confirmed); err != nil {
			return LifecycleResult{}, false, err
		}
	}
	s.bookings[uid] = confirmed

	return LifecycleResult{
		Booking:     confirmed,
		SideEffects: sideEffectNames(plannedSideEffects),
	}, true, nil
}

func (s *Store) Decline(ctx context.Context, requestID string, uid string, req DeclineRequest) (LifecycleResult, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok, err := s.findLocked(ctx, requestID, uid)
	if err != nil {
		return LifecycleResult{}, false, err
	}
	if !ok {
		return LifecycleResult{}, false, nil
	}
	if err := s.validator().ValidateDecline(req); err != nil {
		return LifecycleResult{}, false, err
	}
	if !confirmableStatus(existing.Status) {
		return LifecycleResult{}, false, validationError(errCodeInvalidBookingStatus, "Booking is not declinable")
	}

	declined := fixtureBooking(requestID, mergeBooking(existing, Booking{
		Status:    "rejected",
		UpdatedAt: "2026-01-01T00:20:00.000Z",
	}))
	plannedSideEffects, err := s.sideEffectPort().PlanBookingDeclined(ctx, BookingDeclinedSideEffect{
		Booking: sideEffectSnapshot(declined),
		Reason:  req.Reason,
	})
	if err != nil {
		return LifecycleResult{}, false, err
	}
	if s.repo != nil {
		if err := s.repo.Save(ctx, plannedSideEffects, declined); err != nil {
			return LifecycleResult{}, false, err
		}
	}
	s.bookings[uid] = declined

	return LifecycleResult{
		Booking:     declined,
		SideEffects: sideEffectNames(plannedSideEffects),
	}, true, nil
}

func fixtureBooking(requestID string, overrides Booking) Booking {
	base := Booking{
		UID:         PrimaryFixtureUID,
		ID:          987,
		Title:       "Fixture Event",
		Status:      "accepted",
		Start:       FixtureBookingStart,
		End:         FixtureBookingEnd,
		EventTypeID: FixtureEventTypeID,
		Attendees: []Attendee{
			{
				ID:       321,
				Name:     "Fixture Attendee",
				Email:    "fixture-attendee@example.test",
				TimeZone: FixtureTimeZone,
			},
		},
		Responses: map[string]any{
			"name":  "Fixture Attendee",
			"email": "fixture-attendee@example.test",
		},
		Metadata: map[string]any{
			"fixture": "personal-basic",
		},
		CreatedAt:              "2026-01-01T00:00:00.000Z",
		UpdatedAt:              "2026-01-01T00:00:00.000Z",
		RequestID:              requestID,
		SelectedCalendarRef:    FixtureSelectedCalendarRef,
		DestinationCalendarRef: FixtureDestinationCalendarRef,
		OwnerUserID:            FixtureOwnerUserID,
		HostUserIDs:            []int{FixtureOwnerUserID},
	}
	merged := mergeBooking(base, overrides)
	if merged.SelectedCalendarRef == "" {
		merged.SelectedCalendarRef = FixtureSelectedCalendarRef
	}
	if merged.DestinationCalendarRef == "" {
		merged.DestinationCalendarRef = FixtureDestinationCalendarRef
	}
	if merged.ExternalCalendarEventID == "" {
		merged.ExternalCalendarEventID = fixtureExternalCalendarEventID(merged.UID)
	}
	return merged
}

func pendingFixtureBooking(requestID string, uid string) Booking {
	return fixtureBooking(requestID, Booking{
		UID:    uid,
		ID:     pendingFixtureID(uid),
		Status: "pending",
		Start:  "2026-05-03T15:00:00.000Z",
		End:    "2026-05-03T15:30:00.000Z",
		Metadata: map[string]any{
			"fixture": "pending-host-action",
		},
	})
}

func pendingFixtureID(uid string) int {
	if uid == PendingDeclineFixtureUID {
		return 989
	}
	return 988
}

func endForStart(start string) (string, error) {
	parsed, err := time.Parse(time.RFC3339Nano, start)
	if err != nil {
		return "", validationError(errCodeInvalidStartTime, "Start time must be an RFC3339 timestamp")
	}
	return parsed.Add(time.Duration(slots.FixtureDuration) * time.Minute).UTC().Format("2006-01-02T15:04:05.000Z"), nil
}

func bookingTimeZone(booking Booking) string {
	for _, attendee := range booking.Attendees {
		if attendee.TimeZone != "" {
			return attendee.TimeZone
		}
	}
	return FixtureTimeZone
}

func confirmableStatus(status string) bool {
	return status == "pending" || status == "awaiting_host"
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
	if overrides.SelectedCalendarRef != "" {
		base.SelectedCalendarRef = overrides.SelectedCalendarRef
	}
	if overrides.DestinationCalendarRef != "" {
		base.DestinationCalendarRef = overrides.DestinationCalendarRef
	}
	if overrides.ExternalCalendarEventID != "" {
		base.ExternalCalendarEventID = overrides.ExternalCalendarEventID
	}
	if overrides.OwnerUserID != 0 {
		base.OwnerUserID = overrides.OwnerUserID
	}
	if overrides.HostUserIDs != nil {
		base.HostUserIDs = overrides.HostUserIDs
	}
	return base
}

func fixtureExternalCalendarEventID(uid string) string {
	if uid == "" {
		return ""
	}
	return "google-event-" + uid
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

	var created Booking
	switch uid {
	case PrimaryFixtureUID:
		created = fixtureBooking(requestID, Booking{})
	case PendingConfirmFixtureUID, PendingDeclineFixtureUID:
		created = pendingFixtureBooking(requestID, uid)
	default:
		return Booking{}, false, nil
	}
	if s.repo != nil {
		if _, _, err := s.repo.SaveCreated(ctx, created, "", nil); err != nil {
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

func (s *Store) availabilityPort() SlotAvailabilityPort {
	if s.slotAvailability != nil {
		return s.slotAvailability
	}
	return NewSlotServiceAvailabilityPort(slots.NewService())
}

func (s *Store) sideEffectPort() SideEffectPort {
	if s.sideEffects != nil {
		return s.sideEffects
	}
	return FixtureSideEffectPort{}
}
