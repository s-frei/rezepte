package db

import (
	"sync"

	"golang.org/x/text/collate"
	"golang.org/x/text/language"
	"modernc.org/sqlite"
)

// collateUnicode names a collation, registered with the driver for every
// connection this package opens, that orders text alphabetically across
// scripts and diacritics: "Äpfel" sorts beside "Apfel" and case only breaks
// ties. SQLite's own BINARY and NOCASE compare bytes and fold ASCII alone,
// which puts every word with a non-ASCII initial after "z".
const collateUnicode = "unicode"

func init() {
	// A Collator is not safe for concurrent use, and SQLite calls a
	// collation from whichever connection is sorting.
	var mu sync.Mutex
	c := collate.New(language.Und)
	sqlite.MustRegisterCollationUtf8(collateUnicode, func(a, b string) int {
		mu.Lock()
		defer mu.Unlock()
		return c.CompareString(a, b)
	})
}
