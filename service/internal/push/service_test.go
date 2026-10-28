package push

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/s-frei/rezepte/service/internal/db/dbtest"
	"github.com/s-frei/rezepte/service/internal/db/sqlc"
	"github.com/s-frei/rezepte/service/internal/user"
)

const fcm = "https://fcm.googleapis.com/fcm/send/abc"

type fixture struct {
	svc        *Service
	alice, bob string
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	conn := dbtest.Open(t)
	users := user.NewService(conn, "")
	var ids []string
	for _, name := range []string{"alice", "bob"} {
		u, err := users.Create(context.Background(), user.CreateParams{Username: name, Password: "pw", Role: user.RoleUser})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, u.ID)
	}
	return fixture{svc: NewService(conn, "https://rezepte.example", http.DefaultClient), alice: ids[0], bob: ids[1]}
}

// receiver is a device: its key pair and auth secret.
type receiver struct {
	key  *ecdh.PrivateKey
	auth []byte
}

func newReceiver(t *testing.T) receiver {
	t.Helper()
	k, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return receiver{key: k, auth: bytes.Repeat([]byte{7}, 16)}
}

func (r receiver) sub(endpoint string) Subscription {
	return Subscription{Endpoint: endpoint, P256dh: r.key.PublicKey().Bytes(), Auth: r.auth}
}

// decrypt undoes Encrypt the way a browser does (RFC 8291 from the
// receiver's side), so the test does not reuse the code under test.
func (r receiver) decrypt(t *testing.T, body []byte) []byte {
	t.Helper()
	salt, rs, idlen := body[:16], binary.BigEndian.Uint32(body[16:20]), int(body[20])
	if rs != 4096 || idlen != 65 {
		t.Fatalf("header rs=%d idlen=%d", rs, idlen)
	}
	asPub, err := ecdh.P256().NewPublicKey(body[21 : 21+idlen])
	if err != nil {
		t.Fatal(err)
	}
	secret, err := r.key.ECDH(asPub)
	if err != nil {
		t.Fatal(err)
	}
	info := "WebPush: info\x00" + string(r.key.PublicKey().Bytes()) + string(asPub.Bytes())
	ikm, _ := hkdf.Key(sha256.New, secret, r.auth, info, 32)
	cek, _ := hkdf.Key(sha256.New, ikm, salt, "Content-Encoding: aes128gcm\x00", 16)
	nonce, _ := hkdf.Key(sha256.New, ikm, salt, "Content-Encoding: nonce\x00", 12)
	block, _ := aes.NewCipher(cek)
	gcm, _ := cipher.NewGCM(block)
	plain, err := gcm.Open(nil, nonce, body[21+idlen:], nil)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if plain[len(plain)-1] != 2 {
		t.Fatalf("padding delimiter %x", plain[len(plain)-1])
	}
	return plain[:len(plain)-1]
}

func alarm(endpoint string, fireAt time.Time) Alarm {
	return Alarm{Endpoint: endpoint, FireAt: fireAt, Title: "Rinderrouladen · done", Body: "Step 5 · 90 minutes", URL: "/recipes/rinderrouladen/cook?step=5"}
}

func TestSubscribeRejectsForeignEndpoint(t *testing.T) {
	f := newFixture(t)
	r := newReceiver(t)
	for _, ep := range []string{
		"http://fcm.googleapis.com/fcm/send/abc",
		"https://10.0.0.1/x",
		"https://evil.example/fcm.googleapis.com",
		"https://fcm.googleapis.com.evil.example/x",
		"https://evilfcm.googleapis.com/x",
		"https://user@evil.example/x",
		"/relative",
	} {
		if err := f.svc.Subscribe(context.Background(), f.alice, r.sub(ep)); !errors.Is(err, ErrForeignEndpoint) {
			t.Errorf("%s: err %v, want ErrForeignEndpoint", ep, err)
		}
	}
	for _, ep := range []string{
		fcm,
		"https://updates.push.services.mozilla.com/wpush/v2/x",
		"https://web.push.apple.com/x",
		"https://wns2-par02p.notify.windows.com/w/?token=x",
	} {
		if err := f.svc.Subscribe(context.Background(), f.alice, r.sub(ep)); err != nil {
			t.Errorf("%s: %v", ep, err)
		}
	}
}

func TestSubscribeRejectsBadKeys(t *testing.T) {
	f := newFixture(t)
	r := newReceiver(t)
	s := r.sub(fcm)
	s.P256dh = s.P256dh[:10]
	if err := f.svc.Subscribe(context.Background(), f.alice, s); !errors.Is(err, ErrInvalid) {
		t.Fatalf("short key: %v", err)
	}
	s = r.sub(fcm)
	s.Auth = []byte{1}
	if err := f.svc.Subscribe(context.Background(), f.alice, s); !errors.Is(err, ErrInvalid) {
		t.Fatalf("short auth: %v", err)
	}
}

