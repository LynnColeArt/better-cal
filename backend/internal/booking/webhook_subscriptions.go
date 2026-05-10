package booking

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
)

const webhookSignatureHeaderName = "X-Cal-Signature-256"

var ErrEmptyWebhookSigningKeyRef = errors.New("empty webhook signing key ref")
var ErrEmptyWebhookSigningSecret = errors.New("empty webhook signing secret")

type WebhookSubscription struct {
	ID            int64
	SubscriberURL string
	TriggerEvent  WebhookTriggerEvent
	SigningKeyRef string
	Active        bool
}

type WebhookSubscriptionStore interface {
	ReadWebhookSubscriptionsByTrigger(context.Context, WebhookTriggerEvent) ([]WebhookSubscription, error)
}

type WebhookSubscriptionWriter interface {
	SaveWebhookSubscription(context.Context, WebhookSubscription) error
}

type WebhookSigningSecretResolver interface {
	ResolveWebhookSigningSecret(context.Context, string) (string, bool, error)
}

type FixtureWebhookSigningSecretResolver struct {
	secrets map[string]string
}

func NewFixtureWebhookSigningSecretResolver(secrets map[string]string) FixtureWebhookSigningSecretResolver {
	cloned := map[string]string{}
	for keyRef, secret := range secrets {
		cloned[keyRef] = secret
	}
	return FixtureWebhookSigningSecretResolver{secrets: cloned}
}

func (r FixtureWebhookSigningSecretResolver) ResolveWebhookSigningSecret(_ context.Context, keyRef string) (string, bool, error) {
	if keyRef == "" {
		return "", false, ErrEmptyWebhookSigningKeyRef
	}
	secret, ok := r.secrets[keyRef]
	if !ok {
		return "", false, nil
	}
	if secret == "" {
		return "", false, ErrEmptyWebhookSigningSecret
	}
	return secret, true, nil
}

func FixtureWebhookSubscriptions(subscriberURL string, signingKeyRef string) []WebhookSubscription {
	if subscriberURL == "" || signingKeyRef == "" {
		return nil
	}

	triggers := []WebhookTriggerEvent{
		WebhookTriggerBookingCancelled,
		WebhookTriggerBookingRescheduled,
		WebhookTriggerBookingConfirmed,
		WebhookTriggerBookingRejected,
	}
	subscriptions := make([]WebhookSubscription, 0, len(triggers))
	for _, trigger := range triggers {
		subscriptions = append(subscriptions, WebhookSubscription{
			SubscriberURL: subscriberURL,
			TriggerEvent:  trigger,
			SigningKeyRef: signingKeyRef,
			Active:        true,
		})
	}
	return subscriptions
}

func SeedWebhookSubscriptions(ctx context.Context, repo WebhookSubscriptionWriter, subscriptions []WebhookSubscription) error {
	if repo == nil {
		return errors.New("nil webhook subscription writer")
	}
	for _, subscription := range subscriptions {
		if err := repo.SaveWebhookSubscription(ctx, subscription); err != nil {
			return err
		}
	}
	return nil
}

func signWebhookBody(body string, secret string) (string, error) {
	if secret == "" {
		return "", ErrEmptyWebhookSigningSecret
	}
	mac := hmac.New(sha256.New, []byte(secret))
	if _, err := mac.Write([]byte(body)); err != nil {
		return "", fmt.Errorf("sign webhook body: %w", err)
	}
	return "sha256=" + hex.EncodeToString(mac.Sum(nil)), nil
}
