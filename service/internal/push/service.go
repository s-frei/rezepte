package push

import (
	"bytes"
	"context"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
	"uuid"

	"github.com/s-frei/rezepte/service/internal/db"
	"github.com/s-frei/rezepte/service/internal/db/sqlc"
)

// The limits a member's devices and alarms stay within.
const (
	maxSubscriptions = 20
	maxAlarms        = 50
	maxAhead         = 7 * 24 * time.Hour
	// maxLate is how far in the past a new alarm may lie: a timer that ended
	// on the way to the server still gets its push.
	maxLate = time.Minute
	// staleAfter drops an alarm unsent: the cook has long seen the banner
	// or missed it.
	staleAfter  = time.Hour
	sendTimeout = 10 * time.Second
	// testEvery is how often one device may be sent a test push.
	testEvery = 10 * time.Second
)

// tick is how often Start sends due alarms.
var tick = 5 * time.Second

// pushHosts are the push services an endpoint may name; a host equal to one
// or below it passes. Anything else would let a member make the server POST
// to a host of their choosing, an internal one included.
var pushHosts = []string{
	"fcm.googleapis.com",
	// Chrome's newer endpoints; only this host, never all of google.com.
	"jmt17.google.com",
	"updates.push.services.mozilla.com",
	"push.services.mozilla.com",
	"web.push.apple.com",
	"notify.windows.com",
}

// Errors the handler maps to responses.
var (
	ErrForeignEndpoint     = errors.New("endpoint is not a known push service")
	ErrUnknownSubscription = errors.New("no such subscription for this account")
	ErrTooMany             = errors.New("too many subscriptions or alarms")
	ErrInvalid             = errors.New("invalid subscription or alarm")
	ErrTooSoon             = errors.New("a test push was just sent to this device")
)

// Message is a notification push to a member's devices. The service worker
// reads Kind to tell it from a timer alarm; pushes with one Tag replace
// each other, and Collapse is what it shows when one is already there.
type Message struct {
	Kind     string    `json:"kind"`
	Title    string    `json:"title"`
	Body     string    `json:"body"`
	URL      string    `json:"url"`
	Tag      string    `json:"tag"`
	Collapse *Collapse `json:"collapse,omitempty"`
}

// Collapse is the copy for a push that replaces one already shown with its
// tag. BodyMany holds a literal {count} the service worker fills.
type Collapse struct {
	Title    string `json:"title"`
	BodyOne  string `json:"bodyOne"`
	BodyMany string `json:"bodyMany"`
}

// Alarm is a push to send to a device of the member at FireAt.
type Alarm struct {
	Endpoint string
	FireAt   time.Time
	Title    string
	Body     string
	URL      string
}

// Service stores subscriptions and alarms and sends the due ones.
type Service struct {
	conn      *sql.DB
	q         *sqlc.Queries
	subject   string
	client    *http.Client
	allowHost func(string) bool
	now       func() time.Time

	keyMu sync.Mutex
	key   *ecdsa.PrivateKey

	testMu   sync.Mutex
	lastTest map[string]time.Time // by endpoint
}

// NewService returns a Service on conn. subject is the instance's public URL,
// which push services may contact the operator through; without an https
// one it is "mailto:rezepte@localhost". client is copied with a timeout and
// without redirects: a push service answers, it does not send us elsewhere.
func NewService(conn *sql.DB, subject string, client *http.Client) *Service {
	if !strings.HasPrefix(subject, "https://") {
		subject = "mailto:rezepte@localhost"
	}
	c := http.Client{}
	if client != nil {
		c = *client
	}
	if c.Timeout == 0 {
		c.Timeout = sendTimeout
	}
	c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &Service{conn: conn, q: sqlc.New(conn), subject: subject, client: &c, allowHost: knownPushHost,
		now: time.Now, lastTest: map[string]time.Time{}}
}

func knownPushHost(host string) bool {
	for _, h := range pushHosts {
		if host == h || strings.HasSuffix(host, "."+h) {
			return true
		}
	}
	return false
}

