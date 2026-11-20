package mealplan

import (
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"
)

// feedEvent is one all-day event of the calendar feed: UID is the entry's id
// (writeICal adds "@host"), Day is YYYY-MM-DD, URL is empty for a free text.
type feedEvent struct {
	UID, Day, Summary, URL string
}

// lineOctets is how long a content line may be (RFC 5545 3.1), CRLF not
// counted; a continuation line spends one of them on its leading space.
const lineOctets = 75

// icalText escapes a TEXT value (RFC 5545 3.3.11).
var icalText = strings.NewReplacer(`\`, `\\`, ";", `\;`, ",", `\,`, "\r\n", `\n`, "\n", `\n`, "\r", `\n`)

// writeICal writes events as an iCalendar file with all-day events, stamped
// now. Lines end in CRLF and fold at 75 octets without splitting a rune.
func writeICal(w io.Writer, host string, events []feedEvent, now time.Time) error {
	var bw strings.Builder
	line := func(s string) {
		first := true
		for {
			limit := lineOctets
			if !first {
				limit--
				bw.WriteString(" ")
			}
			if len(s) <= limit {
				bw.WriteString(s + "\r\n")
				return
			}
			cut := limit
			for !utf8.RuneStart(s[cut]) {
				cut--
			}
			bw.WriteString(s[:cut] + "\r\n")
			s, first = s[cut:], false
		}
	}
	line("BEGIN:VCALENDAR")
	line("VERSION:2.0")
	line("PRODID:-//Rezepte//Meal plan//EN")
	line("CALSCALE:GREGORIAN")
	line("METHOD:PUBLISH")
	line("X-WR-CALNAME:Rezepte")
	dtstamp := now.UTC().Format("20060102T150405Z")
	for _, e := range events {
		d, err := time.Parse(dayLayout, e.Day)
		if err != nil {
			return fmt.Errorf("feed event %s: %w", e.UID, err)
		}
		line("BEGIN:VEVENT")
		line("UID:" + e.UID + "@" + host)
		line("DTSTAMP:" + dtstamp)
		line("DTSTART;VALUE=DATE:" + d.Format("20060102"))
		line("DTEND;VALUE=DATE:" + d.AddDate(0, 0, 1).Format("20060102"))
		line("SUMMARY:" + icalText.Replace(e.Summary))
		if e.URL != "" {
			line("URL:" + e.URL)
			line("DESCRIPTION:" + icalText.Replace(e.URL))
		}
		line("END:VEVENT")
	}
	line("END:VCALENDAR")
	if _, err := io.WriteString(w, bw.String()); err != nil {
		return fmt.Errorf("write calendar: %w", err)
	}
	return nil
}
