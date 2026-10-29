package push

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/s-frei/rezepte/service/internal/auth"
	"github.com/s-frei/rezepte/service/internal/mail"
)

type keyOutput struct {
	Body struct {
		Key string `json:"key" doc:"The VAPID public key, base64url, to subscribe with as applicationServerKey"`
	}
}

type subscribeInput struct {
	Body struct {
		Endpoint string `json:"endpoint" maxLength:"1024" doc:"The push service URL PushManager.subscribe returned; https on a known push service only"`
		P256dh   string `json:"p256dh" maxLength:"128" doc:"The subscription's p256dh key, base64url"`
		Auth     string `json:"auth" maxLength:"64" doc:"The subscription's auth secret, base64url"`
	}
}

type endpointInput struct {
	Body struct {
		Endpoint string `json:"endpoint" maxLength:"1024"`
	}
}

type alarmInput struct {
	Body struct {
		Endpoint string    `json:"endpoint" maxLength:"1024" doc:"A subscription of the current user"`
		FireAt   time.Time `json:"fireAt" doc:"When to send, at most 7 days ahead and at most a minute past"`
		Title    string    `json:"title" minLength:"1" maxLength:"120"`
		Body     string    `json:"body" maxLength:"200"`
		URL      string    `json:"url" maxLength:"300" doc:"The page a click opens; starts with /recipes/"`
	}
}

type alarmOutput struct {
	Body struct {
		ID string `json:"id" doc:"The alarm's id, which the push carries as its tag"`
	}
}

type cancelInput struct {
	ID string `path:"id" maxLength:"64"`
}

type noContent struct{}

func decodeKey(s string) []byte {
	b, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(s, "="))
	if err != nil {
		return nil
	}
	return b
}

// problem maps the service's errors to responses.
func problem(err error) error {
	switch {
	case errors.Is(err, ErrForeignEndpoint), errors.Is(err, ErrInvalid), errors.Is(err, ErrTooMany):
		return huma.Error422UnprocessableEntity(err.Error())
	case errors.Is(err, ErrUnknownSubscription):
		return huma.Error404NotFound(err.Error())
	case errors.Is(err, ErrTooSoon):
		return huma.Error429TooManyRequests(err.Error())
	}
	return err
}

// Register installs the push routes, all for the session's own devices.
func Register(api huma.API, s *Service) {
	huma.Register(api, huma.Operation{
		OperationID: "get-push-key",
		Method:      http.MethodGet,
		Path:        "/api/v1/push/key",
		Summary:     "Read the key a device subscribes to pushes with",
		Tags:        []string{"push"},
		Security:    auth.SessionSecurity,
		Errors:      []int{401},
	}, func(ctx context.Context, _ *struct{}) (*keyOutput, error) {
		if _, err := auth.CurrentUser(ctx); err != nil {
			return nil, err
		}
		k, err := s.PublicKey(ctx)
		if err != nil {
			return nil, err
		}
		out := &keyOutput{}
		out.Body.Key = k
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "put-push-subscription",
		Method:        http.MethodPut,
		Path:          "/api/v1/push/subscription",
		Summary:       "Store a device's push subscription",
		Description:   "The subscription belongs to the calling session and ends with it: signing out, a password change or a reset stops the device's pushes. An endpoint another account had moves to the current user, without that account's alarms. At most 20 per user; an endpoint that is not https on a known push service is 422.",
		Tags:          []string{"push"},
		Security:      auth.SessionSecurity,
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{401, 422},
	}, func(ctx context.Context, in *subscribeInput) (*noContent, error) {
		u, err := auth.CurrentUser(ctx)
		if err != nil {
			return nil, err
		}
		sessionID, ok := auth.SessionIDFrom(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("authentication required")
		}
		sub := Subscription{Endpoint: in.Body.Endpoint, P256dh: decodeKey(in.Body.P256dh), Auth: decodeKey(in.Body.Auth)}
		if err := s.Subscribe(ctx, u.ID, sessionID, sub); err != nil {
			return nil, problem(err)
		}
		return &noContent{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "delete-push-subscription",
		Method:        http.MethodDelete,
		Path:          "/api/v1/push/subscription",
		Summary:       "Delete a device's push subscription and its alarms",
		Description:   "One that is not there is no error.",
		Tags:          []string{"push"},
		Security:      auth.SessionSecurity,
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{401},
	}, func(ctx context.Context, in *endpointInput) (*noContent, error) {
		u, err := auth.CurrentUser(ctx)
		if err != nil {
			return nil, err
		}
		if err := s.Unsubscribe(ctx, u.ID, in.Body.Endpoint); err != nil {
			return nil, err
		}
		return &noContent{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "create-push-alarm",
		Method:      http.MethodPost,
		Path:        "/api/v1/push/alarms",
		Summary:     "Schedule a push to a device",
		Description: "The push is sent at fireAt to a subscription of the current user, which is 404 otherwise. At most 50 per subscription. Sent once, never retried; one more than an hour late is dropped.",
		Tags:        []string{"push"},
		Security:    auth.SessionSecurity,
		Errors:      []int{401, 404, 422},
	}, func(ctx context.Context, in *alarmInput) (*alarmOutput, error) {
		u, err := auth.CurrentUser(ctx)
		if err != nil {
			return nil, err
		}
		b := in.Body
		id, err := s.Schedule(ctx, u.ID, Alarm{Endpoint: b.Endpoint, FireAt: b.FireAt, Title: b.Title, Body: b.Body, URL: b.URL})
		if err != nil {
			return nil, problem(err)
		}
		out := &alarmOutput{}
		out.Body.ID = id
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "delete-push-alarm",
		Method:        http.MethodDelete,
		Path:          "/api/v1/push/alarms/{id}",
		Summary:       "Cancel a scheduled push",
		Description:   "One already sent, another user's or never there is no error and changes nothing.",
		Tags:          []string{"push"},
		Security:      auth.SessionSecurity,
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{401},
	}, func(ctx context.Context, in *cancelInput) (*noContent, error) {
		u, err := auth.CurrentUser(ctx)
		if err != nil {
			return nil, err
		}
		if err := s.Cancel(ctx, u.ID, in.ID); err != nil {
			return nil, err
		}
		return &noContent{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "create-push-test",
		Method:        http.MethodPost,
		Path:          "/api/v1/push/test",
		Summary:       "Send a test push to a device",
		Description:   "Pushes a test notification in the current user's language to one of their subscriptions, which is 404 otherwise. At most one every 10 seconds per subscription, 429 otherwise.",
		Tags:          []string{"push"},
		Security:      auth.SessionSecurity,
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{401, 404, 429},
	}, func(ctx context.Context, in *endpointInput) (*noContent, error) {
		u, err := auth.CurrentUser(ctx)
		if err != nil {
			return nil, err
		}
		m := Message{Kind: "test", URL: "/settings/notifications", Tag: "test"}
		if m.Title, err = mail.Text(u.Locale, "mail_push_test_title", nil); err != nil {
			return nil, err
		}
		if m.Body, err = mail.Text(u.Locale, "mail_push_test_body", nil); err != nil {
			return nil, err
		}
		if err := s.SendTest(ctx, u.ID, in.Body.Endpoint, m); err != nil {
			return nil, problem(err)
		}
		return &noContent{}, nil
	})
}
