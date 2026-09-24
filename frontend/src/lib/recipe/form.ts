import type { FieldError } from '$lib/api/client';
import {
	emptyInput,
	type IngredientGroup,
	type IngredientRef,
	type RecipeInput
} from '$lib/api/recipes';
import { m } from '$lib/paraglide/messages';
import { getLocale } from '$lib/paraglide/runtime';
import { wordPattern } from './references';

/**
 * The editor's form model: the same shape as `RecipeInput`, but every field
 * the user types into is a plain string (so an input can bind to it and stay
 * empty, and so `1,5` survives until it is parsed), and every list entry
 * carries a client-side `id` used as the `{#each}` key and as
 * svelte-dnd-action's item id. `toInput` converts it back into a
 * `RecipeInput`.
 */
export type FormIngredient = {
	id: string;
	/** Raw text; `1,5` and `1.5` both parse (see `parseNumber`). */
	quantity: string;
	unit: string;
	name: string;
	note: string;
};

export type FormGroup = {
	id: string;
	name: string;
	ingredients: FormIngredient[];
};

/**
 * A reference while it is being edited: anchored to the ingredient row, not to
 * the names that row happens to carry right now.
 *
 * This is the middle one of the feature's three anchors - client id in the
 * editor, names on the wire, foreign keys in the database. Holding the wire's
 * names here instead would mean a link is pinned to whatever the group and the
 * row were called at the moment it was made, so renaming either would break
 * every link in the recipe; `toInput` reads the names off the row again
 * instead, at save time.
 */
export type FormRef = {
	word: string;
	/** `FormIngredient.id` of the row this points at, or null when the
	 *  incoming reference named a row this recipe no longer has. */
	ingredientId: string | null;
	/** What the reference named when it arrived. Kept only so an
	 *  unresolved one can still be shown and sent back unchanged. */
	groupName: string | null;
	ingredientName: string;
};

export type FormStep = {
	id: string;
	text: string;
	references: FormRef[];
};

export type RecipeForm = {
	title: string;
	description: string;
	servings: string;
	prepMinutes: string;
	cookMinutes: string;
	sourceUrl: string;
	tags: string[];
	ingredientGroups: FormGroup[];
	steps: FormStep[];
};

/**
 * Validation messages keyed by form field. Top-level fields use their
 * `RecipeInput` name (`title`, `servings`, ...); a problem with a single
 * ingredient row is keyed `quantity:<rowId>` or `name:<rowId>` so the row
 * can render it inline.
 */
export type FieldErrors = Record<string, string>;

/** Most tags the API accepts (`maxItems:"20"` on `Input.Tags`). */
export const MAX_TAGS = 20;

/** Field order used to pick the error to scroll to; also the set of
 * top-level locations `applyServerErrors` understands. */
const FIELD_ORDER = [
	'title',
	'description',
	'servings',
	'prepMinutes',
	'cookMinutes',
	'sourceUrl',
	'tags',
	'ingredientGroups',
	'steps'
];

// Monotonic counter rather than `crypto.randomUUID()`: ids only have to be
// unique within the page (dnd and `{#each}` keys), they never leave the
// browser, and a counter keeps tests deterministic.
let idCounter = 0;

/** Returns a fresh client-side id for a group, ingredient row or step. */
export function newId(): string {
	idCounter += 1;
	return `f${idCounter}`;
}

/** A blank ingredient row. */
export function newIngredient(): FormIngredient {
	return { id: newId(), quantity: '', unit: '', name: '', note: '' };
}

/** A blank, unnamed group holding one blank row. */
export function newGroup(): FormGroup {
	return { id: newId(), name: '', ingredients: [newIngredient()] };
}

/** A blank step. */
export function newStep(): FormStep {
	return { id: newId(), text: '', references: [] };
}

/** True when a string is empty or only whitespace. */
export function isBlank(value: string): boolean {
	return value.trim().length === 0;
}

/**
 * Parses a user-typed number, accepting the German decimal comma (`1,5`) as
 * well as a point (`1.5`). Blank input and anything that isn't a finite
 * number both yield `null` - callers that need to tell those apart check
 * `isBlank` first.
 *
 * Numbers and `null` are accepted as well: a `bind:value` on an `<input
 * type="number">` replaces the form's string with one of those, and this
 * running before validation must not throw when that happens.
 */
