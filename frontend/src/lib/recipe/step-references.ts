import type { IngredientGroup } from '$lib/api/recipes';
import { parseNumber, type FormGroup, type FormRef, type FormStep } from './form';
import { isWordChar, suggestReferences, wordPattern } from './references';
import { formatQuantity } from './format';

/**
 * The logic behind the step editor's ingredient links, kept out of
 * `StepEditor.svelte` so it can be tested without a browser: the editor's
 * Vitest project runs in node and cannot mount a ProseMirror view.
 */

/** One choosable ingredient in the `@` picker, and the label of one chip. */
export type PickerEntry = {
	/** The `FormIngredient.id` of the row this entry stands for: unique within
	 * one recipe, the key of the picker's `{#each}`, and what a reference made
	 * from this entry is anchored to. The option's DOM id is built from the
	 * step and the row's position instead, because `aria-activedescendant` has
	 * to name the row as it currently stands in the narrowed list. */
	ingredientId: string;
	/** The group's name, or null for the unnamed group. */
	group: string | null;
	name: string;
	/** Quantity and unit already joined for display: `100 g`, `1`, or ``. */
	amount: string;
};

/** Words the author has dismissed, keyed by step id. Never persisted. */
export type DismissedWords = Record<string, string[]>;

/** One ingredient row, shaped the way the API will see it, id still attached. */
type ShapedRow = {
	/** `FormIngredient.id`, which is what a reference is anchored to. */
	id: string;
	quantity: number | null;
	unit: string | null;
	name: string;
};

type ShapedGroup = { name: string | null; rows: ShapedRow[] };

/**
 * The form's groups as resolution will see them: every string trimmed, blanks
 * turned into null, and rows without a name dropped - they carry nothing to
 * link to, and an empty name would make every other row in the group
 * "ambiguous". An unnamed group left with no rows goes too: `toInput` drops
 * it, so the API never sees it, and counting it would make the one unnamed
 * group the API does see look like one of two.
 *
 * The matcher and the picker both start here, through `toIngredientGroups` and
 * `pickerEntries`, so the two can never disagree about which rows exist or what
 * they are called. They would have to agree exactly: the matcher decides what
 * is proposed and the picker decides what can be resolved, and a row the one
 * sees and the other does not is a link the author is offered and then refused.
 */
function shapeGroups(groups: FormGroup[]): ShapedGroup[] {
	return groups
		.map((group) => ({
			name: blankToNull(group.name),
			rows: group.ingredients
				.filter((row) => row.name.trim() !== '')
				.map((row) => ({
					id: row.id,
					quantity: parseNumber(row.quantity),
					unit: blankToNull(row.unit),
					name: row.name.trim()
				}))
		}))
		.filter((group) => group.name !== null || group.rows.length > 0);
}

/**
 * The same groups as the API type, which is what the matcher takes: the client
 * ids stripped, because nothing downstream of a save may carry one.
 */
export function toIngredientGroups(groups: FormGroup[]): IngredientGroup[] {
	return shapeGroups(groups).map((group) => ({
		name: group.name,
		ingredients: group.rows.map(({ quantity, unit, name }) => ({
			quantity,
			unit,
			name,
			note: null
		}))
	}));
}

function blankToNull(value: string): string | null {
	const trimmed = value.trim();
	return trimmed === '' ? null : trimmed;
}

/**
 * The ingredients the picker may offer.
 *
 * Only the ones the API can resolve back to a single row: it refuses a
 * reference whose group name occurs twice (two unnamed groups included) or
 * whose ingredient name occurs twice inside its group, with a 422. Offering
 * such a row would hand the author a link that cannot be saved, so it is left
 * out rather than shown and then rejected.
 */
export function pickerEntries(groups: FormGroup[]): PickerEntry[] {
	const shaped = shapeGroups(groups);
	const groupCount = tally(shaped.map((group) => group.name ?? ''));
	const out: PickerEntry[] = [];
	for (const group of shaped) {
		if ((groupCount.get(group.name ?? '') ?? 0) > 1) continue;
		const nameCount = tally(group.rows.map((row) => row.name));
		for (const row of group.rows) {
			if ((nameCount.get(row.name) ?? 0) > 1) continue;
			out.push({
				ingredientId: row.id,
				group: group.name,
				name: row.name,
				amount: [formatQuantity(row.quantity), row.unit ?? ''].join(' ').trim()
			});
		}
	}
	return out;
}

function tally(values: string[]): Map<string, number> {
	const counts = new Map<string, number>();
	for (const value of values) {
		counts.set(value, (counts.get(value) ?? 0) + 1);
	}
	return counts;
}

