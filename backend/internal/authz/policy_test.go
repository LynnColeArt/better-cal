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
	} {
		if !registered[string(policy)] {
			t.Fatalf("policy %q is missing from contracts/registries/policies.json", policy)
		}
	}
}
