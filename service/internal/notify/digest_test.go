package notify

import (
	"bytes"
	"context"
	stdimage "image"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/s-frei/rezepte/service/internal/recipe"
	"github.com/s-frei/rezepte/service/internal/user"
)

// at is a wall-clock time on October 28, 2026 (day 0) or a later day, in
// the server's time zone.
func at(day, hour, minute int) time.Time {
	return time.Date(2026, 10, 28+day, hour, minute, 0, 0, time.Local)
}

// added creates a recipe the way the app does, at the given time.
func (f fanout) added(t *testing.T, createdBy user.User, title string, when time.Time) recipe.Recipe {
	t.Helper()
	in, err := recipe.Samples("en")
	if err != nil {
		t.Fatal(err)
	}
	in[0].Title = title
	f.recipes.SetClock(func() time.Time { return when })
	r, err := f.recipes.CreateAnnounced(context.Background(), createdBy.ID, in[0])
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func (f fanout) run(t *testing.T, now time.Time) {
	t.Helper()
	if err := f.svc.RunRecipeDigest(context.Background(), now); err != nil {
		t.Fatal(err)
	}
}

// sent maps each recipient to the titles of their last digest.
func (f fanout) sent() map[string][]string {
	got := map[string][]string{}
	for _, d := range f.mailer.digests {
		var titles []string
		for _, r := range d.Recipes {
			titles = append(titles, r.Title)
		}
		got[d.To] = titles
	}
	return got
}

func (f fanout) household(t *testing.T) (mila, jonas user.User) {
	t.Helper()
	return f.person(t, "mila", user.RoleAdmin, true), f.person(t, "jonas", user.RoleUser, true)
}

func TestDigestRunsOncePerDay(t *testing.T) {
	f := newFanout(t)
	mila, jonas := f.household(t)
	f.added(t, mila, "Dal", at(0, 12, 0))
	f.run(t, at(0, 17, 5))
	f.run(t, at(0, 17, 5))
	if len(f.mailer.digests) != 1 || f.mailer.digests[0].To != jonas.Email {
		t.Fatalf("sent %+v, want one digest to jonas", f.mailer.digests)
	}
	d := f.mailer.digests[0]
	if d.Unsubscribe != f.svc.UnsubscribeLinks(jonas.ID, KindRecipe) || d.Locale != "de" {
		t.Errorf("digest %+v", d)
	}
	if r := d.Recipes[0]; r.CreatorName != mila.DisplayName || r.CreatorColor != mila.Color || r.Servings == 0 ||
		r.URLPath == "" {
		t.Errorf("recipe %+v", r)
	}

	// An earlier hour after the day's run does not run it again.
	if _, err := f.conn.ExecContext(context.Background(), "UPDATE instance_settings SET recipe_digest_hour = 9"); err != nil {
		t.Fatal(err)
	}
	f.added(t, mila, "Pho", at(0, 13, 0))
	f.run(t, at(0, 18, 0))
	if len(f.mailer.digests) != 1 {
		t.Fatalf("sent %d digests on day 0", len(f.mailer.digests))
	}
	f.run(t, at(1, 9, 0))
	if got := f.sent()[jonas.Email]; len(f.mailer.digests) != 2 || len(got) != 1 || got[0] != "Pho" {
		t.Fatalf("day 1: %+v", f.mailer.digests)
	}
}

func TestDigestWaitsForTheHour(t *testing.T) {
	f := newFanout(t)
	mila, _ := f.household(t)
	f.added(t, mila, "Dal", at(0, 12, 0))
	f.run(t, at(0, 16, 59))
	if len(f.mailer.digests) != 0 {
		t.Fatalf("sent %+v before 17:00", f.mailer.digests)
	}
	st, err := f.svc.q.GetRecipeDigestState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if st.RecipeDigestLastDay != "" {
		t.Errorf("claimed %q", st.RecipeDigestLastDay)
	}
	f.run(t, at(0, 17, 0))
	if len(f.mailer.digests) != 1 {
		t.Fatalf("sent %+v at 17:00", f.mailer.digests)
	}
}

func TestDigestHoldsYoungRecipes(t *testing.T) {
	f := newFanout(t)
	mila, jonas := f.household(t)
	f.added(t, mila, "Dal", at(0, 16, 30))
	f.run(t, at(0, 17, 0))
	if len(f.mailer.digests) != 0 {
		t.Fatalf("sent %+v, the recipe is half an hour old", f.mailer.digests)
	}
	f.run(t, at(1, 17, 0))
	if got := f.sent()[jonas.Email]; len(got) != 1 || got[0] != "Dal" {
		t.Fatalf("day 1: %+v", f.mailer.digests)
	}
}

func TestDigestSkipsOwnRecipes(t *testing.T) {
	f := newFanout(t)
	mila, jonas := f.household(t)
	f.added(t, mila, "Dal", at(0, 10, 0))
	f.added(t, mila, "Pho", at(0, 11, 0))
	f.run(t, at(0, 17, 0))
	got := f.sent()
	if _, ok := got[mila.Email]; ok || len(got[jonas.Email]) != 2 || got[jonas.Email][0] != "Dal" {
		t.Fatalf("sent %v", got)
	}
}

func TestDigestRespectsOptOut(t *testing.T) {
	for _, off := range []map[Kind]map[Channel]bool{
		{KindRecipe: {ChannelMail: false}},
		{KindRecipe: {ChannelMail: false}, KindComment: {ChannelMail: false}},
	} {
		f := newFanout(t)
		mila, jonas := f.household(t)
		f.person(t, "uwe", user.RoleUser, false) // no confirmed address
		if _, err := f.svc.SetPreferences(context.Background(), jonas, Prefs{Kinds: off}); err != nil {
			t.Fatal(err)
		}
		f.added(t, mila, "Dal", at(0, 10, 0))
		f.run(t, at(0, 17, 0))
		if len(f.mailer.digests) != 0 {
			t.Fatalf("opted out %v: sent %+v", off, f.mailer.digests)
		}
	}
}

func TestDigestIgnoresUnannounced(t *testing.T) {
	f := newFanout(t)
	mila, _ := f.household(t)
	f.recipes.SetClock(func() time.Time { return at(0, 10, 0) })
	f.recipe(t, mila.ID) // an archive import or the demo seed
	f.run(t, at(0, 17, 0))
	f.run(t, at(1, 17, 0))
	if len(f.mailer.digests) != 0 {
		t.Fatalf("sent %+v", f.mailer.digests)
	}
}

func TestDigestDropsDeleted(t *testing.T) {
	f := newFanout(t)
	mila, jonas := f.household(t)
	r := f.added(t, mila, "Dal", at(0, 10, 0))
	f.added(t, mila, "Pho", at(0, 11, 0))
	if err := f.recipes.Delete(context.Background(), r.ID, mila); err != nil {
		t.Fatal(err)
	}
	f.run(t, at(0, 17, 0))
	if got := f.sent()[jonas.Email]; len(got) != 1 || got[0] != "Pho" {
		t.Fatalf("sent %v", f.sent())
	}
}

func TestDigestMailOffClaimsNothing(t *testing.T) {
	f := newFanout(t)
	mila, _ := f.household(t)
	f.added(t, mila, "Dal", at(0, 10, 0))
	f.mailer.off = true
	f.run(t, at(0, 17, 0))
	f.mailer.off = false
	f.run(t, at(0, 17, 1))
	if len(f.mailer.digests) != 1 {
		t.Fatalf("sent %+v, want the run once mail is back", f.mailer.digests)
	}
}

// cover uploads a photo to r, which becomes its cover.
func (f fanout) cover(t *testing.T, r recipe.Recipe, actor user.User) {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, stdimage.NewRGBA(stdimage.Rect(0, 0, 400, 300)), nil); err != nil {
		t.Fatal(err)
	}
	if _, err := f.images.Upload(context.Background(), r.ID, actor, &buf); err != nil {
		t.Fatal(err)
	}
}

