package notify_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"maps"
	mpart "mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/s-frei/rezepte/service/internal/auth"
	"github.com/s-frei/rezepte/service/internal/config"
	"github.com/s-frei/rezepte/service/internal/db/dbtest"
	"github.com/s-frei/rezepte/service/internal/httpserver"
	"github.com/s-frei/rezepte/service/internal/mail"
	"github.com/s-frei/rezepte/service/internal/notify"
	"github.com/s-frei/rezepte/service/internal/sign"
	"github.com/s-frei/rezepte/service/internal/user"
)

const publicURL = "https://rezepte.example.org"

var testKey = bytes.Repeat([]byte{7}, 32)

type mailOff struct{}

func (mailOff) Enabled(context.Context) bool                    { return false }
func (mailOff) SendComment(context.Context, mail.Comment) error { return mail.ErrDisabled }
func (mailOff) SendInviteAccepted(context.Context, mail.InviteAccepted) error {
	return mail.ErrDisabled
}
func (mailOff) SendRecipeDigest(context.Context, mail.RecipeDigest) error { return mail.ErrDisabled }

type fixture struct {
	h     http.Handler
	svc   *notify.Service
	users map[string]user.User
	us    *user.Service
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	conn := dbtest.Open(t)
	users := user.NewService(conn, "")
	f := fixture{users: map[string]user.User{}, us: users}
	for name, role := range map[string]user.Role{"olga": user.RoleSuperadmin, "adam": user.RoleAdmin, "mia": user.RoleUser} {
		u, err := users.Create(context.Background(), user.CreateParams{Username: name, Password: "pw", Role: role})
		if err != nil {
			t.Fatal(err)
		}
		f.users[name] = u
	}
	cfg, _ := config.LoadFrom(map[string]string{})
	sessions := auth.NewService(conn, users)
	tokens := auth.NewTokenService(conn, users)
	srv := httpserver.New(cfg, slog.New(slog.DiscardHandler), fstest.MapFS{},
		httpserver.WithAPIMiddleware(auth.Middleware(sessions, tokens, false)))
	auth.Register(srv.API(), sessions, false)
	f.svc = notify.NewService(conn, testKey, users, nil, nil, mailOff{}, publicURL)
	notify.Register(srv.API(), f.svc)
	f.h = srv.Handler()
	return f
}

func (f fixture) do(method, path, contentType, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Host = "localhost:8060"
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	f.h.ServeHTTP(rec, req)
	return rec
}

func (f fixture) login(t *testing.T, username string) *http.Cookie {
	t.Helper()
	rec := f.do(http.MethodPost, "/api/v1/auth/login", "application/json", `{"username":"`+username+`","password":"pw"}`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("login %s: %d %s", username, rec.Code, rec.Body.String())
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == auth.CookieName {
			return c
		}
	}
	t.Fatal("no session cookie")
	return nil
}

// prefsBody is the wire shape of the preferences, a channel map per kind
// and per group.
type prefsBody struct {
	Kinds  map[string]map[string]bool `json:"kinds"`
	Groups map[string]map[string]bool `json:"groups"`
}

func decodePrefs(t *testing.T, rec *httptest.ResponseRecorder) prefsBody {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var body prefsBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return body
}

// mailKinds is every kind's own mail choice.
func mailKinds(p prefsBody) map[string]bool {
	got := map[string]bool{}
	for k, chs := range p.Kinds {
		if on, ok := chs["mail"]; ok {
			got[k] = on
		}
	}
	return got
}

const prefsPath = "/api/v1/auth/me/notifications"

func TestPreferencesDefaultOn(t *testing.T) {
	f := newFixture(t)
	for _, name := range []string{"olga", "adam"} {
		got := mailKinds(decodePrefs(t, f.do(http.MethodGet, prefsPath, "", "", f.login(t, name))))
		if want := map[string]bool{"comment": true, "recipe": true, "tasty": true, "invite_accepted": true}; !maps.Equal(got, want) {
			t.Errorf("%s: %v, want %v", name, got, want)
		}
	}
	if rec := f.do(http.MethodGet, prefsPath, "", "", nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("without session: %d", rec.Code)
	}
}

