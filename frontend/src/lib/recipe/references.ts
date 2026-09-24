import type { IngredientGroup, IngredientRef, Step } from '$lib/api/recipes';
import { formatQuantityFor } from './scale';

/** One piece of a step's text. `quantity` is present on a resolved reference. */
export type Segment = { text: string; quantity?: string };

/**
 * Unicode letters and digits, shared by every place in this file that needs
 * to know what counts as "part of a word". JavaScript's `\b` is ASCII-only,
 * so `\bÖl\b` and `\bÄpfel\b` misbehave - not an edge case in a German recipe
 * book. This class with the `u` flag is what makes the boundary correct.
 */
const WORD_CHAR_CLASS = '\\p{L}\\p{N}';

/**
 * Matches `word` where it stands as a whole word, with the Unicode boundary
 * `WORD_CHAR_CLASS` defines. Exported because the editor has to underline the
 * very occurrence `segmentStep` will later annotate - two different notions of
 * "the same word" would show the author one thing and the cook another.
 *
 * Deliberately without the `g` flag: `exec` then always returns the earliest
 * match, which is the one occurrence both places annotate.
 */
export function wordPattern(word: string): RegExp {
	const escaped = word.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
	return new RegExp(`(?<![${WORD_CHAR_CLASS}])${escaped}(?![${WORD_CHAR_CLASS}])`, 'u');
}

/**
 * Whether one character is part of a word, by the very class `wordPattern`
 * draws its boundaries with. Exported for the editor, which grows a selection
 * out to whole words before anchoring a reference to it: a word that ended
 * anywhere else than where `wordPattern` says it does would be a link the API
 * refuses and a decoration over the wrong run of text.
 */
export function isWordChar(char: string): boolean {
	return new RegExp(`^[${WORD_CHAR_CLASS}]$`, 'u').test(char);
}

/**
 * Resolves a reference to its ingredient, or undefined when it no longer
 * points anywhere. Ambiguity resolves to undefined rather than to a guess, so a
 * reference can lose its quantity but never show the wrong one.
 */
function lookup(groups: IngredientGroup[], group: string | null, ingredient: string) {
	const matches = groups.filter((g) => (g.name ?? null) === group);
	if (matches.length !== 1) return undefined;
	const rows = matches[0].ingredients.filter((i) => i.name === ingredient);
	return rows.length === 1 ? rows[0] : undefined;
}

/**
 * Splits a step's text into plain pieces and referenced words. Every quantity
 * runs through `formatQuantityFor`, the same function the ingredient list uses,
 * so a reference and the list can never disagree.
 *
 * Only the FIRST occurrence of a referenced word is annotated - deliberately,
 * not an oversight: `wordPattern(...).exec()` below has no `g` flag, so it
 * always returns the earliest match. Repeating "(100 g)" on a later mention
 * would read as an ADDITIONAL 100 g rather than the same amount, and a number
 * that can be misread is exactly what this feature must never produce.
 *
 * A step whose `references` name the same `word` twice is likewise resolved
 * deliberately rather than by array order: the API rejects that at write
 * time (one word, one meaning, within a step), but this also runs against
 * unsaved editor state that has not been validated yet. The first reference
 * for a given word wins; every later one naming the same word is dropped
 * outright, so which ingredient (if any) it pointed at never matters.
 */
export function segmentStep(
	step: Step,
	groups: IngredientGroup[],
	servings: number,
	baseServings: number
): Segment[] {
	type Hit = { start: number; end: number; quantity: string };
	const hits: Hit[] = [];
	const seenWords = new Set<string>();
	for (const ref of step.references) {
		if (seenWords.has(ref.word)) continue;
		seenWords.add(ref.word);
		const found = lookup(groups, ref.groupName, ref.ingredientName);
		if (!found) continue;
		const match = wordPattern(ref.word).exec(step.text);
		if (!match) continue;
		// No number, no annotation: a unit with nothing to count (`(Prise)`)
		// is more confusing than the plain word alone.
		const amount = formatQuantityFor(found.quantity, baseServings, servings);
		if (!amount) continue;
		const quantity = [amount, found.unit ?? ''].join(' ').trim();
		hits.push({ start: match.index, end: match.index + match[0].length, quantity });
	}
	hits.sort((a, b) => a.start - b.start);

	const out: Segment[] = [];
	let cursor = 0;
	for (const hit of hits) {
		if (hit.start < cursor) continue; // overlapping hits: first one wins
		if (hit.start > cursor) out.push({ text: step.text.slice(cursor, hit.start) });
		out.push({ text: step.text.slice(hit.start, hit.end), quantity: hit.quantity });
		cursor = hit.end;
	}
	if (cursor < step.text.length) out.push({ text: step.text.slice(cursor) });
	return out;
}

