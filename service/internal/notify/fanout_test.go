package notify

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	stdimage "image"
	"image/jpeg"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/s-frei/rezepte/service/internal/avatar"
	"github.com/s-frei/rezepte/service/internal/db"
	"github.com/s-frei/rezepte/service/internal/db/dbtest"
	"github.com/s-frei/rezepte/service/internal/db/sqlc"
	"github.com/s-frei/rezepte/service/internal/image"
	"github.com/s-frei/rezepte/service/internal/mail"
	"github.com/s-frei/rezepte/service/internal/recipe"
	"github.com/s-frei/rezepte/service/internal/user"
)

// fakeMailer records what would be sent; off answers like mail.Service
// with mail switched off.
type fakeMailer struct {
	off bool
	err error // returned by every send when set
	// stall makes the first send wait until its context ends, like an SMTP
	// server that never answers.
	stall    bool
	comments []mail.Comment
	invites  []mail.InviteAccepted
	digests  []mail.RecipeDigest
	// sent runs after each recorded digest.
	sent func()
}

func (m *fakeMailer) Enabled(context.Context) bool { return !m.off }

func (m *fakeMailer) SendComment(ctx context.Context, c mail.Comment) error {
	if m.off {
		return mail.ErrDisabled
	}
	if m.stall {
		m.stall = false
		<-ctx.Done()
		return fmt.Errorf("%w: %w", mail.ErrSend, ctx.Err())
	}
	if m.err != nil {
		return m.err
	}
	m.comments = append(m.comments, c)
	return nil
}

func (m *fakeMailer) SendInviteAccepted(_ context.Context, in mail.InviteAccepted) error {
	if m.off {
		return mail.ErrDisabled
	}
	if m.err != nil {
		return m.err
	}
	m.invites = append(m.invites, in)
	return nil
}

func (m *fakeMailer) SendRecipeDigest(ctx context.Context, d mail.RecipeDigest) error {
	if m.off {
		return mail.ErrDisabled
	}
	if m.err != nil {
		return m.err
	}
	if err := ctx.Err(); err != nil { // as an SMTP dialog would end
		return fmt.Errorf("%w: %w", mail.ErrSend, err)
	}
	m.digests = append(m.digests, d)
	if m.sent != nil {
		m.sent()
	}
	return nil
}

type fanout struct {
	svc       *Service
	mailer    *fakeMailer
	users     *user.Service
	recipes   *recipe.Service
	images    *image.Service
	conn      *sql.DB
	avatarDir string
	imageDir  string
}

func newFanout(t *testing.T) fanout {
	t.Helper()
	conn := dbtest.Open(t)
	f := fanout{mailer: &fakeMailer{}, users: user.NewService(conn, ""), recipes: recipe.NewService(conn, ""),
		conn: conn, avatarDir: t.TempDir(), imageDir: t.TempDir()}
	avatars := avatar.NewService(conn, f.avatarDir, nil)
	f.images = image.NewService(conn, f.imageDir)
	f.svc = NewService(conn, bytes.Repeat([]byte{7}, 32), f.users, avatars, f.images, f.mailer, "https://r.example.org")
	return f
}

// person creates an account; verified confirms its address.
func (f fanout) person(t *testing.T, name string, role user.Role, verified bool) user.User {
	t.Helper()
	ctx := context.Background()
	u, err := f.users.Create(ctx, user.CreateParams{Username: name, Password: "pw", Role: role, Email: name + "@example.org", Locale: "de"})
	if err != nil {
		t.Fatal(err)
	}
	if verified {
		if _, err := f.users.MarkEmailVerified(ctx, u.ID, u.Email); err != nil {
			t.Fatal(err)
		}
	}
	return u
}

