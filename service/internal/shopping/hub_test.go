package shopping_test

import (
	"testing"

	"github.com/s-frei/rezepte/service/internal/shopping"
)

// signaled reports whether ch holds a signal right now.
func signaled(ch <-chan struct{}) bool {
	select {
	case _, ok := <-ch:
		return ok
	default:
		return false
	}
}

func closed(ch <-chan struct{}) bool {
	select {
	case _, ok := <-ch:
		return !ok
	default:
		return false
	}
}

func TestPublishReachesSubscribers(t *testing.T) {
	h := shopping.NewHub()
	a1, cancel1 := h.Subscribe("A")
	defer cancel1()
	a2, cancel2 := h.Subscribe("A")
	defer cancel2()
	b, cancel3 := h.Subscribe("B")
	defer cancel3()
	h.Publish("A")
	if !signaled(a1) || !signaled(a2) {
		t.Fatal("a subscriber of A got no signal")
	}
	if signaled(b) {
		t.Fatal("the subscriber of B got a signal")
	}
}

func TestPublishCoalesces(t *testing.T) {
	h := shopping.NewHub()
	ch, cancel := h.Subscribe("A")
	defer cancel()
	h.Publish("A")
	h.Publish("A")
	h.Publish("A") // must not block on a full channel
	if !signaled(ch) {
		t.Fatal("no signal")
	}
	if signaled(ch) {
		t.Fatal("three publishes left more than one signal")
	}
}

func TestCancelStopsDelivery(t *testing.T) {
	h := shopping.NewHub()
	ch, cancel := h.Subscribe("A")
	cancel()
	cancel() // twice is harmless
	h.Publish("A")
	h.PublishAll()
	if signaled(ch) {
		t.Fatal("signal after cancel")
	}
}

func TestPublishAllReachesEveryList(t *testing.T) {
	h := shopping.NewHub()
	a, cancelA := h.Subscribe("A")
	defer cancelA()
	b, cancelB := h.Subscribe("B")
	defer cancelB()
	h.PublishAll()
	if !signaled(a) || !signaled(b) {
		t.Fatal("PublishAll missed a list")
	}
}

func TestHubCloseEndsStreams(t *testing.T) {
	h := shopping.NewHub()
	ch, cancel := h.Subscribe("A")
	h.Close()
	if !closed(ch) {
		t.Fatal("channel open after Close")
	}
	cancel() // after Close is harmless
	h.Publish("A")
	late, cancelLate := h.Subscribe("A")
	defer cancelLate()
	if !closed(late) {
		t.Fatal("Subscribe after Close returned an open channel")
	}
}

func TestNilHubIsOff(t *testing.T) {
	var h *shopping.Hub
	h.Publish("A")
	h.PublishAll()
	ch, cancel := h.Subscribe("A")
	defer cancel()
	if !closed(ch) {
		t.Fatal("a nil hub's subscription is open")
	}
}
