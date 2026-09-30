package transfer_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/s-frei/rezepte/service/internal/transfer"
)

func TestSweepTempRemovesOnlyLeftoverZips(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"export-1.zip", "import-2.zip", "rezepte.db", "images-backup.zip"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := transfer.SweepTemp(dir); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dir)
	var left []string
	for _, e := range entries {
		left = append(left, e.Name())
	}
	slices.Sort(left)
	if !slices.Equal(left, []string{"images-backup.zip", "rezepte.db"}) {
		t.Fatalf("left = %v", left)
	}
}
