package authz

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/LynnColeArt/better-cal/backend/internal/auth"
)

func TestAuthorizerAllowsRequiredPermission(t *testing.T) {
	authorizer := NewAuthorizer()
	decision := authorizer.Authorize(auth.Principal{
		Type:        "user",
		Permissions: []string{"booking:read"},
	}, PolicyBookingRead)

	if !decision.Allowed {
		t.Fatalf("decision = %+v", decision)
	}
}

func TestAuthorizerDeniesUnknownPolicy(t *testing.T) {
	authorizer := NewAuthorizer()
	decision := authorizer.Authorize(auth.Principal{
		Type:        "user",
		Permissions: []string{"booking:read", "booking:write"},
	}, Policy("policy.unknown"))

	if decision.Allowed {
		t.Fatal("unknown policy was allowed")
	}
}

func TestAuthorizerDeniesMissingPrincipalType(t *testing.T) {
	authorizer := NewAuthorizer()
	decision := authorizer.Authorize(auth.Principal{
		Permissions: []string{"booking:read"},
	}, PolicyBookingRead)

	if decision.Allowed {
		t.Fatal("principal without type was allowed")
	}
}

func TestAuthorizerDoesNotTreatReadAsWrite(t *testing.T) {
	authorizer := NewAuthorizer()
	decision := authorizer.Authorize(auth.Principal{
		Type:        "user",
		Permissions: []string{"booking:read"},
	}, PolicyBookingWrite)

	if decision.Allowed {
		t.Fatal("booking:read principal was allowed to write")
	}
}

func TestAuthorizerDoesNotTreatBookingWriteAsHostAction(t *testing.T) {
	authorizer := NewAuthorizer()
	decision := authorizer.Authorize(auth.Principal{
		Type:        "user",
		Permissions: []string{"booking:read", "booking:write"},
	}, PolicyBookingHostAction)

	if decision.Allowed {
		t.Fatal("booking:write principal was allowed to perform host action")
	}
}

func TestAuthorizeBookingAllowsOwnerReadAndWrite(t *testing.T) {
	authorizer := NewAuthorizer()
	principal := auth.Principal{
		ID:          123,
		Type:        "user",
		Permissions: []string{"booking:read", "booking:write"},
	}
	resource := BookingResource{OwnerUserID: 123, HostUserIDs: []int{123}}

	for _, policy := range []Policy{PolicyBookingRead, PolicyBookingWrite} {
		if decision := authorizer.AuthorizeBooking(principal, policy, resource); !decision.Allowed {
			t.Fatalf("%s decision = %+v", policy, decision)
		}
	}
}

func TestAuthorizeBookingDeniesPermissionedWrongOwner(t *testing.T) {
	authorizer := NewAuthorizer()
	decision := authorizer.AuthorizeBooking(auth.Principal{
		ID:          999,
		Type:        "user",
		Permissions: []string{"booking:read", "booking:write"},
	}, PolicyBookingWrite, BookingResource{OwnerUserID: 123, HostUserIDs: []int{123}})

	if decision.Allowed {
		t.Fatal("wrong owner with booking:write permission was allowed")
	}
}

func TestAuthorizeBookingHostActionRequiresHost(t *testing.T) {
	authorizer := NewAuthorizer()
	resource := BookingResource{OwnerUserID: 123, HostUserIDs: []int{123}}

	allowed := authorizer.AuthorizeBooking(auth.Principal{
		ID:          123,
		Type:        "user",
		Permissions: []string{"booking:host-action"},
	}, PolicyBookingHostAction, resource)
	if !allowed.Allowed {
		t.Fatalf("host decision = %+v", allowed)
	}

	denied := authorizer.AuthorizeBooking(auth.Principal{
		ID:          999,
		Type:        "user",
		Permissions: []string{"booking:host-action"},
	}, PolicyBookingHostAction, resource)
	if denied.Allowed {
		t.Fatal("non-host with booking:host-action permission was allowed")
	}
}

func TestPolicyConstantsExistInContractRegistry(t *testing.T) {
	raw, err := os.ReadFile("../../../contracts/registries/policies.json")
	if err != nil {
		t.Fatal(err)
	}
	var registry struct {
		Policies []struct {
			ID string `json:"id"`
		} `json:"policies"`
	}
	if err := json.Unmarshal(raw, &registry); err != nil {
		t.Fatal(err)
	}

	registered := make(map[string]bool, len(registry.Policies))
	for _, policy := range registry.Policies {
		registered[policy.ID] = true
	}

	for _, policy := range []Policy{
		PolicyMeRead,
		PolicyOAuth2Read,
		PolicyPlatformClientRead,
		PolicyBookingRead,
		PolicyBookingWrite,
		PolicyBookingHostAction,
		PolicySlotsRead,
	} {
		if !registered[string(policy)] {
			t.Fatalf("policy %q is missing from contracts/registries/policies.json", policy)
		}
	}
}
