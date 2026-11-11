package shopping_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/s-frei/rezepte/service/internal/recipe"

	"github.com/s-frei/rezepte/service/internal/i18n"
	"github.com/s-frei/rezepte/service/internal/shopping"
)

func TestParseListText(t *testing.T) {
	rows := []struct {
		name, text, list string
		items            int
	}{
		{"spec example", "Liste: Wocheneinkauf\n2 l Milch, 1,5 %\n500 g Mehl\n6 Eier\n1 Bund Petersilie, glatt\n1 Glas Kapern\nZahnpasta", "Wocheneinkauf", 6},
		{"fence and chatter", "Hier ist sie:\n```\nListe: Party\n- [x] 2 Flaschen Wasser\n```\nViel Spaß!", "Party", 1},
		{"heading lines skipped", "**Obst:**\n3 Äpfel\nGemüse:\n1 Gurke", "", 2},
		{"upper case key", "LISTE: X\n1 Ei", "X", 1},
		{"english lower case key", "list: X\n1 Ei", "X", 1},
		{"second name line skipped", "Liste: A\n1 Ei\nListe: B\n2 Eier", "A", 2},
		{"merged row", "2 Stück + 200 g Zwiebeln", "", 2},
		{"merged row without unit", "2 + 200 g + 1 kg Zwiebeln", "", 3},
	}
	for _, r := range rows {
		lt, err := shopping.ParseListText(r.text)
		if err != nil {
			t.Fatalf("%s: %v", r.name, err)
		}
		if lt.Name != r.list || len(lt.Items) != r.items {
			t.Errorf("%s: name %q, %d items %+v", r.name, lt.Name, len(lt.Items), lt.Items)
		}
	}
	lt, _ := shopping.ParseListText("Liste: Wocheneinkauf\n2 l Milch, 1,5 %")
	wantItem(t, lt.Items[0], recipe.Ingredient{Quantity: numPtr(2), Unit: strPtr("l"), Name: "Milch", Note: strPtr("1,5 %")})
	lt, _ = shopping.ParseListText("```\nListe: Party\n- [x] 2 Flaschen Wasser\n```")
	wantItem(t, lt.Items[0], recipe.Ingredient{Quantity: numPtr(2), Unit: strPtr("Flaschen"), Name: "Wasser"})
	lt, _ = shopping.ParseListText("2 + 200 g + 1 kg Zwiebeln, rot")
	for i, want := range []recipe.Ingredient{
		{Quantity: numPtr(2), Name: "Zwiebeln", Note: strPtr("rot")},
		{Quantity: numPtr(200), Unit: strPtr("g"), Name: "Zwiebeln", Note: strPtr("rot")},
		{Quantity: numPtr(1), Unit: strPtr("kg"), Name: "Zwiebeln", Note: strPtr("rot")},
	} {
		wantItem(t, lt.Items[i], want)
	}
}

