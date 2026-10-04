// Package demo fills an empty Rezepte instance with the sample recipes and
// their photos, so documentation screenshots and manual testing have
// realistic data. A sample set with embedded photos gets those; a set
// without any gets deterministic placeholders instead. It writes only
// through the domain services and only when the recipes table is empty.
package demo

import (
	"bytes"
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"path"
	"strings"
	"time"

	"github.com/s-frei/rezepte/service/internal/auth"
	"github.com/s-frei/rezepte/service/internal/db"
	"github.com/s-frei/rezepte/service/internal/db/sqlc"
	"github.com/s-frei/rezepte/service/internal/image"
	"github.com/s-frei/rezepte/service/internal/recipe"
	"github.com/s-frei/rezepte/service/internal/settings"
	"github.com/s-frei/rezepte/service/internal/share"
	"github.com/s-frei/rezepte/service/internal/user"
)

// Credentials of the admin --demo creates when REZEPTE_ADMIN_PASSWORD is
// unset and the users table is empty.
const (
	AdminUser     = "demo"
	AdminPassword = "demo1234"
)

// imagedRecipes is how many samples, counted from the top of the list, get
// a placeholder image when their set has no photos. The rest show the
// overview's placeholder tiles.
const imagedRecipes = 9

// photos holds the demo photos as photos/<locale>/<slug>/<n>.jpg, numbered
// from 1 in gallery order. They are derived from the AI-generated originals
// in assets/demo-photos by `mise run demo-photos`; do not edit them here.
//
//go:embed photos
var embedded embed.FS

// photos is what photosFor reads; a variable so tests can seed a set that
// has no photos.
var photos fs.FS = embedded

// Members are the household the demo adds beside its admin, so that what
// one member shows another is on screen from the first start. Each signs in
// with the development-credential password, the username with 1234
// appended.
var Members = []string{"mila", "jonas"}

// displayNames names the demo's people after the pantry, per locale, so
// nobody takes them for real accounts and a login name never reads the same
// as the name shown beside it. Each keeps its username's initial, which is
// what the author circles show. A locale without its own set uses English.
var displayNames = map[user.Locale]map[string]string{
	"de": {AdminUser: "Dattel Dill", "mila": "Mila Majoran", "jonas": "Jonas Zimt", Invited: "Noah Nelke"},
	"en": {AdminUser: "Damson Dill", "mila": "Mila Marjoram", "jonas": "Jonas Cinnamon", Invited: "Noah Nutmeg"},
}

// displayName is the name the demo gives username in locale.
func displayName(locale user.Locale, username string) string {
	names, ok := displayNames[locale]
	if !ok {
		names = displayNames["en"]
	}
	return names[username]
}

// Invited is the one household member the demo leaves without a password,
// so the people list shows an account with an open setup link from the
// first start - what an admin sees while somebody has not yet signed in.
const Invited = "noah"

// memberMarks lists, per member, the samples (by index, the overview's order
// from the top) they mark tasty: the top card gets two marks, the rest of
// the first row one, so the count and the tasty order both have something
// to show.
var memberMarks = map[string][]int{
	"mila":  {0, 1, 3},
	"jonas": {0, 2, 5},
}

// memberRecipes lists, per member, the samples (by index, as in
// memberMarks) they wrote, so the admin meets recipes that are not their own:
// ones to mark tasty, and an author's name that is somebody else's. None is
// one its author marks, and none is the first sample or Macaroni Cheese,
// which the documentation photographs as the admin's.
var memberRecipes = map[string][]int{
	"jonas": {1, 8},
	"mila":  {2, 10},
}

// sampleComment is one demo comment on a recipe. author is a
// username (AdminUser for the demo person); ago places it in the past so the
// diary shows "today", "yesterday" and a weekday. Sample 0's two are timed
// for the user docs' frozen 10:30: a weekday and yesterday, both at dinner.
type sampleComment struct {
	index  int
	author string
	ago    time.Duration
	text   map[user.Locale]string
}

