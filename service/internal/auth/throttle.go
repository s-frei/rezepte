package auth

import (
	"strings"
	"sync"
	"time"
)

const (
	// freeAttempts is how many login attempts a name gets before a lock.
	freeAttempts = 5
	// firstLock is the lock after the last free attempt; every further
	// attempt doubles it, up to maxLock.
	firstLock = 30 * time.Second
	maxLock   = 15 * time.Minute
	// forgetAfter drops a name nobody has tried for that long.
	forgetAfter = 24 * time.Hour
	// throttleCapacity bounds how many names are tracked at once.
	throttleCapacity = 10_000
)

// throttle counts login attempts per username and locks a name once it has
// used its free attempts. An attempt is counted when it begins, not when its
// password turns out wrong, so a burst of parallel requests cannot all pass
// the check before the first failure lands. A successful login forgets the
// name again.
type throttle struct {
	mu       sync.Mutex
	capacity int
	entries  map[string]*attempts
}

type attempts struct {
	count       int
	lockedUntil time.Time
	lastSeen    time.Time
}

func newThrottle(capacity int) *throttle {
	return &throttle{capacity: capacity, entries: make(map[string]*attempts)}
}

// throttleKey folds a username the way the users table compares it:
// trimmed like Authenticate trims it, and with ASCII letters lowercased like
// its COLLATE NOCASE column, which leaves every other letter as it is.
func throttleKey(username string) string {
	return strings.Map(func(r rune) rune {
		if 'A' <= r && r <= 'Z' {
			return r + ('a' - 'A')
		}
		return r
	}, strings.TrimSpace(username))
}

// begin counts an attempt for username at now. It reports false and the
// time left when the name is locked; a refused attempt is not counted, but
// it keeps the name fresh so an attacked name is not the one evicted.
func (t *throttle) begin(username string, now time.Time) (time.Duration, bool) {
	key := throttleKey(username)
	t.mu.Lock()
	defer t.mu.Unlock()

	a, ok := t.entries[key]
	if ok && now.Sub(a.lastSeen) >= forgetAfter {
		delete(t.entries, key)
		ok = false
	}
	if !ok {
		t.makeRoom(now)
		a = &attempts{}
		t.entries[key] = a
	}
	a.lastSeen = now
	if wait := a.lockedUntil.Sub(now); wait > 0 {
		return wait, false
	}
	a.count++
	if a.count >= freeAttempts {
		a.lockedUntil = now.Add(lockFor(a.count))
	}
	return 0, true
}

// succeed forgets username after a successful login.
func (t *throttle) succeed(username string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.entries, throttleKey(username))
}

// sweep drops every name nobody has tried for forgetAfter.
func (t *throttle) sweep(now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.sweepLocked(now)
}

func (t *throttle) sweepLocked(now time.Time) {
	for key, a := range t.entries {
		if now.Sub(a.lastSeen) >= forgetAfter {
			delete(t.entries, key)
		}
	}
}

// makeRoom frees a slot when the throttle is full: idle names first, then
// the least recently seen name that still has free attempts, and a name
// that has used them only when every name has. Evicting a locked name hands
// it its free attempts back, so flooding the map with fresh names must not
// be a way to reach one.
func (t *throttle) makeRoom(now time.Time) {
	if len(t.entries) < t.capacity {
		return
	}
	t.sweepLocked(now)
	if len(t.entries) < t.capacity {
		return
	}
	var victimKey string
	var victim *attempts
	for key, a := range t.entries {
		if victim == nil || evictsBefore(a, victim) {
			victimKey, victim = key, a
		}
	}
	delete(t.entries, victimKey)
}

// evictsBefore reports whether a goes before b when the throttle is full.
func evictsBefore(a, b *attempts) bool {
	aSpent, bSpent := a.count >= freeAttempts, b.count >= freeAttempts
	if aSpent != bSpent {
		return !aSpent
	}
	return a.lastSeen.Before(b.lastSeen)
}

// lockFor is the lock that follows the count-th attempt: firstLock after
// the last free one, doubling from there up to maxLock.
func lockFor(count int) time.Duration {
	lock := firstLock
	for range count - freeAttempts {
		lock *= 2
		if lock >= maxLock {
			return maxLock
		}
	}
	return lock
}