func (s *Service) checkEndpoint(endpoint string) error {
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "https" || u.User != nil || !s.allowHost(u.Hostname()) {
		return ErrForeignEndpoint
	}
	return nil
}

// vapidKey returns the instance's signing key, generating and storing it on
// first use.
func (s *Service) vapidKey(ctx context.Context) (*ecdsa.PrivateKey, error) {
	s.keyMu.Lock()
	defer s.keyMu.Unlock()
	if s.key != nil {
		return s.key, nil
	}
	raw, err := s.q.GetVapidKey(ctx)
	if err != nil {
		return nil, fmt.Errorf("get vapid key: %w", err)
	}
	if raw == nil {
		k, err := NewVAPIDKey()
		if err != nil {
			return nil, err
		}
		if raw, err = MarshalVAPIDKey(k); err != nil {
			return nil, err
		}
		if err := s.q.SetVapidKeyIfMissing(ctx, raw); err != nil {
			return nil, fmt.Errorf("store vapid key: %w", err)
		}
		// Another process may have stored its key first; that one wins.
		if raw, err = s.q.GetVapidKey(ctx); err != nil {
			return nil, fmt.Errorf("get vapid key: %w", err)
		}
	}
	k, err := ParseVAPIDKey(raw)
	if err != nil {
		return nil, err
	}
	s.key = k
	return k, nil
}

// PublicKey returns the applicationServerKey browsers subscribe with,
// generating and storing the key pair on first use.
func (s *Service) PublicKey(ctx context.Context) (string, error) {
	k, err := s.vapidKey(ctx)
	if err != nil {
		return "", err
	}
	return PublicKey(k), nil
}

// Subscribe stores sub for userID. An endpoint another account had moves to
// this one, without that account's alarms.
func (s *Service) Subscribe(ctx context.Context, userID string, sub Subscription) error {
	if err := s.checkEndpoint(sub.Endpoint); err != nil {
		return err
	}
	if _, err := ecdh.P256().NewPublicKey(sub.P256dh); err != nil || len(sub.Auth) != 16 {
		return ErrInvalid
	}
	return db.Tx(ctx, s.conn, func(q *sqlc.Queries) error {
		_, err := q.GetPushSubscriptionForUser(ctx, sqlc.GetPushSubscriptionForUserParams{UserID: userID, Endpoint: sub.Endpoint})
		if errors.Is(err, sql.ErrNoRows) {
			n, err := q.CountPushSubscriptionsByUser(ctx, userID)
			if err != nil {
				return fmt.Errorf("count subscriptions: %w", err)
			}
			if n >= maxSubscriptions {
				return ErrTooMany
			}
			if err := q.DeletePushSubscriptionByEndpoint(ctx, sub.Endpoint); err != nil {
				return fmt.Errorf("release endpoint: %w", err)
			}
		} else if err != nil {
			return fmt.Errorf("get subscription: %w", err)
		}
		err = q.UpsertPushSubscription(ctx, sqlc.UpsertPushSubscriptionParams{
			ID: uuid.NewV7().String(), UserID: userID, Endpoint: sub.Endpoint,
			P256dh: sub.P256dh, Auth: sub.Auth, CreatedAt: db.FormatTime(time.Now()),
		})
		if err != nil {
			return fmt.Errorf("store subscription: %w", err)
		}
		return nil
	})
}

// Unsubscribe deletes userID's subscription of endpoint and its alarms; one
// that is not there is no error.
func (s *Service) Unsubscribe(ctx context.Context, userID, endpoint string) error {
	if err := s.q.DeletePushSubscription(ctx, sqlc.DeletePushSubscriptionParams{UserID: userID, Endpoint: endpoint}); err != nil {
		return fmt.Errorf("delete subscription: %w", err)
	}
	return nil
}