func TestMemberHasNoAdminKinds(t *testing.T) {
	f := newFixture(t)
	mia := f.login(t, "mia")
	got := decodePrefs(t, f.do(http.MethodGet, prefsPath, "", "", mia))
	if want := map[string]bool{"comment": true, "recipe": true, "tasty": true}; !maps.Equal(mailKinds(got), want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if _, ok := got.Groups["admin"]; ok {
		t.Errorf("member has the admin group: %v", got.Groups)
	}
	for _, body := range []string{
		`{"kinds":{"invite_accepted":{"mail":false}}}`,
		`{"kinds":{"nonsense":{"mail":false}}}`,
		`{"kinds":{"comment":{"sms":false}}}`,
		`{"groups":{"admin":{"push":false}}}`,
	} {
		if rec := f.do(http.MethodPut, prefsPath, "application/json", body, mia); rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("put %s: %d %s", body, rec.Code, rec.Body.String())
		}
	}
	if ok, _ := f.svc.Wants(context.Background(), f.users["mia"].ID, notify.KindComment, notify.ChannelMail); !ok {
		t.Error("refused write changed comment")
	}
}

func TestSetPreferencesRoundTrip(t *testing.T) {
	f := newFixture(t)
	adam := f.login(t, "adam")
	got := decodePrefs(t, f.do(http.MethodPut, prefsPath, "application/json", `{"kinds":{"comment":{"mail":false}}}`, adam))
	want := map[string]bool{"comment": false, "recipe": true, "tasty": true, "invite_accepted": true}
	if !maps.Equal(mailKinds(got), want) || !got.Kinds["comment"]["push"] {
		t.Errorf("put: %v, want mail %v", got, want)
	}
	if got := mailKinds(decodePrefs(t, f.do(http.MethodGet, prefsPath, "", "", adam))); !maps.Equal(got, want) {
		t.Errorf("get after put: %v, want %v", got, want)
	}
	// Writing the same value twice is not an error (the opt-out insert ignores a conflict).
	decodePrefs(t, f.do(http.MethodPut, prefsPath, "application/json", `{"kinds":{"comment":{"mail":false}}}`, adam))
	got = decodePrefs(t, f.do(http.MethodPut, prefsPath, "application/json", `{"kinds":{"comment":{"mail":true},"invite_accepted":{"mail":false}}}`, adam))
	if want := map[string]bool{"comment": true, "recipe": true, "tasty": true, "invite_accepted": false}; !maps.Equal(mailKinds(got), want) {
		t.Errorf("second put: %v, want %v", got, want)
	}
	// Another account's preferences are its own.
	if got := mailKinds(decodePrefs(t, f.do(http.MethodGet, prefsPath, "", "", f.login(t, "olga")))); !maps.Equal(got, map[string]bool{"comment": true, "recipe": true, "tasty": true, "invite_accepted": true}) {
		t.Errorf("olga: %v", got)
	}
}

func TestPreferencesHandlerShape(t *testing.T) {
	f := newFixture(t)
	olga := f.login(t, "olga")
	on := map[string]bool{"mail": true, "push": true}
	got := decodePrefs(t, f.do(http.MethodGet, prefsPath, "", "", olga))
	for _, k := range []string{"comment", "recipe", "tasty", "invite_accepted"} {
		if !maps.Equal(got.Kinds[k], on) {
			t.Errorf("get %s: %v", k, got.Kinds[k])
		}
	}
	if !maps.Equal(got.Kinds["mail_failing"], map[string]bool{"push": true}) || !maps.Equal(got.Groups["household"], on) || !maps.Equal(got.Groups["admin"], on) {
		t.Errorf("get: %+v", got)
	}
	got = decodePrefs(t, f.do(http.MethodPut, prefsPath, "application/json", `{"groups":{"household":{"push":false}}}`, olga))
	if !maps.Equal(got.Groups["household"], map[string]bool{"mail": true, "push": false}) || !maps.Equal(got.Groups["admin"], on) {
		t.Errorf("put groups: %v", got.Groups)
	}
	for _, k := range []string{"comment", "recipe", "tasty"} {
		if !got.Kinds[k]["push"] {
			t.Errorf("put changed %s's own push choice: %v", k, got.Kinds[k])
		}
	}
}

type state struct {
	Username        string          `json:"username"`
	Email           string          `json:"email"`
	Kind            string          `json:"kind"`
	Group           string          `json:"group"`
	KindSubscribed  bool            `json:"kindSubscribed"`
	GroupSubscribed bool            `json:"groupSubscribed"`
	GroupKinds      map[string]bool `json:"groupKinds"`
}

