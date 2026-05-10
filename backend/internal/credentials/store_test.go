package credentials

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/LynnColeArt/better-cal/backend/internal/integrations"
)

func TestStoreReadsFixtureCredentialMetadata(t *testing.T) {
	store := NewStore()

	items, err := store.ReadCredentialMetadata(context.Background(), fixtureUserID)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("credential metadata count = %d", len(items))
	}
	if items[0].CredentialRef != "google-calendar-credential-fixture" {
		t.Fatalf("credential ref = %q", items[0].CredentialRef)
	}
	if items[0].Provider != "google-calendar-fixture" {
		t.Fatalf("provider = %q", items[0].Provider)
	}
	if len(items[0].Scopes) != 2 {
		t.Fatalf("scopes = %#v", items[0].Scopes)
	}
}

func TestCredentialMetadataJSONDoesNotExposeSecrets(t *testing.T) {
	items := fixtureCredentialMetadata(fixtureUserID)
	raw, err := json.Marshal(items)
	if err != nil {
		t.Fatal(err)
	}
	body := strings.ToLower(string(raw))

	for _, forbidden := range []string{
		"secret",
		"token",
		"encrypted",
		"refresh",
		"access_token",
		"refresh_token",
		"providerpayload",
		"rawprovider",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("credential metadata exposed forbidden term %q: %s", forbidden, body)
		}
	}
}

func TestStoreRejectsInvalidCredentialMetadata(t *testing.T) {
	if err := ValidateCredentialMetadata(CredentialMetadata{}); !errors.Is(err, ErrInvalidCredentialMetadata) {
		t.Fatalf("err = %v", err)
	}
}

func TestStoreClonesCredentialScopes(t *testing.T) {
	store := NewStore()

	items, err := store.ReadCredentialMetadata(context.Background(), fixtureUserID)
	if err != nil {
		t.Fatal(err)
	}
	items[0].Scopes[0] = "mutated"

	items, err = store.ReadCredentialMetadata(context.Background(), fixtureUserID)
	if err != nil {
		t.Fatal(err)
	}
	if items[0].Scopes[0] == "mutated" {
		t.Fatal("credential scopes were mutated through read result")
	}
}

func TestStoreRefreshesCredentialStatusFromProvider(t *testing.T) {
	store := NewStore(WithStatusProvider(staticStatusProvider{
		snapshot: integrations.StatusSnapshot{
			Credentials: []integrations.CredentialStatus{
				{
					CredentialRef: "google-calendar-credential-fixture",
					Provider:      "google-calendar-fixture",
					AccountRef:    "google-account-fixture",
					Status:        "reauth_required",
					StatusCode:    "oauth_reauth_required",
				},
			},
		},
	}))

	if err := store.RefreshProviderStatus(context.Background(), fixtureUserID); err != nil {
		t.Fatal(err)
	}
	items, err := store.ReadCredentialMetadata(context.Background(), fixtureUserID)
	if err != nil {
		t.Fatal(err)
	}
	if items[0].Status != "reauth_required" {
		t.Fatalf("status = %q", items[0].Status)
	}
	if items[0].StatusCode != "oauth_reauth_required" {
		t.Fatalf("status code = %q", items[0].StatusCode)
	}
	if items[0].StatusCheckedAt == "" {
		t.Fatal("status checked timestamp was not set")
	}
}

func TestStoreRejectsUnknownCredentialStatusRefresh(t *testing.T) {
	store := NewStore(WithStatusProvider(staticStatusProvider{
		snapshot: integrations.StatusSnapshot{
			Credentials: []integrations.CredentialStatus{
				{
					CredentialRef: "missing-credential",
					Provider:      "google-calendar-fixture",
					AccountRef:    "google-account-fixture",
					Status:        "active",
				},
			},
		},
	}))

	err := store.RefreshProviderStatus(context.Background(), fixtureUserID)
	if !errors.Is(err, ErrInvalidCredentialStatusSnapshot) {
		t.Fatalf("err = %v", err)
	}
}

type staticStatusProvider struct {
	snapshot integrations.StatusSnapshot
}

func (p staticStatusProvider) ReadStatus(context.Context, integrations.StatusInput) (integrations.StatusSnapshot, error) {
	return p.snapshot, nil
}