func TestParseListTextErrors(t *testing.T) {
	for _, text := range []string{"", "  \n\n", "Liste: X", "Obst:"} {
		if _, err := shopping.ParseListText(text); !errors.Is(err, shopping.ErrNoItems) {
			t.Errorf("%q: %v, want ErrNoItems", text, err)
		}
	}
	if _, err := shopping.ParseListText(strings.Repeat("1 Ei\n", 200)); err != nil {
		t.Errorf("200 items: %v", err)
	}
	if _, err := shopping.ParseListText(strings.Repeat("1 Ei\n", 201)); !errors.Is(err, shopping.ErrTooManyItems) {
		t.Errorf("201 items: %v, want ErrTooManyItems", err)
	}
	long := []struct {
		name, text string
		line       int
	}{
		{"one word", "Liste: A\n1 Ei\n2 " + strings.Repeat("x", 121), 3},
		{"many words", "1 Ei\n" + strings.Repeat("Wort ", 30) + "Ende, glatt", 2},
		{"note", "1 Ei\n2 l Milch, " + strings.Repeat("n", 201), 2},
		{"quantity only, long line", "1 Ei\n2, " + strings.Repeat("x", 200), 2},
		{"after a fence", "Hier:\n```\n1 Ei\n" + strings.Repeat("x", 121) + "\n```", 4},
		// 400 digits parse to +Inf, which no JSON answer can carry: no amount, too long a name.
		{"absurd amount", "1 Ei\n" + strings.Repeat("9", 400) + " g Mehl", 2},
	}
	for _, r := range long {
		var le *shopping.LineError
		if _, err := shopping.ParseListText(r.text); !errors.As(err, &le) || le.Line != r.line {
			t.Errorf("%s: %v, want line %d", r.name, err, r.line)
		}
	}
	for _, text := range []string{
		"2 " + strings.Repeat("x", 120) + ", " + strings.Repeat("n", 200),
		"1 l Milch (" + strings.Repeat("a", 60) + "), " + strings.Repeat("b", 60), // a note joined from two parts
		"1-2 l Milch, " + strings.Repeat("n", 150),                                // a range plus a long note
		strings.Repeat("Wort ", 14) + "Zwiebel(n)",                                // a long name with a plural marker
	} {
		if _, err := shopping.ParseListText(text); err != nil {
			t.Errorf("%q fits: %v", text, err)
		}
	}
}

func TestParseFreeKeepsListShapes(t *testing.T) {
	cases := []struct {
		in   string
		want recipe.Ingredient
	}{
		{"1 l Milch 1,5 %", recipe.Ingredient{Quantity: numPtr(1), Unit: strPtr("l"), Name: "Milch 1,5 %"}},
		{"Eier (6 Stück)", recipe.Ingredient{Name: "Eier", Note: strPtr("6 Stück")}},
		{"Milch - fettarm", recipe.Ingredient{Name: "Milch", Note: strPtr("fettarm")}},
		{"½ Bund Dill", recipe.Ingredient{Quantity: numPtr(0.5), Unit: strPtr("Bund"), Name: "Dill"}},
		{"2", recipe.Ingredient{Name: "2"}},
		{"0 g Salz", recipe.Ingredient{Name: "0 g Salz"}}, // zero is no amount
		{"100001 g Mehl", recipe.Ingredient{Name: "100001 g Mehl"}},
	}
	for _, c := range cases {
		wantItem(t, shopping.ParseFree(c.in), c.want)
	}
}

func TestListTextRoundTrip(t *testing.T) {
	// The text of LIST_TEXT_CASES in frontend/src/lib/shopping/rows.test.ts,
	// with the items an import of it makes.
	rows := []struct {
		text  string
		items []recipe.Ingredient
	}{
		{"2 Zwiebeln", []recipe.Ingredient{{Quantity: numPtr(2), Name: "Zwiebeln"}}},
		{"1 Bund Petersilie, glatt", []recipe.Ingredient{{Quantity: numPtr(1), Unit: strPtr("Bund"), Name: "Petersilie", Note: strPtr("glatt")}}},
		{"2 l Milch, 1,5 %, fettarm", []recipe.Ingredient{{Quantity: numPtr(2), Unit: strPtr("l"), Name: "Milch", Note: strPtr("1,5 %, fettarm")}}},
		{"2 Stück + 200 g Zwiebeln", []recipe.Ingredient{
			{Quantity: numPtr(2), Unit: strPtr("Stück"), Name: "Zwiebeln"},
			{Quantity: numPtr(200), Unit: strPtr("g"), Name: "Zwiebeln"},
		}},
		// rows.test.ts's English listText test.
		{"5 pc + 200 g Onion", []recipe.Ingredient{
			{Quantity: numPtr(5), Unit: strPtr("pc"), Name: "Onion"},
			{Quantity: numPtr(200), Unit: strPtr("g"), Name: "Onion"},
		}},
	}
	for _, r := range rows {
		lt, err := shopping.ParseListText(r.text)
		if err != nil || len(lt.Items) != len(r.items) {
			t.Fatalf("%q: %v %+v", r.text, err, lt.Items)
		}
		for i := range r.items {
			wantItem(t, lt.Items[i], r.items[i])
		}
	}
}

