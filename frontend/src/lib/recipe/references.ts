import type {
	IngredientGroup,
	IngredientRef,
	Step,
	StepPhoto,
	StepTime
} from '#lib/api/recipes.js';
import { formatQuantityFor } from './scale';

/**
 * One piece of a step's text. `quantity` is present on a resolved reference, `time` on a stored step time,
 * `photo` on every piece inside a photo phrase (consecutive pieces with the same photo form one trigger).
 */
export type Segment = { text: string; quantity?: string; time?: StepTime; photo?: StepPhoto };

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

/** The word's first occurrence in `text`, or null. */
export function firstWordMatch(text: string, word: string): { from: number; to: number } | null {
	const match = wordPattern(word).exec(text);
	return match === null ? null : { from: match.index, to: match.index + match[0].length };
}

/**
 * Every occurrence of `word` after the first. Only the first carries a
 * reference and its quantity; the editor draws these fainter, so the author
 * sees the word is known without reading it as a second amount.
 */
export function laterWordMatches(text: string, word: string): { from: number; to: number }[] {
	const pattern = wordPattern(word);
	const all = [...text.matchAll(new RegExp(pattern.source, `${pattern.flags}g`))];
	return all.slice(1).map((match) => ({ from: match.index, to: match.index + match[0].length }));
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
	type Hit = { start: number; end: number; quantity?: string; time?: StepTime };
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
	// Stored times are marked like references: first occurrence only, earlier start wins.
	for (const time of step.times) {
		const match = wordPattern(time.phrase).exec(step.text);
		if (match) hits.push({ start: match.index, end: match.index + match[0].length, time });
	}
	hits.sort((a, b) => a.start - b.start);

	const out: (Segment & { at: number })[] = [];
	let cursor = 0;
	for (const hit of hits) {
		if (hit.start < cursor) continue; // overlapping hits: first one wins
		if (hit.start > cursor) out.push({ text: step.text.slice(cursor, hit.start), at: cursor });
		const text = step.text.slice(hit.start, hit.end);
		out.push(
			hit.time
				? { text, time: hit.time, at: hit.start }
				: { text, quantity: hit.quantity, at: hit.start }
		);
		cursor = hit.end;
	}
	if (cursor < step.text.length) out.push({ text: step.text.slice(cursor), at: cursor });

	// Photo phrases are boundaries, not hits: they cut the pieces above and tag
	// what lies inside, so a referenced word keeps its quantity. First occurrence
	// only; a time is never cut, and phrases never overlap (the server refuses
	// both).
	const ranges: { start: number; end: number; photo: StepPhoto }[] = [];
	for (const photo of step.photos ?? []) {
		const match = wordPattern(photo.phrase).exec(step.text);
		if (!match) continue;
		const start = match.index;
		const end = start + match[0].length;
		ranges.push({ start, end, photo });
	}
	return out.flatMap(({ at, ...piece }): Segment[] => {
		if (piece.time) return [piece];
		const end = at + piece.text.length;
		const cuts = ranges.filter((r) => r.start < end && r.end > at);
		if (cuts.length === 0) return [piece];
		const parts: Segment[] = [];
		let from = at;
		const push = (to: number, photo?: StepPhoto) => {
			if (to <= from) return;
			const text = piece.text.slice(from - at, to - at);
			// The quantity belongs to the last part only: one amount, shown once,
			// after the whole referenced word rather than between its halves.
			const last = to === end;
			parts.push({
				text,
				...(last && piece.quantity ? { quantity: piece.quantity } : {}),
				...(photo ? { photo } : {})
			});
			from = to;
		};
		for (const r of cuts.sort((a, b) => a.start - b.start)) {
			push(Math.max(r.start, at));
			push(Math.min(r.end, end), r.photo);
		}
		push(end);
		return parts;
	});
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
 * groups), this proposes nothing - that is precisely the case only the author
 * can decide, so `matchStep` reports the word with its candidates instead and
 * the editor puts them side by side.
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
	return matchStep(text, groups).refs;
}

/** A word that names the same ingredient in more than one group. */
export type AmbiguousWord = {
	word: string;
	candidates: { groupName: string | null; ingredientName: string }[];
};

/**
 * The matcher's whole answer for a step: the proposals `suggestReferences`
 * returns, and the words it stays silent on because they name an ingredient
 * in several groups. Those only the author can decide, so the editor offers
 * the candidates side by side instead of guessing one into a proposal; an
 * ambiguity touching a group the service cannot single out is left out, as
 * a proposal into one would be.
 */
export function matchStep(
	text: string,
	groups: IngredientGroup[]
): { refs: IngredientRef[]; ambiguous: AmbiguousWord[] } {
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
	const ambiguous: AmbiguousWord[] = [];
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
			// exact.length > 1: the name is ambiguous in this recipe - no
			// proposal, but the author is offered the candidates.
			if (exact.length > 1 && exact.every((e) => e.usable) && !taken.has(word)) {
				taken.add(word);
				ambiguous.push({
					word,
					candidates: exact.map((e) => ({ groupName: e.group, ingredientName: e.name }))
				});
			}
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
	return { refs: out, ambiguous };
}
