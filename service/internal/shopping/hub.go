package shopping

import "sync"

// Hub fans "list changed" signals out to the open event streams. A signal
// carries nothing: the stream tells the client to refetch. Every channel
// holds one signal at most, so a burst of changes coalesces and a publisher
// never waits for a slow reader. A nil Hub is off: it publishes nothing and
// its subscriptions are closed at once.
type Hub struct {
	mu     sync.Mutex
	subs   map[string]map[chan struct{}]struct{}
	closed bool
}

// NewHub returns an empty, open hub.
func NewHub() *Hub {
	return &Hub{subs: map[string]map[chan struct{}]struct{}{}}
}

// Subscribe returns a channel that receives a signal after each change to
// the list, and a cancel that drops it. The channel is closed when the hub
// closes; after Close it is closed from the start.
func (h *Hub) Subscribe(listID string) (events <-chan struct{}, cancel func()) {
	ch := make(chan struct{}, 1)
	if h == nil {
		close(ch)
		return ch, func() {}
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		close(ch)
		return ch, func() {}
	}
	if h.subs[listID] == nil {
		h.subs[listID] = map[chan struct{}]struct{}{}
	}
	h.subs[listID][ch] = struct{}{}
	return ch, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		delete(h.subs[listID], ch)
		if len(h.subs[listID]) == 0 {
			delete(h.subs, listID)
		}
	}
}

// Publish signals every stream open on the lists.
func (h *Hub) Publish(listIDs ...string) {
	if h == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, id := range listIDs {
		for ch := range h.subs[id] {
			signal(ch)
		}
	}
}

// PublishAll signals every open stream, for a change that reaches every
// list such as a department override.
func (h *Hub) PublishAll() {
	if h == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, chans := range h.subs {
		for ch := range chans {
			signal(ch)
		}
	}
}

// Close closes every subscription so the streams end, as on shutdown.
func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return
	}
	h.closed = true
	for _, chans := range h.subs {
		for ch := range chans {
			close(ch)
		}
	}
	h.subs = nil
}

// signal leaves a signal unless one is waiting already.
func signal(ch chan struct{}) {
	select {
	case ch <- struct{}{}:
	default:
	}
}
