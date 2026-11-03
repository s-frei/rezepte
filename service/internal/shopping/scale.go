package shopping

import "math"

// Mirrors frontend/src/lib/recipe/scale.ts; keep the two in step.

const snapTolerance = 0.04

var (
	snapBelowOne = []float64{0, 0.25, 1.0 / 3, 0.5, 2.0 / 3, 0.75, 1}
	thirds       = []float64{1.0 / 3, 2.0 / 3}
)

// ScaleQuantity multiplies q from `from` servings to `to` servings. Equal or
// non-positive servings return q untouched.
func ScaleQuantity(q, from, to float64) float64 {
	if from == to || from <= 0 || to <= 0 {
		return q
	}
	return q * to / from
}

func snap(fraction float64, targets []float64) (float64, bool) {
	for _, t := range targets {
		if math.Abs(fraction-t) <= snapTolerance+1e-9 {
			return t, true
		}
	}
	return 0, false
}

// RoundScaled rounds a scaled quantity the way a cook reads it: below 1 snap
// to quarters/thirds or one decimal, 1 to <10 thirds or quarters, from 10
// whole numbers. The sign is preserved.
func RoundScaled(q float64) float64 {
	sign := 1.0
	if q < 0 {
		sign = -1
	}
	abs := math.Abs(q)
	var r float64
	switch {
	case abs < 1:
		if t, ok := snap(abs, snapBelowOne); ok {
			r = t
		} else {
			r = jsRound(abs*10) / 10
		}
	case abs < 10:
		whole := math.Floor(abs)
		if t, ok := snap(abs-whole, thirds); ok {
			r = whole + t
		} else {
			r = jsRound(abs*4) / 4
		}
	default:
		r = jsRound(abs)
	}
	return sign * r
}

// jsRound is Math.round for non-negative x: halves round up.
func jsRound(x float64) float64 { return math.Floor(x + 0.5) }