func TestSubscribeMovesEndpointToNewUser(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	r := newReceiver(t)
	if err := f.svc.Subscribe(ctx, f.alice, r.sub(fcm)); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.Subscribe(ctx, f.bob, r.sub(fcm)); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Schedule(ctx, f.alice, alarm(fcm, time.Now().Add(time.Minute))); !errors.Is(err, ErrUnknownSubscription) {
		t.Fatalf("alice still owns the endpoint: %v", err)
	}
	if _, err := f.svc.Schedule(ctx, f.bob, alarm(fcm, time.Now().Add(time.Minute))); err != nil {
		t.Fatalf("bob: %v", err)
	}
}

func TestScheduleNeedsOwnSubscription(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if err := f.svc.Subscribe(ctx, f.bob, newReceiver(t).sub(fcm)); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Schedule(ctx, f.alice, alarm(fcm, time.Now().Add(time.Minute))); !errors.Is(err, ErrUnknownSubscription) {
		t.Fatalf("err %v", err)
	}
}

func TestScheduleValidates(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if err := f.svc.Subscribe(ctx, f.alice, newReceiver(t).sub(fcm)); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	for name, a := range map[string]Alarm{
		"past":      alarm(fcm, now.Add(-2*time.Minute)),
		"far":       alarm(fcm, now.Add(8*24*time.Hour)),
		"other url": func() Alarm { a := alarm(fcm, now.Add(time.Minute)); a.URL = "https://evil.example/"; return a }(),
	} {
		if _, err := f.svc.Schedule(ctx, f.alice, a); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: err %v, want ErrInvalid", name, err)
		}
	}
	if _, err := f.svc.Schedule(ctx, f.alice, alarm(fcm, now.Add(-30*time.Second))); err != nil {
		t.Errorf("half a minute late: %v", err)
	}
}

func TestCancelMissingIsNoError(t *testing.T) {
	f := newFixture(t)
	if err := f.svc.Cancel(context.Background(), f.alice, "no-such-alarm"); err != nil {
		t.Fatal(err)
	}
}

func TestCancelOnlyOwn(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if err := f.svc.Subscribe(ctx, f.bob, newReceiver(t).sub(fcm)); err != nil {
		t.Fatal(err)
	}
	id, err := f.svc.Schedule(ctx, f.bob, alarm(fcm, time.Now().Add(time.Minute)))
	if err != nil {
		t.Fatal(err)
	}
	if err := f.svc.Cancel(ctx, f.alice, id); err != nil {
		t.Fatal(err)
	}
	if n, _ := f.svc.q.CountAlarmsBySubscription(ctx, f.subID(t, f.bob, fcm)); n != 1 {
		t.Fatalf("alice cancelled bob's alarm: %d left", n)
	}
	if err := f.svc.Cancel(ctx, f.bob, id); err != nil {
		t.Fatal(err)
	}
	if n, _ := f.svc.q.CountAlarmsBySubscription(ctx, f.subID(t, f.bob, fcm)); n != 0 {
		t.Fatalf("%d left after bob's cancel", n)
	}
}

func (f fixture) subID(t *testing.T, userID, endpoint string) string {
	t.Helper()
	s, err := f.svc.q.GetPushSubscriptionForUser(context.Background(), sqlc.GetPushSubscriptionForUserParams{UserID: userID, Endpoint: endpoint})
	if err != nil {
		t.Fatal(err)
	}
	return s.ID
}

// pushServer is a TLS push service the test lets the service reach.
type pushServer struct {
	*httptest.Server
	mu     sync.Mutex
	got    []*http.Request
	bodies [][]byte
	status int
}

func newPushServer(t *testing.T, f fixture, status int) *pushServer {
	t.Helper()
	p := &pushServer{status: status}
	p.Server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		p.mu.Lock()
		p.got, p.bodies = append(p.got, r), append(p.bodies, b)
		p.mu.Unlock()
		w.WriteHeader(p.status)
	}))
	t.Cleanup(p.Close)
	f.svc.client = p.Client()
	f.svc.allowHost = func(h string) bool { return h == "127.0.0.1" }
	return p
}

