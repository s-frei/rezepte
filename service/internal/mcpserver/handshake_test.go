package mcpserver

import (
	"encoding/json"
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/s-frei/rezepte/service/internal/auth"
)

// rpc posts one JSON-RPC message to the MCP endpoint the way a desktop
// client does and returns the status and body. version, when set, goes out
// as the MCP-Protocol-Version header every request after initialize carries.
func rpc(t *testing.T, url, token, version, body string) (int, []byte) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if version != "" {
		req.Header.Set("MCP-Protocol-Version", version)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	out, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	return res.StatusCode, out
}

// TestOlderProtocolHandshake speaks the handshake the SDK's own client no
// longer uses: it negotiates the newest revision, which skips initialize,
// while Claude Code, Cursor and VS Code still send initialize with a 2025
// revision, then notifications/initialized, then every request with the
// MCP-Protocol-Version header. Raw HTTP, so the test pins the wire format
// those clients see rather than what an SDK client tolerates.
func TestOlderProtocolHandshake(t *testing.T) {
	for _, version := range []string{"2025-06-18", "2025-11-25"} {
		t.Run(version, func(t *testing.T) {
			e := newEnv(t)
			e.seed(t, "Leek soup", "soup")
			token, _, err := e.tokens.Create(t.Context(), e.owner.ID, "desktop", []string{auth.ScopeRecipesRead}, nil)
			if err != nil {
				t.Fatal(err)
			}

			status, body := rpc(t, e.url, token, "", `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"`+version+`","capabilities":{},"clientInfo":{"name":"desktop","version":"1"}}}`)
			var initRes struct {
				Result struct {
					ProtocolVersion string                `json:"protocolVersion"`
					ServerInfo      struct{ Name string } `json:"serverInfo"`
				} `json:"result"`
			}
			if err := json.Unmarshal(body, &initRes); status != http.StatusOK || err != nil {
				t.Fatalf("initialize: status %d, err %v, body %s", status, err, body)
			}
			if initRes.Result.ProtocolVersion != version || initRes.Result.ServerInfo.Name != "rezepte" {
				t.Fatalf("initialize result = %+v, want version %s from rezepte", initRes.Result, version)
			}

			status, body = rpc(t, e.url, token, version, `{"jsonrpc":"2.0","method":"notifications/initialized"}`)
			if status != http.StatusAccepted || len(body) != 0 {
				t.Fatalf("notifications/initialized: status %d, body %q, want 202 and no body", status, body)
			}

			status, body = rpc(t, e.url, token, version, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
			var list struct {
				Result struct {
					Tools []struct{ Name string } `json:"tools"`
				} `json:"result"`
			}
			if err := json.Unmarshal(body, &list); status != http.StatusOK || err != nil {
				t.Fatalf("tools/list: status %d, err %v, body %s", status, err, body)
			}
			names := make([]string, 0, len(list.Result.Tools))
			for _, tl := range list.Result.Tools {
				names = append(names, tl.Name)
			}
			slices.Sort(names)
			if want := []string{"get_recipe", "list_tags", "search_recipes"}; !slices.Equal(names, want) {
				t.Fatalf("tools = %v, want %v", names, want)
			}

			status, body = rpc(t, e.url, token, version, `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"list_tags","arguments":{}}}`)
			var callRes struct {
				Result struct {
					IsError           bool `json:"isError"`
					StructuredContent struct {
						Tags []struct{ Name string } `json:"tags"`
					} `json:"structuredContent"`
				} `json:"result"`
			}
			if err := json.Unmarshal(body, &callRes); status != http.StatusOK || err != nil {
				t.Fatalf("tools/call: status %d, err %v, body %s", status, err, body)
			}
			if callRes.Result.IsError || len(callRes.Result.StructuredContent.Tags) != 1 || callRes.Result.StructuredContent.Tags[0].Name != "soup" {
				t.Fatalf("list_tags: %s", body)
			}
		})
	}
}