func decodeState(t *testing.T, rec *httptest.ResponseRecorder) state {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var s state
	if err := json.Unmarshal(rec.Body.Bytes(), &s); err != nil {
		t.Fatal(err)
	}
	return s
}

// query returns the u, k and s parameters of a mailed link.
func query(t *testing.T, link string) url.Values {
	t.Helper()
	u, err := url.Parse(link)
	if err != nil {
		t.Fatal(err)
	}
	return u.Query()
}

func getPath(q url.Values) string {
	return "/api/v1/unsubscribe?" + url.Values{"u": q["u"], "k": q["k"], "s": q["s"]}.Encode()
}

func postBody(q url.Values, scope string, subscribed bool) string {
	b, _ := json.Marshal(map[string]any{"u": q.Get("u"), "k": q.Get("k"), "s": q.Get("s"), "scope": scope, "subscribed": subscribed})
	return string(b)
}

func TestUnsubscribeTokenBoundToUserAndKind(t *testing.T) {
	f := newFixture(t)
	q := query(t, f.svc.UnsubscribeLinks(f.users["adam"].ID, notify.KindComment).KindURL)
	if got := decodeState(t, f.do(http.MethodGet, getPath(q), "", "", nil)); got.Username != "adam" || got.Kind != "comment" || got.Group != "household" {
		t.Errorf("own link: %+v", got)
	}
	signer := sign.New(testKey, "rezepte unsubscribe")
	ghost, _ := signer.Sign("ghost:comment", time.Hour)
	unknownKind, _ := signer.Sign(f.users["adam"].ID+":digest", time.Hour)
	// A member may not have invite_accepted, even with a valid signature.
	memberAdmin, _ := signer.Sign(f.users["mia"].ID+":invite_accepted", time.Hour)
	for name, v := range map[string]url.Values{
		"other user":   {"u": {f.users["mia"].ID}, "k": q["k"], "s": q["s"]},
		"other kind":   {"u": q["u"], "k": {"invite_accepted"}, "s": q["s"]},
		"other key":    {"u": q["u"], "k": q["k"], "s": {"x" + q.Get("s")}},
		"unknown user": {"u": {"ghost"}, "k": {"comment"}, "s": {ghost}},
		"unknown kind": {"u": q["u"], "k": {"digest"}, "s": {unknownKind}},
		"kind of role": {"u": {f.users["mia"].ID}, "k": {"invite_accepted"}, "s": {memberAdmin}},
	} {
		if rec := f.do(http.MethodGet, getPath(v), "", "", nil); rec.Code != http.StatusGone {
			t.Errorf("get %s: %d %s", name, rec.Code, rec.Body.String())
		}
		if rec := f.do(http.MethodPost, "/api/v1/unsubscribe", "application/json", postBody(v, "group", false), nil); rec.Code != http.StatusGone {
			t.Errorf("post %s: %d %s", name, rec.Code, rec.Body.String())
		}
		if rec := f.do(http.MethodPost, "/api/v1/unsubscribe/one-click?"+v.Encode(), "application/x-www-form-urlencoded", "List-Unsubscribe=One-Click", nil); rec.Code != http.StatusGone {
			t.Errorf("one-click %s: %d %s", name, rec.Code, rec.Body.String())
		}
	}
	for _, name := range []string{"adam", "mia"} {
		got, err := f.svc.Preferences(context.Background(), f.users[name])
		if err != nil {
			t.Fatal(err)
		}
		for k, chs := range got.Kinds {
			for ch, on := range chs {
				if !on {
					t.Errorf("%s: %s/%s switched off by a refused link", name, k, ch)
				}
			}
		}
	}
}

func TestUnsubscribeExpired(t *testing.T) {
	f := newFixture(t)
	// The service signs for 90 days; a token signed that long ago, plus a day,
	// is one signed with a TTL a day in the past.
	signer := sign.New(testKey, "rezepte unsubscribe")
	id := f.users["mia"].ID
	old, _ := signer.Sign(id+":comment", -24*time.Hour)
	if rec := f.do(http.MethodGet, getPath(url.Values{"u": {id}, "k": {"comment"}, "s": {old}}), "", "", nil); rec.Code != http.StatusGone {
		t.Errorf("expired: %d %s", rec.Code, rec.Body.String())
	}
	// And the links the service makes last the 90 days: the token starts
	// with its expiry in Unix seconds, base 36.
	q := query(t, f.svc.UnsubscribeLinks(id, notify.KindComment).KindURL)
	exp, _, _ := strings.Cut(q.Get("s"), ".")
	unix, err := strconv.ParseInt(exp, 36, 64)
	if want := time.Now().Add(90 * 24 * time.Hour).Unix(); err != nil || unix < want-2 || unix > want+2 {
		t.Errorf("expiry %d, want about %d (%v)", unix, want, err)
	}
}

