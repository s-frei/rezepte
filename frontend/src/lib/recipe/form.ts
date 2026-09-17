import type { FieldError } from '$lib/api/client';
import { emptyInput, type IngredientGroup, type RecipeInput } from '$lib/api/recipes';
import { m } from '$lib/paraglide/messages';

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

export type FormStep = {
	id: string;
	text: string;
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
	return { id: newId(), text: '' };
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
	const normalised = value.trim().replace(',', '.');
	if (normalised === '') {
		return null;
	}
	const parsed = Number(normalised);
	return Number.isFinite(parsed) ? parsed : null;
}

/** Renders a stored quantity back into editable text (`1.5` -> `1,5`). */
function quantityToText(quantity: number | null): string {
	return quantity === null ? '' : String(quantity).replace('.', ',');
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
	const steps = recipe.steps.map((text) => ({ id: newId(), text }));

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
		steps: form.steps.map((step) => step.text.trim()).filter((text) => text.length > 0)
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
 * the right section and scroll to it.
 */
export function applyServerErrors(errors: FieldError[]): FieldErrors {
	const mapped: FieldErrors = {};
	for (const error of errors) {
		const field = serverField(error.location);
		// First message wins - later ones for the same field would only
		// overwrite the one the user is most likely to act on.
		if (field !== null && !(field in mapped)) {
			mapped[field] = error.message;
		}
	}
	return mapped;
}

function serverField(location: string): string | null {
	if (!location.startsWith('body.')) {
		return null;
	}
	const head = location.slice('body.'.length).split(/[.[]/, 1)[0];
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
	if (field === 'ingredientGroups') {
		return 'editor-section-ingredients';
	}
	if (field === 'steps') {
		return 'editor-section-steps';
	}
	return `editor-${field}`;
}

/** Normalises a typed tag: trimmed and lower-cased, so `Dessert` and
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
