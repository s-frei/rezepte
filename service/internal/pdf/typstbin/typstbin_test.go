//go:build unix

package typstbin

import (
	"bufio"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

var fakeBin = []byte("#!/bin/sh\necho typst\n")

func TestExtractOnce(t *testing.T) {
	e := &extractor{bin: fakeBin, tmp: t.TempDir()}
	paths := make([]string, 20)
	var wg sync.WaitGroup
	for i := range paths {
		wg.Go(func() {
			p, err := e.path(t.Context())
			if err != nil {
				t.Error(err)
			}
			paths[i] = p
		})
	}
	wg.Wait()
	for _, p := range paths {
		if p != paths[0] {
			t.Fatalf("paths differ: %q and %q", p, paths[0])
		}
	}
	if want := filepath.Join(e.tmp, dirName(fakeBin), "typst"); paths[0] != want {
		t.Errorf("path = %q, want %q", paths[0], want)
	}
	entries, err := os.ReadDir(filepath.Dir(paths[0]))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("dir holds %d entries, want only typst", len(entries))
	}
	fi, err := os.Stat(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o500 {
		t.Errorf("mode = %v, want 0500", fi.Mode().Perm())
	}
}

func TestExtractReplacesRemovedBinary(t *testing.T) {
	e := &extractor{bin: fakeBin, tmp: t.TempDir()}
	p, err := e.path(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Dir(p)); err != nil {
		t.Fatal(err)
	}
	p, err = e.path(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Errorf("binary not restored: %v", err)
	}
}

func TestFallbackDirIsReused(t *testing.T) {
	tmp := t.TempDir()
	foreign := filepath.Join(tmp, dirName(fakeBin))
	if err := os.Mkdir(foreign, 0o700); err != nil {
		t.Fatal(err)
	}
	setMode(t, foreign, 0o777)
	e := &extractor{bin: fakeBin, tmp: tmp}
	first, err := e.path(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(first); err != nil {
		t.Fatal(err)
	}
	second, err := e.path(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if second != first {
		t.Errorf("rewrite went to %q, want the fallback dir again (%q)", second, first)
	}
}

func TestExtractRejectsSymlink(t *testing.T) {
	tmp := t.TempDir()
	real := filepath.Join(t.TempDir(), "elsewhere")
	if err := os.Mkdir(real, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(tmp, dirName(fakeBin))
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	e := &extractor{bin: fakeBin, tmp: tmp}
	p, err := e.path(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if dir := filepath.Dir(p); dir == link || strings.HasPrefix(p, real) {
		t.Errorf("extracted through the symlink: %q", p)
	}
	if entries, _ := os.ReadDir(real); len(entries) != 0 {
		t.Error("the symlink target received a file")
	}
}

func setMode(t *testing.T, dir string, mode os.FileMode) {
	t.Helper()
	if err := os.Chmod(dir, mode); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
}

func TestExtractRejectsForeignDir(t *testing.T) {
	tmp := t.TempDir()
	foreign := filepath.Join(tmp, dirName(fakeBin))
	if err := os.Mkdir(foreign, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(foreign, 0o777); err != nil {
		t.Fatal(err)
	}
	e := &extractor{bin: fakeBin, tmp: tmp}
	p, err := e.path(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Dir(p)
	if dir == foreign {
		t.Fatal("extracted into the world-writable dir")
	}
	if filepath.Dir(dir) != tmp || !strings.HasPrefix(filepath.Base(dir), "rezepte-typst-") {
		t.Errorf("fallback dir = %q, want a rezepte-typst-* dir in %q", dir, tmp)
	}
	fi, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o700 {
		t.Errorf("fallback dir mode = %v, want 0700", fi.Mode().Perm())
	}
}

func TestSweepStaleKeepsCurrent(t *testing.T) {
	tmp := t.TempDir()
	keep := []string{dirName(binary), "other"}
	gone := []string{"rezepte-typst-0.0.1-deadbeef", "rezepte-typst-123456"}
	for _, name := range append(keep, gone...) {
		if err := os.Mkdir(filepath.Join(tmp, name), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := SweepStale(tmp); err != nil {
		t.Fatal(err)
	}
	for _, name := range keep {
		if _, err := os.Stat(filepath.Join(tmp, name)); err != nil {
			t.Errorf("%s removed: %v", name, err)
		}
	}
	for _, name := range gone {
		if _, err := os.Stat(filepath.Join(tmp, name)); !os.IsNotExist(err) {
			t.Errorf("%s still there", name)
		}
	}
}

func TestFallbackToPath(t *testing.T) {
	want, err := exec.LookPath("typst")
	if err != nil {
		t.Fatalf("typst not on PATH; mise provides it: %v", err)
	}
	e := &extractor{tmp: t.TempDir()}
	got, err := e.path(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("path = %q, want %q", got, want)
	}
}

func TestUnavailable(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	e := &extractor{tmp: t.TempDir()}
	if _, err := e.path(t.Context()); !errors.Is(err, ErrUnavailable) {
		t.Errorf("err = %v, want ErrUnavailable", err)
	}
}

func TestVersionMatchesMise(t *testing.T) {
	f, err := os.Open("../../../../mise.toml")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		if v, ok := strings.CutPrefix(strings.TrimSpace(s.Text()), "typst = "); ok {
			if v = strings.Trim(v, `"`); v != Version {
				t.Errorf("mise.toml pins typst %s, VERSION says %s", v, Version)
			}
			return
		}
	}
	t.Fatal(`mise.toml has no typst = "..." line`)
}

// Runs only in a build with the typst tag:
// go test -tags typst ./internal/pdf/typstbin/... -run TestEmbeddedRuns
func TestEmbeddedRuns(t *testing.T) {
	if !Embedded() {
		t.Skip("built without the typst tag")
	}
	e := &extractor{bin: binary, tmp: t.TempDir()}
	p, err := e.path(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	out, err := exec.CommandContext(t.Context(), p, "--version").Output()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), Version) {
		t.Errorf("typst --version = %q, want %s", out, Version)
	}
}
