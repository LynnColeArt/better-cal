package integrations

import "context"

type StatusInput struct {
	UserID int
}

type StatusSnapshot struct {
	Credentials         []CredentialStatus
	CalendarConnections []CalendarConnectionStatus
}

type CredentialStatus struct {
	CredentialRef string
	Provider      string
	AccountRef    string
	Status        string
	StatusCode    string
}

type CalendarConnectionStatus struct {
	ConnectionRef string
	Provider      string
	AccountRef    string
	Status        string
	StatusCode    string
}

type StatusProviderAdapter interface {
	ReadStatus(context.Context, StatusInput) (StatusSnapshot, error)
}