func TestDigestMissingPhoto(t *testing.T) {
	f := newFanout(t)
	mila, jonas := f.household(t)
	dal := f.added(t, mila, "Dal", at(0, 10, 0))
	pho := f.added(t, mila, "Pho", at(0, 11, 0))
	f.cover(t, dal, mila)
	f.cover(t, pho, mila)
	thumbs, err := filepath.Glob(filepath.Join(f.imageDir, pho.ID, "*_thumb.jpg"))
	if err != nil || len(thumbs) != 1 {
		t.Fatalf("thumbs %v, %v", thumbs, err)
	}
	if err := os.Remove(thumbs[0]); err != nil {
		t.Fatal(err)
	}
	buf := logged(t)
	f.run(t, at(0, 17, 0))
	if len(f.mailer.digests) != 1 || f.mailer.digests[0].To != jonas.Email {
		t.Fatalf("sent %+v", f.mailer.digests)
	}
	if !strings.Contains(buf.String(), "photo unreadable") {
		t.Errorf("the missing photo was not logged: %s", buf)
	}
	rs := f.mailer.digests[0].Recipes
	if len(rs) != 2 || len(rs[0].Photo) == 0 || rs[1].Photo != nil {
		t.Errorf("photos: Dal %d bytes, Pho %d bytes", len(rs[0].Photo), len(rs[1].Photo))
	}
}