func TestParseListTextAIAnswers(t *testing.T) {
	// Real answers to the list instruction; only drugstore lines go without an amount.
	without := map[string]int{"sonnet-a-1.txt": 0, "sonnet-a-2.txt": 0, "sonnet-b-1.txt": 9, "sonnet-b-2.txt": 0}
	items := map[string]int{"sonnet-a-1.txt": 30, "sonnet-a-2.txt": 5, "sonnet-b-1.txt": 44, "sonnet-b-2.txt": 7}
	for file, want := range without {
		raw, err := os.ReadFile(filepath.Join("testdata", "ai", file))
		if err != nil {
			t.Fatal(err)
		}
		lt, err := shopping.ParseListText(string(raw))
		if err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		n := 0
		for _, it := range lt.Items {
			if it.Quantity == nil {
				n++
			}
		}
		if lt.Name == "" || len(lt.Items) != items[file] || n != want {
			t.Errorf("%s: name %q, %d items, %d without amount; want %d, %d", file, lt.Name, len(lt.Items), n, items[file], want)
		}
	}
}

func TestImportListNames(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	rows := []struct{ text, name, want string }{
		{"Liste: Party\n1 Ei", "  Getippt  ", "Getippt"},
		{"Liste: Party\n1 Ei", "", "Party"},
		{"1 Ei", " ", "Einkaufsliste"},
		{"Liste: " + strings.Repeat("Wort ", 20) + "\n1 Ei", "", strings.TrimSpace(strings.Repeat("Wort ", 16))},
	}
	for _, r := range rows {
		l, err := e.svc.ImportList(ctx, e.a.ID, r.text, r.name, "Einkaufsliste")
		if err != nil {
			t.Fatal(err)
		}
		if l.Name != r.want || len(l.Items) != 1 || l.Items[0].Name != "Ei" || l.Items[0].AddedBy == nil {
			t.Errorf("%q: %+v", r.text, l)
		}
	}
	if _, err := e.svc.ImportList(ctx, e.a.ID, "Liste: Leer", "", "Einkaufsliste"); !errors.Is(err, shopping.ErrNoItems) {
		t.Fatalf("no items: %v", err)
	}
	lists, _ := e.svc.Lists(ctx, e.a.ID)
	if len(lists) != len(rows) {
		t.Fatalf("a refused import made a list: %d lists", len(lists))
	}
}

func TestImportItemsAppends(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	l := e.list(t)
	if _, err := e.svc.AddFree(ctx, e.a.ID, l.ID, "Brot"); err != nil {
		t.Fatal(err)
	}
	ch, cancel := e.svc.Hub().Subscribe(l.ID)
	defer cancel()
	l, err := e.svc.ImportItems(ctx, e.a.ID, l.ID, "Liste: Anders\n2 l Milch\n2 + 200 g Zwiebeln")
	if err != nil {
		t.Fatal(err)
	}
	if !signaled(ch) {
		t.Error("import did not publish")
	}
	names := []string{}
	for _, it := range l.Items {
		names = append(names, it.Name)
	}
	if l.Name != "Woche" || strings.Join(names, ",") != "Brot,Milch,Zwiebeln,Zwiebeln" {
		t.Fatalf("name %q, items %v", l.Name, names)
	}
	if _, err := e.svc.ImportItems(ctx, e.b.ID, l.ID, "1 Ei"); !errors.Is(err, shopping.ErrNotFound) {
		t.Fatalf("stranger: %v", err)
	}
}

func wantItem(t *testing.T, got, want recipe.Ingredient) {
	t.Helper()
	if !sameNum(got.Quantity, want.Quantity) || !sameStr(got.Unit, want.Unit) || got.Name != want.Name || !sameStr(got.Note, want.Note) {
		t.Errorf("got %v %v %q %v, want %v %v %q %v", show(got.Quantity), show(got.Unit), got.Name, show(got.Note), show(want.Quantity), show(want.Unit), want.Name, show(want.Note))
	}
}