func TestUnsubscribeGetChangesNothing(t *testing.T) {
	f := newFixture(t)
	links := f.svc.UnsubscribeLinks(f.users["mia"].ID, notify.KindComment)
	if !strings.HasPrefix(links.KindURL, publicURL+"/unsubscribe?") || links.GroupURL != links.KindURL+"&scope=group" ||
		!strings.HasPrefix(links.OneClickURL, publicURL+"/api/v1/unsubscribe/one-click?") {
		t.Errorf("links: %+v", links)
	}
	for range 2 {
		got := decodeState(t, f.do(http.MethodGet, getPath(query(t, links.KindURL)), "", "", nil))
		if !got.KindSubscribed || !got.GroupSubscribed {
			t.Errorf("get: %+v", got)
		}
	}
	// A crawler following the one-click link with GET changes nothing either.
	if rec := f.do(http.MethodGet, strings.TrimPrefix(links.OneClickURL, publicURL), "", "", nil); rec.Code == http.StatusOK {
		t.Errorf("GET one-click: %d", rec.Code)
	}
	if ok, err := f.svc.Wants(context.Background(), f.users["mia"].ID, notify.KindComment, notify.ChannelMail); err != nil || !ok {
		t.Errorf("wants after GETs: %v %v", ok, err)
	}
}

// Whoever holds an old link may be the old mailbox: after an address change
// the page shows the new address masked, as the address-changed notice does.
func TestUnsubscribeMasksTheAddress(t *testing.T) {
	f := newFixture(t)
	q := query(t, f.svc.UnsubscribeLinks(f.users["mia"].ID, notify.KindComment).KindURL)
	const addr = "mia.new@example.org"
	if err := f.us.SetEmailIfEmpty(context.Background(), f.users["mia"].ID, addr, true); err != nil {
		t.Fatal(err)
	}
	want := mail.MaskAddress(addr)
	if got := decodeState(t, f.do(http.MethodGet, getPath(q), "", "", nil)); got.Email != want {
		t.Errorf("get email %q, want %q", got.Email, want)
	}
	rec := f.do(http.MethodPost, "/api/v1/unsubscribe", "application/json", postBody(q, "kind", false), nil)
	if got := decodeState(t, rec); got.Email != want {
		t.Errorf("post email %q, want %q", got.Email, want)
	}
	if strings.Contains(rec.Body.String(), addr) {
		t.Errorf("post answer holds the full address: %s", rec.Body)
	}
}

func TestUnsubscribePostKindGroupAndUndo(t *testing.T) {
	f := newFixture(t)
	adam := f.users["adam"]
	ctx := context.Background()
	comment := query(t, f.svc.UnsubscribeLinks(adam.ID, notify.KindComment).KindURL)
	invite := query(t, f.svc.UnsubscribeLinks(adam.ID, notify.KindInviteAccepted).KindURL)
	post := func(q url.Values, scope string, subscribed bool) state {
		return decodeState(t, f.do(http.MethodPost, "/api/v1/unsubscribe", "application/json", postBody(q, scope, subscribed), nil))
	}
	// Whether adam gets k by mail.
	prefs := func() map[notify.Kind]bool {
		p := map[notify.Kind]bool{}
		for _, k := range []notify.Kind{notify.KindComment, notify.KindInviteAccepted} {
			ok, err := f.svc.Wants(ctx, adam.ID, k, notify.ChannelMail)
			if err != nil {
				t.Fatal(err)
			}
			p[k] = ok
		}
		return p
	}

	if got := post(invite, "kind", false); got.KindSubscribed || !got.GroupSubscribed || got.Group != "admin" {
		t.Errorf("kind off: %+v", got)
	}
	if p := prefs(); p[notify.KindInviteAccepted] || !p[notify.KindComment] {
		t.Errorf("after kind off: %v", p)
	}
	if got := post(invite, "kind", true); !got.KindSubscribed {
		t.Errorf("undo: %+v", got)
	}

	if got := post(comment, "group", false); got.KindSubscribed || got.GroupSubscribed || got.Group != "household" {
		t.Errorf("group off: %+v", got)
	}
	if p := prefs(); p[notify.KindComment] || !p[notify.KindInviteAccepted] {
		t.Errorf("household off touched the admin group: %v", p)
	}
	if got := post(comment, "group", true); !got.KindSubscribed || !got.GroupSubscribed {
		t.Errorf("group undo: %+v", got)
	}
	if rec := f.do(http.MethodPost, "/api/v1/unsubscribe", "application/json", postBody(comment, "everything", false), nil); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("unknown scope: %d", rec.Code)
	}
}