// Schedule stores a for userID's subscription of a.Endpoint and returns its
// id, which the push carries as its tag.
func (s *Service) Schedule(ctx context.Context, userID string, a Alarm) (string, error) {
	now := time.Now()
	if a.FireAt.Before(now.Add(-maxLate)) || a.FireAt.After(now.Add(maxAhead)) || !strings.HasPrefix(a.URL, "/recipes/") {
		return "", ErrInvalid
	}
	id := uuid.NewV7().String()
	err := db.Tx(ctx, s.conn, func(q *sqlc.Queries) error {
		sub, err := q.GetPushSubscriptionForUser(ctx, sqlc.GetPushSubscriptionForUserParams{UserID: userID, Endpoint: a.Endpoint})
		if errors.Is(err, sql.ErrNoRows) {
			return ErrUnknownSubscription
		}
		if err != nil {
			return fmt.Errorf("get subscription: %w", err)
		}
		n, err := q.CountAlarmsBySubscription(ctx, sub.ID)
		if err != nil {
			return fmt.Errorf("count alarms: %w", err)
		}
		if n >= maxAlarms {
			return ErrTooMany
		}
		err = q.InsertTimerAlarm(ctx, sqlc.InsertTimerAlarmParams{
			ID: id, SubscriptionID: sub.ID, FireAt: db.FormatTime(a.FireAt),
			Title: a.Title, Body: a.Body, Url: a.URL, CreatedAt: db.FormatTime(now),
		})
		if err != nil {
			return fmt.Errorf("insert alarm: %w", err)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return id, nil
}

// Cancel deletes the alarm id if it belongs to one of userID's
// subscriptions. One already sent or never there is no error: the cook
// paused or stopped a timer, and that is what they wanted.
func (s *Service) Cancel(ctx context.Context, userID, id string) error {
	if err := s.q.DeleteTimerAlarmForUser(ctx, sqlc.DeleteTimerAlarmForUserParams{ID: id, UserID: userID}); err != nil {
		return fmt.Errorf("delete alarm: %w", err)
	}
	return nil
}

// Start sends due alarms every tick, until ctx ends, in its own goroutine.
// The returned channel closes once it has stopped, after a send under way.
func (s *Service) Start(ctx context.Context) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.tick(ctx)
			}
		}
	}()
	return done
}

func (s *Service) tick(ctx context.Context) {
	// A panic would end the process, and with it every later alarm.
	defer func() {
		if r := recover(); r != nil {
			slog.ErrorContext(ctx, "push sender panicked", "panic", r)
		}
	}()
	s.RunDue(ctx, time.Now())
}

// RunDue sends every alarm due at now and deletes it, sent or not: there is
// no retry. One more than staleAfter late is deleted unsent. A push service
// answering 404 or 410 has forgotten the device, so its subscription goes.
func (s *Service) RunDue(ctx context.Context, now time.Time) {
	due, err := s.q.ListDueTimerAlarms(ctx, db.FormatTime(now))
	if err != nil {
		slog.WarnContext(ctx, "list due alarms", "err", err)
		return
	}
	for _, a := range due {
		// Stop between alarms on shutdown; the rest are sent after a restart.
		if ctx.Err() != nil {
			return
		}
		// Deleted before the send, so a crash mid-send cannot send it twice.
		// No row deleted: the cook cancelled it after the list was read.
		if n, err := s.q.DeleteTimerAlarm(ctx, a.ID); err != nil {
			slog.WarnContext(ctx, "delete alarm", "err", err)
			continue
		} else if n == 0 {
			continue
		}
		if fireAt, err := db.ParseTime(a.FireAt); err != nil || now.Sub(fireAt) > staleAfter {
			continue
		}
		// The send finishes on shutdown; the client's timeout bounds it.
		s.sendAlarm(context.WithoutCancel(ctx), a)
	}
}

// SendToUser pushes m to every device of userID, each with its own
// timeout. A device the push service has forgotten is dropped and the rest
// still get it; failures are logged, never retried.
func (s *Service) SendToUser(ctx context.Context, userID string, m Message) {
	subs, err := s.q.ListPushSubscriptionsOfUser(ctx, userID)
	if err != nil {
		slog.WarnContext(ctx, "list subscriptions", "err", err)
		return
	}
	payload, err := json.Marshal(m)
	if err != nil {
		slog.WarnContext(ctx, "push payload", "err", err)
		return
	}
	for _, sub := range subs {
		sctx, cancel := context.WithTimeout(ctx, sendTimeout)
		s.send(sctx, sub.Endpoint, sub.P256dh, sub.Auth, payload)
		cancel()
	}
}

