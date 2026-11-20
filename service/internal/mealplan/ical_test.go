package mealplan

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

var stamp = time.Date(2026, 11, 20, 12, 30, 0, 0, time.FixedZone("CET", 3600))

func ical(t *testing.T, events ...feedEvent) string {
	t.Helper()
	var b strings.Builder
	if err := writeICal(&b, "rezepte.example.org", events, stamp); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

// unfold joins continuation lines as RFC 5545 3.1 says.
func unfold(s string) string { return strings.ReplaceAll(s, "\r\n ", "") }

func TestICalFoldsAtOctetsNotRunes(t *testing.T) {
	for _, summary := range []string{
		string([]rune(strings.Repeat("Käsespätzle mit Röstzwiebeln ", 4))[:100]),
		strings.Repeat("ä", 100), // two octets each, so an octet cut lands inside one
	} {
		folds(t, summary)
	}
}

func folds(t *testing.T, summary string) {
	t.Helper()
	out := ical(t, feedEvent{UID: "e1", Day: "2026-11-22", Summary: summary})
	for line := range strings.SplitSeq(strings.TrimSuffix(out, "\r\n"), "\r\n") {
		if len(line) > 75 {
			t.Errorf("line of %d octets: %q", len(line), line)
		}
		if !utf8.ValidString(line) {
			t.Errorf("line splits a rune: %q", line)
		}
	}
	if !strings.Contains(unfold(out), "\r\nSUMMARY:"+summary+"\r\n") {
		t.Fatalf("unfolded summary lost:\n%s", out)
	}
}

func TestICalEscapes(t *testing.T) {
	out := ical(t, feedEvent{UID: "e1", Day: "2026-11-22", Summary: "Reste; Salat, Brot\nmorgen"})
	if !strings.Contains(out, "\r\nSUMMARY:Reste\\; Salat\\, Brot\\nmorgen\r\n") {
		t.Fatalf("not escaped:\n%s", out)
	}
	if out := ical(t, feedEvent{UID: "e1", Day: "2026-11-22", Summary: `a\b`}); !strings.Contains(out, `SUMMARY:a\\b`) {
		t.Fatalf("backslash not escaped:\n%s", out)
	}
}

func TestICalAllDayDates(t *testing.T) {
	out := ical(t, feedEvent{UID: "e1", Day: "2026-11-22", Summary: "Dal", URL: "https://rezepte.example.org/recipes/dal"},
		feedEvent{UID: "e2", Day: "2026-12-31", Summary: "Fondue"})
	for _, want := range []string{
		"BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Rezepte//Meal plan//EN\r\n",
		"X-WR-CALNAME:Rezepte\r\n",
		"UID:e1@rezepte.example.org\r\n",
		"DTSTAMP:20261120T113000Z\r\n",
		"DTSTART;VALUE=DATE:20261122\r\nDTEND;VALUE=DATE:20261123\r\n",
		"URL:https://rezepte.example.org/recipes/dal\r\nDESCRIPTION:https://rezepte.example.org/recipes/dal\r\n",
		"DTSTART;VALUE=DATE:20261231\r\nDTEND;VALUE=DATE:20270101\r\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in\n%s", want, out)
		}
	}
	if strings.Count(out, "BEGIN:VEVENT") != 2 || strings.Count(out, "URL:") != 1 || !strings.HasSuffix(out, "END:VCALENDAR\r\n") {
		t.Fatalf("shape:\n%s", out)
	}
}

func TestICalUsesCRLF(t *testing.T) {
	out := ical(t, feedEvent{UID: "e1", Day: "2026-11-22", Summary: "Dal"})
	if strings.Count(out, "\n") != strings.Count(out, "\r\n") || !strings.HasSuffix(out, "\r\n") {
		t.Fatalf("bare LF in\n%q", out)
	}
}