/**
 * The picker's rows for a query: the ones whose name contains it, the ones
 * starting with it first. Matching on the whole name rather than its head noun
 * is deliberate - the author is typing to find a row in their own list, not
 * feeding the matcher.
 */
export function matchEntries(entries: PickerEntry[], query: string): PickerEntry[] {
	const q = query.trim().toLowerCase();
	if (q === '') return entries;
	const starts: PickerEntry[] = [];
	const contains: PickerEntry[] = [];
	for (const entry of entries) {
		const name = entry.name.toLowerCase();
		if (name.startsWith(q)) starts.push(entry);
		else if (name.includes(q)) contains.push(entry);
	}
	return [...starts, ...contains];
}

/**
 * The entry a reference points at, or undefined when it points nowhere.
 *
 * By row, not by name: a reference carries the client id of the row it was
 * made from, so renaming that row - or its group - moves the link rather than
 * breaking it. A reference that arrived naming a row the recipe no longer has
 * carries no id at all and is unresolved by definition.
 */
export function entryFor(entries: PickerEntry[], ref: FormRef): PickerEntry | undefined {
	if (ref.ingredientId === null) return undefined;
	return entries.find((entry) => entry.ingredientId === ref.ingredientId);
}

/**
 * The entry a name pair points at. Only the matcher needs this: a proposal is
 * made of names and has to find its row before it can become a reference.
 */
function entryByName(
	entries: PickerEntry[],
	group: string | null,
	ingredient: string
): PickerEntry | undefined {
	return entries.find((entry) => entry.group === group && entry.name === ingredient);
}

/** `Zucker · 100 g · Grütze` - the unnamed group and a missing amount drop out. */
export function entryLabel(entry: PickerEntry): string {
	return [entry.name, entry.amount, entry.group ?? ''].filter((part) => part !== '').join(' · ');
}

/**
 * Whether a reference still points at a row the API can resolve.
 *
 * Renaming the row, or its group, does NOT make this false: the reference
 * carries the row's client id, so the link stays resolved and `toInput` sends
 * the new names. What makes it false is the row being deleted, the reference
 * never having found a row when the recipe was loaded, or the row's name or
 * group name turning ambiguous - the last of these is the one the API would
 * answer with a 422; the first two never reach it, because `validate` refuses
 * the save.
 */
export function isResolved(ref: FormRef, entries: PickerEntry[]): boolean {
	return entryFor(entries, ref) !== undefined;
}

/**
 * A reference in one line, for accessible names. A reference whose row has
 * meanwhile been deleted, or that never found one, still names an ingredient
 * and a group, so it is written out from the reference itself rather than
 * hidden - the author has to be able to find and remove a link that no longer
 * points anywhere. A renamed row needs none of that: the link still resolves,
 * and the label reads the new name off the entry like any other.
 */
export function refLabel(ref: FormRef, entries: PickerEntry[]): string {
	const entry = entryFor(entries, ref);
	if (entry) return entryLabel(entry);
	return [ref.ingredientName, ref.groupName ?? ''].filter((part) => part !== '').join(' · ');
}

/** What the editor shows for one reference, split so each part gets its own place. */
export type RefView = {
	name: string;
	/** `100 g`, `1`, or `` for an ingredient without a quantity, or one that is gone. */
	amount: string;
	/** The group, but only where it tells two rows apart, or where the row is gone. */
	group: string | null;
	resolved: boolean;
};

/**
 * A reference as the editor shows it. The group is left out unless it says
 * something: in a recipe with one `Zwiebel`, "Klopse" beside it is noise, and
 * on every link of a step it is what made the list read as clutter. It stays
 * where the same name occurs in two groups - the case the group exists for -
 * and on a link whose row is gone, where it is part of saying which one.
 */
export function refView(ref: FormRef, entries: PickerEntry[]): RefView {
	const entry = entryFor(entries, ref);
	if (entry === undefined) {
		return { name: ref.ingredientName, amount: '', group: ref.groupName, resolved: false };
	}
	const twice = entries.filter((other) => other.name === entry.name).length > 1;
	return {
		name: entry.name,
		amount: entry.amount,
		group: twice ? entry.group : null,
		resolved: true
	};
}

/**
 * The marked word the caret stands inside, or null. `caret` is an offset into
 * the step's plain text, and only the first occurrence of a word counts - the
 * one the editor underlines. Inside means strictly: a caret at either edge is
 * where somebody types next to the word, not into it, and the word's card
 * must not pop up under their fingers while they finish typing it.
 */
export function wordAtCaret(text: string, words: string[], caret: number): string | null {
	for (const word of words) {
		const hit = firstWordMatch(text, word);
		if (hit !== null && hit.from < caret && caret < hit.to) return word;
	}
	return null;
}