export function parseNumber(value: string | number | null): number | null {
	if (value === null) {
		return null;
	}
	if (typeof value === 'number') {
		return Number.isFinite(value) ? value : null;
	}
	const normalized = value.trim().replace(',', '.');
	if (normalized === '') {
		return null;
	}
	const parsed = Number(normalized);
	return Number.isFinite(parsed) ? parsed : null;
}

// Cached per locale, exactly as `format.ts`'s `decimals()`/`longDate()` and
// `user-sort.ts`'s `nameCollator()` cache theirs - getLocale() only changes
// after Paraglide's setLocale() reloads the page.
let decimalSeparator: string | undefined;
let decimalSeparatorLocale: string | undefined;

/** The locale's own decimal separator (`.` in English, `,` in German). */
function localeDecimalSeparator(): string {
	const locale = getLocale();
	if (decimalSeparator === undefined || decimalSeparatorLocale !== locale) {
		const part = new Intl.NumberFormat(locale).formatToParts(1.1).find((p) => p.type === 'decimal');
		decimalSeparator = part?.value ?? '.';
		decimalSeparatorLocale = locale;
	}
	return decimalSeparator;
}

/**
 * Renders a stored quantity back into editable text, swapping only the
 * decimal separator for the locale's own (`1.5` -> `1.5` in English, `1,5`
 * in German). Deliberately not routed through `formatQuantity`: that
 * function renders fraction glyphs (`1 ½`) for display, which an editable
 * field must not show back to the person who typed a plain decimal, and it
 * rounds to two decimals, which would silently drop precision the cook
 * actually stored (`1.333` must stay `1.333`, not become `1.33`).
 */
function quantityToText(quantity: number | null): string {
	return quantity === null ? '' : String(quantity).replace('.', localeDecimalSeparator());
}

function blankToNull(value: string): string | null {
	const trimmed = value.trim();
	return trimmed === '' ? null : trimmed;
}

/** True when a row carries any content at all - blank rows are dropped by `toInput`. */
function isFilledRow(row: FormIngredient): boolean {
	return !isBlank(row.quantity) || !isBlank(row.unit) || !isBlank(row.name) || !isBlank(row.note);
}

/**
 * Builds the form model from a recipe (or any `RecipeInput`, which is how
 * `emptyForm` reuses it).
 *
 * Empty collections are padded so the editor always has something to type
 * into: at least one group, at least one row per group and at least one
 * step. `toInput` drops those again if they stay blank.
 */
export function fromRecipe(recipe: RecipeInput): RecipeForm {
	const groups = recipe.ingredientGroups.map((group) => ({
		id: newId(),
		name: group.name ?? '',
		ingredients:
			group.ingredients.length > 0
				? group.ingredients.map((ingredient) => ({
						id: newId(),
						quantity: quantityToText(ingredient.quantity),
						unit: ingredient.unit ?? '',
						name: ingredient.name,
						note: ingredient.note ?? ''
					}))
				: [newIngredient()]
	}));
	const steps = recipe.steps.map((step) => ({
		id: newId(),
		text: step.text,
		references: step.references.map((ref) => ({
			word: ref.word,
			ingredientId: rowFor(groups, ref),
			groupName: ref.groupName,
			ingredientName: ref.ingredientName
		}))
	}));

	return {
		title: recipe.title,
		description: recipe.description,
		servings: String(recipe.servings),
		prepMinutes: recipe.prepMinutes === null ? '' : String(recipe.prepMinutes),
		cookMinutes: recipe.cookMinutes === null ? '' : String(recipe.cookMinutes),
		sourceUrl: recipe.sourceUrl ?? '',
		tags: [...recipe.tags],
		ingredientGroups: groups.length > 0 ? groups : [newGroup()],
		steps: steps.length > 0 ? steps : [newStep()]
	};
}

/**
 * The client id of the row a stored reference names, or null when it names no
 * row this recipe still has.
 *
 * The rule is the server's (`findGroup`/`findIngredient` in
 * `service/internal/recipe/references.go`): the group whose trimmed name
 * equals the reference's, with null and a blank name both meaning the group
 * without a name, then the ingredient in that group whose trimmed name
 * matches.
 *
 * Taking the first match where the server refuses an ambiguous one is safe
 * because the ambiguity cannot be in the data: `resolveRefs` runs on every
 * write path, before the transaction, and refuses a document in which a
 * reference's group name or ingredient name occurs twice. So a stored
 * reference that resolves here resolves to exactly one row - the database
 * cannot hold the state in which "first" and "only" differ. The recipes this
 * sees are the ones the server handed out, and nothing else reaches
 * `fromRecipe`.
 */
