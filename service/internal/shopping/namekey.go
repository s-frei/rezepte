// Package shopping holds the pure rules behind shopping lists: how item names
// merge, which department an item belongs to, and how quantities scale.
package shopping

import "strings"

// irregularPlurals maps plural last words that suffix rules cannot reach.
var irregularPlurals = map[string]string{
	"eier": "ei", "eiern": "ei", "pilze": "pilz", "äpfel": "apfel", "nüsse": "nuss", "würste": "wurst", "ananas": "ananas",
}

// accents folds French and Spanish accents so "Crème fraîche" and "creme
// fraiche" meet. ä, ö, ü and ß stay: Mohl/Möhl differ in German.
var accents = strings.NewReplacer("é", "e", "è", "e", "ê", "e", "ë", "e", "î", "i", "ï", "i", "à", "a", "â", "a", "ô", "o", "ç", "c", "ñ", "n", "á", "a", "í", "i", "ó", "o", "ú", "u", "û", "u", "ù", "u")

// NameKey normalizes an item name so that "Zwiebel", "Zwiebeln" and
// " zwiebeln " merge: lowercase, accent-folded, trimmed, inner spaces collapsed, and the
// plural ending stripped from the last word. Singular and plural of a word
// reduce to the same key, which need not be a real word ("tomate").
//
// ponytail: a handful of German and English suffix rules, not a stemmer;
// irregulars beyond irregularPlurals (mice, Kinder) stay separate keys.
// Upgrade to a per-language lemma table if that bites.
func NameKey(name string) string {
	words := strings.Fields(accents.Replace(strings.ToLower(name)))
	if len(words) == 0 {
		return ""
	}
	last := len(words) - 1
	words[last] = singular(words[last])
	return strings.Join(words, " ")
}

func singular(w string) string {
	if s, ok := irregularPlurals[w]; ok {
		return s
	}
	switch {
	case len(w) < 4:
		return w
	case strings.HasSuffix(w, "ies"):
		return w[:len(w)-3] + "y"
	case strings.HasSuffix(w, "oes"):
		return w[:len(w)-2]
	case hasAnySuffix(w, "sses", "xes", "ches", "shes", "zes"):
		return w[:len(w)-2]
	case hasAnySuffix(w, "en", "eln", "ern"):
		return w[:len(w)-1]
	case hasAnySuffix(w, "ss", "us", "is"):
		return w
	case strings.HasSuffix(w, "s"):
		return w[:len(w)-1]
	}
	return w
}

func hasAnySuffix(w string, suffixes ...string) bool {
	for _, s := range suffixes {
		if strings.HasSuffix(w, s) {
			return true
		}
	}
	return false
}