func (f fanout) recipe(t *testing.T, createdBy string) recipe.Recipe {
	t.Helper()
	in, err := recipe.Samples("en")
	if err != nil {
		t.Fatal(err)
	}
	r, err := f.recipes.Create(context.Background(), createdBy, in[0])
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func (f fanout) comment(t *testing.T, author user.User, recipeID string) int64 {
	t.Helper()
	c, err := f.recipes.AddComment(context.Background(), author, recipeID, "<b>Mehr Kreuzkümmel</b>", nil)
	if err != nil {
		t.Fatal(err)
	}
	return c.ID
}

func TestCommentMailsFollowersButNotAuthor(t *testing.T) {
	ctx := context.Background()
	f := newFanout(t)
	olga := f.person(t, "olga", user.RoleAdmin, true)
	mia := f.person(t, "mia", user.RoleUser, true)
	adam := f.person(t, "adam", user.RoleUser, true)
	f.person(t, "tom", user.RoleUser, true) // never touched the recipe
	r := f.recipe(t, olga.ID)
	f.comment(t, mia, r.ID)
	f.comment(t, adam, r.ID) // adam follows from now on, but this is his own
	id := f.comment(t, adam, r.ID)

	if err := f.svc.mailComment(ctx, r.ID, id); err != nil {
		t.Fatal(err)
	}
	got := map[string]mail.Comment{}
	for _, c := range f.mailer.comments {
		got[c.To] = c
	}
	if len(f.mailer.comments) != 2 || len(got) != 2 {
		t.Fatalf("sent %+v", f.mailer.comments)
	}
	if c := got[olga.Email]; c.Reason != mail.ReasonCreated || c.Locale != "de" {
		t.Errorf("creator: %+v", c)
	}
	c := got[mia.Email]
	if c.Reason != mail.ReasonCommented {
		t.Errorf("commenter: %+v", c)
	}
	if c.RecipeTitle != r.Title || c.RecipeURLPath != "/recipes/"+r.Slug+"?tab=comments" ||
		c.AuthorName != adam.DisplayName || c.AuthorColor != adam.Color || c.Body != "<b>Mehr Kreuzkümmel</b>" || c.At.IsZero() {
		t.Errorf("content: %+v", c)
	}
	if want := f.svc.UnsubscribeLinks(mia.ID, KindComment); c.Unsubscribe != want {
		t.Errorf("unsubscribe %+v, want %+v", c.Unsubscribe, want)
	}
}

// Each follower's mail has its own timeout: one that stalls costs only that
// follower.
func TestCommentMailStallCostsOneFollower(t *testing.T) {
	orig := sendTimeout
	sendTimeout = 50 * time.Millisecond
	t.Cleanup(func() { sendTimeout = orig })
	ctx := context.Background()
	f := newFanout(t)
	olga := f.person(t, "olga", user.RoleAdmin, true)
	mia := f.person(t, "mia", user.RoleUser, true)
	adam := f.person(t, "adam", user.RoleUser, true)
	r := f.recipe(t, olga.ID)
	f.comment(t, mia, r.ID)
	id := f.comment(t, adam, r.ID)
	f.mailer.stall = true
	if err := f.svc.mailComment(ctx, r.ID, id); err != nil {
		t.Fatal(err)
	}
	if len(f.mailer.comments) != 1 {
		t.Fatalf("sent %+v, want the follower after the stalled one", f.mailer.comments)
	}
}

func TestCommentMailCarriesTheAuthorsPicture(t *testing.T) {
	ctx := context.Background()
	f := newFanout(t)
	olga := f.person(t, "olga", user.RoleAdmin, true)
	mia := f.person(t, "mia", user.RoleUser, true)
	adam := f.person(t, "adam", user.RoleUser, true)
	r := f.recipe(t, olga.ID)
	// A file and row as avatar.Service.Set leaves them, without decoding a picture.
	const avatarID = "0b7f2f4e-3c1d-4a8e-9f00-1234567890ab"
	if err := os.MkdirAll(filepath.Join(f.avatarDir, mia.ID), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.avatarDir, mia.ID, avatarID+".jpg"), []byte("jpeg"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := f.conn.ExecContext(ctx, "UPDATE users SET avatar_id = ? WHERE id = ?", avatarID, mia.ID); err != nil {
		t.Fatal(err)
	}
	f.comment(t, adam, r.ID) // no picture: the initial
	if err := f.svc.mailComment(ctx, r.ID, f.comment(t, mia, r.ID)); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.mailComment(ctx, r.ID, f.comment(t, adam, r.ID)); err != nil {
		t.Fatal(err)
	}
	pictures := map[string]string{}
	for _, c := range f.mailer.comments {
		pictures[c.AuthorName] = string(c.AuthorAvatar)
	}
	if pictures[mia.DisplayName] != "jpeg" || pictures[adam.DisplayName] != "" {
		t.Errorf("pictures by author = %q", pictures)
	}
}

func TestCommentRespectsOptOut(t *testing.T) {
	ctx := context.Background()
	f := newFanout(t)
	olga := f.person(t, "olga", user.RoleAdmin, true)
	mia := f.person(t, "mia", user.RoleUser, true)
	r := f.recipe(t, olga.ID)
	if _, err := f.svc.SetPreferences(ctx, olga, Prefs{Kinds: map[Kind]map[Channel]bool{KindComment: {ChannelMail: false}}}); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.mailComment(ctx, r.ID, f.comment(t, mia, r.ID)); err != nil {
		t.Fatal(err)
	}
	if len(f.mailer.comments) != 0 {
		t.Fatalf("sent %+v", f.mailer.comments)
	}
}

func TestCommentSkipsUnverified(t *testing.T) {
	ctx := context.Background()
	f := newFanout(t)
	olga := f.person(t, "olga", user.RoleAdmin, false)
	mia := f.person(t, "mia", user.RoleUser, true)
	r := f.recipe(t, olga.ID)
	if err := f.svc.mailComment(ctx, r.ID, f.comment(t, mia, r.ID)); err != nil {
		t.Fatal(err)
	}
	if len(f.mailer.comments) != 0 {
		t.Fatalf("sent %+v", f.mailer.comments)
	}
}

func TestCommentWithMailOff(t *testing.T) {
	ctx := context.Background()
	f := newFanout(t)
	f.mailer.off = true
	olga := f.person(t, "olga", user.RoleAdmin, true)
	mia := f.person(t, "mia", user.RoleUser, true)
	r := f.recipe(t, olga.ID)
	if err := f.svc.mailComment(ctx, r.ID, f.comment(t, mia, r.ID)); err != nil {
		t.Fatal(err)
	}
	if len(f.mailer.comments) != 0 {
		t.Fatalf("sent %+v", f.mailer.comments)
	}
}

func TestInviteAcceptedSkips(t *testing.T) {
	ctx := context.Background()
	f := newFanout(t)
	admin := f.person(t, "adam", user.RoleAdmin, true)
	demoted := f.person(t, "dora", user.RoleUser, true)
	optedOut := f.person(t, "otto", user.RoleAdmin, true)
	unverified := f.person(t, "uwe", user.RoleAdmin, false)
	newcomer := f.person(t, "noah", user.RoleUser, false)
	if _, err := f.svc.SetPreferences(ctx, optedOut, Prefs{Kinds: map[Kind]map[Channel]bool{KindInviteAccepted: {ChannelMail: false}}}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name     string
		inviter  user.User
		newcomer user.User
	}{
		{"demoted", demoted, newcomer},
		{"opted out", optedOut, newcomer},
		{"unverified", unverified, newcomer},
		{"self", admin, admin},
		{"gone", user.User{ID: "missing"}, newcomer},
	} {
		f.svc.InviteAccepted(ctx, tc.inviter.ID, tc.newcomer)
		if len(f.mailer.invites) != 0 {
			t.Fatalf("%s: sent %+v", tc.name, f.mailer.invites)
		}
	}
	f.svc.InviteAccepted(ctx, admin.ID, newcomer)
	if len(f.mailer.invites) != 1 {
		t.Fatalf("admin: sent %+v", f.mailer.invites)
	}
	in := f.mailer.invites[0]
	if in.To != admin.Email || in.Username != "noah" || in.DisplayName != newcomer.DisplayName || in.Locale != "de" ||
		in.Unsubscribe != f.svc.UnsubscribeLinks(admin.ID, KindInviteAccepted) {
		t.Errorf("invite: %+v", in)
	}
}

// logged captures the default logger for one test.
func logged(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

// A failure other than mail off or a refused send (deliver logs that one) is
// logged without the address, for every follower and for the inviter.
func TestSendFailuresAreLogged(t *testing.T) {
	ctx := context.Background()
	f := newFanout(t)
	olga := f.person(t, "olga", user.RoleAdmin, true)
	mia := f.person(t, "mia", user.RoleUser, true)
	adam := f.person(t, "adam", user.RoleUser, true)
	r := f.recipe(t, olga.ID)
	f.comment(t, mia, r.ID)
	id := f.comment(t, adam, r.ID)

	buf := logged(t)
	f.mailer.err = fmt.Errorf("%w: refused", mail.ErrSend)
	if err := f.svc.mailComment(ctx, r.ID, id); err != nil {
		t.Fatal(err)
	}
	f.svc.InviteAccepted(ctx, olga.ID, mia)
	if buf.Len() != 0 {
		t.Fatalf("a refused send was logged twice: %s", buf)
	}

	f.mailer.err = errors.New("render failed")
	if err := f.svc.mailComment(ctx, r.ID, id); err != nil {
		t.Fatal(err)
	}
	f.svc.InviteAccepted(ctx, olga.ID, mia)
	out := buf.String()
	if n := strings.Count(out, "render failed"); n != 3 {
		t.Errorf("logged %d failures, want 2 followers and 1 inviter: %s", n, out)
	}
	if strings.Contains(out, "@") {
		t.Errorf("log holds an address: %s", out)
	}
}

// A comment's photos are read once for the whole fan-out: every follower
// gets the same bytes, made into mail photos from the _thumb.jpg files.
func TestCommentMailCarriesPhotosLoadedOnce(t *testing.T) {
	ctx := context.Background()
	f := newFanout(t)
	olga := f.person(t, "olga", user.RoleAdmin, true)
	mia := f.person(t, "mia", user.RoleUser, true)
	adam := f.person(t, "adam", user.RoleUser, true)
	r := f.recipe(t, olga.ID)
	f.comment(t, mia, r.ID) // mia follows: two followers get adam's comment
	var ids []string
	for i := range 2 {
		id := fmt.Sprintf("0b7f2f4e-3c1d-4a8e-9f00-12345678900%d", i)
		if err := sqlc.New(f.conn).InsertCommentImage(ctx, sqlc.InsertCommentImageParams{
			ID: id, RecipeID: r.ID, UploaderID: &adam.ID, Width: 4, Height: 4, SizeBytes: 100, CreatedAt: db.FormatTime(time.Now()),
		}); err != nil {
			t.Fatal(err)
		}
		dir := recipe.CommentImageDir(f.imageDir, r.ID, id)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, stdimage.NewRGBA(stdimage.Rect(0, 0, 4, 4)), nil); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, id+"_thumb.jpg"), buf.Bytes(), 0o600); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	c, err := f.recipes.AddComment(ctx, adam, r.ID, "mit Fotos", ids)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.svc.mailComment(ctx, r.ID, c.ID); err != nil {
		t.Fatal(err)
	}
	if len(f.mailer.comments) != 2 {
		t.Fatalf("sent %d mails, want 2", len(f.mailer.comments))
	}
	for _, m := range f.mailer.comments {
		if len(m.Photos) != 2 || len(m.Photos[0]) == 0 {
			t.Fatalf("mail to %s has %d photos", m.To, len(m.Photos))
		}
		if _, err := jpeg.Decode(bytes.NewReader(m.Photos[0])); err != nil {
			t.Errorf("photo is no JPEG: %v", err)
		}
	}
	if a, b := f.mailer.comments[0].Photos, f.mailer.comments[1].Photos; &a[0][0] != &b[0][0] {
		t.Error("the second follower got a copy: photos were read per follower")
	}
}
