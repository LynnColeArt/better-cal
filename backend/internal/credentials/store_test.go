package credentials

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
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