func show[T any](p *T) any {
	if p == nil {
		return nil
	}
	return *p
}

// The list instruction, pasted back whole, gives its example list: the name
// from the name line and every item exactly. A new app language adds its row.
func TestEveryAppLanguageListInstructionImports(t *testing.T) {
	const howto = "see docs/memory/content/howtos/add-an-import-language.mdx"
	want := map[string]struct {
		name  string
		items []recipe.Ingredient
	}{
		"de": {"Wocheneinkauf", []recipe.Ingredient{
			{Quantity: numPtr(2), Unit: strPtr("l"), Name: "Milch", Note: strPtr("1,5 %")},
			{Quantity: numPtr(500), Unit: strPtr("g"), Name: "Mehl"},
			{Quantity: numPtr(6), Name: "Eier"},
			{Quantity: numPtr(1), Unit: strPtr("Bund"), Name: "Petersilie", Note: strPtr("glatt")},
			{Quantity: numPtr(1), Unit: strPtr("Glas"), Name: "Kapern"},
			{Name: "Zahnpasta"},
		}},
		"en": {"Groceries", []recipe.Ingredient{
			{Quantity: numPtr(2), Unit: strPtr("l"), Name: "milk", Note: strPtr("1.5 %")},
			{Quantity: numPtr(500), Unit: strPtr("g"), Name: "flour"},
			{Quantity: numPtr(6), Name: "eggs"},
			{Quantity: numPtr(1), Unit: strPtr("bunch"), Name: "parsley", Note: strPtr("flat-leaf")},
			{Quantity: numPtr(1), Unit: strPtr("jar"), Name: "capers"},
			{Name: "toothpaste"},
		}},
	}
	for _, l := range i18n.Locales() {
		body, err := os.ReadFile("../../../frontend/messages/" + l + ".json")
		if err != nil {
			t.Fatal(err)
		}
		var catalog map[string]any
		if err := json.Unmarshal(body, &catalog); err != nil {
			t.Fatalf("frontend/messages/%s.json: %v", l, err)
		}
		instruction, _ := catalog["ai_instruction_list"].(string)
		parts := strings.SplitN(instruction, "```", 3)
		w, ok := want[l]
		if len(parts) < 3 || !ok {
			t.Errorf("%s: ai_instruction_list is missing, has no fenced example, or this test has no row for it; %s", l, howto)
			continue
		}
		word, _, _ := strings.Cut(strings.TrimSpace(parts[1]), ":")
		if !slices.Contains(shopping.ListNameWords, word) || !strings.Contains(parts[0], word+":") {
			t.Errorf("%s: ai_instruction_list names the list with %q, which is not in listNameWords or not in its prose; %s", l, word, howto)
		}
		lt, err := shopping.ParseListText(instruction)
		if err != nil || lt.Name != w.name || len(lt.Items) != len(w.items) {
			t.Errorf("%s: ai_instruction_list reads as %q, %+v, %v; want %q and %d items", l, lt.Name, lt.Items, err, w.name, len(w.items))
			continue
		}
		for i := range w.items {
			wantItem(t, lt.Items[i], w.items[i])
		}
		// Every unit the instruction allows reads as a unit.
		i := strings.Index(parts[0], "g, kg")
		if i < 0 {
			t.Errorf("%s: ai_instruction_list lists no units; %s", l, howto)
			continue
		}
		units, _, _ := strings.Cut(parts[0][i:], ".")
		for _, unit := range strings.Split(units, ", ") {
			if it, err := shopping.ParseListText("1 " + unit + " Test"); err != nil || it.Items[0].Unit == nil || *it.Items[0].Unit != unit {
				t.Errorf("%s: ai_instruction_list allows the unit %q, which the list does not read as a unit; %s", l, unit, howto)
			}
		}
	}
}
