package recipeimport_test

import (
	"fmt"
	"reflect"

	"github.com/s-frei/rezepte/service/internal/recipe"
)

func ptr[T any](v T) *T { return &v }

func equalIngredient(a, b recipe.Ingredient) bool { return reflect.DeepEqual(a, b) }

func deref[T any](p *T) any {
	if p == nil {
		return nil
	}
	return *p
}

func show(i recipe.Ingredient) string {
	return fmt.Sprintf("{q=%v u=%v n=%q note=%v}", deref(i.Quantity), deref(i.Unit), i.Name, deref(i.Note))
}
