package pdf

import (
	"fmt"
	"os"
	"path/filepath"
)

// SweepTemp removes every pdf-* work dir in dataDir. A request removes its
// own dir on every path out; this is the safety net for a process that was
// killed mid-render. It runs once at startup, before any request.
func SweepTemp(dataDir string) error {
	matches, err := filepath.Glob(filepath.Join(dataDir, "pdf-*"))
	if err != nil {
		return fmt.Errorf("sweep pdf-*: %w", err)
	}
	for _, m := range matches {
		if err := os.RemoveAll(m); err != nil {
			return fmt.Errorf("sweep %s: %w", m, err)
		}
	}
	return nil
}
