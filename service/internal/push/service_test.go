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

	"github.com/s-frei/rezepte/service/internal/db"
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
		if err := sqlc.New(conn).CreateSession(context.Background(), sqlc.CreateSessionParams{
			ID: sess(u.ID), UserID: u.ID, ExpiresAt: db.FormatTime(time.Now().Add(time.Hour)), CreatedAt: db.FormatTime(time.Now()),
		}); err != nil {
			t.Fatal(err)
		}
	}
	return fixture{svc: NewService(conn, "https://rezepte.example", http.DefaultClient), alice: ids[0], bob: ids[1]}
}

// sess is the id of the session newFixture signs userID in with.
func sess(userID string) string { return "session-" + userID }

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
		"https://google.com/fcm/send/x",
		"https://evil.google.com/fcm/send/x",
		"/relative",
	} {
		if err := f.svc.Subscribe(context.Background(), f.alice, sess(f.alice), r.sub(ep)); !errors.Is(err, ErrForeignEndpoint) {
			t.Errorf("%s: err %v, want ErrForeignEndpoint", ep, err)
		}
	}
	for _, ep := range []string{
		fcm,
		"https://jmt17.google.com/fcm/send/x",
		"https://updates.push.services.mozilla.com/wpush/v2/x",
		"https://web.push.apple.com/x",
		"https://wns2-par02p.notify.windows.com/w/?token=x",
	} {
		if err := f.svc.Subscribe(context.Background(), f.alice, sess(f.alice), r.sub(ep)); err != nil {
			t.Errorf("%s: %v", ep, err)
		}
	}
}

func TestSubscribeRejectsBadKeys(t *testing.T) {
	f := newFixture(t)
	r := newReceiver(t)
	s := r.sub(fcm)
	s.P256dh = s.P256dh[:10]
	if err := f.svc.Subscribe(context.Background(), f.alice, sess(f.alice), s); !errors.Is(err, ErrInvalid) {
		t.Fatalf("short key: %v", err)
	}
	s = r.sub(fcm)
	s.Auth = []byte{1}
	if err := f.svc.Subscribe(context.Background(), f.alice, sess(f.alice), s); !errors.Is(err, ErrInvalid) {
		t.Fatalf("short auth: %v", err)
	}
}

func TestSubscribeMovesEndpointToNewUser(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	p := newPushServer(t, f, http.StatusCreated)
	r := newReceiver(t)
	ep := p.URL + "/push/1"
	if err := f.svc.Subscribe(ctx, f.alice, sess(f.alice), r.sub(ep)); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if _, err := f.svc.Schedule(ctx, f.alice, alarm(ep, now.Add(time.Minute))); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.Subscribe(ctx, f.bob, sess(f.bob), r.sub(ep)); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Schedule(ctx, f.alice, alarm(ep, now.Add(time.Minute))); !errors.Is(err, ErrUnknownSubscription) {
		t.Fatalf("alice still owns the endpoint: %v", err)
	}
	f.svc.RunDue(ctx, now.Add(2*time.Minute))
	if len(p.got) != 0 {
		t.Fatal("alice's alarm was sent to bob's device")
	}
	if _, err := f.svc.Schedule(ctx, f.bob, alarm(ep, now.Add(time.Minute))); err != nil {
		t.Fatalf("bob: %v", err)
	}
}

func TestScheduleNeedsOwnSubscription(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if err := f.svc.Subscribe(ctx, f.bob, sess(f.bob), newReceiver(t).sub(fcm)); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Schedule(ctx, f.alice, alarm(fcm, time.Now().Add(time.Minute))); !errors.Is(err, ErrUnknownSubscription) {
		t.Fatalf("err %v", err)
	}
}

