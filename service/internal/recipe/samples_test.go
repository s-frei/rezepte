package recipe_test

import (
	"context"
	"strings"
	"testing"

	"github.com/s-frei/rezepte/service/internal/recipe"
	"github.com/s-frei/rezepte/service/internal/user"
)

func TestSamplesPerLocale(t *testing.T) {
	for _, l := range user.Locales {
		in, err := recipe.Samples(l)
		if err != nil {
			t.Fatalf("Samples(%q): %v", l, err)
		}
		if len(in) != 12 {
			t.Errorf("Samples(%q) returned %d recipes, want 12", l, len(in))
		}
		for i, r := range in {
			if strings.TrimSpace(r.Title) == "" {
				t.Errorf("Samples(%q)[%d] has an empty title", l, i)
			}
			if len(r.IngredientGroups) == 0 || len(r.Steps) == 0 {
				t.Errorf("Samples(%q)[%d] %q has no ingredient groups or no steps", l, i, r.Title)
			}
		}
	}
}

// TestSamplesSaveInEveryLocale runs every sample through Create, which is
// where an ingredient reference is resolved: a word missing from its step or
// a name the ingredient list does not hold fails here rather than at the
// first demo start.
func TestSamplesSaveInEveryLocale(t *testing.T) {
	ctx := context.Background()
	for _, l := range user.Locales {
		svc, userID := setup(t)
		in, err := recipe.Samples(l)
		if err != nil {
			t.Fatalf("Samples(%q): %v", l, err)
		}
		refs := 0
		for _, r := range in {
			if _, err := svc.Create(ctx, userID, r); err != nil {
				t.Errorf("Samples(%q) %q: %v", l, r.Title, err)
			}
			for _, s := range r.Steps {
				refs += len(s.References)
			}
		}
		if refs == 0 {
			t.Errorf("Samples(%q) carry no ingredient references", l)
		}
	}
}

func TestSamplesDifferPerLocale(t *testing.T) {
	de, err := recipe.Samples("de")
	if err != nil {
		t.Fatalf("Samples(de): %v", err)
	}
	en, err := recipe.Samples("en")
	if err != nil {
		t.Fatalf("Samples(en): %v", err)
	}
	if de[0].Title == en[0].Title {
		t.Errorf("both sets start with %q; they should be different recipes", de[0].Title)
	}
}

func TestSamplesFallsBackToTheBaseLocale(t *testing.T) {
	got, err := recipe.Samples("xx")
	if err != nil {
		t.Fatalf("Samples(xx): %v", err)
	}
	want, err := recipe.Samples(user.BaseLocale)
	if err != nil {
		t.Fatalf("Samples(%q): %v", user.BaseLocale, err)
	}
	if got[0].Title != want[0].Title {
		t.Errorf("Samples(xx) starts with %q, want the %q set starting with %q", got[0].Title, user.BaseLocale, want[0].Title)
	}
}

func TestSamplesReturnsAFreshCopy(t *testing.T) {
	a, _ := recipe.Samples("de")
	a[0].Title = "changed"
	b, _ := recipe.Samples("de")
	if b[0].Title == "changed" {
		t.Fatal("Samples shares state between calls")
	}
}

func TestCountFollowsCreateAndDelete(t *testing.T) {
	ctx := context.Background()
	svc, userID := setup(t) // service_test.go: fresh DB with one user
	if n, err := svc.Count(ctx); err != nil || n != 0 {
		t.Fatalf("Count on empty DB = %d, %v; want 0, nil", n, err)
	}
	samples, _ := recipe.Samples("de") // German search terms in other tests key off this set
	r, err := svc.Create(ctx, userID, samples[0])
	if err != nil {
		t.Fatal(err)
	}
	if n, _ := svc.Count(ctx); n != 1 {
		t.Fatalf("Count after create = %d, want 1", n)
	}
	if err := svc.Delete(ctx, r.ID); err != nil {
		t.Fatal(err)
	}
	if n, _ := svc.Count(ctx); n != 0 {
		t.Fatalf("Count after delete = %d, want 0", n)
	}
}
