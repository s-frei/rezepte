//go:build unix

// Package typstbin provides the Typst CLI that renders PDFs. A build with the
// typst tag embeds the official release binary for its platform (fetched and
// checksummed by fetch.sh); Path writes it to a private temp directory on
// first use. Without an embedded binary Path falls back to typst on PATH.
// It is unix-only: the private-directory check reads syscall.Stat_t.
package typstbin

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
)

//go:embed VERSION
var versionFile string

// Version is the Typst release this package embeds or expects on PATH.
var Version = strings.TrimSpace(versionFile)

// ErrUnavailable means neither an embedded binary nor typst on PATH exists.
var ErrUnavailable = errors.New("typst unavailable")

const prefix = "rezepte-typst-"

var std = &extractor{bin: binary, tmp: os.TempDir()}

// Embedded reports whether this build carries the Typst binary.
func Embedded() bool { return len(binary) > 0 }

// Path returns the typst executable, extracting the embedded binary once per
// process. The parent directory is private to this process's user (0700).
func Path(ctx context.Context) (string, error) { return std.path(ctx) }

// SweepStale removes rezepte-typst-* directories in tmpDir that are not the
// current version's and belong to this user. It is the safety net for an
// earlier version or an abnormal stop.
func SweepStale(tmpDir string) error {
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		return fmt.Errorf("read %s: %w", tmpDir, err)
	}
	current := dirName(binary)
	var errs []error
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, prefix) || name == current {
			continue
		}
		dir := filepath.Join(tmpDir, name)
		fi, err := os.Lstat(dir)
		if err != nil || !ownedByUs(fi) {
			continue
		}
		if err := os.RemoveAll(dir); err != nil {
			errs = append(errs, fmt.Errorf("remove %s: %w", dir, err))
		}
	}
	return errors.Join(errs...)
}

// dirName is the directory a binary extracts to: version plus 8 hex of its
// SHA-256, so two builds of one version never share a file.
func dirName(bin []byte) string {
	sum := sha256.Sum256(bin)
	return fmt.Sprintf("%s%s-%x", prefix, Version, sum[:4])
}

type extractor struct {
	bin []byte
	tmp string

	mu        sync.Mutex
	extracted string // path of the extracted binary once written
}

func (e *extractor) path(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if len(e.bin) == 0 {
		p, err := exec.LookPath("typst")
		if err != nil {
			return "", ErrUnavailable
		}
		return p, nil
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	// A temp cleaner or another instance's sweep can remove the file while
	// the process runs; size and mode are the cheap check, a miss rewrites.
	if e.extracted != "" {
		if fi, err := os.Lstat(e.extracted); err == nil && fi.Mode().IsRegular() &&
			fi.Mode().Perm() == 0o500 && fi.Size() == int64(len(e.bin)) {
			return e.extracted, nil
		}
	}
	p, err := e.write()
	if err != nil {
		return "", fmt.Errorf("%w: extract: %w", ErrUnavailable, err)
	}
	e.extracted = p
	return p, nil
}

func (e *extractor) write() (string, error) {
	dir, err := e.privateDir()
	if err != nil {
		return "", err
	}
	f, err := os.CreateTemp(dir, "typst-*")
	if err != nil {
		return "", fmt.Errorf("create: %w", err)
	}
	defer os.Remove(f.Name()) // a no-op after the rename
	if _, err := f.Write(e.bin); err != nil {
		_ = f.Close()
		return "", fmt.Errorf("write: %w", err)
	}
	if err := f.Chmod(0o500); err != nil {
		_ = f.Close()
		return "", fmt.Errorf("chmod: %w", err)
	}
	if err := f.Close(); err != nil {
		return "", fmt.Errorf("close: %w", err)
	}
	target := filepath.Join(dir, "typst")
	if err := os.Rename(f.Name(), target); err != nil {
		return "", fmt.Errorf("rename: %w", err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		return "", fmt.Errorf("verify: %w", err)
	}
	if sha256.Sum256(got) != sha256.Sum256(e.bin) {
		_ = os.Remove(target)
		return "", errors.New("verify: written binary differs from the embedded one")
	}
	return target, nil
}

// privateDir returns the directory to write into: the one of an earlier
// extraction while it is still private, else the version's directory, created
// if absent. A directory that is a symlink, belongs to another user or is not
// 0700 could let someone swap the binary, so a fresh MkdirTemp directory takes
// its place.
func (e *extractor) privateDir() (string, error) {
	if e.extracted != "" {
		if dir := filepath.Dir(e.extracted); isPrivate(dir) {
			return dir, nil
		}
	}
	dir := filepath.Join(e.tmp, dirName(e.bin))
	if err := os.Mkdir(dir, 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
		return "", fmt.Errorf("mkdir: %w", err)
	}
	if isPrivate(dir) {
		return dir, nil
	}
	dir, err := os.MkdirTemp(e.tmp, prefix+"*")
	if err != nil {
		return "", fmt.Errorf("mkdir temp: %w", err)
	}
	return dir, nil
}

func isPrivate(dir string) bool {
	fi, err := os.Lstat(dir)
	return err == nil && fi.IsDir() && fi.Mode().Perm() == 0o700 && ownedByUs(fi)
}

func ownedByUs(fi fs.FileInfo) bool {
	st, ok := fi.Sys().(*syscall.Stat_t)
	return ok && int(st.Uid) == os.Getuid()
}