function rowFor(groups: FormGroup[], ref: IngredientRef): string | null {
	const wanted = blankToNull(ref.groupName ?? '');
	const group = groups.find((candidate) => blankToNull(candidate.name) === wanted);
	const row = group?.ingredients.find(
		(candidate) => candidate.name.trim() === ref.ingredientName.trim()
	);
	return row?.id ?? null;
}

/**
 * The group and the row carrying `id`, or null when no row does any more.
 *
 * A row whose name has been cleared counts as gone: `toInput` drops it, or
 * `validate` refuses it, so a link to it has nothing to name. It comes back
 * the moment the row is given a name again, because the id never changed.
 */
function rowById(
	groups: FormGroup[],
	id: string
): { group: FormGroup; row: FormIngredient } | null {
	for (const group of groups) {
		for (const row of group.ingredients) {
			if (row.id === id) {
				return isBlank(row.name) ? null : { group, row };
			}
		}
	}
	return null;
}

/**
 * True for a reference that pointed at a row when the recipe was loaded and
 * whose row has since been deleted, or emptied of its name.
 *
 * This is NOT the same state as a reference that never resolved, and the two
 * must never share a fallback. The names a deleted row's reference remembers
 * are known-stale: delete the group `Vanillesoße` and rename `Grütze` to
 * `Vanillesoße`, and those names now describe a DIFFERENT row. Sending them
 * would have the server resolve the link happily onto 100 g where the author
 * linked 50 g - a wrong quantity in somebody's kitchen, the one failure this
 * feature exists to rule out. `validate` refuses the save instead.
 */
function pointsAtDeletedRow(groups: FormGroup[], ref: FormRef): boolean {
	return ref.ingredientId !== null && rowById(groups, ref.ingredientId) === null;
}

/**
 * The references whose word still stands in `text`.
 *
 * Editing a step never throws a link away: one whose word has been typed over
 * or cut stays on the step, only out of sight, so undoing the edit or pasting
 * the sentence back brings the link back with the word. The editor's history
 * covers the text alone, and a link removed on every keystroke would be gone
 * for good by the time Cmd+Z restored its word. What is out of sight is left
 * out of everything that counts - the chips, `validate` and the payload - so a
 * save never carries a link to a word the API cannot find.
 */
export function presentReferences(refs: FormRef[], text: string): FormRef[] {
	return refs.filter((ref) => wordPattern(ref.word).test(text));
}

/**
 * A reference as the API takes it, or null when it must not be sent at all.
 *
 * Three states, and they are deliberately not one:
 *
 * - The row is still there: its CURRENT group and ingredient names go out, so
 *   a rename carries the link with it.
 * - The reference never found a row when the recipe was loaded
 *   (`ingredientId` is null): the names it arrived with are all there is, so
 *   they go out unchanged. The editor has already drawn it broken and the API
 *   refuses it with a 422 naming it - louder, and more honest, than a save
 *   that quietly drops a link the author can still see.
 * - The row was deleted while the recipe was open: null, because the names it
 *   remembers may now belong to another row (see `pointsAtDeletedRow`).
 *   `validate` stops the save before this is reached, so nothing is lost this
 *   way; the drop is the belt to that braces, and it degrades to "no
 *   reference" rather than to "wrong reference".
 */
function refToInput(groups: FormGroup[], ref: FormRef): IngredientRef | null {
	if (ref.ingredientId === null) {
		return { word: ref.word, groupName: ref.groupName, ingredientName: ref.ingredientName };
	}
	const found = rowById(groups, ref.ingredientId);
	if (found === null) {
		return null;
	}
	return {
		word: ref.word,
		groupName: blankToNull(found.group.name),
		ingredientName: found.row.name.trim()
	};
}

/** The form model for a brand new recipe. */
export function emptyForm(): RecipeForm {
	return fromRecipe(emptyInput());
}

/**
 * Converts the form back into the API payload: strips the client-side ids,
 * drops fully empty ingredient rows and empty steps, trims every string and
 * turns the ones that stay empty into `null`.
 *
 * Assumes `validate` passed - a blank `servings` becomes `0`, which the API
 * rejects, rather than being silently replaced with a made-up default.
 */
