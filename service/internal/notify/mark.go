package notify

import (
	"context"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/s-frei/rezepte/service/internal/user"
)

// markLabel keeps push mark tokens apart from every other token signed
// with the instance key.
const markLabel = "rezepte push mark"

// markTTL is how long a push's icon address works. A push lives an hour
// at the push service (TTL 3600) and the browser fetches the icon as it
// shows the notification, so a day is room enough.
const markTTL = 24 * time.Hour

// markURL is the icon a push about u shows: u's face from get-push-mark.
// The notification system fetches it, and whether it sends the session
// cookie is not something any browser promises, so the address carries its
// own signature for u instead (see features/notifications).
func (s *Service) markURL(u user.User) string {
	token, _ := s.marks.Sign(u.ID, markTTL)
	return "/marks/" + url.PathEscape(u.ID) + ".png?s=" + url.QueryEscape(token)
}

// mark renders the face file names (<user id>.png) when token is valid for
// it; ok is false for a bad token, an unknown account or a failed render.
func (s *Service) mark(ctx context.Context, file, token string) (png []byte, etag string, ok bool) {
	id, found := strings.CutSuffix(file, ".png")
	if !found || !s.marks.Verify(id, token) || s.avatars == nil {
		return nil, "", false
	}
	u, err := s.users.ByID(ctx, id)
	if err != nil {
		return nil, "", false
	}
	png, etag, err = s.avatars.Mark(u)
	if err != nil {
		slog.WarnContext(ctx, "push: mark failed", "user", id, "err", err)
		return nil, "", false
	}
	return png, etag, true
}
