package authz

import "github.com/LynnColeArt/better-cal/backend/internal/auth"

type Policy string

const (
	PolicyMeRead             Policy = "policy.me.read"
	PolicyOAuth2Read         Policy = "policy.oauth2.client.read"
	PolicyPlatformClientRead Policy = "policy.platform-client.read"
	PolicyBookingRead        Policy = "policy.booking.read"
	PolicyBookingWrite       Policy = "policy.booking.write"
)

type Decision struct {
	Allowed bool
	Reason  string
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

func hasPermission(permissions []string, required string) bool {
	for _, permission := range permissions {
		if permission == required {
			return true
		}
	}
	return false
}