// memberComments are the demo's diary entries. Samples 0 and 3 are the
// admin's own recipes with entries the admin has not seen; on sample 1
// (Jonas's) the admin asked and Jonas answered, so the admin sees the dot as
// a participant; sample 2 (Mila's) is a conversation the admin is not part
// of and shows no dot for them. They are listed oldest first, the order
// seedComments writes them in, so ids follow time.
var memberComments = []sampleComment{
	{0, "mila", 6*24*time.Hour + 14*time.Hour + 50*time.Minute, map[user.Locale]string{
		"de": "Hab es mit Kartoffeln vom Markt gemacht, die mehligen sind hier wirklich besser.",
		"en": "Made it with floury potatoes from the market, they really are better here.",
	}},
	{1, AdminUser, 4*24*time.Hour + 2*time.Hour, map[user.Locale]string{
		"de": "Geht das auch ohne Rotwein?",
		"en": "Does this work without the black pudding?",
	}},
	{1, "jonas", 3*24*time.Hour + 20*time.Hour, map[user.Locale]string{
		"de": "Ja, mit kräftiger Brühe und einem Schuss Essig. Schmeckt anders, aber gut.",
		"en": "Yes, add an extra sausage and some mushrooms. Different, but good.",
	}},
	{2, "jonas", 3 * 24 * time.Hour, map[user.Locale]string{
		"de": "Die Soße ist mir zu dünn geworden, wie lange lässt du sie einkochen?",
		"en": "My sauce came out too thin, how long do you reduce it?",
	}},
	{3, "jonas", 2*24*time.Hour + 5*time.Hour, map[user.Locale]string{
		"de": "Doppelte Menge gemacht, nach einem Abend war nichts mehr da.",
		"en": "Made a double batch, nothing was left after one evening.",
	}},
	{2, "mila", 2*24*time.Hour + 4*time.Hour, map[user.Locale]string{
		"de": "Gut zehn Minuten, bis sie am Löffel hängen bleibt.",
		"en": "A good ten minutes, until it coats the spoon.",
	}},
	{0, "jonas", 14*time.Hour + 45*time.Minute, map[user.Locale]string{
		"de": "Ich habe die Kapern erst zum Schluss in die Soße gegeben, das gibt den letzten Pfiff.",
		"en": "Ours needed 10 more minutes in the oven, otherwise the crust stays pale.",
	}},
	{3, "mila", 2 * time.Hour, map[user.Locale]string{
		"de": "Beim nächsten Mal nehme ich etwas weniger Käse, die Röstzwiebeln bringen schon genug Würze mit.",
		"en": "Next time I'll go lighter on the salt, the malt vinegar adds enough already.",
	}},
}

// sampleAge is how long before Summary.Now the samples were written: older
// than the oldest entry in memberComments, so no entry predates its recipe.
const sampleAge = 7 * 24 * time.Hour

// lockedSample is the one of Mila's samples she locked, so the lock line
// shows under a recipe and the editor offers the policy another member
// cannot change.
const lockedSample = 2

// sharedSample is one public link the demo creates: the sample (by index,
// as in memberMarks) and the link's lifetime in days.
type sharedSample struct {
	index, days int
}

// Demo sharing: the admin and each member hold public links of their own, so
// Shared links, its filters and a recipe's marker have something to show,
// but public sharing itself is off, as on every instance until its owner
// turns it on - the links are paused, and turning sharing on is part of what
// one tries out. New links default to a week and run at most a month.
// Nothing touches the first sample: the documentation photographs it, and
// the admin's own link would put its marker into those pictures.
const (
	shareDefaultDays = 7
	shareMaxDays     = 30
)

var (
	adminShares  = []sharedSample{{4, 30}}
	memberShares = map[string][]sharedSample{
		"mila":  {{1, 7}, {3, 30}},
		"jonas": {{2, 7}, {5, 30}},
	}
)

// sampleToken is one API token the demo issues to its admin: what it may
// do, when it was issued and expires (days relative to the seed, an expiry
// of 0 meaning none) and when it was last used (0 for never).
type sampleToken struct {
	names                       map[user.Locale]string
	scopes                      []string
	issued, expires, lastUsedAt int
}

