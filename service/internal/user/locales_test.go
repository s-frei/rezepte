package user_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/s-frei/rezepte/service/internal/user"
)

// The set of interface languages cannot live in one place: Go needs it as a
// slice, SQLite as a CHECK constraint, and inlang as JSON that Paraglide
// compiles the frontend from. No import reaches across all three - go:embed
// cannot leave the module, and the constraint is inside a migration that has
// already run everywhere.
//
// So the three lists stay, and these tests hold them to each other. Adding a
// language means editing all three; forgetting one fails here, naming the
// file that is behind, instead of at runtime with a 422 nobody expects.

// repoFile resolves a path relative to the repository root. The tests run in
// service/internal/user, four levels down.
func repoFile(t *testing.T, rel string) string {
	t.Helper()
	return filepath.Join("..", "..", "..", rel)
}

func TestMigrationCheckMatchesLocales(t *testing.T) {
	path := repoFile(t, "service/internal/db/migrations/0001_users_and_sessions.sql")
	sql, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}

	// locale TEXT NOT NULL CHECK (locale IN ('en', 'de'))
	re := regexp.MustCompile(`CHECK\s*\(\s*locale\s+IN\s*\(([^)]*)\)`)
	match := re.FindSubmatch(sql)
	if match == nil {
		t.Fatalf("no CHECK constraint on users.locale found in %s", path)
	}
	found := regexp.MustCompile(`'([^']*)'`).FindAllStringSubmatch(string(match[1]), -1)

	if len(found) != len(user.Locales) {
		t.Fatalf("migration allows %d locales, user.Locales has %d - add the missing one to %s",
			len(found), len(user.Locales), path)
	}
	for i, m := range found {
		if user.Locale(m[1]) != user.Locales[i] {
			t.Errorf("migration locale %d is %q, user.Locales has %q", i, m[1], user.Locales[i])
		}
	}
}

func TestInlangSettingsMatchLocales(t *testing.T) {
	path := repoFile(t, "frontend/project.inlang/settings.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read inlang settings: %v", err)
	}
	var settings struct {
		BaseLocale string   `json:"baseLocale"`
		Locales    []string `json:"locales"`
	}
	if err := json.Unmarshal(raw, &settings); err != nil {
		t.Fatalf("parse inlang settings: %v", err)
	}

	if len(settings.Locales) != len(user.Locales) {
		t.Fatalf("%s lists %d locales, user.Locales has %d - add the missing one",
			path, len(settings.Locales), len(user.Locales))
	}
	for i, l := range settings.Locales {
		if user.Locale(l) != user.Locales[i] {
			t.Errorf("inlang locale %d is %q, user.Locales has %q", i, l, user.Locales[i])
		}
	}

	// Locales[0] is the base locale on both sides: the Go service falls back
	// to it when no REZEPTE_LOCALE is set, Paraglide when no strategy
	// resolves. They have to be the same language or an unconfigured
	// instance renders in one and stores the other.
	if user.Locale(settings.BaseLocale) != user.Locales[0] {
		t.Errorf("inlang baseLocale is %q, user.Locales[0] is %q", settings.BaseLocale, user.Locales[0])
	}
}

func TestEveryLocaleHasACatalogue(t *testing.T) {
	for _, l := range user.Locales {
		path := repoFile(t, filepath.Join("frontend", "messages", string(l)+".json"))
		if _, err := os.Stat(path); err != nil {
			t.Errorf("no message catalogue for %q: %v", l, err)
		}
	}
}

func TestEveryLocaleHasSampleRecipes(t *testing.T) {
	for _, l := range user.Locales {
		path := repoFile(t, filepath.Join("service", "internal", "recipe", "testdata", "recipes."+string(l)+".json"))
		if _, err := os.Stat(path); err != nil {
			t.Errorf("no sample recipes for %q: %v", l, err)
		}
	}
}
