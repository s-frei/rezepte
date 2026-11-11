// British spellings that docs/memory/content/conventions/language.mdx rules
// out, as whole words with their common inflections. A letter on either side
// ends the match, but `_` and digits do not, so `cancelled_at` is caught.
// `analyses` is left out: it is also the American plural of analysis.
const WORDS = [
	'colour(s|ed|ing|ful|less)?',
	'favourite[sd]?',
	'centre[sd]?',
	'centring',
	'normalis(e|es|ed|ing|ation|ations)',
	'behaviour(s|al)?',
	'grey(s|ed|ing|ish|er|est)?',
	'catalogue[sd]?',
	'cancell(ed|ing)',
	'labell(ed|ing)',
	'initialis(e|es|ed|ing|ation)',
	'organis(e|es|ed|ing|ation|ations|er|ers)',
	'(milli)?litres?',
	'metres?',
	'analys(e|ed|ing)'
];
const PATTERN = new RegExp(`(?<![a-z])(${WORDS.join('|')})(?![a-z])`, 'gi');

// Each exclusion with its reason.
const EXCLUDED: [RegExp, string][] = [
	[/^frontend\/messages\/de\.json$/, 'the German catalog'],
	[/\.de\.json$/, 'German sample data such as the German recipes'],
	[/^docs\/memory\/content\/conventions\/language\.mdx$/, 'quotes the forbidden spellings'],
	[/(^|\/)(bun\.lock|mise\.lock|go\.sum)$/, 'lockfiles name third-party packages, such as @img/colour'],
	[/^mise\/lib\/guards\/spelling(\.test)?\.ts$/, 'this checker lists the spellings it forbids']
];

export const excluded = (path: string) => EXCLUDED.some(([re]) => re.test(path));

export function hits(text: string): { line: number; word: string }[] {
	return text.split('\n').flatMap((l, i) => [...l.matchAll(PATTERN)].map((m) => ({ line: i + 1, word: m[0] })));
}