// Demo API tokens: one running and recently used, one without an expiry,
// one expired - the three shapes the API page's key tags draw, so the list
// shows its lifetime bars and its stamp instead of the empty placeholder.
var sampleTokens = []sampleToken{
	{
		names:  map[user.Locale]string{"en": "Claude Code on the laptop", "de": "Claude Code am Laptop"},
		scopes: []string{"recipes:read", "recipes:write"},
		issued: -45, expires: 45, lastUsedAt: -1,
	},
	{
		names:  map[user.Locale]string{"en": "Shopping list shortcut", "de": "Einkaufsliste-Kurzbefehl"},
		scopes: []string{"recipes:read"},
		issued: -120, lastUsedAt: -10,
	},
	{
		names:  map[user.Locale]string{"en": "Backup script", "de": "Backup-Skript"},
		scopes: []string{"recipes:read", "recipes:write", "recipes:delete", "users:read"},
		issued: -100, expires: -70,
	},
}

// ErrNoUsers is returned when no user exists to own the sample recipes.
var ErrNoUsers = errors.New("demo: no user to own the sample recipes")

// Summary reports what Seed did.
type Summary struct {
	Recipes int
	Images  int
	// RecipeIDs are the created recipes in sample order, the overview's
	// order from the top, for SeedMembers to mark.
	RecipeIDs []string
	// Members are the members AddMembers created, for SeedMembers to mark
	// and share as.
	Members []user.User
	// Skipped is true when the recipes table was not empty; nothing was written.
	Skipped bool
	// Now is the instant the samples were stamped at; SeedMembers dates the
	// diary entries and the admin's API tokens from it.
	Now time.Time
}

// Seed creates the sample recipes in locale's language, owned by the user
// named owner (or the first user by name when no such user exists) except
// for those memberRecipes gives to one of members, and
// uploads their images below imageDir. When any sample of the set has
// embedded photos, every sample gets its own photos, the first as cover, and
// a sample without photos stays without an image. A set with no photos at
// all gets a placeholder for each of its first imagedRecipes samples. It is
// idempotent: a database that already holds recipes is left untouched.
// The samples are stamped sampleAge before now, a minute apart, so the
// overview's order and the dates a recipe page prints follow now rather than
// the seeding run.
func Seed(ctx context.Context, conn *sql.DB, imageDir, owner string, members []user.User, locale user.Locale, now time.Time, logger *slog.Logger) (Summary, error) {
	recipes := recipe.NewService(conn, imageDir)
	n, err := recipes.Count(ctx)
	if err != nil {
		return Summary{}, err
	}
	if n > 0 {
		logger.Info("demo: recipes present, not seeding", "count", n)
		return Summary{Skipped: true}, nil
	}
	o, err := findOwner(ctx, user.NewService(conn, ""), owner)
	if err != nil {
		return Summary{}, err
	}
	samples, err := recipe.Samples(locale)
	if err != nil {
		return Summary{}, err
	}
	sets := make([][][]byte, len(samples))
	withPhotos := false
	for i, s := range samples {
		if sets[i], err = photosFor(recipe.Slugify(s.Title)); err != nil {
			return Summary{}, err
		}
		withPhotos = withPhotos || len(sets[i]) > 0
	}
	authors := make([]user.User, len(samples))
	for i := range authors {
		authors[i] = o
	}
	for _, m := range members {
		for _, i := range memberRecipes[m.Username] {
			if i < len(authors) {
				authors[i] = m
			}
		}
	}
	if lockedSample < len(samples) && authors[lockedSample].Username == "mila" {
		samples[lockedSample].EditPolicy = recipe.PolicyLocked
	}
	images := image.NewService(conn, imageDir)
	sum := Summary{RecipeIDs: make([]string, len(samples)), Members: members, Now: now}
	// The overview sorts by updated_at desc: seeding back to front puts the
	// first sample on top.
	for i := len(samples) - 1; i >= 0; i-- {
		a := authors[i]
		stamp := now.Add(-sampleAge - time.Duration(i)*time.Minute)
		clock := func() time.Time { return stamp }
		recipes.SetClock(clock)
		images.SetClock(clock)
		r, err := recipes.Create(ctx, a.ID, samples[i])
		if err != nil {
			return sum, fmt.Errorf("create sample %q: %w", samples[i].Title, err)
		}
		sum.Recipes++
		sum.RecipeIDs[i] = r.ID
		if withPhotos {
			for n, data := range sets[i] {
				if _, err := images.Upload(ctx, r.ID, a, bytes.NewReader(data)); err != nil {
					return sum, fmt.Errorf("upload photo %d for %q: %w", n+1, r.Title, err)
				}
				sum.Images++
			}
		} else if i < imagedRecipes {
			data, err := Placeholder(i, r.Title)
			if err != nil {
				return sum, err
			}
			if _, err := images.Upload(ctx, r.ID, a, bytes.NewReader(data)); err != nil {
				return sum, fmt.Errorf("upload placeholder for %q: %w", r.Title, err)
			}
			sum.Images++
		}
	}
	logger.Info("demo: sample data seeded", "recipes", sum.Recipes, "images", sum.Images)
	return sum, nil
}

