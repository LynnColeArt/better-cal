package authz

import "github.com/LynnColeArt/better-cal/backend/internal/auth"

type Policy string

const (
	PolicyMeRead             Policy = "policy.me.read"
	PolicyOAuth2Read         Policy = "policy.oauth2.client.read"
	PolicyPlatformClientRead Policy = "policy.platform-client.read"
	PolicyBookingRead        Policy = "policy.booking.read"
	PolicyBookingWrite       Policy = "policy.booking.write"
	PolicyBookingHostAction  Policy = "policy.booking.host-action"
	PolicySlotsRead          Policy = "policy.slots.read"
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
			PolicyMeRead:             {"me:read"},
			PolicyOAuth2Read:         {"oauth-client:read"},
			PolicyPlatformClientRead: {"platform-client:read"},
			PolicyBookingRead:        {"booking:read"},
			PolicyBookingWrite:       {"booking:write"},
			PolicyBookingHostAction:  {"booking:host-action"},
			PolicySlotsRead:          {},
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
