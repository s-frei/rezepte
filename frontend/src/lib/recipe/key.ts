/**
 * The normalized key of an ingredient name, so "Zwiebel" and "Zwiebeln"
 * count as one. Mirrors `Key` in service/internal/ingredient/key.go, which
 * owns the rule; the shared vectors in its testdata/covers.json pin both.
 */
export function ingredientKey(name: string): string {
	const words = [...name.toLowerCase()]
		.map((c) => ACCENTS[c] ?? c)
		.join('')
		.split(/\s+/)
		.filter((w) => w !== '');
	if (words.length === 0) {
		return '';
	}
	words[words.length - 1] = singular(words[words.length - 1]);
	return words.join(' ');
}

const IRREGULAR_PLURALS = new Map([
	['eier', 'ei'],
	['eiern', 'ei'],
	['pilze', 'pilz'],
	['äpfel', 'apfel'],
	['nüsse', 'nuss'],
	['würste', 'wurst'],
	['ananas', 'ananas']
]);

// French and Spanish accents fold; ä, ö, ü and ß stay (Mohl/Möhl differ).
const ACCENTS: Record<string, string> = {
	é: 'e',
	è: 'e',
	ê: 'e',
	ë: 'e',
	î: 'i',
	ï: 'i',
	à: 'a',
	â: 'a',
	ô: 'o',
	ç: 'c',
	ñ: 'n',
	á: 'a',
	í: 'i',
	ó: 'o',
	ú: 'u',
	û: 'u',
	ù: 'u'
};

// Go's len() counts bytes, so "öl" is 3 long there and keeps its word.
const bytes = (w: string) => new TextEncoder().encode(w).length;

function singular(w: string): string {
	const end = (...suffixes: string[]) => suffixes.some((s) => w.endsWith(s));
	const irregular = IRREGULAR_PLURALS.get(w);
	if (irregular !== undefined) return irregular;
	if (bytes(w) < 4) return w;
	if (end('ies')) return w.slice(0, -3) + 'y';
	if (end('oes')) return w.slice(0, -2);
	if (end('sses', 'xes', 'ches', 'shes', 'zes')) return w.slice(0, -2);
	if (end('en', 'eln', 'ern')) return w.slice(0, -1);
	if (end('ss', 'us', 'is')) return w;
	if (end('s')) return w.slice(0, -1);
	return w;
}