// A group link switches the group's mail switch alone: the kinds keep their
// own choices, the undo removes only the group row, push is never touched.
func TestUnsubscribeGroupSwitchesMaster(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	adam := f.users["adam"]
	q := query(t, f.svc.UnsubscribeLinks(adam.ID, notify.KindComment).KindURL)
	post := func(scope string, subscribed bool) state {
		return decodeState(t, f.do(http.MethodPost, "/api/v1/unsubscribe", "application/json", postBody(q, scope, subscribed), nil))
	}
	if _, err := f.svc.SetPreferences(ctx, adam, notify.Prefs{Kinds: map[notify.Kind]map[notify.Channel]bool{notify.KindRecipe: off(notify.ChannelMail)}}); err != nil {
		t.Fatal(err)
	}
	if got := post("group", false); got.KindSubscribed || got.GroupSubscribed ||
		!maps.Equal(got.GroupKinds, map[string]bool{"comment": false, "recipe": false, "tasty": false}) {
		t.Errorf("group off: %+v", got)
	}
	p, err := f.svc.Preferences(ctx, adam)
	if err != nil {
		t.Fatal(err)
	}
	if p.Groups[notify.GroupHousehold][notify.ChannelMail] || !p.Groups[notify.GroupHousehold][notify.ChannelPush] ||
		!p.Kinds[notify.KindComment][notify.ChannelMail] || p.Kinds[notify.KindRecipe][notify.ChannelMail] {
		t.Errorf("group off wrote more than the group mail row: %+v", p)
	}
	for k, chs := range p.Kinds {
		if on, ok := chs[notify.ChannelPush]; ok && !on {
			t.Errorf("%s push switched off", k)
		}
	}
	if got := post("group", true); !got.KindSubscribed || !got.GroupSubscribed ||
		!maps.Equal(got.GroupKinds, map[string]bool{"comment": true, "recipe": false, "tasty": true}) {
		t.Errorf("undo: %+v", got)
	}
}

func TestOneClickOptsOutKindOnly(t *testing.T) {
	// RFC 8058 says a sender SHOULD post multipart/form-data, most post the
	// urlencoded form, and some nothing: the signed address alone decides.
	var multipart bytes.Buffer
	mw := mpart.NewWriter(&multipart)
	_ = mw.WriteField("List-Unsubscribe", "One-Click")
	_ = mw.Close()
	for name, tc := range map[string]struct{ contentType, body string }{
		"urlencoded": {"application/x-www-form-urlencoded", "List-Unsubscribe=One-Click"},
		"multipart":  {mw.FormDataContentType(), multipart.String()},
		"empty":      {"", ""},
	} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			adam := f.users["adam"]
			path := strings.TrimPrefix(f.svc.UnsubscribeLinks(adam.ID, notify.KindComment).OneClickURL, publicURL)
			rec := f.do(http.MethodPost, path, tc.contentType, tc.body, nil)
			if rec.Code != http.StatusOK || rec.Body.Len() != 0 {
				t.Fatalf("one-click: %d %q", rec.Code, rec.Body.String())
			}
			p, err := f.svc.Preferences(context.Background(), adam)
			if err != nil {
				t.Fatal(err)
			}
			if !maps.Equal(p.Kinds[notify.KindComment], map[notify.Channel]bool{notify.ChannelMail: false, notify.ChannelPush: true}) ||
				!maps.Equal(p.Kinds[notify.KindInviteAccepted], both) || !maps.Equal(p.Groups[notify.GroupHousehold], both) {
				t.Errorf("after one-click: %+v", p)
			}
			// Again is fine: the kind is already off.
			if rec := f.do(http.MethodPost, path, tc.contentType, tc.body, nil); rec.Code != http.StatusOK {
				t.Errorf("second one-click: %d %s", rec.Code, rec.Body.String())
			}
		})
	}
}
