package image

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/s-frei/rezepte/service/internal/sign"
	"github.com/s-frei/rezepte/service/internal/user"
)

// ErrNoPublicURL is Issue's answer when the instance does not know its own
// address: a link is never built from a request's Host header.
var ErrNoPublicURL = errors.New("REZEPTE_PUBLIC_URL is not set")

// linkTTL is how long an upload link works. It may be used for several
// photos within that time.
const linkTTL = 15 * time.Minute

// UploadLinks issues and checks the links an MCP client uploads local
// photos to, so the bytes never pass through the model and the model never
// sees an API token. A link binds one recipe and one user.
type UploadLinks struct {
	signer    *sign.Signer
	publicURL string
	users     *user.Service
	ttl       time.Duration
}

// NewUploadLinks draws a fresh signing key: a restart ends every open link,
// which costs at most a re-issued link.
func NewUploadLinks(publicURL string, users *user.Service) *UploadLinks {
	key := make([]byte, 32)
	_, _ = rand.Read(key) // crypto/rand.Read never fails on supported platforms
	return &UploadLinks{signer: sign.New(key, "rezepte photo upload"), publicURL: publicURL, users: users, ttl: linkTTL}
}

func linkSubject(recipeID, userID string) string { return recipeID + "\n" + userID }

// Issue returns an absolute link that adds photos to recipeID as userID,
// and when it stops working. Editing rights are checked on upload.
func (l *UploadLinks) Issue(recipeID, userID string) (string, time.Time, error) {
	if l.publicURL == "" {
		return "", time.Time{}, ErrNoPublicURL
	}
	token, expires := l.signer.Sign(linkSubject(recipeID, userID), l.ttl)
	q := url.Values{"user": {userID}, "token": {token}}
	return l.publicURL + "/api/v1/recipes/" + url.PathEscape(recipeID) + "/images/link?" + q.Encode(), expires, nil
}

// linkForm is uploadForm without the declared-type check: the curl line
// photo_upload_link hands out lets curl pick the part's type from the file
// extension, and curl sends application/octet-stream for .webp or a file
// without one. decode sniffs the bytes either way (415 when not an image).
type linkForm struct {
	File huma.FormFile `form:"file" required:"true"`
}

type linkInput struct {
	ID      string `path:"id"`
	User    string `query:"user"`
	Token   string `query:"token"`
	RawBody huma.MultipartFormFiles[linkForm]
}

// requireLink refuses a request without a valid link before huma parses
// the body: the route needs no account, so a stranger must not get it to
// read megabytes.
func requireLink(api huma.API, links *UploadLinks) func(ctx huma.Context, next func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		if !links.signer.Verify(linkSubject(ctx.Param("id"), ctx.Query("user")), ctx.Query("token")) {
			_ = huma.WriteErr(api, ctx, http.StatusForbidden, "upload link invalid or expired")
			return
		}
		next(ctx)
	}
}

// RegisterUploadLink installs the upload route the links point at. It
// carries no Security: the signed token is the whole credential. Hidden,
// because only photo_upload_link hands it out.
func RegisterUploadLink(api huma.API, svc *Service, links *UploadLinks) {
	huma.Register(api, huma.Operation{
		OperationID:   "upload-image-by-link",
		Method:        http.MethodPost,
		Path:          "/api/v1/recipes/{id}/images/link",
		Summary:       "Upload an image through a signed upload link",
		Tags:          []string{"images"},
		Hidden:        true,
		DefaultStatus: http.StatusCreated,
		MaxBodyBytes:  MaxUploadBytes,
		Middlewares:   huma.Middlewares{requireLink(api, links), LimitUpload(api)},
		Errors:        []int{403, 404, 413, 415, 422},
	}, func(ctx context.Context, in *linkInput) (*imageOutput, error) {
		file := in.RawBody.Data().File
		defer file.Close()
		u, err := links.users.ByID(ctx, in.User)
		if errors.Is(err, user.ErrNotFound) {
			return nil, huma.Error403Forbidden("upload link invalid or expired")
		}
		if err != nil {
			return nil, fmt.Errorf("load link user: %w", err)
		}
		img, err := svc.Upload(ctx, in.ID, u, file)
		if err != nil {
			return nil, imageErr(err)
		}
		return &imageOutput{Body: img}, nil
	})
}
