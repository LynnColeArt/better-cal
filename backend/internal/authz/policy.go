package authz

import "github.com/LynnColeArt/better-cal/backend/internal/auth"

type Policy string

const (
	PolicyMeRead                    Policy = "policy.me.read"
	PolicyOAuth2Read                Policy = "policy.oauth2.client.read"
	PolicyOAuth2TokenExchange       Policy = "policy.oauth2.token.exchange"
	PolicyPlatformClientRead        Policy = "policy.platform-client.read"
	PolicyBookingRead               Policy = "policy.booking.read"
	PolicyBookingWrite              Policy = "policy.booking.write"
	PolicyBookingHostAction         Policy = "policy.booking.host-action"
	PolicySlotsRead                 Policy = "policy.slots.read"
	PolicyCalendarConnectionsRead   Policy = "policy.calendar-connections.read"
	PolicyCalendarsRead             Policy = "policy.calendars.read"
	PolicyCredentialsRead           Policy = "policy.credentials.read"
	PolicySelectedCalendarsRead     Policy = "policy.selected-calendars.read"
	PolicySelectedCalendarsWrite    Policy = "policy.selected-calendars.write"
	PolicyDestinationCalendarsRead  Policy = "policy.destination-calendars.read"
	PolicyDestinationCalendarsWrite Policy = "policy.destination-calendars.write"
)

type Decision struct {
	Allowed bool
	Reason  string
}

type BookingResource struct {
	OwnerUserID int
	HostUserIDs []int
}

type Authorizer struct {
	requiredPermissions map[Policy][]string
}

func NewAuthorizer() *Authorizer {
	return &Authorizer{
		requiredPermissions: map[Policy][]string{
			PolicyMeRead:                    {"me:read"},
			PolicyOAuth2Read:                {"oauth-client:read"},
			PolicyOAuth2TokenExchange:       {"oauth-token:exchange"},
			PolicyPlatformClientRead:        {"platform-client:read"},
			PolicyBookingRead:               {"booking:read"},
			PolicyBookingWrite:              {"booking:write"},
			PolicyBookingHostAction:         {"booking:host-action"},
			PolicySlotsRead:                 {},
			PolicyCalendarConnectionsRead:   {"calendar-connections:read"},
			PolicyCalendarsRead:             {"calendars:read"},
			PolicyCredentialsRead:           {"credentials:read"},
			PolicySelectedCalendarsRead:     {"selected-calendars:read"},
			PolicySelectedCalendarsWrite:    {"selected-calendars:write"},
			PolicyDestinationCalendarsRead:  {"destination-calendars:read"},
			PolicyDestinationCalendarsWrite: {"destination-calendars:write"},
		},
	}
}

func (a *Authorizer) Authorize(principal auth.Principal, policy Policy) Decision {
	requiredPermissions, ok := a.requiredPermissions[policy]
	if !ok {
		return Decision{Allowed: false, Reason: "unknown policy"}
	}
	if principal.Type == "" {
		return Decision{Allowed: false, Reason: "missing principal type"}
	}
	if len(requiredPermissions) == 0 {
		return Decision{Allowed: true}
	}
	for _, permission := range requiredPermissions {
		if !hasPermission(principal.Permissions, permission) {
			return Decision{Allowed: false, Reason: "missing permission"}
		}
	}
	return Decision{Allowed: true}
}

func (a *Authorizer) AuthorizeBooking(principal auth.Principal, policy Policy, resource BookingResource) Decision {
	decision := a.Authorize(principal, policy)
	if !decision.Allowed {
		return decision
	}

	switch policy {
	case PolicyBookingRead, PolicyBookingWrite:
		if resource.OwnerUserID == 0 {
			return Decision{Allowed: false, Reason: "missing booking owner"}
		}
		if principal.ID == resource.OwnerUserID {
			return Decision{Allowed: true}
		}
		return Decision{Allowed: false, Reason: "principal does not own booking resource"}
	case PolicyBookingHostAction:
		if len(resource.HostUserIDs) == 0 {
			return Decision{Allowed: false, Reason: "missing booking hosts"}
		}
		for _, hostID := range resource.HostUserIDs {
			if principal.ID == hostID {
				return Decision{Allowed: true}
			}
		}
		return Decision{Allowed: false, Reason: "principal is not booking host"}
	default:
		return decision
	}
}

func hasPermission(permissions []string, required string) bool {
	for _, permission := range permissions {
		if permission == required {
			return true
		}
	}
	return false
}
