package share_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/s-frei/rezepte/service/internal/db"
	"github.com/s-frei/rezepte/service/internal/db/dbtest"
	"github.com/s-frei/rezepte/service/internal/db/sqlc"
	"github.com/s-frei/rezepte/service/internal/recipe"
	"github.com/s-frei/rezepte/service/internal/settings"
	"github.com/s-frei/rezepte/service/internal/share"
	"github.com/s-frei/rezepte/service/internal/user"
)

type env struct {
	conn     *sql.DB
	shares   *share.Service
	settings *settings.Service
	users    *user.Service
	recipes  *recipe.Service
	owner    user.User
	member   user.User
	recipe   recipe.Recipe
	clock    time.Time
}

// setup makes a household with sharing switched on, a member who may share,
// and one recipe of theirs.
func setup(t *testing.T) *env {
	t.Helper()
	ctx := context.Background()
	conn := dbtest.Open(t)
	users := user.NewService(conn, "")
	owner, err := users.Create(ctx, user.CreateParams{Username: "olga", Password: "pw", Role: user.RoleSuperadmin})
	if err != nil {
		t.Fatal(err)
	}
	member, err := users.Create(ctx, user.CreateParams{Username: "mira", Password: "pw", Role: user.RoleUser})
	if err != nil {
		t.Fatal(err)
	}
	st := settings.NewService(conn)
	if _, err := st.SetPublicShares(ctx, owner, true); err != nil {
		t.Fatal(err)
	}
	recipes := recipe.NewService(conn, "")
	r, err := recipes.Create(ctx, member.ID, recipe.Input{
		Title: "Gurkensalat", Servings: 2,
		IngredientGroups: []recipe.IngredientGroup{{Ingredients: []recipe.Ingredient{{Name: "Gurke"}}}},
		Steps:            []recipe.Step{{Text: "Schneiden."}},
	})
	if err != nil {
		t.Fatal(err)
	}
	e := &env{
		conn: conn, settings: st, users: users, recipes: recipes,
		owner: owner, member: member, recipe: r, clock: time.Now().UTC(),
	}
	e.shares = share.NewService(conn, st, users)
	e.shares.SetClock(func() time.Time { return e.clock })
	return e
}

