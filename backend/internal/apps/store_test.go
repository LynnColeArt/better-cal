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

func TestStoreCreatesInstallIntentForCatalogApp(t *testing.T) {
	store := NewStore()

	intent, err := store.CreateInstallIntent(context.Background(), 123, "google-calendar")
	if err != nil {
		t.Fatal(err)
	}
	if intent.InstallIntentRef == "" {
		t.Fatal("install intent ref was empty")
	}
	if intent.AppSlug != "google-calendar" {
		t.Fatalf("app slug = %q", intent.AppSlug)
	}
	if intent.UserID != 123 {
		t.Fatalf("user id = %d", intent.UserID)
	}
	if intent.Status != InstallIntentStatusPending {
		t.Fatalf("status = %q", intent.Status)
	}
	if intent.CreatedAt == "" || intent.UpdatedAt == "" {
		t.Fatalf("timestamps were not populated: %#v", intent)
	}
}

func TestStoreRejectsInstallIntentForUnknownApp(t *testing.T) {
	store := NewStore()

	if _, err := store.CreateInstallIntent(context.Background(), 123, "unknown-app"); !errors.Is(err, ErrAppNotFound) {
		t.Fatalf("err = %v", err)
	}
}

func TestStoreRejectsInvalidInstallIntentInputs(t *testing.T) {
	store := NewStore()

	if _, err := store.CreateInstallIntent(context.Background(), 0, "google-calendar"); !errors.Is(err, ErrInvalidInstallIntent) {
		t.Fatalf("zero user err = %v", err)
	}
	if _, err := store.CreateInstallIntent(context.Background(), 123, " "); !errors.Is(err, ErrInvalidInstallIntent) {
		t.Fatalf("blank app slug err = %v", err)
	}
	if err := ValidateInstallIntent(AppInstallIntent{}); !errors.Is(err, ErrInvalidInstallIntent) {
		t.Fatalf("validate err = %v", err)
	}
}

func TestAppInstallIntentJSONDoesNotExposeSecrets(t *testing.T) {
	intent := AppInstallIntent{
		InstallIntentRef: "app-intent-fixture",
		UserID:           123,
		AppSlug:          "google-calendar",
		Status:           InstallIntentStatusPending,
		CreatedAt:        "2026-01-01T00:00:00.000Z",
		UpdatedAt:        "2026-01-01T00:00:00.000Z",
	}
	raw, err := json.Marshal(intent)
	if err != nil {
		t.Fatal(err)
	}
	body := strings.ToLower(string(raw))
	if strings.Contains(body, "userid") || strings.Contains(body, "user_id") {
		t.Fatalf("install intent exposed internal user id: %s", body)
	}

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
		"accountref",
		"accountlabel",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("install intent exposed forbidden term %q: %s", forbidden, body)
		}
	}
}
