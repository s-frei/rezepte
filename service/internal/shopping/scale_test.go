package shopping_test

import (
	"math"
	"testing"

	"github.com/s-frei/rezepte/service/internal/shopping"
)

func TestRoundScaledMatchesFrontend(t *testing.T) {
	// Rows from frontend/src/lib/recipe/scale.test.ts plus the brief's.
	rows := []struct{ q, want float64 }{
		{0.24, 0.25}, {0.26, 0.25}, {0.29, 0.25}, {0.3, 1.0 / 3}, {0.34, 1.0 / 3}, {0.5, 0.5}, {0.66, 2.0 / 3}, {0.74, 0.75},
		{0.97, 1}, {0.4, 0.4}, {0.17, 0.2}, {0.12, 0.1}, {0.04, 0},
		{1.1, 1}, {1.15, 1.25}, {1.3, 1 + 1.0/3}, {1.33, 1 + 1.0/3}, {1.66, 1 + 2.0/3}, {2.6, 2.5}, {2.9, 3}, {7.375, 7.5}, {9.99, 10},
		{10.4, 10}, {12.4, 12}, {12.5, 13}, {333.33, 333}, {1250, 1250}, {1333.4, 1333},
		{0, 0}, {2, 2}, {400, 400}, {-0.5, -0.5}, {-1.5, -1.5}, {-12.4, -12},
	}
	for _, r := range rows {
		if got := shopping.RoundScaled(r.q); math.Abs(got-r.want) > 1e-9 {
			t.Errorf("%v → %v, want %v", r.q, got, r.want)
		}
	}
}

func TestScaleQuantity(t *testing.T) {
	rows := []struct{ q, from, to, want float64 }{
		{200, 4, 8, 400}, {200, 4, 6, 300}, {3, 4, 1, 0.75}, {0.5, 4, 2, 0.25}, {150, 4, 3, 112.5}, {500, 4, 6, 750},
	}
	for _, r := range rows {
		if got := shopping.ScaleQuantity(r.q, r.from, r.to); math.Abs(got-r.want) > 1e-9 {
			t.Errorf("%v %v→%v: %v, want %v", r.q, r.from, r.to, got, r.want)
		}
	}
}

func TestScaleQuantityGuards(t *testing.T) {
	if shopping.ScaleQuantity(1.33, 3, 3) != 1.33 {
		t.Error("from == to")
	}
	if shopping.ScaleQuantity(200, 0, 8) != 200 || shopping.ScaleQuantity(200, -1, 8) != 200 {
		t.Error("from <= 0")
	}
	if shopping.ScaleQuantity(200, 4, 0) != 200 || shopping.ScaleQuantity(200, 4, -2) != 200 {
		t.Error("to <= 0")
	}
}