/**
 * The matcher's proposals for one step that the author has neither confirmed
 * nor dismissed. A word already carrying a reference is never proposed again:
 * the API allows one reference per word per step.
 *
 * Everything the matcher returns has to be found in the same list the picker
 * offers, which is also where a proposal picks up the client id that anchors
 * it. `acceptAll` turns every open proposal into a real reference on save, so a
 * proposal the API cannot resolve would fail the save with a message about the
 * ingredient list, shown under a step, for a link the author never made. This
 * is the structural guarantee that nothing unusable is ever offered; the
 * matcher itself already declines to propose such a row, and these two are
 * deliberately belt and braces.
 */
export function pendingFor(
	step: FormStep,
	groups: FormGroup[],
	dismissed: string[] = []
): FormRef[] {
	const confirmed = new Set(step.references.map((ref) => ref.word));
	const refused = new Set(dismissed);
	const entries = pickerEntries(groups);
	const out: FormRef[] = [];
	for (const ref of suggestReferences(step.text, toIngredientGroups(groups))) {
		if (confirmed.has(ref.word) || refused.has(ref.word)) continue;
		const entry = entryByName(entries, ref.groupName, ref.ingredientName);
		// Not in the picker's list means the API could not resolve it either;
		// this is the second of the two gates the doc comment describes.
		if (entry === undefined) continue;
		out.push({
			word: ref.word,
			ingredientId: entry.ingredientId,
			groupName: entry.group,
			ingredientName: entry.name
		});
	}
	return out;
}

/** How many proposals are open across the whole recipe. */
export function countPending(
	steps: FormStep[],
	groups: FormGroup[],
	dismissed: DismissedWords = {}
): number {
	return steps.reduce(
		(total, step) => total + pendingFor(step, groups, dismissed[step.id] ?? []).length,
		0
	);
}

/**
 * Confirms every open proposal, in place. The save button says it will do
 * this, and "Accept now" does it on demand; both go through here so the
 * two can never disagree.
 */
export function acceptAll(
	steps: FormStep[],
	groups: FormGroup[],
	dismissed: DismissedWords = {}
): void {
	for (const step of steps) {
		for (const ref of pendingFor(step, groups, dismissed[step.id] ?? [])) {
			step.references = addReference(step.references, ref);
		}
	}
}

/**
 * Adds a reference, replacing any that already names the same word - the API
 * rejects a step whose references name one word twice, and the newer choice is
 * the one the author just made.
 */
export function addReference(refs: FormRef[], ref: FormRef): FormRef[] {
	const at = refs.findIndex((existing) => existing.word === ref.word);
	if (at < 0) return [...refs, ref];
	const out = [...refs];
	out[at] = ref;
	return out;
}

/** Drops the reference naming `word`. */
export function removeReference(refs: FormRef[], word: string): FormRef[] {
	return refs.filter((ref) => ref.word !== word);
}

/**
 * The word a selection points at, grown out to whole words.
 *
 * `from` and `to` are offsets into the step's own text, and the two may be
 * equal: a caret grows in both directions, so standing anywhere inside a word
 * picks that word. A selection that begins or ends inside a word takes the
 * whole word, because a reference is anchored to a whole word - the API
 * refuses half of one, and the decoration would sit over the wrong run of
 * text. Spaces and punctuation at either edge are dropped, and a range that
 * holds no letter or digit at all is null, which is what a caret between two
 * words gives.
 *
 * A selection spanning several words keeps them, space and all: the anchor is
 * a run of text that occurs in the sentence as a whole word, so "Crème
 * fraîche" is as linkable as "Fleisch".
 */
export function wordAt(text: string, from: number, to: number): string | null {
	let start = Math.max(0, Math.min(from, to, text.length));
	let end = Math.min(Math.max(from, to), text.length);
	if (end < start) end = start;
	while (start > 0 && isWordChar(text[start - 1])) start--;
	while (end < text.length && isWordChar(text[end])) end++;
	while (start < end && !isWordChar(text[start])) start++;
	while (end > start && !isWordChar(text[end - 1])) end--;
	return start < end ? text.slice(start, end) : null;
}

/**
 * The longest word a reference may anchor to, in characters, as the API
 * counts them. It matches the longest ingredient name, so whatever the `@`
 * picker inserts always fits; only a selection spanning several words can
 * outgrow it.
 */
export const MAX_WORD_LENGTH = 120;

/** Whether `word` is short enough for the API to take as a reference. */
export function fitsWordLimit(word: string): boolean {
	return [...word].length <= MAX_WORD_LENGTH;
}

/** The word's first occurrence in `text`, or null. */
export function firstWordMatch(text: string, word: string): { from: number; to: number } | null {
	const match = wordPattern(word).exec(text);
	return match === null ? null : { from: match.index, to: match.index + match[0].length };
}
