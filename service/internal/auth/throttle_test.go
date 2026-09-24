package auth

import (
	"fmt"
	"testing"
	"time"
)

var t0 = time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)

// fail runs n attempts for name at now, failing the test if one is refused.
func fail(t *testing.T, th *throttle, name string, n int, now time.Time) {
	t.Helper()
	for i := range n {
		if wait, ok := th.begin(name, now); !ok {
			t.Fatalf("attempt %d for %q refused, wait %v", i+1, name, wait)
		}
	}
}

func TestThrottleAllowsFiveAttemptsThenLocks(t *testing.T) {
	th := newThrottle(100)
	fail(t, th, "sam", 5, t0)
	wait, ok := th.begin("sam", t0)
	if ok {
		t.Fatal("sixth attempt allowed, want refused")
	}
	if wait != 30*time.Second {
		t.Fatalf("wait = %v, want 30s", wait)
	}
}

func TestThrottleReportsTheRemainingLock(t *testing.T) {
	th := newThrottle(100)
	fail(t, th, "sam", 5, t0)
	if wait, _ := th.begin("sam", t0.Add(20*time.Second)); wait != 10*time.Second {
		t.Fatalf("wait = %v, want 10s", wait)
	}
}

func TestThrottleDoublesTheLockPerFurtherFailure(t *testing.T) {
	th := newThrottle(100)
	fail(t, th, "sam", 5, t0)
	now := t0.Add(30 * time.Second)
	fail(t, th, "sam", 1, now)
	if wait, ok := th.begin("sam", now); ok || wait != time.Minute {
		t.Fatalf("after sixth failure: wait = %v, ok = %v; want 1m, refused", wait, ok)
	}
}

func TestThrottleCapsTheLock(t *testing.T) {
	th := newThrottle(100)
	now := t0
	fail(t, th, "sam", 5, now)
	for range 40 {
		wait, _ := th.begin("sam", now)
		now = now.Add(wait)
		fail(t, th, "sam", 1, now)
	}
	if wait, ok := th.begin("sam", now); ok || wait != 15*time.Minute {
		t.Fatalf("wait = %v, ok = %v; want 15m, refused", wait, ok)
	}
}

func TestThrottleResetsOnSuccess(t *testing.T) {
	th := newThrottle(100)
	fail(t, th, "sam", 4, t0)
	th.succeed("sam")
	fail(t, th, "sam", 5, t0)
}

func TestThrottleForgetsAfterADayWithoutAttempts(t *testing.T) {
	th := newThrottle(100)
	fail(t, th, "sam", 5, t0)
	fail(t, th, "sam", 5, t0.Add(forgetAfter))
}

func TestThrottleFoldsCaseAndSpace(t *testing.T) {
	th := newThrottle(100)
	fail(t, th, "Sam", 3, t0)
	fail(t, th, " sam ", 2, t0)
	if _, ok := th.begin("SAM", t0); ok {
		t.Fatal("sixth attempt across spellings allowed, want refused")
	}
}

// SQLite's NOCASE folds ASCII letters only, so "jürgen" and "jÜrgen" are
// two accounts and must not share a count.
func TestThrottleFoldsASCIIOnly(t *testing.T) {
	th := newThrottle(100)
	fail(t, th, "jürgen", 5, t0)
	fail(t, th, "jÜrgen", 5, t0)
	if _, ok := th.begin("JüRGEN", t0); ok {
		t.Fatal("ASCII-folded spelling allowed, want it to share jürgen's lock")
	}
}

func TestThrottleKeepsNamesApart(t *testing.T) {
	th := newThrottle(100)
	fail(t, th, "sam", 5, t0)
	fail(t, th, "alex", 5, t0)
}

func TestThrottleEvictsTheLeastRecentlySeenNameWhenFull(t *testing.T) {
	th := newThrottle(3)
	fail(t, th, "victim", 5, t0)
	fail(t, th, "a", 1, t0.Add(time.Second))
	fail(t, th, "b", 1, t0.Add(2*time.Second))
	// A refused attempt counts as seen, so a name under attack stays fresh.
	if _, ok := th.begin("victim", t0.Add(3*time.Second)); ok {
		t.Fatal("victim allowed while locked")
	}
	fail(t, th, "c", 1, t0.Add(4*time.Second))
	if len(th.entries) != 3 {
		t.Fatalf("entries = %d, want 3", len(th.entries))
	}
	if _, ok := th.entries["a"]; ok {
		t.Fatal("a survived, want it evicted as least recently seen")
	}
	if _, ok := th.begin("victim", t0.Add(5*time.Second)); ok {
		t.Fatal("victim's lock was evicted")
	}
}

// Flooding the throttle with fresh names must not buy a locked name its
// free attempts back: a name that has used them is evicted only once every
// other name has too.
func TestThrottleFloodingDoesNotEvictALockedName(t *testing.T) {
	th := newThrottle(3)
	fail(t, th, "victim", 5, t0)
	for i := range 10 {
		fail(t, th, fmt.Sprint("spray", i), 1, t0.Add(time.Duration(i+1)*time.Second))
	}
	if _, ok := th.begin("victim", t0.Add(20*time.Second)); ok {
		t.Fatal("victim allowed after a flood of fresh names, want still locked")
	}
}

func TestThrottleEvictsALockedNameOnlyWhenAllAre(t *testing.T) {
	th := newThrottle(2)
	fail(t, th, "first", 5, t0)
	fail(t, th, "second", 5, t0.Add(time.Second))
	fail(t, th, "third", 1, t0.Add(2*time.Second))
	if _, ok := th.entries["first"]; ok {
		t.Fatal("first survived, want it evicted as the least recently seen locked name")
	}
	if len(th.entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(th.entries))
	}
}

func TestThrottleSweepDropsIdleNames(t *testing.T) {
	th := newThrottle(100)
	for i := range 10 {
		fail(t, th, fmt.Sprint("user", i), 1, t0)
	}
	fail(t, th, "fresh", 1, t0.Add(forgetAfter))
	th.sweep(t0.Add(forgetAfter))
	if len(th.entries) != 1 {
		t.Fatalf("entries = %d after sweep, want 1", len(th.entries))
	}
}
