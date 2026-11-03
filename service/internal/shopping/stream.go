package shopping

import (
	"io"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
)

// pingEvery is how often an idle stream sends a comment, so proxies and
// browsers do not take it for dead. A variable so tests can shorten it.
var pingEvery = 25 * time.Second

// stream answers with the list's Server-Sent Events: "changed" after every
// change, until the request ends, the hub closes or the caller loses access.
// The caller's access was checked before; it is checked again on every
// signal, since removing a member is a change of the list too.
func (s *Service) stream(userID, listID string) *huma.StreamResponse {
	return &huma.StreamResponse{Body: func(hctx huma.Context) {
		r, w := humago.Unwrap(hctx)
		events, cancel := s.hub.Subscribe(listID)
		defer cancel()
		h := w.Header()
		h.Set("Content-Type", "text/event-stream")
		h.Set("Cache-Control", "no-cache")
		h.Set("X-Accel-Buffering", "no") // nginx: do not buffer the stream
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
				if ok, err := s.CanAccess(r.Context(), userID, listID); err != nil || !ok {
					return
				}
				if !send("event: changed\ndata: {}\n\n") {
					return
				}
			}
		}
	}}
}
