// Package tokenapi exposes API token management (admin only) over HTTP. It is
// a separate package for the same reason as userapi: it needs auth.UserFrom
// and auth.SessionSecurity, and package auth already imports package user.
//
// Every operation here declares auth.SessionSecurity, so no API token can
// reach them - a token able to mint tokens would defeat both expiry and
// revocation.
package tokenapi

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/s-frei/rezepte/service/internal/auth"
	"github.com/s-frei/rezepte/service/internal/user"
)

// APIToken is a token as the API shows it. It never carries the secret.
type APIToken struct {
	ID         string     `json:"id" doc:"Token id"`
	Name       string     `json:"name" doc:"Label the admin gave it"`
	Prefix     string     `json:"prefix" doc:"First characters of the token, for matching it against a client configuration"`
	Scopes     []string   `json:"scopes" doc:"What the token may do"`
	OwnerID    string     `json:"ownerId" doc:"User the token acts as"`
	OwnerName  string     `json:"ownerName" doc:"Login name of that user"`
	CreatedAt  time.Time  `json:"createdAt"`
	ExpiresAt  *time.Time `json:"expiresAt,omitempty" doc:"Absent when the token never expires"`
	LastUsedAt *time.Time `json:"lastUsedAt,omitempty" doc:"Accurate to the hour; absent when never used"`
}

// APITokenList is the response body of list-api-tokens.
type APITokenList struct {
	Items []APIToken `json:"items"`
}

// CreatedAPIToken is the response to create-api-token and nothing else. The
// raw value lives on this type alone, so it cannot leak into the list.
type CreatedAPIToken struct {
	Token string `json:"token" doc:"The token itself. Shown once and never recoverable."`
	APIToken
}

type listOutput struct {
	Body APITokenList
}

type createInput struct {
	Body struct {
		Name   string   `json:"name" minLength:"1" maxLength:"64"`
		Scopes []string `json:"scopes" minItems:"1" uniqueItems:"true" enum:"recipes:read,recipes:write,users:read,users:write"`
		// ExpiresInDays keeps the server as the clock: a client cannot set an
		// expiry in the past or centuries out. Absent means no expiry.
		ExpiresInDays *int `json:"expiresInDays,omitempty" enum:"30,90,365"`
	}
}

type createOutput struct {
	Body CreatedAPIToken
}

type deleteInput struct {
	ID string `path:"id"`
}

type deleteOutput struct{}

// requireAdmin returns the calling user or a 401/403 huma error.
func requireAdmin(ctx context.Context) (user.User, error) {
	u, ok := auth.UserFrom(ctx)
	if !ok {
		return user.User{}, huma.Error401Unauthorized("authentication required")
	}
	if !canManageAllTokens(u) {
		return user.User{}, huma.Error403Forbidden("admin role required")
	}
	return u, nil
}

// canManageAllTokens reports whether u may see and revoke every token in the
// instance, not only their own. Today that is every admin, the instance owner
// included - IsAdmin rather than a comparison against RoleAdmin, which would
// exclude the owner from the tokens on their own instance. Narrowing this to
// the owner alone is a change to the token feature rather than to the rank
// rules: it needs the list scoped by owner and the revoke checked against it,
// neither of which exists yet.
func canManageAllTokens(u user.User) bool { return u.Role.IsAdmin() }

func toResponse(t auth.Token) APIToken {
	return APIToken{
		ID:         t.ID,
		Name:       t.Name,
		Prefix:     t.Prefix,
		Scopes:     t.Scopes,
		OwnerID:    t.OwnerID,
		OwnerName:  t.OwnerName,
		CreatedAt:  t.CreatedAt,
		ExpiresAt:  t.ExpiresAt,
		LastUsedAt: t.LastUsedAt,
	}
}

// Register installs list, create and delete for API tokens. All three require
// an admin session; see the package comment for why a token cannot use them.
func Register(api huma.API, tokens *auth.TokenService) {
	huma.Register(api, huma.Operation{
		OperationID: "list-api-tokens",
		Method:      http.MethodGet,
		Path:        "/api/v1/tokens",
		Summary:     "List API tokens",
		Tags:        []string{"tokens"},
		Security:    auth.SessionSecurity,
		Errors:      []int{401, 403},
	}, func(ctx context.Context, _ *struct{}) (*listOutput, error) {
		if _, err := requireAdmin(ctx); err != nil {
			return nil, err
		}
		list, err := tokens.List(ctx)
		if err != nil {
			return nil, err
		}
		items := make([]APIToken, 0, len(list))
		for _, t := range list {
			items = append(items, toResponse(t))
		}
		return &listOutput{Body: APITokenList{Items: items}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "create-api-token",
		Method:        http.MethodPost,
		Path:          "/api/v1/tokens",
		Summary:       "Create an API token",
		Description:   "The raw token is returned once and cannot be retrieved afterwards.",
		Tags:          []string{"tokens"},
		Security:      auth.SessionSecurity,
		DefaultStatus: http.StatusCreated,
		Errors:        []int{401, 403, 422},
	}, func(ctx context.Context, in *createInput) (*createOutput, error) {
		u, err := requireAdmin(ctx)
		if err != nil {
			return nil, err
		}
		var expires *time.Time
		if in.Body.ExpiresInDays != nil {
			at := time.Now().AddDate(0, 0, *in.Body.ExpiresInDays)
			expires = &at
		}
		raw, tok, err := tokens.Create(ctx, u.ID, in.Body.Name, in.Body.Scopes, expires)
		if errors.Is(err, auth.ErrInvalidScope) {
			return nil, huma.Error422UnprocessableEntity(err.Error())
		}
		if err != nil {
			return nil, err
		}
		return &createOutput{Body: CreatedAPIToken{Token: raw, APIToken: toResponse(tok)}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "delete-api-token",
		Method:        http.MethodDelete,
		Path:          "/api/v1/tokens/{id}",
		Summary:       "Revoke an API token",
		Tags:          []string{"tokens"},
		Security:      auth.SessionSecurity,
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{401, 403, 404},
	}, func(ctx context.Context, in *deleteInput) (*deleteOutput, error) {
		if _, err := requireAdmin(ctx); err != nil {
			return nil, err
		}
		if err := tokens.Delete(ctx, in.ID); errors.Is(err, auth.ErrNoToken) {
			return nil, huma.Error404NotFound("token not found")
		} else if err != nil {
			return nil, err
		}
		return &deleteOutput{}, nil
	})
}
