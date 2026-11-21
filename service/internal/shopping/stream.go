package shopping

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"

	"github.com/s-frei/rezepte/service/internal/db/sqlc"
)

// pingEvery is how often an idle stream sends a comment, so proxies and
// browsers do not take it for dead. A variable so tests can shorten it.
var pingEvery = 25 * time.Second

// stream answers with the list's Server-Sent Events.
func (s *Service) stream(userID, listID string) *huma.StreamResponse {
	return s.hub.Stream(listID, func(ctx context.Context) (int64, error) {
		return s.q.GetShoppingListRevision(ctx, sqlc.GetShoppingListRevisionParams{ID: listID, UserID: userID})
	})
}

// Stream answers with the Server-Sent Events of the thing id names, a list or
// a meal plan: "changed" after every change, until the request ends, the hub
// closes or the caller loses access. The caller's access was checked before;
// rev checks it again on every signal (an error means no access or no thing),
// since removing a member is a change too. Each event carries the revision,
// so a page that shows it already skips the refetch.
func (h *Hub) Stream(id string, rev func(context.Context) (int64, error)) *huma.StreamResponse {
	return &huma.StreamResponse{Body: func(hctx huma.Context) {
		r, w := humago.Unwrap(hctx)
		events, cancel := h.Subscribe(id)
		defer cancel()
		hd := w.Header()
		hd.Set("Content-Type", "text/event-stream")
		hd.Set("Cache-Control", "no-cache")
		hd.Set("X-Accel-Buffering", "no") // nginx: do not buffer the stream
		w.WriteHeader(http.StatusOK)
		rc := http.NewResponseController(w)
		send := func(msg string) bool {
			if _, err := io.WriteString(w, msg); err != nil {
				return false
			}
			return rc.Flush() == nil
		}
		if !send("retry: 3000\n\n") {
			return
		}
		ping := time.NewTicker(pingEvery)
		defer ping.Stop()
		for {
			select {
			case <-r.Context().Done():
				return
			case <-ping.C:
				if !send(": ping\n\n") {
					return
				}
			case _, open := <-events:
				if !open {
					return
				}
				n, err := rev(r.Context())
				if err != nil {
					return
				}
				if !send(fmt.Sprintf("event: changed\ndata: {\"revision\":%d}\n\n", n)) {
					return
				}
			}
		}
	}}
}
