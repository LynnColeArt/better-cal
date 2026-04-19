package booking

import (
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"
)

const (
	errCodeInvalidAttendeeEmail = "INVALID_ATTENDEE_EMAIL"
	errCodeInvalidEventType     = "INVALID_EVENT_TYPE"
	errCodeInvalidStartTime     = "INVALID_START_TIME"
	errCodeInvalidTimeZone      = "INVALID_TIME_ZONE"
	errCodeSecretField          = "SECRET_FIELD_NOT_ALLOWED"
	errCodeSlotUnavailable      = "SLOT_UNAVAILABLE"
)

type BookingValidator interface {
	ValidateCreate(CreateRequest) error
	ValidateCancel(CancelRequest) error
	ValidateReschedule(RescheduleRequest) error
}

type DefaultValidator struct{}

type ErrorWithCode struct {
	Code    string
	Message string
}

func (e *ErrorWithCode) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func ValidationFromError(err error) (*ErrorWithCode, bool) {
	var codedErr *ErrorWithCode
	if errors.As(err, &codedErr) {
		return codedErr, true
	}
	return nil, false
}

func (DefaultValidator) ValidateCreate(req CreateRequest) error {
	if req.EventTypeID != 0 && req.EventTypeID != FixtureEventTypeID {
		return validationError(errCodeInvalidEventType, "Event type is not available for booking")
	}
	if err := validateTimestamp(req.Start, errCodeInvalidStartTime); err != nil {
		return err
	}
	if err := validateAttendee(req.Attendee); err != nil {
		return err
	}
	if hasSecretField(req.Responses) || hasSecretField(req.Metadata) {
		return validationError(errCodeSecretField, "Secret-bearing fields are not allowed in booking responses or metadata")
	}
	return nil
}

func (DefaultValidator) ValidateCancel(CancelRequest) error {
	return nil
}

func (DefaultValidator) ValidateReschedule(req RescheduleRequest) error {
	return validateTimestamp(req.Start, errCodeInvalidStartTime)
}

func validateAttendee(attendee Attendee) error {
	if attendee.Email != "" {
		parsed, err := mail.ParseAddress(attendee.Email)
		if err != nil || parsed.Address != attendee.Email {
			return validationError(errCodeInvalidAttendeeEmail, "Attendee email is invalid")
		}
	}
	if attendee.TimeZone != "" && !validTimeZone(attendee.TimeZone) {
		return validationError(errCodeInvalidTimeZone, "Attendee time zone is invalid")
	}
	return nil
}

func validateTimestamp(value string, code string) error {
	if value == "" {
		return nil
	}
	if _, err := time.Parse(time.RFC3339Nano, value); err != nil {
		return validationError(code, "Start time must be an RFC3339 timestamp")
	}
	return nil
}

func validTimeZone(value string) bool {
	switch value {
	case "UTC", "Etc/UTC", FixtureTimeZone:
		return true
	}
	_, err := time.LoadLocation(value)
	return err == nil
}

func hasSecretField(value any) bool {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if secretEchoField(strings.ToLower(key)) || hasSecretField(child) {
				return true
			}
		}
	case []any:
		for _, child := range typed {
			if hasSecretField(child) {
				return true
			}
		}
	}
	return false
}

func secretEchoField(key string) bool {
	switch key {
	case "authorization", "apikey", "key", "plaintextkey", "clientsecret", "client_secret", "access_token",
		"accesstoken", "refresh_token", "refreshtoken", "credential", "credentials", "providertoken",
		"webhooksecret", "password", "newpassword", "currentpassword", "secret":
		return true
	default:
		return false
	}
}

func validationError(code string, message string) error {
	return &ErrorWithCode{Code: code, Message: message}
}
