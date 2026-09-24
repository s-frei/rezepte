package mcpserver

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/jsonschema-go/jsonschema"
)

// recipeDefs are the huma component schemas a recipe document is built from.
var recipeDefs = []string{"Input", "IngredientGroup", "Ingredient", "Step", "IngredientRef"}

// recipeInputSchema is create_recipe's input schema: {"recipe": Input}.
//
// The constraints on recipe.Input live only in its huma struct tags -
// recipe.Service validates none of them - so the tool schema is taken from
// the same registry REST validates against rather than inferred again. The
// SDK validates every call against it, which is what keeps a 10,000-character
// title or a javascript: source URL out through MCP as well.
func recipeInputSchema(api huma.API) (*jsonschema.Schema, error) {
	return recipeSchema(api, false)
}

// recipeUpdateSchema is update_recipe's input schema: {"id", "recipe": Input}.
func recipeUpdateSchema(api huma.API) (*jsonschema.Schema, error) {
	return recipeSchema(api, true)
}

func recipeSchema(api huma.API, withID bool) (*jsonschema.Schema, error) {
	defs, err := componentDefs(api)
	if err != nil {
		return nil, err
	}
	requireReferences(defs)
	delete(prop(defs, "Input"), "$schema")

	props := map[string]any{
		"recipe": map[string]any{"$ref": "#/$defs/Input"},
	}
	required := []string{"recipe"}
	if withID {
		props["id"] = map[string]any{"type": "string", "minLength": 1, "description": "The recipe's id, as get_recipe returns it"}
		required = []string{"id", "recipe"}
	}
	doc := map[string]any{
		"type":                 "object",
		"properties":           props,
		"required":             required,
		"additionalProperties": false,
		"$defs":                defs,
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("marshal recipe schema: %w", err)
	}
	var s jsonschema.Schema
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("unmarshal recipe schema: %w", err)
	}
	return &s, nil
}

// componentDefs copies the recipe components out of the huma registry as
// plain JSON maps, pointing their $refs at #/$defs instead of the OpenAPI
// document's #/components/schemas.
func componentDefs(api huma.API) (map[string]any, error) {
	all := api.OpenAPI().Components.Schemas.Map()
	defs := make(map[string]any, len(recipeDefs))
	for _, name := range recipeDefs {
		s, ok := all[name]
		if !ok {
			return nil, fmt.Errorf("schema %q not registered; call recipe.Register first", name)
		}
		raw, err := json.Marshal(s)
		if err != nil {
			return nil, fmt.Errorf("marshal %s: %w", name, err)
		}
		raw = []byte(strings.ReplaceAll(string(raw), "#/components/schemas/", "#/$defs/"))
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			return nil, fmt.Errorf("unmarshal %s: %w", name, err)
		}
		defs[name] = m
	}
	return defs, nil
}

// requireReferences makes every step state its references. REST leaves the
// field optional, so a client that drops it silently deletes every
// ingredient link in the recipe; a model rewriting a step is exactly such a
// client, so over MCP it has to send them, or an explicit [].
func requireReferences(defs map[string]any) {
	step := defs["Step"].(map[string]any)
	req, _ := step["required"].([]any)
	step["required"] = append(req, "references")
	prop(defs, "Step")["references"].(map[string]any)["type"] = "array"
}

func prop(defs map[string]any, name string) map[string]any {
	return defs[name].(map[string]any)["properties"].(map[string]any)
}