/**
 * Words that look like an ingredient but actually name the finished dish or a
 * cooking medium: "Bratkartoffeln" and "Kartoffelpuffern" ought to say
 * nothing about Kartoffeln, and "Salzwasser" ought to say nothing about
 * Wasser or Salz.
 *
 * This list gates ONLY the compound-overlap branch of `suggestReferences`,
 * never an exact head-noun match - an exact match always wins on its own.
 * That split matters here specifically: the brief's original regex,
 *   /^(brat|röst|salz)|(puffer|kuchen|salat|suppe|wasser|teig|masse|füllung)n?$/iu
 * binds its alternation as `^(brat|röst|salz)` with no anchor on the rest,
 * so it matches the BARE word "Salz" too - which, if it were consulted for
 * every word rather than only for compound candidates, would mean the
 * ingredient Salz could never match its own name. The first alternative
 * below is written to require at least one more character after the prefix,
 * so it only ever fires on a compound ("Salzwasser"), never on "Salz" alone;
 * combined with gating only the compound branch, this is belt and braces.
 */
const NOT_INGREDIENTS =
	/^(?:brat|röst|salz).+|(?:puffer|kuchen|salat|suppe|wasser|teig|masse|füllung)n?$/iu;

/**
 * Shortest non-overlapping remainder a compound match must leave behind, on
 * whichever side is longer. Below this, the "compound" is really just the
 * ingredient's head noun plus a stray letter or two - "Kreis" (a circle) is
 * not "Reis" (rice) with a compound prefix, even though "kreis".endsWith
 * ("reis") is true and both strings clear the 4-character floor below.
 */
const MIN_COMPOUND_REMAINDER = 3;

/** Head noun of a possibly multi-word name ("Crème fraîche" -> "fraîche"). */
function head(name: string): string {
	const parts = name.trim().toLowerCase().split(/\s+/);
	return parts[parts.length - 1] ?? '';
}

/**
 * Proposes references for one step's text, for the editor to offer as
 * suggestions a human then accepts or discards. Nothing this returns is ever
 * rendered to a cook directly, which is exactly why it is allowed to be
 * aggressive: a wrong proposal costs one click, not a wrong number in
 * someone's kitchen.
 *
 * Because a human reviews every hit, this uses compound splitting (82 % of a
 * sample corpus's steps) rather than the conservative exact match alone
 * (66 %, but zero false positives). Inflection stemming is deliberately
 * absent: measured against the corpus it bought one real hit
 * ("Semmelbröseln") and three false ones ("salzen" -> Salz).
 *
 * R4: an exact head-noun match always wins and is never subject to
 * `NOT_INGREDIENTS` - that list exists solely to stop the compound-overlap
 * branch from proposing an intermediate product or a cooking medium. When a
 * name is ambiguous within the recipe (the same ingredient name in two
 * groups), this stays silent - that is precisely the case only the author
 * can decide.
 *
 * A row in a group the service cannot single out is never proposed either.
 * `findGroup` in `service/internal/recipe/references.go` refuses a reference
 * whose group name occurs twice - two unnamed groups included - with a 422, so
 * a proposal into one of them is a link the author could never save. Those
 * rows still count towards ambiguity above: a word that names an entry in an
 * unusable group as well as one in a usable group stays silent rather than
 * quietly resolving to whichever of the two happens to be saveable.
 */
export function suggestReferences(text: string, groups: IngredientGroup[]): IngredientRef[] {
	const groupCount = new Map<string, number>();
	for (const g of groups) {
		const key = g.name ?? '';
		groupCount.set(key, (groupCount.get(key) ?? 0) + 1);
	}
	const entries = groups.flatMap((g) =>
		g.ingredients.map((i) => ({
			group: g.name ?? null,
			name: i.name,
			usable: (groupCount.get(g.name ?? '') ?? 0) === 1
		}))
	);
	const out: IngredientRef[] = [];
	const taken = new Set<string>();

	for (const found of text.matchAll(new RegExp(`[${WORD_CHAR_CLASS}]+`, 'gu'))) {
		const word = found[0];
		if (taken.has(word)) continue;
		const w = word.toLowerCase();

		// Exact match always wins, and is never filtered by NOT_INGREDIENTS:
		// that is what lets the ingredient "Salz" match the word "Salz".
		const exact = entries.filter((e) => head(e.name) === w);
		if (exact.length > 0) {
			if (exact.length === 1 && exact[0].usable) {
				taken.add(word);
				out.push({ word, groupName: exact[0].group, ingredientName: exact[0].name });
			}
			// exact.length > 1: the name is ambiguous in this recipe - stay silent.
			continue;
		}

		if (NOT_INGREDIENTS.test(w)) continue;

		// An ordinary German plural ("Zwiebeln" against ingredient "Zwiebel")
		// is a known, accepted gap here: the remainder is only 1 character,
		// same as "Kreis" against "Reis", so MIN_COMPOUND_REMAINDER rejects
		// it too. Inflection stemming was measured and rejected (see the
		// doc comment above) - this is that trade-off surfacing again, not
		// an oversight.
		const compound = entries.filter((e) => {
			const h = head(e.name);
			if (h.length < 4 || w.length < 4) return false;
			if (h.endsWith(w)) return h.length - w.length >= MIN_COMPOUND_REMAINDER;
			if (w.endsWith(h)) return w.length - h.length >= MIN_COMPOUND_REMAINDER;
			return false;
		});

		if (compound.length !== 1 || !compound[0].usable) continue;
		taken.add(word);
		out.push({ word, groupName: compound[0].group, ingredientName: compound[0].name });
	}
	return out;
}