// photosFor returns the embedded photos of the sample whose slug is slug,
// 1.jpg first, stopping at the first missing number; nil when it has none.
// The slug alone identifies the sample: sample sets are different dishes,
// and a language falling back to another's set finds that set's photos.
func photosFor(slug string) ([][]byte, error) {
	dirs, err := fs.Glob(photos, path.Join("photos", "*", slug))
	if err != nil || len(dirs) == 0 {
		return nil, err
	}
	var set [][]byte
	for n := 1; ; n++ {
		data, err := fs.ReadFile(photos, path.Join(dirs[0], fmt.Sprintf("%d.jpg", n)))
		if errors.Is(err, fs.ErrNotExist) {
			return set, nil
		}
		if err != nil {
			return nil, fmt.Errorf("read photo %d of %s: %w", n, slug, err)
		}
		set = append(set, data)
	}
}

// findOwner resolves the user named username, falling back to the first
// user (List orders by username) so seeding works whatever the instance
// owner is called. The recipes it creates are that user's own, so the edit
// rule lets them add the placeholder images whatever their role.
func findOwner(ctx context.Context, users *user.Service, username string) (user.User, error) {
	list, err := users.List(ctx)
	if err != nil {
		return user.User{}, err
	}
	if len(list) == 0 {
		return user.User{}, ErrNoUsers
	}
	for _, u := range list {
		if u.Username == username {
			return u, nil
		}
	}
	return list[0], nil
}

// AddMembers creates Members, plus Invited, in locale's language, before
// Seed, so the samples in memberRecipes can be theirs, and gives the admin
// its demo display name. The admin and every member get an unverified
// sample email (name@example.com), profile data a real account would fill
// in eventually - the admin's prefills the test mail. Invited gets neither
// an email nor a password, so it signs in only through a setup link or an
// identity provider - SeedMembers issues that link, and sending it asks for
// the address. Like the seed it writes
// nothing to an instance that already holds recipes, since the members
// belong to the sample data rather than to an instance in use, and a name
// somebody already holds is left to them: that account gets no recipes,
// marks, links or email from the demo.
//
// The members' passwords are public, so only a demo that runs on the
// published demo credentials may call it; an operator who set their own
// admin password gets no accounts they did not ask for.
func AddMembers(ctx context.Context, conn *sql.DB, locale user.Locale, logger *slog.Logger) ([]user.User, error) {
	n, err := recipe.NewService(conn, "").Count(ctx)
	if err != nil || n > 0 {
		return nil, err
	}
	users := user.NewService(conn, "")
	if err := nameAdmin(ctx, users, locale); err != nil {
		return nil, err
	}
	var members []user.User
	add := func(name, password string) error {
		m, err := users.Create(ctx, user.CreateParams{
			Username:    name,
			Password:    password,
			Role:        user.RoleUser,
			DisplayName: displayName(locale, name),
			Locale:      locale,
		})
		if errors.Is(err, user.ErrUsernameTaken) {
			logger.Info("demo: member name taken, not seeding it", "user", name)
			return nil
		}
		if err != nil {
			return fmt.Errorf("create member %s: %w", name, err)
		}
		if name != Invited {
			addr := name + "@example.com"
			if m, err = users.SetProfile(ctx, m.ID, user.ProfileUpdate{Email: &addr}); err != nil {
				return fmt.Errorf("set email for %s: %w", name, err)
			}
		}
		members = append(members, m)
		return nil
	}
	for _, name := range Members {
		if err := add(name, name+"1234"); err != nil {
			return nil, err
		}
	}
	if err := add(Invited, ""); err != nil {
		return nil, err
	}
	logger.Info("demo: members added", "users", strings.Join(Members, ", "))
	return members, nil
}