export function toInput(form: RecipeForm): RecipeInput {
	const servings = parseNumber(form.servings);
	const groups: IngredientGroup[] = form.ingredientGroups
		.map((group) => ({
			name: blankToNull(group.name),
			ingredients: group.ingredients.filter(isFilledRow).map((row) => ({
				quantity: parseNumber(row.quantity),
				unit: blankToNull(row.unit),
				name: row.name.trim(),
				note: blankToNull(row.note)
			}))
		}))
		// A group with neither a name nor any content is just an empty editor
		// row; a named but empty group is something the user typed, so it stays.
		.filter((group) => group.name !== null || group.ingredients.length > 0);

	return {
		title: form.title.trim(),
		description: form.description.trim(),
		servings: servings === null ? 0 : Math.trunc(servings),
		prepMinutes: minutesOrNull(form.prepMinutes),
		cookMinutes: minutesOrNull(form.cookMinutes),
		sourceUrl: blankToNull(form.sourceUrl),
		tags: form.tags.map((tag) => tag.trim()).filter((tag) => tag.length > 0),
		// The API requires at least one group (`minItems:"1"`).
		ingredientGroups: groups.length > 0 ? groups : [{ name: null, ingredients: [] }],
		steps: form.steps
			.filter((step) => !isBlank(step.text))
			.map((step) => ({
				text: step.text.trim(),
				references: presentReferences(step.references, step.text).flatMap(
					(ref) => refToInput(form.ingredientGroups, ref) ?? []
				)
			}))
	};
}

function minutesOrNull(value: string): number | null {
	const parsed = parseNumber(value);
	return parsed === null ? null : Math.round(parsed);
}

/**
 * Client-side validation, mirroring the constraints the API enforces so the
 * common mistakes never cost a round trip. Returns an empty record when the
 * form is ready to submit.
 */
export function validate(form: RecipeForm): FieldErrors {
	const errors: FieldErrors = {};

	if (isBlank(form.title)) {
		errors.title = m.editor_validation_title_required();
	}

	const servings = parseNumber(form.servings);
	if (servings === null || !Number.isInteger(servings) || servings < 1 || servings > 99) {
		errors.servings = m.editor_validation_servings();
	}

	for (const field of ['prepMinutes', 'cookMinutes'] as const) {
		const message = numberError(form[field]);
		if (message) {
			errors[field] = message;
		}
	}

	if (!isBlank(form.sourceUrl) && !isHttpUrl(form.sourceUrl)) {
		errors.sourceUrl = m.editor_validation_source_url();
	}

	let namedRows = 0;
	for (const group of form.ingredientGroups) {
		for (const row of group.ingredients) {
			if (!isFilledRow(row)) {
				continue;
			}
			const message = numberError(row.quantity);
			if (message) {
				errors[`quantity:${row.id}`] = message;
			}
			if (isBlank(row.name)) {
				errors[`name:${row.id}`] = m.editor_validation_ingredient_name();
			} else {
				namedRows += 1;
			}
		}
	}
	if (namedRows === 0) {
		errors.ingredientGroups = m.editor_validation_ingredients();
	}

	// A link whose row was deleted while the recipe was open is the one case
	// that must stop a save rather than degrade. Its remembered names may since
	// have come to describe another row, so sending them could re-point the
	// link at a different quantity, and dropping it would take the author's
	// work away without a word. The chip is already drawn broken; this says so
	// where the save can see it, keyed to the step so the editor scrolls there,
	// and leaves removing the link to the author.
	//
	// Blank steps are skipped for the same reason `toInput` drops them: they
	// are not submitted, so an error under one would block a save over
	// something nobody is saving.
	for (const step of form.steps) {
		if (isBlank(step.text)) {
			continue;
		}
		const present = presentReferences(step.references, step.text);
		if (present.some((ref) => pointsAtDeletedRow(form.ingredientGroups, ref))) {
			errors[`step:${step.id}`] = m.editor_validation_reference_deleted();
		}
	}

	return errors;
}

/**
 * True for an absolute http(s) URL, mirroring the API's `^https?://`
 * pattern. Anything else - `javascript:` above all - must never be stored,
 * because the detail page renders the source as a link the user can click.
 */
function isHttpUrl(value: string): boolean {
	// `new URL` + try/catch rather than `URL.canParse`, which older browsers
	// lack; a missing API here would throw out of `validate()` and block
	// every save, not just the URL check.
	try {
		const { protocol } = new URL(value.trim());
		return protocol === 'http:' || protocol === 'https:';
	} catch {
		return false;
	}
}

