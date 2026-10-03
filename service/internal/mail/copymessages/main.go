// Command copymessages copies the mail_* keys of frontend/messages into
// internal/mail/messages, because go:embed cannot reach outside the module.
// `mise run //service:generate` runs it; messages_test.go fails on drift.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/s-frei/rezepte/service/internal/mail"
)

func main() {
	if err := run("../frontend/messages", "internal/mail/messages"); err != nil {
		fmt.Fprintln(os.Stderr, "copymessages:", err)
		os.Exit(1)
	}
}

func run(from, to string) error {
	files, err := filepath.Glob(filepath.Join(from, "*.json"))
	if err != nil {
		return fmt.Errorf("list catalogs: %w", err)
	}
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			return fmt.Errorf("read %s: %w", f, err)
		}
		keys, err := mail.FilterCatalog(raw)
		if err != nil {
			return fmt.Errorf("%s: %w", f, err)
		}
		out, err := json.MarshalIndent(keys, "", "\t")
		if err != nil {
			return fmt.Errorf("encode %s: %w", f, err)
		}
		if err := os.WriteFile(filepath.Join(to, filepath.Base(f)), append(out, '\n'), 0o644); err != nil { //nolint:gosec // a checked-in source file, not a secret
			return fmt.Errorf("write %s: %w", f, err)
		}
	}
	return nil
}