// nameAdmin gives AdminUser its demo display name and sample email;
// EnsureSuperadmin created it with the username as its name.
func nameAdmin(ctx context.Context, users *user.Service, locale user.Locale) error {
	list, err := users.List(ctx)
	if err != nil {
		return fmt.Errorf("list users: %w", err)
	}
	for _, u := range list {
		if u.Username != AdminUser {
			continue
		}
		name := displayName(locale, AdminUser)
		addr := AdminUser + "@example.com"
		if _, err := users.SetProfile(ctx, u.ID, user.ProfileUpdate{DisplayName: &name, Email: &addr}); err != nil {
			return fmt.Errorf("name %s: %w", AdminUser, err)
		}
	}
	return nil
}

// SeedMembers issues Invited's open setup link, links demoIdentityMember
// (jonas) to an identity at issuer when it is non-empty (an operator running
// the demo without OIDC configured passes ""), marks the samples in
// memberMarks tasty on behalf of the members Seed was given, and creates the
// public links in adminShares and memberShares - the admin's as the user
// named owner, who must be the instance owner, since only the owner
// switches sharing on, which creating a link needs. It is switched off
// again once the links exist, so they start out paused. The diary entries
// in memberComments are dated back from sum.Now. Last it issues that admin
// the sampleTokens, dated from sum.Now too. After a skipped seed it writes
// nothing.
func SeedMembers(ctx context.Context, conn *sql.DB, sum Summary, owner, issuer string) error {
	if sum.Skipped {
		return nil
	}
	users := user.NewService(conn, "")
	recipes := recipe.NewService(conn, "")
	instance := settings.NewService(conn)
	shares := share.NewService(conn, instance, users)
	admin, err := findOwner(ctx, users, owner)
	if err != nil {
		return err
	}
	for _, m := range sum.Members {
		if m.Username != Invited {
			continue
		}
		// Issued once, by the run that wrote the samples: a later --demo run
		// over the same data directory is a skipped seed and returns above,
		// so the link keeps its first expiry.
		if _, err := auth.NewService(conn, users).IssueSetupLink(ctx, admin, m.ID); err != nil {
			return fmt.Errorf("issue setup link for %s: %w", m.Username, err)
		}
	}
	if issuer != "" {
		for _, m := range sum.Members {
			if m.Username != demoIdentityMember {
				continue
			}
			sub := dexSubject(m.Username, dexConnector)
			if err := users.LinkIdentity(ctx, m.ID, issuer, sub); err != nil {
				return fmt.Errorf("link %s's demo identity: %w", m.Username, err)
			}
		}
	}
	sharing := admin.Role.IsSuperadmin()
	if sharing {
		if _, err := instance.SetPublicShares(ctx, admin, true); err != nil {
			return fmt.Errorf("turn public sharing on: %w", err)
		}
		def, maxDays := shareDefaultDays, shareMaxDays
		if _, err := instance.SetShareLifetimes(ctx, admin, &def, &maxDays); err != nil {
			return fmt.Errorf("set share lifetimes: %w", err)
		}
		if err := shareSamples(ctx, shares, admin, sum, adminShares); err != nil {
			return err
		}
	}
	for _, m := range sum.Members {
		for _, i := range memberMarks[m.Username] {
			if i >= len(sum.RecipeIDs) {
				continue
			}
			if err := recipes.SetTasty(ctx, m.ID, sum.RecipeIDs[i], true); err != nil {
				return fmt.Errorf("mark sample %d tasty for %s: %w", i, m.Username, err)
			}
		}
		if sharing {
			if err := shareSamples(ctx, shares, m, sum, memberShares[m.Username]); err != nil {
				return err
			}
		}
	}
	if err := seedComments(ctx, conn, sum, admin); err != nil {
		return err
	}
	if sharing {
		if _, err := instance.SetPublicShares(ctx, admin, false); err != nil {
			return fmt.Errorf("turn public sharing off: %w", err)
		}
	}
	return issueSampleTokens(ctx, auth.NewTokenService(conn, users), admin, sum.Now)
}