func (e *env) admin(t *testing.T) user.User {
	t.Helper()
	a, err := e.users.Create(context.Background(), user.CreateParams{Username: "anh", Password: "pw", Role: user.RoleAdmin})
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func intPtr(i int) *int { return &i }

func TestCreateRefusesLifetimeAboveTheMaximum(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	if _, err := e.settings.SetShareLifetimes(ctx, e.owner, nil, intPtr(30)); err != nil {
		t.Fatal(err)
	}
	if _, err := e.shares.Create(ctx, e.member, e.recipe.ID, nil); !errors.Is(err, share.ErrLifetime) {
		t.Errorf("permanent above a 30 day maximum: err = %v, want ErrLifetime", err)
	}
	if _, err := e.shares.Create(ctx, e.member, e.recipe.ID, intPtr(365)); !errors.Is(err, share.ErrLifetime) {
		t.Errorf("365 days above a 30 day maximum: err = %v, want ErrLifetime", err)
	}
	if _, err := e.shares.Create(ctx, e.member, e.recipe.ID, intPtr(2)); !errors.Is(err, share.ErrLifetime) {
		t.Errorf("2 days (not one of the lifetimes): err = %v, want ErrLifetime", err)
	}
}

func TestCreateTwiceReturnsTheExistingShare(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	first, err := e.shares.Create(ctx, e.member, e.recipe.ID, intPtr(7))
	if err != nil {
		t.Fatal(err)
	}
	again, err := e.shares.Create(ctx, e.member, e.recipe.ID, intPtr(30))
	if !errors.Is(err, share.ErrExists) {
		t.Fatalf("second create: err = %v, want ErrExists", err)
	}
	if again.ID != first.ID || again.Path != first.Path {
		t.Errorf("existing share = %+v, want %+v", again, first)
	}
}

// An own-expired share is terminal: it must not block a fresh Create (no
// ErrExists for a dead link), and the row it replaces must actually be gone
// afterwards, not merely superseded.
func TestCreateAfterOwnExpiryIgnoresTheDeadShare(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	dead, err := e.shares.Create(ctx, e.member, e.recipe.ID, intPtr(1))
	if err != nil {
		t.Fatal(err)
	}
	e.clock = e.clock.Add(2 * 24 * time.Hour)
	fresh, err := e.shares.Create(ctx, e.member, e.recipe.ID, intPtr(7))
	if err != nil {
		t.Fatalf("create after the old link's own expiry: err = %v, want a fresh share", err)
	}
	if fresh.ID == dead.ID || fresh.Path == dead.Path {
		t.Errorf("fresh share = %+v, want a new id and token, not the dead one %+v", fresh, dead)
	}
	got, err := e.shares.Mine(ctx, e.member, e.recipe.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != fresh.ID {
		t.Errorf("Mine after re-create = %+v, want the fresh share %+v", got, fresh)
	}
}

// Mine and List must not surface an own-expired share: it is gone in every
// way that matters, whether or not Sweep has removed the row yet.
func TestMineAndListTreatOwnExpiryAsGone(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	if _, err := e.shares.Create(ctx, e.member, e.recipe.ID, intPtr(1)); err != nil {
		t.Fatal(err)
	}
	e.clock = e.clock.Add(2 * 24 * time.Hour)

	if _, err := e.shares.Mine(ctx, e.member, e.recipe.ID); !errors.Is(err, share.ErrNotFound) {
		t.Errorf("Mine: err = %v, want ErrNotFound", err)
	}
	mine, err := e.shares.List(ctx, e.member, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(mine) != 0 {
		t.Errorf("List(own) = %v, want none", mine)
	}
	all, err := e.shares.List(ctx, e.admin(t), true)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 0 {
		t.Errorf("List(all) = %v, want none", all)
	}
}

// A live existing share wins over the lifetime check: a member who already
// shares gets their link back even when the requested (or now-current)
// lifetime would otherwise be refused, such as after the owner lowers the
// maximum below what the existing share was created with.
func TestCreateExistingShareWinsOverALoweredMaximum(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	first, err := e.shares.Create(ctx, e.member, e.recipe.ID, intPtr(30))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.settings.SetShareLifetimes(ctx, e.owner, nil, intPtr(7)); err != nil {
		t.Fatal(err)
	}
	again, err := e.shares.Create(ctx, e.member, e.recipe.ID, intPtr(30))
	if !errors.Is(err, share.ErrExists) {
		t.Fatalf("err = %v, want ErrExists (the existing share wins over the lowered maximum)", err)
	}
	if again.ID != first.ID {
		t.Errorf("existing share = %+v, want %+v", again, first)
	}
}

// A row inserted directly, bypassing the service, is found by
// liveOwnShare's own SELECT before Create ever reaches an INSERT - so this
// never exercises isUniqueViolation (service_internal_test.go does, with a
// real duplicate insert). It still matters on its own: Create must treat
// that row exactly like one it created itself, and hand it back as
// ErrExists.
func TestCreateFindsADirectlyInsertedRowAsExisting(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	q := sqlc.New(e.conn)
	row, err := q.CreateShare(ctx, sqlc.CreateShareParams{
		ID: "raced-share", Token: "raced-token-aaaaaaaaaaaaaa",
		RecipeID: e.recipe.ID, CreatedBy: e.member.ID,
		CreatedAt: db.FormatTime(e.clock),
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := e.shares.Create(ctx, e.member, e.recipe.ID, intPtr(7))
	if !errors.Is(err, share.ErrExists) {
		t.Fatalf("err = %v, want ErrExists", err)
	}
	if got.ID != row.ID || got.Path != "/s/"+row.Token {
		t.Errorf("share = %+v, want the directly-inserted row %+v", got, row)
	}
}

func TestCreateSharingOff(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	if _, err := e.settings.SetPublicShares(ctx, e.owner, false); err != nil {
		t.Fatal(err)
	}
	if _, err := e.shares.Create(ctx, e.member, e.recipe.ID, nil); !errors.Is(err, share.ErrSharingOff) {
		t.Errorf("err = %v, want ErrSharingOff", err)
	}
}

func TestCreateWithdrawnUser(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	if _, err := e.users.SetCanSharePublicly(ctx, e.owner, e.member.ID, false); err != nil {
		t.Fatal(err)
	}
	if _, err := e.shares.Create(ctx, e.member, e.recipe.ID, nil); !errors.Is(err, share.ErrNotAllowed) {
		t.Errorf("err = %v, want ErrNotAllowed", err)
	}
}

func TestCreateUnknownRecipe(t *testing.T) {
	e := setup(t)
	if _, err := e.shares.Create(context.Background(), e.member, "nope", nil); !errors.Is(err, share.ErrRecipeNotFound) {
		t.Errorf("err = %v, want ErrRecipeNotFound", err)
	}
}

func TestRevokeOfAnothersShare(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	got, err := e.shares.Create(ctx, e.member, e.recipe.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	other, err := e.users.Create(ctx, user.CreateParams{Username: "beni", Password: "pw", Role: user.RoleUser})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.shares.Revoke(ctx, other, got.ID); !errors.Is(err, share.ErrNotFound) {
		t.Errorf("another user revoking: err = %v, want ErrNotFound", err)
	}
	if err := e.shares.Revoke(ctx, e.admin(t), got.ID); err != nil {
		t.Errorf("admin revoking: err = %v, want nil", err)
	}
	if _, err := e.shares.Mine(ctx, e.member, e.recipe.ID); !errors.Is(err, share.ErrNotFound) {
		t.Errorf("after admin revoke: err = %v, want ErrNotFound", err)
	}
}

func TestRevokeAllRequiresAdmin(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	if _, err := e.shares.Create(ctx, e.member, e.recipe.ID, nil); err != nil {
		t.Fatal(err)
	}
	if err := e.shares.RevokeAll(ctx, e.member); !errors.Is(err, share.ErrAdminRequired) {
		t.Errorf("member: err = %v, want ErrAdminRequired", err)
	}
	if err := e.shares.RevokeAll(ctx, e.admin(t)); err != nil {
		t.Fatal(err)
	}
	got, err := e.shares.List(ctx, e.owner, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("shares after RevokeAll = %v, want none", got)
	}
}

func TestDeletingTheRecipeRemovesItsShare(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	if _, err := e.shares.Create(ctx, e.member, e.recipe.ID, nil); err != nil {
		t.Fatal(err)
	}
	if err := e.recipes.Delete(ctx, e.recipe.ID, e.member); err != nil {
		t.Fatal(err)
	}
	got, err := e.shares.List(ctx, e.owner, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("shares after deleting the recipe = %v, want none", got)
	}
}

func TestDeletingTheUserRemovesTheirShare(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	if _, err := e.shares.Create(ctx, e.member, e.recipe.ID, nil); err != nil {
		t.Fatal(err)
	}
	if err := e.users.Delete(ctx, e.owner, e.member.ID); err != nil {
		t.Fatal(err)
	}
	got, err := e.shares.List(ctx, e.owner, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("shares after deleting the user = %v, want none", got)
	}
}

func TestResolve(t *testing.T) {
	e := setup(t)
	ctx := context.Background()

	valid, err := e.shares.Create(ctx, e.member, e.recipe.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	validToken := valid.Path[len("/s/"):]
	if id, ok, err := e.shares.Resolve(ctx, validToken); err != nil || !ok || id != e.recipe.ID {
		t.Errorf("valid token: id=%q ok=%v err=%v, want %q true nil", id, ok, err, e.recipe.ID)
	}
	if _, ok, err := e.shares.Resolve(ctx, validToken+"x"); err != nil || ok {
		t.Errorf("token with an appended character: ok=%v err=%v, want false nil", ok, err)
	}

	if err := e.shares.Revoke(ctx, e.member, valid.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := e.shares.Resolve(ctx, validToken); err != nil || ok {
		t.Errorf("revoked: ok=%v err=%v, want false nil", ok, err)
	}

	paused, err := e.shares.Create(ctx, e.member, e.recipe.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	pausedToken := paused.Path[len("/s/"):]
	if _, err := e.settings.SetPublicShares(ctx, e.owner, false); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := e.shares.Resolve(ctx, pausedToken); err != nil || ok {
		t.Errorf("paused (sharing off): ok=%v err=%v, want false nil", ok, err)
	}
	if _, err := e.settings.SetPublicShares(ctx, e.owner, true); err != nil {
		t.Fatal(err)
	}
	if err := e.shares.Revoke(ctx, e.member, paused.ID); err != nil {
		t.Fatal(err)
	}

	e.clock = e.clock.Add(-40 * 24 * time.Hour)
	expired, err := e.shares.Create(ctx, e.member, e.recipe.ID, intPtr(1))
	if err != nil {
		t.Fatal(err)
	}
	expiredToken := expired.Path[len("/s/"):]
	limited, err := e.shares.Create(ctx, e.owner, e.recipe.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	limitedToken := limited.Path[len("/s/"):]
	e.clock = e.clock.Add(40 * 24 * time.Hour)
	if _, err := e.settings.SetShareLifetimes(ctx, e.owner, nil, intPtr(30)); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := e.shares.Resolve(ctx, expiredToken); err != nil || ok {
		t.Errorf("expired: ok=%v err=%v, want false nil", ok, err)
	}
	if _, ok, err := e.shares.Resolve(ctx, limitedToken); err != nil || ok {
		t.Errorf("limited: ok=%v err=%v, want false nil", ok, err)
	}
}

func TestSweepDeletesOwnExpiryButKeepsLimited(t *testing.T) {
	e := setup(t)
	ctx := context.Background()

	e.clock = e.clock.Add(-40 * 24 * time.Hour)
	expired, err := e.shares.Create(ctx, e.member, e.recipe.ID, intPtr(1))
	if err != nil {
		t.Fatal(err)
	}
	limited, err := e.shares.Create(ctx, e.owner, e.recipe.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	e.clock = e.clock.Add(40 * 24 * time.Hour)
	if _, err := e.settings.SetShareLifetimes(ctx, e.owner, nil, intPtr(30)); err != nil {
		t.Fatal(err)
	}

	if err := e.shares.Sweep(ctx, e.clock); err != nil {
		t.Fatal(err)
	}

	if _, err := e.shares.Mine(ctx, e.member, e.recipe.ID); !errors.Is(err, share.ErrNotFound) {
		t.Errorf("own-expired %q after sweep: err = %v, want ErrNotFound (row deleted)", expired.ID, err)
	}
	got, err := e.shares.Mine(ctx, e.owner, e.recipe.ID)
	if err != nil {
		t.Fatalf("limited %q after sweep: %v, want the row kept", limited.ID, err)
	}
	if got.Status != share.StatusLimited {
		t.Errorf("kept share status = %q, want %q", got.Status, share.StatusLimited)
	}
}
