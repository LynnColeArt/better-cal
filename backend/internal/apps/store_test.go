package apps

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestStoreReadsFixtureAppCatalog(t *testing.T) {
	store := NewStore()

	items, err := store.ReadAppCatalog(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("app catalog count = %d", len(items))
	}
	if items[0].AppSlug != "google-calendar" {
		t.Fatalf("first app slug = %q", items[0].AppSlug)
	}
	if items[0].Provider != "google-calendar-fixture" {
		t.Fatalf("provider = %q", items[0].Provider)
	}
	if len(items[0].Capabilities) == 0 {
		t.Fatalf("capabilities = %#v", items[0].Capabilities)
	}
}

func TestAppCatalogJSONDoesNotExposeSecrets(t *testing.T) {
	items := fixtureAppCatalog()
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
		"credentialref",
		"providerpayload",
		"rawprovider",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("app catalog exposed forbidden term %q: %s", forbidden, body)
		}
	}
}

func TestStoreRejectsInvalidAppMetadata(t *testing.T) {
	if err := ValidateAppMetadata(AppMetadata{}); !errors.Is(err, ErrInvalidAppMetadata) {
		t.Fatalf("err = %v", err)
	}
}

func TestStoreClonesAppCapabilities(t *testing.T) {
	store := NewStore()

	items, err := store.ReadAppCatalog(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	items[0].Capabilities[0] = "mutated"

	items, err = store.ReadAppCatalog(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if items[0].Capabilities[0] == "mutated" {
		t.Fatal("app capabilities were mutated through read result")
	}
}
