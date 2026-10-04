package mcpserver

import (
	"strings"
	"testing"

	"github.com/s-frei/rezepte/service/internal/auth"
)

type diaryEntry struct {
	Author string `json:"author"`
	Body   string `json:"body"`
	New    bool   `json:"new"`
}

func TestKitchenDiaryTools(t *testing.T) {
	e := newEnv(t)
	r := e.seed(t, "Leek soup")
	jana := e.member(t, "jana")
	if _, err := e.svc.AddComment(t.Context(), jana, r.ID, "Kapern zum Schluss"); err != nil {
		t.Fatal(err)
	}
	cs := e.connect(t, auth.ScopeRecipesRead, auth.ScopeRecipesWrite)
	list := func() []diaryEntry {
		t.Helper()
		return structured[struct {
			Comments []diaryEntry `json:"comments"`
		}](t, call(t, cs, "list_comments", map[string]any{"slug": r.Slug})).Comments
	}

	if got := list(); len(got) != 1 || got[0].Author != jana.DisplayName || !got[0].New {
		t.Fatalf("list = %+v, want one new entry by %q", got, jana.DisplayName)
	}
	if err := e.users.Delete(t.Context(), e.owner, jana.ID); err != nil {
		t.Fatal(err)
	}
	if got := list(); len(got) != 1 || got[0].Author != "former member" {
		t.Fatalf("after delete = %+v", got)
	}

	added := structured[diaryEntry](t, call(t, cs, "add_comment", map[string]any{"id": r.ID, "body": "15 Minuten reichen"}))
	if added.Body != "15 Minuten reichen" {
		t.Fatalf("added = %+v", added)
	}
	if got := list(); len(got) != 2 || got[1].Author != e.owner.DisplayName || got[1].New {
		t.Fatalf("after add = %+v", got)
	}

	res := call(t, cs, "add_comment", map[string]any{"id": r.ID, "body": "  "})
	if !res.IsError || !strings.Contains(text(res), "1 to 2000") {
		t.Fatalf("blank body: isError=%v %q", res.IsError, text(res))
	}

	for _, tc := range []struct {
		args map[string]any
		want string
	}{
		{map[string]any{"id": r.ID, "slug": r.Slug}, "pass exactly one of id or slug"},
		{map[string]any{}, "pass exactly one of id or slug"},
		{map[string]any{"id": "nope"}, "recipe not found"},
		{map[string]any{"slug": "nope"}, "recipe not found"},
	} {
		res := call(t, cs, "list_comments", tc.args)
		if !res.IsError || !strings.Contains(text(res), tc.want) {
			t.Fatalf("list_comments %v: isError=%v %q, want %q", tc.args, res.IsError, text(res), tc.want)
		}
	}

	for _, name := range toolNames(t, e.connect(t, auth.ScopeRecipesRead)) {
		if name == "add_comment" {
			t.Fatal("add_comment listed for a read-only token")
		}
	}
}
