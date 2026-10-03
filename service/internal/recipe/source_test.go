package recipe_test

import (
	"context"
	"testing"

	"github.com/s-frei/rezepte/service/internal/recipe"
)

func TestNormalizeSource(t *testing.T) {
	for in, want := range map[string]string{
		"https://www.chefkoch.de/rezepte/1/a.html?utm_source=x&utm_medium=y#comments": "https://www.chefkoch.de/rezepte/1/a.html",
		"https://example.com/r?id=3&utm_campaign=z":                                   "https://example.com/r?id=3",
		"HTTPS://Example.com/r":                                                       "https://example.com/r",
		"not a url":                                                                   "not a url",
	} {
		if got := recipe.NormalizeSource(in); got != want {
			t.Errorf("NormalizeSource(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFindBySource(t *testing.T) {
	svc, uid := setup(t)
	ctx := context.Background()
	in := loadFixtures(t)[0]
	in.Title = "Käsespätzle"
	src := "https://www.chefkoch.de/rezepte/1/a.html?utm_source=app"
	in.SourceURL = &src
	if _, err := svc.Create(ctx, uid, in); err != nil {
		t.Fatal(err)
	}

	m, err := svc.FindBySource(ctx, []string{"https://www.chefkoch.de/rezepte/1/a.html#top"})
	if err != nil || m == nil || m.Title != "Käsespätzle" || m.CreatedBy.Username != "sam" {
		t.Fatalf("match = %+v, %v", m, err)
	}
	if m, _ := svc.FindBySource(ctx, []string{"https://www.chefkoch.de/rezepte/2/b.html"}); m != nil {
		t.Fatalf("unexpected match %+v", m)
	}
	if m, err := svc.FindBySource(ctx, []string{"", " "}); m != nil || err != nil {
		t.Fatalf("empty candidates matched %+v, %v", m, err)
	}
}
