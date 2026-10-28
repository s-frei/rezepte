package notify

import (
	"context"
	"errors"
	"net/http"
	"reflect"

	"github.com/danielgtaylor/huma/v2"

	"github.com/s-frei/rezepte/service/internal/auth"
	"github.com/s-frei/rezepte/service/internal/httpserver"
)

type preferencesBody struct {
	Kinds map[Kind]bool `json:"kinds" doc:"Every kind the account may have, true when it gets that mail"`
}

type preferencesOutput struct {
	Body preferencesBody
}

type setPreferencesInput struct {
	Body struct {
		Kinds map[Kind]bool `json:"kinds" doc:"The kinds to switch; kinds left out keep their value"`
	}
}

type linkQuery struct {
	U string `query:"u" doc:"The account the link was mailed to"`
	K string `query:"k" doc:"The kind of mail the link came in"`
	S string `query:"s" doc:"The link's signature"`
}

type unsubscribeOutput struct {
	Body unsubscribeState
}

type unsubscribeInput struct {
	Body struct {
		U          string `json:"u"`
		K          string `json:"k"`
		S          string `json:"s"`
		Scope      string `json:"scope" enum:"kind,group" doc:"kind switches only the link's kind, group every kind of its group the account may have"`
		Subscribed bool   `json:"subscribed" doc:"false unsubscribes, true undoes it"`
		Kinds      []Kind `json:"kinds,omitempty" doc:"With scope group, only these kinds of the group; omitted is all of them. An undo sends the kinds that were on before."`
	}
}

// oneClickInput has no RawBody: huma makes a RawBody required, and a sender
// may post an empty body as well as urlencoded or multipart form data
// (RFC 8058 asks for multipart). The signed address is the authorization;
// the body is never read. The fields repeat linkQuery's because huma left
// the query of an embedded linkQuery empty (TestOneClickOptsOutKindOnly).
type oneClickInput struct {
	U string `query:"u" doc:"The account the link was mailed to"`
	K string `query:"k" doc:"The kind of mail the link came in"`
	S string `query:"s" doc:"The link's signature"`
}

// gone is every unsubscribe route's answer for a link that does not work,
// without the schema link a stranger could not follow.
func gone(err error) error {
	if errors.Is(err, errLinkGone) {
		return httpserver.PublicError(http.StatusGone, errLinkGone.Error())
	}
	return err
}

// Register installs the session's own preferences under /auth/me and the
// unsubscribe routes a mailed link reaches. The unsubscribe routes declare
// no Security: the signed link is the whole authorization, and it reaches
// only the preferences of the account it names.
func Register(api huma.API, s *Service) {
	huma.Register(api, huma.Operation{
		OperationID: "get-own-notifications",
		Method:      http.MethodGet,
		Path:        "/api/v1/auth/me/notifications",
		Summary:     "Read which notification mails the current user gets",
		Tags:        []string{"auth"},
		Security:    auth.SessionSecurity,
		Errors:      []int{401},
	}, func(ctx context.Context, _ *struct{}) (*preferencesOutput, error) {
		u, ok := auth.UserFrom(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("authentication required")
		}
		prefs, err := s.Preferences(ctx, u)
		if err != nil {
			return nil, err
		}
		return &preferencesOutput{Body: preferencesBody{Kinds: prefs}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "set-own-notifications",
		Method:      http.MethodPut,
		Path:        "/api/v1/auth/me/notifications",
		Summary:     "Switch notification mails of the current user",
		Description: "Security notices are not kinds and cannot be switched off. A kind the account may not have is 422.",
		Tags:        []string{"auth"},
		Security:    auth.SessionSecurity,
		Errors:      []int{401, 422},
	}, func(ctx context.Context, in *setPreferencesInput) (*preferencesOutput, error) {
		u, ok := auth.UserFrom(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("authentication required")
		}
		prefs, err := s.SetPreferences(ctx, u, in.Body.Kinds)
		if errors.Is(err, ErrUnknownKind) {
			return nil, huma.Error422UnprocessableEntity(err.Error())
		}
		if err != nil {
			return nil, err
		}
		return &preferencesOutput{Body: preferencesBody{Kinds: prefs}}, nil
	})

	// Inline rather than a $ref, as in share.RegisterPublic: a $ref response
	// gets a "$schema" key and a Link header to a schema route that needs a
	// session.
	state := api.OpenAPI().Components.Schemas.Schema(reflect.TypeFor[unsubscribeState](), false, "")
	stateResponses := map[string]*huma.Response{
		"200": {Content: map[string]*huma.MediaType{"application/json": {Schema: state}}},
	}

	huma.Register(api, huma.Operation{
		OperationID: "get-unsubscribe",
		Method:      http.MethodGet,
		Path:        "/api/v1/unsubscribe",
		Summary:     "Read what an unsubscribe link reaches",
		Description: "No session needed; changes nothing. A forged or expired link, or one naming an account or kind that is gone, answers 410.",
		Tags:        []string{"public"},
		Errors:      []int{410},
		Responses:   stateResponses,
	}, func(ctx context.Context, in *linkQuery) (*unsubscribeOutput, error) {
		u, k, err := s.resolve(ctx, in.U, in.K, in.S)
		if err != nil {
			return nil, gone(err)
		}
		st, err := s.state(ctx, u, k)
		if err != nil {
			return nil, err
		}
		return &unsubscribeOutput{Body: st}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "unsubscribe",
		Method:      http.MethodPost,
		Path:        "/api/v1/unsubscribe",
		Summary:     "Unsubscribe with a mailed link, or undo it",
		Description: "No session needed. Takes the link's u, k and s. Answers 410 like get-unsubscribe.",
		Tags:        []string{"public"},
		Errors:      []int{410, 422},
		Responses:   stateResponses,
	}, func(ctx context.Context, in *unsubscribeInput) (*unsubscribeOutput, error) {
		u, k, err := s.resolve(ctx, in.Body.U, in.Body.K, in.Body.S)
		if err != nil {
			return nil, gone(err)
		}
		err = s.switchByLink(ctx, u, k, in.Body.Scope == "group", in.Body.Kinds, in.Body.Subscribed)
		if errors.Is(err, ErrUnknownKind) {
			return nil, huma.Error422UnprocessableEntity("kinds: only kinds of the link's group")
		}
		if err != nil {
			return nil, err
		}
		st, err := s.state(ctx, u, k)
		if err != nil {
			return nil, err
		}
		return &unsubscribeOutput{Body: st}, nil
	})

	// POST only: a link scanner that follows the List-Unsubscribe address
	// with GET must not unsubscribe anybody.
	huma.Register(api, huma.Operation{
		OperationID:   "unsubscribe-one-click",
		Method:        http.MethodPost,
		Path:          "/api/v1/unsubscribe/one-click",
		Summary:       "Unsubscribe from one kind of mail in one click (RFC 8058)",
		Description:   "The address of a mail's List-Unsubscribe header. No session needed. The body, List-Unsubscribe=One-Click as urlencoded or multipart form data, is not read: the signed address alone decides. Switches off the link's kind only and answers 200 with no body, or 410 like get-unsubscribe.",
		Tags:          []string{"public"},
		DefaultStatus: http.StatusOK,
		Errors:        []int{410},
	}, func(ctx context.Context, in *oneClickInput) (*struct{}, error) {
		u, k, err := s.resolve(ctx, in.U, in.K, in.S)
		if err != nil {
			return nil, gone(err)
		}
		return nil, s.switchByLink(ctx, u, k, false, nil, false)
	})
}
