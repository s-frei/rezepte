package user

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/s-frei/rezepte/service/internal/db/dbtest"
)

func TestHashAndVerifyPassword(t *testing.T) {
	encoded, err := HashPassword(context.Background(), "correct horse")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if !strings.HasPrefix(encoded, "$argon2id$v=19$m=65536,t=1,p=4$") {
		t.Fatalf("unexpected format: %s", encoded)
	}
	ok, err := VerifyPassword(context.Background(), encoded, "correct horse")
	if err != nil || !ok {
		t.Fatalf("verify correct = %v, %v", ok, err)
	}
	ok, err = VerifyPassword(context.Background(), encoded, "wrong")
	if err != nil || ok {
		t.Fatalf("verify wrong = %v, %v", ok, err)
	}
}

func TestHashPasswordUsesFreshSalt(t *testing.T) {
	a, _ := HashPassword(context.Background(), "x")
	b, _ := HashPassword(context.Background(), "x")
	if a == b {
		t.Fatal("two hashes of the same password must differ")
	}
}

func TestVerifyPasswordRejectsGarbage(t *testing.T) {
	if _, err := VerifyPassword(context.Background(), "not-a-hash", "x"); err == nil {
		t.Fatal("expected error for malformed hash")
	}
}

func TestVerifyPasswordRejectsMalformed(t *testing.T) {
	encoded, err := HashPassword(context.Background(), "correct horse")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	parts := strings.Split(encoded, "$")
	salt, key := parts[4], parts[5]
	shortSalt := base64.RawStdEncoding.EncodeToString(make([]byte, 8))

	tests := []struct {
		name    string
		encoded string
	}{
		{
			name:    "empty key segment",
			encoded: fmt.Sprintf("$argon2id$v=19$m=65536,t=1,p=4$%s$", salt),
		},
		{
			name:    "version trailing garbage",
			encoded: fmt.Sprintf("$argon2id$v=19x$m=65536,t=1,p=4$%s$%s", salt, key),
		},
		{
			name:    "params trailing garbage",
			encoded: fmt.Sprintf("$argon2id$v=19$m=65536,t=1,p=4x$%s$%s", salt, key),
		},
		{
			name:    "extra param",
			encoded: fmt.Sprintf("$argon2id$v=19$m=65536,t=1,p=4,x=9$%s$%s", salt, key),
		},
		{
			name:    "wrong salt length",
			encoded: fmt.Sprintf("$argon2id$v=19$m=65536,t=1,p=4$%s$%s", shortSalt, key),
		},
		{
			name:    "memory out of bounds",
			encoded: fmt.Sprintf("$argon2id$v=19$m=99999999,t=1,p=4$%s$%s", salt, key),
		},
		{
			name:    "wrong algorithm",
			encoded: fmt.Sprintf("$argon2i$v=19$m=65536,t=1,p=4$%s$%s", salt, key),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, err := VerifyPassword(context.Background(), tt.encoded, "correct horse")
			if err == nil {
				t.Fatalf("expected error for %s", tt.name)
			}
			if ok {
				t.Fatalf("expected ok=false for %s", tt.name)
			}
		})
	}
}

// TestArgonEvaluationsAreCapped runs far more hashes and verifications in
// parallel than the cap allows and records how many key derivations were in
// flight at once: exactly maxConcurrentArgon at the peak, which is what
// bounds the memory a burst of logins can claim.
func TestArgonEvaluationsAreCapped(t *testing.T) {
	ctx := context.Background()
	encoded, err := HashPassword(ctx, "pw")
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	inFlight, peak := 0, 0
	orig := idKey
	idKey = func(password, salt []byte, time, memory uint32, threads uint8, keyLen uint32) []byte {
		mu.Lock()
		inFlight++
		peak = max(peak, inFlight)
		mu.Unlock()
		defer func() {
			mu.Lock()
			inFlight--
			mu.Unlock()
		}()
		return orig(password, salt, time, memory, threads, keyLen)
	}
	t.Cleanup(func() { idKey = orig })

	var wg sync.WaitGroup
	for i := range 4 * maxConcurrentArgon {
		wg.Go(func() {
			if i%2 == 0 {
				_, _ = HashPassword(ctx, "pw")
			} else {
				_, _ = VerifyPassword(ctx, encoded, "pw")
			}
		})
	}
	wg.Wait()
	if peak != maxConcurrentArgon {
		t.Fatalf("peak concurrent derivations = %d, want %d", peak, maxConcurrentArgon)
	}
}

// holdArgon makes every key derivation block until the test ends and
// returns once n derivations or waiters are in line, so a test can fill the
// slots and the queue in front of them.
func holdArgon(t *testing.T, n int) {
	t.Helper()
	release := make(chan struct{})
	orig := idKey
	idKey = func(password, salt []byte, time, memory uint32, threads uint8, keyLen uint32) []byte {
		<-release
		return make([]byte, keyLen)
	}
	var wg sync.WaitGroup
	t.Cleanup(func() {
		close(release)
		wg.Wait()
		idKey = orig
	})
	for range n {
		wg.Go(func() { _, _ = HashPassword(context.Background(), "pw") })
	}
	deadline := time.Now().Add(5 * time.Second)
	for len(argonQueue) < n {
		if time.Now().After(deadline) {
			t.Fatalf("only %d of %d derivations in line", len(argonQueue), n)
		}
		time.Sleep(time.Millisecond)
	}
}

func TestArgonRefusesWhenTheQueueIsFull(t *testing.T) {
	holdArgon(t, maxConcurrentArgon+maxQueuedArgon)
	if _, err := HashPassword(context.Background(), "pw"); !errors.Is(err, ErrBusy) {
		t.Fatalf("err = %v, want ErrBusy", err)
	}
}

func TestArgonWaitGivesUpWithTheContext(t *testing.T) {
	holdArgon(t, maxConcurrentArgon)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := HashPassword(ctx, "pw"); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

// An unknown name must report a full queue exactly like a real one, or a
// 503 would tell which accounts exist.
func TestAuthenticateReportsBusyForAnUnknownNameToo(t *testing.T) {
	ctx := context.Background()
	svc := NewService(dbtest.Open(t))
	if _, err := svc.Create(ctx, CreateParams{Username: "sam", Password: "secret123", Role: RoleAdmin}); err != nil {
		t.Fatal(err)
	}
	dummyHash() // computed before the queue fills, as a running server has
	holdArgon(t, maxConcurrentArgon+maxQueuedArgon)
	for _, name := range []string{"sam", "nobody"} {
		if _, err := svc.Authenticate(ctx, name, "wrong-pw"); !errors.Is(err, ErrBusy) {
			t.Errorf("Authenticate(%q): err = %v, want ErrBusy", name, err)
		}
	}
}
