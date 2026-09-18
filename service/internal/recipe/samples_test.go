package recipe_test

import (
	"context"
	"testing"

	"github.com/s-frei/rezepte/service/internal/recipe"
)

func TestSamplesReturnsTheTwelveGermanRecipes(t *testing.T) {
	in, err := recipe.Samples()
	if err != nil {
		t.Fatalf("Samples: %v", err)
	}
	if len(in) != 12 {
		t.Fatalf("len = %d, want 12", len(in))
	}
	if in[0].Title != "Königsberger Klopse" || len(in[0].IngredientGroups) != 2 {
		t.Fatalf("first sample = %q with %d groups, want Königsberger Klopse with 2 groups", in[0].Title, len(in[0].IngredientGroups))
	}
	if in[11].Title != "Maultaschen in der Brühe" {
		t.Fatalf("last sample = %q, want Maultaschen in der Brühe", in[11].Title)
	}
}

func TestSamplesReturnsAFreshCopy(t *testing.T) {
	a, _ := recipe.Samples()
	a[0].Title = "changed"
	b, _ := recipe.Samples()
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
	samples, _ := recipe.Samples()
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