func TestRunDueSends(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	p := newPushServer(t, f, http.StatusCreated)
	r := newReceiver(t)
	ep := p.URL + "/push/1"
	if err := f.svc.Subscribe(ctx, f.alice, r.sub(ep)); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	a := alarm(ep, now.Add(time.Minute))
	id, err := f.svc.Schedule(ctx, f.alice, a)
	if err != nil {
		t.Fatal(err)
	}

	f.svc.RunDue(ctx, now)
	if len(p.got) != 0 {
		t.Fatal("sent before it was due")
	}
	f.svc.RunDue(ctx, now.Add(2*time.Minute))
	if len(p.got) != 1 {
		t.Fatalf("%d requests, want 1", len(p.got))
	}
	req := p.got[0]
	for k, want := range map[string]string{"Content-Encoding": "aes128gcm", "TTL": "3600", "Urgency": "high"} {
		if got := req.Header.Get(k); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
	if !strings.HasPrefix(req.Header.Get("Authorization"), "vapid t=") {
		t.Errorf("Authorization %q", req.Header.Get("Authorization"))
	}
	var payload map[string]string
	if err := json.Unmarshal(r.decrypt(t, p.bodies[0]), &payload); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"title": a.Title, "body": a.Body, "url": a.URL, "tag": id}
	for k, v := range want {
		if payload[k] != v {
			t.Errorf("payload[%s] = %q, want %q", k, payload[k], v)
		}
	}
	f.svc.RunDue(ctx, now.Add(3*time.Minute))
	if len(p.got) != 1 {
		t.Fatal("alarm sent twice")
	}
}

func TestRunDueDropsStale(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	p := newPushServer(t, f, http.StatusCreated)
	ep := p.URL + "/push/1"
	if err := f.svc.Subscribe(ctx, f.alice, newReceiver(t).sub(ep)); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if _, err := f.svc.Schedule(ctx, f.alice, alarm(ep, now.Add(time.Minute))); err != nil {
		t.Fatal(err)
	}
	f.svc.RunDue(ctx, now.Add(2*time.Hour))
	if len(p.got) != 0 {
		t.Fatal("stale alarm sent")
	}
	if n, _ := f.svc.q.CountAlarmsBySubscription(ctx, f.subID(t, f.alice, ep)); n != 0 {
		t.Fatal("stale alarm kept")
	}
}

func TestRunDueGoneDeletesSubscription(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	p := newPushServer(t, f, http.StatusGone)
	ep := p.URL + "/push/1"
	if err := f.svc.Subscribe(ctx, f.alice, newReceiver(t).sub(ep)); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if _, err := f.svc.Schedule(ctx, f.alice, alarm(ep, now)); err != nil {
		t.Fatal(err)
	}
	f.svc.RunDue(ctx, now.Add(time.Second))
	if len(p.got) != 1 {
		t.Fatalf("%d requests", len(p.got))
	}
	if n, _ := f.svc.q.CountPushSubscriptionsByUser(ctx, f.alice); n != 0 {
		t.Fatal("gone subscription kept")
	}
}

func TestLimits(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	r := newReceiver(t)
	for i := range maxSubscriptions {
		if err := f.svc.Subscribe(ctx, f.alice, r.sub(fcm+string(rune('a'+i)))); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.svc.Subscribe(ctx, f.alice, r.sub(fcm+"-one-more")); !errors.Is(err, ErrTooMany) {
		t.Fatalf("subscription %d: %v", maxSubscriptions+1, err)
	}
	// Renewing a known endpoint is not a new one.
	if err := f.svc.Subscribe(ctx, f.alice, r.sub(fcm+"a")); err != nil {
		t.Fatalf("renew: %v", err)
	}
	for range maxAlarms {
		if _, err := f.svc.Schedule(ctx, f.alice, alarm(fcm+"a", time.Now().Add(time.Minute))); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := f.svc.Schedule(ctx, f.alice, alarm(fcm+"a", time.Now().Add(time.Minute))); !errors.Is(err, ErrTooMany) {
		t.Fatalf("alarm %d: %v", maxAlarms+1, err)
	}
}

func TestPublicKeyIsStable(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	k1, err := f.svc.PublicKey(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// A second service on the same database reads the stored key.
	k2, err := NewService(f.svc.conn, "", nil).PublicKey(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if k1 != k2 || len(k1) != 87 {
		t.Fatalf("keys %q and %q", k1, k2)
	}
}

// TestRunDueLogsNoEndpointPath: the path is the device's capability, and a
// failed POST's error would quote it.
func TestRunDueLogsNoEndpointPath(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	p := newPushServer(t, f, http.StatusCreated)
	ep := p.URL + "/push/secret-path"
	if err := f.svc.Subscribe(ctx, f.alice, newReceiver(t).sub(ep)); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if _, err := f.svc.Schedule(ctx, f.alice, alarm(ep, now)); err != nil {
		t.Fatal(err)
	}
	p.Close()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	f.svc.RunDue(ctx, now.Add(time.Second))
	if !strings.Contains(buf.String(), "push failed") || strings.Contains(buf.String(), "secret-path") {
		t.Fatalf("log: %s", buf.String())
	}
}
