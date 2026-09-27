package preview

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSummaryFoldsAndCuts(t *testing.T) {
	if got := Summary("  Knackig,\n\n frisch  "); got != "Knackig, frisch" {
		t.Errorf("short: %q", got)
	}
	long := strings.Repeat("Gurke und Chili, ", 30)
	got := Summary(long)
	cut, ok := strings.CutSuffix(got, "…")
	if utf8.RuneCountInString(got) > descriptionRunes+1 || !ok || !strings.HasPrefix(long, cut+" ") {
		t.Errorf("long: %d runes %q", utf8.RuneCountInString(got), got)
	}
}
