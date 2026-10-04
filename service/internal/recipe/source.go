package recipe

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

// NormalizeSource is the form two source links are compared in: lower-case
// scheme and host, no fragment, no utm_* parameters. Anything that does not
// parse is returned as is.
func NormalizeSource(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return raw
	}
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	u.Fragment = ""
	q := u.Query()
	for k := range q {
		if strings.HasPrefix(strings.ToLower(k), "utm_") {
			q.Del(k)
		}
	}
	u.RawQuery = q.Encode()
	return u.String()
}

// SourceMatch is a stored recipe that came from the same link.
type SourceMatch struct {
	ID        string `json:"id"`
	Slug      string `json:"slug"`
	Title     string `json:"title"`
	CreatedBy Person `json:"createdBy"`
}

// FindBySource returns the newest recipe whose source is one of urls, after
// normalizing both sides, or nil.
func (s *Service) FindBySource(ctx context.Context, urls []string) (*SourceMatch, error) {
	want := map[string]bool{}
	prefixes := map[string]bool{}
	for _, raw := range urls {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		n := NormalizeSource(raw)
		want[n] = true
		if u, err := url.Parse(n); err == nil && u.Host != "" {
			prefixes[u.Scheme+"://"+u.Host+"/"] = true
		}
	}
	for prefix := range prefixes {
		// A cheap pre-filter (SQLite's LIKE ignores ASCII case, so a stored
		// host in capitals still matches); the comparison below decides.
		rows, err := s.q.ListRecipesBySourcePrefix(ctx, &prefix)
		if err != nil {
			return nil, fmt.Errorf("list recipes by source: %w", err)
		}
		for _, r := range rows {
			if r.SourceUrl == nil || !want[NormalizeSource(*r.SourceUrl)] {
				continue
			}
			people, err := s.peopleByID(ctx, []string{r.CreatedBy})
			if err != nil {
				return nil, fmt.Errorf("load author of %s: %w", r.ID, err)
			}
			a, ok := people[r.CreatedBy]
			if !ok {
				return nil, fmt.Errorf("load author of %s: not found", r.ID)
			}
			return &SourceMatch{ID: r.ID, Slug: r.Slug, Title: r.Title, CreatedBy: a}, nil
		}
	}
	return nil, nil
}