/** `null` when the field is blank or holds a non-negative number. */
function numberError(value: string): string | null {
	if (isBlank(value)) {
		return null;
	}
	const parsed = parseNumber(value);
	if (parsed === null) {
		return m.editor_validation_number();
	}
	return parsed < 0 ? m.editor_validation_number_negative() : null;
}

/**
 * Maps the API's RFC 9457 field errors onto form fields. Huma reports
 * locations like `body.title`, `body.servings` or
 * `body.ingredientGroups[0].ingredients[2].name`; everything below a
 * top-level field collapses onto that field, which is enough to highlight
 * the right section and scroll to it - except a problem inside one step
 * (`body.steps[1]...`), which is keyed `step:<stepId>` the way an ingredient
 * row is keyed `name:<rowId>`, so the editor can point at the step that
 * failed instead of just the section.
 *
 * `steps` is the form's full, unfiltered list, but the server's index counts
 * only the steps `toInput` actually submitted - a blank step in between is
 * never sent and never rejected. Filtered here with the exact predicate
 * `toInput` uses (`!isBlank(step.text)`), so `body.steps[1]` lands on the
 * same step the server saw, not on whatever sits at index 1 of the form.
 */
export function applyServerErrors(errors: FieldError[], steps: FormStep[]): FieldErrors {
	const submitted = steps.filter((step) => !isBlank(step.text));
	const mapped: FieldErrors = {};
	for (const error of errors) {
		const field = serverField(error.location, submitted);
		// First message wins - later ones for the same field would only
		// overwrite the one the user is most likely to act on.
		if (field !== null && !(field in mapped)) {
			mapped[field] = error.message;
		}
	}
	return mapped;
}

function serverField(location: string, steps: FormStep[]): string | null {
	if (!location.startsWith('body.')) {
		return null;
	}
	const rest = location.slice('body.'.length);
	const stepMatch = /^steps\[(\d+)\]/.exec(rest);
	if (stepMatch) {
		const step = steps[Number(stepMatch[1])];
		return step ? `step:${step.id}` : 'steps';
	}
	const head = rest.split(/[.[]/, 1)[0];
	return FIELD_ORDER.includes(head) ? head : null;
}

/**
 * The error to scroll to: the one belonging to the field that comes first in
 * the form. Row-level errors rank with the ingredients section, and ties keep
 * the order `validate` inserted them in (top to bottom).
 */
export function firstErrorField(errors: FieldErrors): string | null {
	let best: string | null = null;
	let bestRank = Number.POSITIVE_INFINITY;
	for (const field of Object.keys(errors)) {
		const rank = fieldRank(field);
		if (rank < bestRank) {
			bestRank = rank;
			best = field;
		}
	}
	return best;
}

function fieldRank(field: string): number {
	const index = FIELD_ORDER.indexOf(field);
	if (index >= 0) {
		return index;
	}
	if (field.startsWith('quantity:') || field.startsWith('name:')) {
		return FIELD_ORDER.indexOf('ingredientGroups');
	}
	if (field.startsWith('step:')) {
		return FIELD_ORDER.indexOf('steps');
	}
	return FIELD_ORDER.length;
}

/** The id of the element to focus/scroll to for a given error field. */
export function anchorId(field: string): string {
	if (field.startsWith('quantity:')) {
		return `ingredient-quantity-${field.slice('quantity:'.length)}`;
	}
	if (field.startsWith('name:')) {
		return `ingredient-name-${field.slice('name:'.length)}`;
	}
	if (field.startsWith('step:')) {
		return `step-${field.slice('step:'.length)}`;
	}
	if (field === 'ingredientGroups') {
		return 'editor-section-ingredients';
	}
	if (field === 'steps') {
		return 'editor-section-steps';
	}
	return `editor-${field}`;
}

/** Normalizes a typed tag: trimmed and lower-cased, so `Dessert` and
 * ` dessert ` are the same tag. */
export function normaliseTag(tag: string): string {
	return tag.trim().toLowerCase();
}

/** A deep copy, used to snapshot the pristine form for the dirty check. */
export function cloneForm(form: RecipeForm): RecipeForm {
	return JSON.parse(JSON.stringify(form)) as RecipeForm;
}

/** True when the form differs from the snapshot it started out as. */
export function isDirty(form: RecipeForm, initial: RecipeForm): boolean {
	return JSON.stringify(form) !== JSON.stringify(initial);
}
