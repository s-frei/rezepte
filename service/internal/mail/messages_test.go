package mail

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// The embedded copy is generated from frontend/messages; a key added there
// without `mise run //service:generate` fails here, inside `mise run check`.
func TestMessagesMatchFrontend(t *testing.T) {
	files, _ := filepath.Glob("../../../frontend/messages/*.json")
	if len(files) == 0 {
		t.Fatal("no frontend catalogs found")
	}
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		want, err := FilterCatalog(raw)
		if err != nil {
			t.Fatal(err)
		}
		locale := strings.TrimSuffix(filepath.Base(f), ".json")
		if got := catalogs[locale]; !reflect.DeepEqual(got, want) {
			t.Errorf("%s: embedded copy is stale; run mise run //service:generate", locale)
		}
	}
}

// The generated copy holds the template keys and nothing else: UI keys about
// mail are settings_mail_*, so the mail_ prefix never pulls UI copy in.
func TestCatalogHoldsOnlyMailKeys(t *testing.T) {
	for locale, keys := range catalogs {
		for k := range keys {
			if !strings.HasPrefix(k, "mail_") {
				t.Errorf("%s: %q is not a mail key", locale, k)
			}
		}
		for _, k := range []string{"mail_reset_subject", "mail_hint_body", "mail_hint_provider_fallback", "mail_confirm_body_admin"} {
			if keys[k] == "" {
				t.Errorf("%s: %q missing", locale, k)
			}
		}
	}
}
