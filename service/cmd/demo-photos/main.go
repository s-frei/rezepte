// Command demo-photos derives the photos demo mode embeds from the
// AI-generated originals kept in Git LFS: every <locale>/<slug>/<n>.jpg under
// -src is scaled down to at most maxSide px and written as JPEG under -dst,
// and any photo set -dst holds that -src no longer has is removed. Only
// .jpg files are read, so the prompts.md beside each set stays behind.
//
// The output depends on nothing but the originals, so rerunning it leaves
// the embedded files byte for byte unchanged. Run it through
// `mise run demo-photos`, which fetches the originals first.
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/jpeg"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"golang.org/x/image/draw"
)

const (
	// maxSide keeps the binary small while staying above the 960 px thumbnail
	// and matching the 1600 px detail variant the gallery shows.
	maxSide = 1600
	quality = 82
)

// errPointer means an original is still a Git LFS pointer file.
var errPointer = errors.New("git lfs pointer, not a photo; run `mise run demo-photos:pull`")

func main() {
	src := flag.String("src", "", "directory of the originals (<locale>/<slug>/<n>.jpg)")
	dst := flag.String("dst", "", "directory the embedded photos are written to")
	flag.Parse()
	if *src == "" || *dst == "" {
		flag.Usage()
		os.Exit(2)
	}
	if err := run(*src, *dst); err != nil {
		fmt.Fprintln(os.Stderr, "demo-photos:", err)
		os.Exit(1)
	}
}

// run rewrites dst from src: the locale directories in dst are removed, then
// every original is shrunk into the same relative path.
func run(src, dst string) error {
	if err := removeSets(dst); err != nil {
		return err
	}
	originals := os.DirFS(src)
	return fs.WalkDir(originals, ".", func(rel string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || path.Ext(rel) != ".jpg" {
			return nil
		}
		data, err := fs.ReadFile(originals, rel)
		if err != nil {
			return fmt.Errorf("read %s: %w", rel, err)
		}
		out, err := shrink(data)
		if err != nil {
			return fmt.Errorf("%s: %w", rel, err)
		}
		target := filepath.Join(dst, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			return fmt.Errorf("create %s: %w", filepath.Dir(target), err)
		}
		if err := os.WriteFile(target, out, 0o600); err != nil {
			return fmt.Errorf("write %s: %w", target, err)
		}
		return nil
	})
}

// removeSets removes every directory directly below dst, so photo sets that left
// the originals leave the embedded set too. Loose files in dst stay.
func removeSets(dst string) error {
	entries, err := os.ReadDir(dst)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read %s: %w", dst, err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if err := os.RemoveAll(filepath.Join(dst, e.Name())); err != nil {
			return fmt.Errorf("remove %s: %w", e.Name(), err)
		}
	}
	return nil
}

// shrink decodes a JPEG, scales it so its longest side is at most maxSide
// (never up) with Catmull-Rom, and re-encodes it without any metadata.
func shrink(data []byte) ([]byte, error) {
	if strings.HasPrefix(string(data), "version https://git-lfs") {
		return nil, errPointer
	}
	src, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w > maxSide || h > maxSide {
		if w >= h {
			w, h = maxSide, int(float64(h)*maxSide/float64(w)+0.5)
		} else {
			w, h = int(float64(w)*maxSide/float64(h)+0.5), maxSide
		}
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Src, nil)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: quality}); err != nil {
		return nil, fmt.Errorf("encode: %w", err)
	}
	return buf.Bytes(), nil
}
