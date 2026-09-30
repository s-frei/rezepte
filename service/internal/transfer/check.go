package transfer

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	stdimage "image"
	_ "image/jpeg" // registers the decoder DecodeConfig uses
	"io"
	"path"
	"slices"
	"strings"

	"github.com/s-frei/rezepte/service/internal/recipe"
)

// folder is one recipe of an import that passed every check.
type folder struct {
	name   string
	file   File
	photos map[string]*zip.File
}

const unexpectedEntry = "unexpected entry; a folder per recipe with recipe.json and .jpg photos is expected"

// check validates the whole zip without writing anything. Its errors are
// *Error, located by the path inside the zip.
func (s *Service) check(zr *zip.Reader) ([]folder, error) {
	jsons := map[string]*zip.File{}
	photos := map[string]map[string]*zip.File{}
	var order []string
	for _, zf := range zr.File {
		name := zf.Name
		// macOS Finder adds __MACOSX/ and dot files when it zips a folder.
		if strings.HasPrefix(name, "__MACOSX/") || strings.HasPrefix(path.Base(name), ".") || strings.HasSuffix(name, "/") {
			continue
		}
		dir, base, ok := strings.Cut(name, "/")
		if !ok || dir == "" || dir == "." || dir == ".." || strings.Contains(base, "/") || path.Clean(name) != name {
			return nil, &Error{Location: name, Msg: unexpectedEntry}
		}
		switch {
		case base == "recipe.json":
			jsons[dir] = zf
		case strings.HasSuffix(base, ".jpg"):
			if photos[dir] == nil {
				photos[dir] = map[string]*zip.File{}
			}
			photos[dir][base] = zf
		default:
			return nil, &Error{Location: name, Msg: unexpectedEntry}
		}
		if !slices.Contains(order, dir) {
			order = append(order, dir)
		}
	}
	if len(order) == 0 {
		return nil, &Error{Msg: "no recipes in this file"}
	}
	out := make([]folder, 0, len(order))
	for _, dir := range order {
		jf, ok := jsons[dir]
		if !ok {
			return nil, &Error{Location: dir + "/recipe.json", Msg: "missing"}
		}
		file, err := s.readFile(dir, jf)
		if err != nil {
			return nil, err
		}
		if err := checkPhotos(dir, file, photos[dir]); err != nil {
			return nil, err
		}
		out = append(out, folder{name: dir, file: file, photos: photos[dir]})
	}
	return out, nil
}

func (s *Service) readFile(dir string, zf *zip.File) (File, error) {
	loc := dir + "/recipe.json"
	raw, err := readLimited(zf, maxJSONBytes)
	if err != nil {
		return File{}, &Error{Location: loc, Msg: err.Error()}
	}
	var head struct {
		Format  string `json:"format"`
		Version any    `json:"version"`
	}
	if err := json.Unmarshal(raw, &head); err != nil {
		return File{}, &Error{Location: loc, Msg: "not valid JSON"}
	}
	if head.Format != Format {
		return File{}, &Error{Location: loc, Msg: fmt.Sprintf("unknown format %q", head.Format)}
	}
	// A missing or non-number version decodes as nil or a non-float64.
	v, ok := head.Version.(float64)
	if !ok || v < 1 {
		return File{}, &Error{Location: loc, Msg: fmt.Sprintf("unsupported version %v", head.Version)}
	}
	if v > Version {
		return File{}, &Error{Location: loc, Msg: fmt.Sprintf("newer version %v than this Rezepte reads (%d); update Rezepte", v, Version)}
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return File{}, &Error{Location: loc, Msg: "not valid JSON"}
	}
	for _, k := range []string{"format", "version", "images", "cover", "editPolicy"} {
		delete(m, k)
	}
	if errs := s.validate(m); len(errs) > 0 {
		return File{}, &Error{Location: loc + ": " + errs[0].Location, Msg: errs[0].Message}
	}
	var file File
	if err := json.Unmarshal(raw, &file); err != nil {
		return File{}, &Error{Location: loc, Msg: "not valid JSON"}
	}
	file.EditPolicy = "" // the target's default applies
	if err := recipe.ResolveRefs(file.IngredientGroups, file.Steps); err != nil {
		var re *recipe.RefError
		if errors.As(err, &re) {
			return File{}, &Error{Location: loc + ": " + re.Location(), Msg: re.Msg}
		}
		return File{}, &Error{Location: loc, Msg: err.Error()}
	}
	return file, nil
}

func checkPhotos(dir string, file File, photos map[string]*zip.File) error {
	loc := dir + "/recipe.json"
	if len(file.Images) > maxPhotos {
		return &Error{Location: loc + ": images", Msg: fmt.Sprintf("at most %d photos", maxPhotos)}
	}
	if file.Cover != nil && !slices.Contains(file.Images, *file.Cover) {
		return &Error{Location: loc + ": cover", Msg: "not one of images"}
	}
	for _, name := range file.Images {
		zf, ok := photos[name]
		if !ok {
			return &Error{Location: dir + "/" + name, Msg: "missing"}
		}
		raw, err := readLimited(zf, maxPhotoBytes)
		if err != nil {
			return &Error{Location: dir + "/" + name, Msg: err.Error()}
		}
		if _, format, err := stdimage.DecodeConfig(bytes.NewReader(raw)); err != nil || format != "jpeg" {
			return &Error{Location: dir + "/" + name, Msg: "not a JPEG"}
		}
	}
	return nil
}

// readLimited reads an entry, trusting the bytes it inflates rather than
// the size its header claims, so a zip bomb stops at limit+1 bytes.
func readLimited(zf *zip.File, limit int64) ([]byte, error) {
	rc, err := zf.Open()
	if err != nil {
		return nil, errors.New("cannot be read")
	}
	defer func() { _ = rc.Close() }()
	raw, err := io.ReadAll(io.LimitReader(rc, limit+1))
	if err != nil {
		return nil, errors.New("cannot be read")
	}
	if int64(len(raw)) > limit {
		return nil, fmt.Errorf("larger than %d bytes", limit)
	}
	return raw, nil
}