// Once the day is claimed its recipes are off the queue, so a shutdown
// that cancels the run must not cost the people not yet mailed.
func TestDigestOutlivesItsContext(t *testing.T) {
	f := newFanout(t)
	mila, _ := f.household(t)
	f.person(t, "olga", user.RoleUser, true)
	f.added(t, mila, "Dal", at(0, 10, 0))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f.mailer.sent = cancel
	if err := f.svc.RunRecipeDigest(ctx, at(0, 17, 0)); err != nil {
		t.Fatal(err)
	}
	if len(f.mailer.digests) != 2 {
		t.Fatalf("sent %+v, want jonas and olga", f.mailer.digests)
	}
}

// A creator that cannot be read after the claim is named a former member;
// the recipe still goes out.
func TestDigestSurvivesACreatorLookupFailure(t *testing.T) {
	f := newFanout(t)
	mila, jonas := f.household(t)
	f.added(t, mila, "Dal", at(0, 10, 0))
	// A row users.Service cannot parse: ByID fails with an error other than not found.
	if _, err := f.conn.ExecContext(context.Background(), "UPDATE users SET created_at = 'garbage' WHERE id = ?", mila.ID); err != nil {
		t.Fatal(err)
	}
	buf := logged(t)
	f.run(t, at(0, 17, 0))
	if len(f.mailer.digests) != 1 || f.mailer.digests[0].To != jonas.Email {
		t.Fatalf("sent %+v", f.mailer.digests)
	}
	if r := f.mailer.digests[0].Recipes[0]; r.Title != "Dal" || r.CreatorName != "" {
		t.Errorf("recipe %+v, want Dal from a former member", r)
	}
	if !strings.Contains(buf.String(), "creator unreadable") {
		t.Errorf("the lookup failure was not logged: %s", buf)
	}
}

// The channel StartRecipeDigest returns closes only once a run under way
// when ctx ends has sent its mails, so main can wait for it on shutdown.
func TestDigestStopWaitsForTheRun(t *testing.T) {
	orig := digestTick
	digestTick = time.Millisecond
	t.Cleanup(func() { digestTick = orig })
	f := newFanout(t)
	mila, _ := f.household(t)
	f.person(t, "olga", user.RoleUser, true)
	f.added(t, mila, "Dal", time.Now().Add(-2*time.Hour))
	if _, err := f.conn.ExecContext(context.Background(), "UPDATE instance_settings SET recipe_digest_hour = 0"); err != nil {
		t.Fatal(err)
	}
	inSend, release := make(chan struct{}), make(chan struct{})
	f.mailer.sent = func() {
		if len(f.mailer.digests) == 1 {
			close(inSend)
			<-release
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := f.svc.StartRecipeDigest(ctx)
	<-inSend
	cancel()
	select {
	case <-done:
		t.Fatal("done before the run sent its mails")
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("not done after the run")
	}
	if len(f.mailer.digests) != 2 {
		t.Fatalf("sent %+v, want jonas and olga", f.mailer.digests)
	}
}