// issueSampleTokens issues sampleTokens to admin, each with its clock set to
// when it was issued and, if it was used, authenticated once at that time so
// the token service records the use itself.
func issueSampleTokens(ctx context.Context, tokens *auth.TokenService, admin user.User, now time.Time) error {
	day := func(offset int) time.Time { return now.AddDate(0, 0, offset) }
	defer tokens.SetClock(time.Now)
	for _, sample := range sampleTokens {
		name, ok := sample.names[admin.Locale]
		if !ok {
			name = sample.names["en"]
		}
		var expires *time.Time
		if sample.expires != 0 {
			at := day(sample.expires)
			expires = &at
		}
		tokens.SetClock(func() time.Time { return day(sample.issued) })
		raw, _, err := tokens.Create(ctx, admin.ID, name, sample.scopes, expires)
		if err != nil {
			return fmt.Errorf("issue demo token %q: %w", name, err)
		}
		if sample.lastUsedAt != 0 {
			tokens.SetClock(func() time.Time { return day(sample.lastUsedAt) })
			if _, err := tokens.Authenticate(ctx, raw); err != nil {
				return fmt.Errorf("use demo token %q: %w", name, err)
			}
		}
	}
	return nil
}

// shareSamples creates a public link to each of links' samples as actor.
func shareSamples(ctx context.Context, shares *share.Service, actor user.User, sum Summary, links []sharedSample) error {
	for _, l := range links {
		if l.index >= len(sum.RecipeIDs) {
			continue
		}
		days := l.days
		if _, err := shares.Create(ctx, actor, sum.RecipeIDs[l.index], &days); err != nil {
			return fmt.Errorf("share sample %d as %s: %w", l.index, actor.Username, err)
		}
	}
	return nil
}

// seedComments writes memberComments in the admin's locale, oldest first so
// ids follow time, and raises the author's watermark after each entry as if
// they had the recipe page open while writing, so a member signing in finds
// only the entries after their own new.
func seedComments(ctx context.Context, conn *sql.DB, sum Summary, admin user.User) error {
	ids := map[string]string{admin.Username: admin.ID}
	for _, m := range sum.Members {
		ids[m.Username] = m.ID
	}
	q := sqlc.New(conn)
	for _, c := range memberComments {
		author := c.author
		if author == AdminUser {
			author = admin.Username
		}
		authorID, ok := ids[author]
		if !ok || c.index >= len(sum.RecipeIDs) {
			continue
		}
		text, ok := c.text[admin.Locale]
		if !ok {
			text = c.text["en"]
		}
		recipeID := sum.RecipeIDs[c.index]
		id, err := q.InsertComment(ctx, sqlc.InsertCommentParams{
			RecipeID: recipeID, AuthorID: &authorID, Body: text, CreatedAt: db.FormatTime(sum.Now.Add(-c.ago)),
		})
		if err != nil {
			return fmt.Errorf("seed comment on sample %d: %w", c.index, err)
		}
		if err := q.RaiseCommentWatermark(ctx, sqlc.RaiseCommentWatermarkParams{
			UserID: authorID, RecipeID: recipeID, LastSeen: id,
		}); err != nil {
			return fmt.Errorf("seed comment watermark: %w", err)
		}
	}
	return nil
}
