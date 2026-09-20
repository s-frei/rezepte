package httpserver

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// appCSSPath is the app's Tailwind theme, the single source of the colours
// scalar_theme.css repeats. It lives outside this Go module, so a checkout
// without the frontend skips the test below rather than failing it.
const appCSSPath = "../../../frontend/src/app.css"

var (
	rxToken = regexp.MustCompile(`--color-[a-z0-9-]+:\s*(#[0-9a-fA-F]{3,8})\s*;`)
	rxHex   = regexp.MustCompile(`#[0-9a-fA-F]{3,8}`)
)

// TestScalarThemeUsesOnlyAppTokens keeps the copied palette honest: every
// colour literal in scalar_theme.css must be one of the --color-* values in
// the app's stylesheet. Recolour the app and this fails until the docs page
// follows.
func TestScalarThemeUsesOnlyAppTokens(t *testing.T) {
	appCSS, err := os.ReadFile(appCSSPath)
	if os.IsNotExist(err) {
		t.Skipf("%s not present - frontend not checked out", appCSSPath)
	}
	if err != nil {
		t.Fatal(err)
	}

	tokens := map[string]bool{}
	for _, m := range rxToken.FindAllStringSubmatch(string(appCSS), -1) {
		tokens[strings.ToLower(m[1])] = true
	}
	if len(tokens) == 0 {
		t.Fatalf("no --color-* tokens found in %s", appCSSPath)
	}

	for _, hex := range rxHex.FindAllString(scalarTheme, -1) {
		if !tokens[strings.ToLower(hex)] {
			t.Errorf("scalar_theme.css uses %s, which is not a --color-* token in %s", hex, appCSSPath)
		}
	}
}
