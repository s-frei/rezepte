// Package update checks GitHub for a newer Rezepte release.
package update

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type version struct {
	major, minor, patch int
	pre                 string
}

var describeSuffix = regexp.MustCompile(`-\d+-g[0-9a-f]+$`)

// parse reads X.Y.Z with an optional -pre part; a git describe suffix is dropped first.
func parse(v string) (version, error) {
	s := describeSuffix.ReplaceAllString(v, "")
	core, pre, _ := strings.Cut(s, "-")
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return version{}, fmt.Errorf("parse version %q: want X.Y.Z", v)
	}
	var n [3]int
	for i, p := range parts {
		x, err := strconv.Atoi(p)
		if err != nil || x < 0 {
			return version{}, fmt.Errorf("parse version %q: %w", v, strconv.ErrSyntax)
		}
		n[i] = x
	}
	return version{n[0], n[1], n[2], pre}, nil
}

// newer reports whether latest is a higher version than current.
func newer(latest, current string) (bool, error) {
	l, err := parse(latest)
	if err != nil {
		return false, err
	}
	c, err := parse(current)
	if err != nil {
		return false, err
	}
	for _, d := range [][2]int{{l.major, c.major}, {l.minor, c.minor}, {l.patch, c.patch}} {
		if d[0] != d[1] {
			return d[0] > d[1], nil
		}
	}
	// Same X.Y.Z: a release beats a pre-release; two pre-releases compare by name.
	switch {
	case l.pre == c.pre:
		return false, nil
	case l.pre == "":
		return true, nil
	case c.pre == "":
		return false, nil
	}
	return l.pre > c.pre, nil
}