// SendTest pushes m to userID's device at endpoint only, at most once per
// testEvery for that device. A device the push service has forgotten is
// dropped and reported as ErrUnknownSubscription.
func (s *Service) SendTest(ctx context.Context, userID, endpoint string, m Message) error {
	sub, err := s.q.GetPushSubscriptionForUser(ctx, sqlc.GetPushSubscriptionForUserParams{UserID: userID, Endpoint: endpoint})
	if errors.Is(err, sql.ErrNoRows) {
		return ErrUnknownSubscription
	}
	if err != nil {
		return fmt.Errorf("get subscription: %w", err)
	}
	if !s.claimTest(endpoint) {
		return ErrTooSoon
	}
	payload, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("payload: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, sendTimeout)
	defer cancel()
	return s.post(ctx, sub.Endpoint, sub.P256dh, sub.Auth, payload)
}

// claimTest reports whether endpoint may get a test now and records it.
// ponytail: in-memory, a restart forgets the last test.
func (s *Service) claimTest(endpoint string) bool {
	s.testMu.Lock()
	defer s.testMu.Unlock()
	now := s.now()
	for ep, at := range s.lastTest {
		if now.Sub(at) >= testEvery {
			delete(s.lastTest, ep)
		}
	}
	if _, ok := s.lastTest[endpoint]; ok {
		return false
	}
	s.lastTest[endpoint] = now
	return true
}

func (s *Service) sendAlarm(ctx context.Context, a sqlc.ListDueTimerAlarmsRow) {
	payload, err := json.Marshal(Message{Kind: "timer", Title: a.Title, Body: a.Body, URL: a.Url, Tag: a.ID})
	if err != nil {
		slog.WarnContext(ctx, "push payload", "err", err)
		return
	}
	s.send(ctx, a.Endpoint, a.P256dh, a.Auth, payload)
}

func (s *Service) send(ctx context.Context, endpoint string, p256dh, auth, payload []byte) {
	host := endpoint
	if u, err := url.Parse(endpoint); err == nil {
		host = u.Host // the path is the device's capability; it stays out of the log
	}
	// A forgotten device was dropped by post; nothing to warn about.
	if err := s.post(ctx, endpoint, p256dh, auth, payload); err != nil && !errors.Is(err, ErrUnknownSubscription) {
		slog.WarnContext(ctx, "push failed", "host", host, "err", err)
	}
}

func (s *Service) post(ctx context.Context, endpoint string, p256dh, auth, payload []byte) error {
	// The endpoint was checked when stored; checked again so that a row
	// from before a narrower allowlist is never reached.
	if err := s.checkEndpoint(endpoint); err != nil {
		return err
	}
	key, err := s.vapidKey(ctx)
	if err != nil {
		return err
	}
	salt := make([]byte, 16)
	_, _ = rand.Read(salt)
	server, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("sender key: %w", err)
	}
	body, err := Encrypt(Subscription{Endpoint: endpoint, P256dh: p256dh, Auth: auth}, payload, salt, server)
	if err != nil {
		return err
	}
	vapid, err := VAPID(key, endpoint, s.subject, time.Now())
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("request: %w", err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Content-Encoding", "aes128gcm")
	req.Header.Set("TTL", "3600")
	req.Header.Set("Urgency", "high")
	req.Header.Set("Authorization", vapid)
	resp, err := s.client.Do(req)
	if err != nil {
		// A *url.Error quotes the whole endpoint; only its cause is logged.
		if ue, ok := errors.AsType[*url.Error](err); ok {
			err = ue.Err
		}
		return fmt.Errorf("post: %w", err)
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	_ = resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone:
		if err := s.q.DeletePushSubscriptionByEndpoint(ctx, endpoint); err != nil {
			return fmt.Errorf("delete gone subscription: %w", err)
		}
		return ErrUnknownSubscription
	case resp.StatusCode >= 300:
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}