func TestScheduleValidates(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if err := f.svc.Subscribe(ctx, f.alice, sess(f.alice), newReceiver(t).sub(fcm)); err != nil {
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
	if err := f.svc.Subscribe(ctx, f.bob, sess(f.bob), newReceiver(t).sub(fcm)); err != nil {
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
	byPath map[string]int // a status for one path instead of status
	after  func()         // runs while a request is being served
}

func newPushServer(t *testing.T, f fixture, status int) *pushServer {
	t.Helper()
	p := &pushServer{status: status}
	p.Server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		p.mu.Lock()
		p.got, p.bodies = append(p.got, r), append(p.bodies, b)
		p.mu.Unlock()
		if p.after != nil {
			p.after()
		}
		if st, ok := p.byPath[r.URL.Path]; ok {
			w.WriteHeader(st)
			return
		}
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
	if err := f.svc.Subscribe(ctx, f.alice, sess(f.alice), r.sub(ep)); err != nil {
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
	if err := f.svc.Subscribe(ctx, f.alice, sess(f.alice), newReceiver(t).sub(ep)); err != nil {
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
	if err := f.svc.Subscribe(ctx, f.alice, sess(f.alice), newReceiver(t).sub(ep)); err != nil {
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
		if err := f.svc.Subscribe(ctx, f.alice, sess(f.alice), r.sub(fcm+string(rune('a'+i)))); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.svc.Subscribe(ctx, f.alice, sess(f.alice), r.sub(fcm+"-one-more")); !errors.Is(err, ErrTooMany) {
		t.Fatalf("subscription %d: %v", maxSubscriptions+1, err)
	}
	// Renewing a known endpoint is not a new one.
	if err := f.svc.Subscribe(ctx, f.alice, sess(f.alice), r.sub(fcm+"a")); err != nil {
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
	if err := f.svc.Subscribe(ctx, f.alice, sess(f.alice), newReceiver(t).sub(ep)); err != nil {
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

func TestRunDueSkipsAlarmCancelledMidBatch(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	p := newPushServer(t, f, http.StatusCreated)
	ep := p.URL + "/push/1"
	if err := f.svc.Subscribe(ctx, f.alice, sess(f.alice), newReceiver(t).sub(ep)); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if _, err := f.svc.Schedule(ctx, f.alice, alarm(ep, now.Add(30*time.Second))); err != nil {
		t.Fatal(err)
	}
	second, err := f.svc.Schedule(ctx, f.alice, alarm(ep, now.Add(40*time.Second)))
	if err != nil {
		t.Fatal(err)
	}
	// The cook stops the second timer while the first is being sent.
	p.after = func() { _ = f.svc.Cancel(ctx, f.alice, second) }
	f.svc.RunDue(ctx, now.Add(time.Minute))
	if len(p.got) != 1 {
		t.Fatalf("%d requests, want 1: the cancelled alarm was sent", len(p.got))
	}
}

func message() Message {
	return Message{Kind: "comment", Title: "Rinderrouladen", Body: "Anna: Lecker!", URL: "/recipes/rinderrouladen?tab=comments", Tag: "comment:1",
		Collapse: &Collapse{Title: "Rinderrouladen", BodyOne: "Anna: Lecker!", BodyMany: "{count} new comments"}}
}

func (p *pushServer) paths() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	var out []string
	for _, r := range p.got {
		out = append(out, r.URL.Path)
	}
	return out
}

func TestSendToUserAllDevices(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	p := newPushServer(t, f, http.StatusCreated)
	r1, r2 := newReceiver(t), newReceiver(t)
	for i, r := range []receiver{r1, r2} {
		if err := f.svc.Subscribe(ctx, f.alice, sess(f.alice), r.sub(p.URL+"/push/"+string(rune('1'+i)))); err != nil {
			t.Fatal(err)
		}
	}
	m := message()
	f.svc.SendToUser(ctx, f.alice, m)
	if got := p.paths(); len(got) != 2 {
		t.Fatalf("requests to %v, want both devices", got)
	}
	for i, r := range []receiver{r1, r2} {
		var got Message
		if err := json.Unmarshal(r.decrypt(t, p.bodies[i]), &got); err != nil {
			t.Fatal(err)
		}
		if got.Kind != m.Kind || got.Title != m.Title || got.Body != m.Body || got.URL != m.URL || got.Tag != m.Tag || got.Collapse == nil || *got.Collapse != *m.Collapse {
			t.Errorf("device %d got %+v", i, got)
		}
	}
	var raw map[string]any
	_ = json.Unmarshal(r1.decrypt(t, p.bodies[0]), &raw)
	if c, _ := raw["collapse"].(map[string]any); c["bodyMany"] != "{count} new comments" || c["bodyOne"] == nil {
		t.Errorf("payload %v", raw)
	}
}

// TestSendToUserDropsGoneKeepsOthers: one dead device among several is
// dropped, and the others still get the push.
func TestSendToUserDropsGoneKeepsOthers(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	p := newPushServer(t, f, http.StatusCreated)
	p.byPath = map[string]int{"/push/1": http.StatusGone}
	for _, path := range []string{"/push/1", "/push/2"} {
		if err := f.svc.Subscribe(ctx, f.alice, sess(f.alice), newReceiver(t).sub(p.URL+path)); err != nil {
			t.Fatal(err)
		}
	}
	f.svc.SendToUser(ctx, f.alice, message())
	if got := p.paths(); len(got) != 2 || got[0] != "/push/1" || got[1] != "/push/2" {
		t.Fatalf("requests to %v", got)
	}
	if n, _ := f.svc.q.CountPushSubscriptionsByUser(ctx, f.alice); n != 1 {
		t.Fatalf("%d subscriptions, want the live one", n)
	}
	f.subID(t, f.alice, p.URL+"/push/2")
}

// TestSendToUserAfterAccountMove: a device another member signed in on no
// longer gets the first account's notifications.
func TestSendToUserAfterAccountMove(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	p := newPushServer(t, f, http.StatusCreated)
	r := newReceiver(t)
	ep := p.URL + "/push/1"
	if err := f.svc.Subscribe(ctx, f.alice, sess(f.alice), r.sub(ep)); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.Subscribe(ctx, f.bob, sess(f.bob), r.sub(ep)); err != nil {
		t.Fatal(err)
	}
	f.svc.SendToUser(ctx, f.alice, message())
	if len(p.paths()) != 0 {
		t.Fatal("alice's notification reached bob's device")
	}
	f.svc.SendToUser(ctx, f.bob, message())
	if len(p.paths()) != 1 {
		t.Fatalf("%d requests for bob, want 1", len(p.paths()))
	}
}

func TestSendTestOnlyOwnEndpoint(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	p := newPushServer(t, f, http.StatusCreated)
	ep := p.URL + "/push/1"
	if err := f.svc.Subscribe(ctx, f.bob, sess(f.bob), newReceiver(t).sub(ep)); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.SendTest(ctx, f.alice, ep, message()); !errors.Is(err, ErrUnknownSubscription) {
		t.Fatalf("err %v, want ErrUnknownSubscription", err)
	}
	if len(p.paths()) != 0 {
		t.Fatal("test pushed to another account's device")
	}
	if err := f.svc.SendTest(ctx, f.bob, ep, message()); err != nil {
		t.Fatal(err)
	}
	if len(p.paths()) != 1 {
		t.Fatalf("%d requests, want 1", len(p.paths()))
	}
}

func TestSendTestGoneIsUnknown(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	p := newPushServer(t, f, http.StatusGone)
	ep := p.URL + "/push/1"
	if err := f.svc.Subscribe(ctx, f.alice, sess(f.alice), newReceiver(t).sub(ep)); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.SendTest(ctx, f.alice, ep, message()); !errors.Is(err, ErrUnknownSubscription) {
		t.Fatalf("err %v, want ErrUnknownSubscription", err)
	}
	if n, _ := f.svc.q.CountPushSubscriptionsByUser(ctx, f.alice); n != 0 {
		t.Fatal("gone subscription kept")
	}
}

func TestSendTestThrottled(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	p := newPushServer(t, f, http.StatusCreated)
	ep := p.URL + "/push/1"
	if err := f.svc.Subscribe(ctx, f.alice, sess(f.alice), newReceiver(t).sub(ep)); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	f.svc.now = func() time.Time { return now }
	if err := f.svc.SendTest(ctx, f.alice, ep, message()); err != nil {
		t.Fatal(err)
	}
	now = now.Add(9 * time.Second)
	if err := f.svc.SendTest(ctx, f.alice, ep, message()); !errors.Is(err, ErrTooSoon) {
		t.Fatalf("err %v, want ErrTooSoon", err)
	}
	now = now.Add(time.Second)
	if err := f.svc.SendTest(ctx, f.alice, ep, message()); err != nil {
		t.Fatalf("after 10 s: %v", err)
	}
	if len(p.paths()) != 2 {
		t.Fatalf("%d requests, want 2", len(p.paths()))
	}
}

func TestTimerPayloadHasKind(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	p := newPushServer(t, f, http.StatusCreated)
	r := newReceiver(t)
	ep := p.URL + "/push/1"
	if err := f.svc.Subscribe(ctx, f.alice, sess(f.alice), r.sub(ep)); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if _, err := f.svc.Schedule(ctx, f.alice, alarm(ep, now)); err != nil {
		t.Fatal(err)
	}
	f.svc.RunDue(ctx, now.Add(time.Second))
	if len(p.bodies) != 1 {
		t.Fatalf("%d requests", len(p.bodies))
	}
	var payload map[string]any
	if err := json.Unmarshal(r.decrypt(t, p.bodies[0]), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["kind"] != "timer" {
		t.Fatalf("payload %v", payload)
	}
	if _, ok := payload["collapse"]; ok {
		t.Fatalf("timer payload carries collapse: %v", payload)
	}
}

// TestSendToUserSkipsEndedSessions: a device whose session was deleted (sign
// out, password change, admin reset) or has expired without being swept
// yet gets nothing; the member's other devices still do.
func TestSendToUserSkipsEndedSessions(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	p := newPushServer(t, f, http.StatusCreated)
	for _, s := range []string{"live", "out", "expired"} {
		if s != "live" {
			if err := f.svc.q.CreateSession(ctx, sqlc.CreateSessionParams{
				ID: s, UserID: f.alice, ExpiresAt: db.FormatTime(time.Now().Add(time.Hour)), CreatedAt: db.FormatTime(time.Now()),
			}); err != nil {
				t.Fatal(err)
			}
		}
		sid := s
		if s == "live" {
			sid = sess(f.alice)
		}
		if err := f.svc.Subscribe(ctx, f.alice, sid, newReceiver(t).sub(p.URL+"/push/"+s)); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.svc.q.DeleteSession(ctx, "out"); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.q.ExtendSession(ctx, sqlc.ExtendSessionParams{ID: "expired", ExpiresAt: db.FormatTime(time.Now().Add(-time.Second))}); err != nil {
		t.Fatal(err)
	}
	f.svc.SendToUser(ctx, f.alice, message())
	if got := p.paths(); len(got) != 1 || got[0] != "/push/live" {
		t.Fatalf("requests to %v, want only the live session's device", got)
	}
}
