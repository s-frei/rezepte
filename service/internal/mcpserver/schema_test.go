package mcpserver

import (
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/google/jsonschema-go/jsonschema"

	"github.com/s-frei/rezepte/service/internal/db/dbtest"
	"github.com/s-frei/rezepte/service/internal/recipe"
)

func registeredAPI(t *testing.T) humatest.TestAPI {
	t.Helper()
	_, api := humatest.New(t)
	recipe.Register(api, recipe.NewService(dbtest.Open(t)))
	return api
}

func resolve(t *testing.T, s *jsonschema.Schema) *jsonschema.Resolved {
	t.Helper()
	r, err := s.Resolve(nil)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	return r
}

func validRecipe() map[string]any {
	return map[string]any{
		"title": "Soup", "description": "", "servings": 2,
		"prepMinutes": nil, "cookMinutes": nil, "sourceUrl": nil,
		"tags": []any{},
		"ingredientGroups": []any{map[string]any{"name": nil, "ingredients": []any{
			map[string]any{"quantity": 1, "unit": nil, "name": "Leek", "note": nil},
		}}},
		"steps": []any{map[string]any{"text": "Cook the leek.", "references": []any{}}},
	}
}

func TestInputSchemaAcceptsValidRecipe(t *testing.T) {
	s, err := recipeInputSchema(registeredAPI(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := resolve(t, s).Validate(map[string]any{"recipe": validRecipe()}); err != nil {
		t.Fatalf("valid recipe rejected: %v", err)
	}
}

func TestInputSchemaCarriesHumaConstraints(t *testing.T) {
	s, err := recipeInputSchema(registeredAPI(t))
	if err != nil {
		t.Fatal(err)
	}
	r := resolve(t, s)
	cases := map[string]func(map[string]any){
		"overlong title":    func(m map[string]any) { m["title"] = string(make([]byte, 201)) },
		"javascript url":    func(m map[string]any) { m["sourceUrl"] = "javascript:alert(1)" },
		"zero servings":     func(m map[string]any) { m["servings"] = 0 },
		"step without refs": func(m map[string]any) { m["steps"] = []any{map[string]any{"text": "Cook."}} },
		"unknown field":     func(m map[string]any) { m["calories"] = 300 },
		"null steps":        func(m map[string]any) { m["steps"] = nil },
		"null groups":       func(m map[string]any) { m["ingredientGroups"] = nil },
		"null tags":         func(m map[string]any) { m["tags"] = nil },
		"null ingredients": func(m map[string]any) {
			m["ingredientGroups"].([]any)[0].(map[string]any)["ingredients"] = nil
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			rec := validRecipe()
			mutate(rec)
			if err := r.Validate(map[string]any{"recipe": rec}); err == nil {
				t.Fatal("accepted, want rejection")
			}
		})
	}
}

func TestUpdateSchemaRequiresID(t *testing.T) {
	s, err := recipeUpdateSchema(registeredAPI(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := resolve(t, s).Validate(map[string]any{"recipe": validRecipe()}); err == nil {
		t.Fatal("accepted without id")
	}
}
