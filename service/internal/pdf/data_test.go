package pdf

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite schema/data.v1.schema.json")

const schemaPath = "schema/data.v1.schema.json"

func TestSchemaUpToDate(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteSchema(&buf); err != nil {
		t.Fatal(err)
	}
	if *update {
		if err := os.WriteFile(schemaPath, buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	got, err := os.ReadFile(schemaPath)
	if err != nil || !bytes.Equal(got, buf.Bytes()) {
		t.Fatal("schema is stale; run: mise exec -- go test ./internal/pdf -run TestSchemaUpToDate -update")
	}
}

// resolve follows a $ref into $defs.
func resolve(t *testing.T, root, s map[string]any) map[string]any {
	t.Helper()
	ref, ok := s["$ref"].(string)
	if !ok {
		return s
	}
	return root["$defs"].(map[string]any)[strings.TrimPrefix(ref, "#/$defs/")].(map[string]any)
}

func typesOf(s map[string]any) []string {
	switch v := s["type"].(type) {
	case string:
		return []string{v}
	case []any:
		var out []string
		for _, x := range v {
			out = append(out, x.(string))
		}
		return out
	}
	return nil
}

func has(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func TestSchemaNullability(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteSchema(&buf); err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	if err := json.Unmarshal(buf.Bytes(), &root); err != nil {
		t.Fatal(err)
	}
	defs := root["$defs"].(map[string]any)
	prop := func(def, name string) map[string]any {
		t.Helper()
		d, ok := defs[def].(map[string]any)
		if !ok {
			t.Fatalf("no def %s", def)
		}
		p, ok := d["properties"].(map[string]any)[name].(map[string]any)
		if !ok {
			t.Fatalf("no property %s.%s", def, name)
		}
		return p
	}
	// Nullable means the type list has null, or the target def does.
	nullable := func(p map[string]any) bool {
		return has(typesOf(p), "null") || has(typesOf(resolve(t, root, p)), "null")
	}
	for _, c := range [][2]string{{"Recipe", "source"}, {"Recipe", "photo"}, {"Recipe", "qr"}, {"Data", "book"}, {"Times", "prep"}} {
		if !nullable(prop(c[0], c[1])) {
			t.Errorf("%s.%s must accept null", c[0], c[1])
		}
	}
	for _, c := range [][2]string{{"Recipe", "tags"}, {"Recipe", "steps"}, {"Group", "ingredients"}} {
		if nullable(prop(c[0], c[1])) {
			t.Errorf("%s.%s must not accept null", c[0], c[1])
		}
	}
}

func TestNullsNotMissing(t *testing.T) {
	d := Data{Recipes: []Recipe{Merge(ClientRecipe{}, ServerRecipe{})}}
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"source":null`, `"photo":null`, `"qr":null`, `"prep":null`, `"book":null`, `"tags":[]`, `"steps":[]`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("missing %s in %s", want, b)
		}
	}
}

func TestFixturesMatchSchema(t *testing.T) {
	files, _ := filepath.Glob("testdata/*.json")
	if len(files) != 5 {
		t.Fatalf("want 5 fixtures, got %v", files)
	}
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.DisallowUnknownFields()
		var d Data
		if err := dec.Decode(&d); err != nil {
			t.Errorf("%s: %v", f, err)
			continue
		}
		if d.SchemaVersion != SchemaVersion || len(d.Recipes) == 0 {
			t.Errorf("%s: bad version or no recipes", f)
		}
		// Round trip must keep every key: a fixture is a complete document.
		back, _ := json.Marshal(d)
		var a, b any
		_ = json.Unmarshal(raw, &a)
		_ = json.Unmarshal(back, &b)
		if !jsonEqual(a, b) {
			t.Errorf("%s: not a complete Data document (round trip differs)", f)
		}
	}
}

func jsonEqual(a, b any) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return bytes.Equal(x, y)
}
