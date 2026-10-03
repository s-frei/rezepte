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
