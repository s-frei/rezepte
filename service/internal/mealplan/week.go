package mealplan

import "time"

const (
	dayLayout = "2006-01-02"
	// maxSpan is how far apart the two ends of a range may be, in days.
	maxSpan = 62
)

// parseDay checks a YYYY-MM-DD day, which must name a real date.
func parseDay(s string) (time.Time, error) {
	d, err := time.Parse(dayLayout, s)
	if err != nil {
		return time.Time{}, invalid("day must be a date as YYYY-MM-DD")
	}
	return d, nil
}

// checkDay validates an optional day; nil is the tray.
func checkDay(day *string) error {
	if day == nil {
		return nil
	}
	_, err := parseDay(*day)
	return err
}

// checkRange validates an inclusive range of at most maxSpan days apart.
func checkRange(from, to string) error {
	f, err := parseDay(from)
	if err != nil {
		return err
	}
	t, err := parseDay(to)
	if err != nil {
		return err
	}
	if t.Before(f) || t.Sub(f) > maxSpan*24*time.Hour {
		return invalid("range must run forward and span at most 62 days")
	}
	return nil
}

// week returns Monday and Sunday of the week now falls in, in now's zone.
func week(now time.Time) (from, to string) {
	mon := now.AddDate(0, 0, -((int(now.Weekday()) + 6) % 7))
	return mon.Format(dayLayout), mon.AddDate(0, 0, 6).Format(dayLayout)
}
