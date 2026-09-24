import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { Person, Recipe, RecipeInput } from '$lib/api/recipes';
import {
	anchorId,
	applyServerErrors,
	cloneForm,
	emptyForm,
	firstErrorField,
	fromRecipe,
	isBlank,
	isDirty,
	newGroup,
	newId,
	newIngredient,
	newStep,
	normaliseTag,
	parseNumber,
	presentReferences,
	toInput,
	validate,
	type RecipeForm
} from './form';

// Most of this file doesn't care about the locale, so it stays at the
// default 'en' throughout (reset before every test below); only the
// quantity-formatting tests further down switch it, the same mock shape as
// format.test.ts uses.
let locale: 'en' | 'de' = 'en';
vi.mock('$lib/paraglide/runtime', () => ({
	getLocale: () => locale,
	experimentalStaticLocale: undefined
}));

beforeEach(() => {
	locale = 'en';
});

function baseInput(overrides: Partial<RecipeInput> = {}): RecipeInput {
	return {
		title: 'Zitronen-Tarte',
		description: 'Frisch und sauer.',
		servings: 8,
		prepMinutes: 30,
		cookMinutes: 45,
		sourceUrl: 'https://example.test/tarte',
		tags: ['dessert', 'backen'],
		ingredientGroups: [
			{
				name: 'Teig',
				ingredients: [
					{ quantity: 250, unit: 'g', name: 'Mehl', note: 'gesiebt' },
					{ quantity: 1.5, unit: 'TL', name: 'Salz', note: null }
				]
			}
		],
		steps: [
			{ text: 'Mehl mischen.', references: [] },
			{ text: 'Backen.', references: [] }
		],
		...overrides
	};
}

/** A recipe whose one step links the word "Mehl" to the row in "Teig". */
function linkedInput(): RecipeInput {
	return baseInput({
		steps: [
			{
				text: 'Mehl mischen.',
				references: [{ word: 'Mehl', groupName: 'Teig', ingredientName: 'Mehl' }]
			}
		]
	});
}

/**
 * Rote Grütze: one ingredient name in two groups, the recipe the whole feature
 * was designed around. Its step links the word to the 50 g row in the custard.
 */
function ambiguousInput(): RecipeInput {
	return baseInput({
		ingredientGroups: [
			{ name: 'Grütze', ingredients: [{ quantity: 100, unit: 'g', name: 'Zucker', note: null }] },
			{
				name: 'Vanillesoße',
				ingredients: [{ quantity: 50, unit: 'g', name: 'Zucker', note: null }]
			}
		],
		steps: [
			{
				text: 'Zucker unterrühren.',
				references: [{ word: 'Zucker', groupName: 'Vanillesoße', ingredientName: 'Zucker' }]
			}
		]
	});
}

/** A valid form, so a test only has to state the one thing it changes. */
function validForm(overrides: Partial<RecipeForm> = {}): RecipeForm {
	return { ...fromRecipe(baseInput()), ...overrides };
}

describe('newId', () => {
	it('never repeats an id', () => {
		const ids = [newId(), newId(), newGroup().id, newIngredient().id, newStep().id];
		expect(new Set(ids).size).toBe(ids.length);
	});
});

describe('isBlank', () => {
	it('treats empty and whitespace-only strings as blank', () => {
		expect(isBlank('')).toBe(true);
		expect(isBlank('   ')).toBe(true);
		expect(isBlank('\t\n')).toBe(true);
	});

	it('treats any other content as filled', () => {
		expect(isBlank('0')).toBe(false);
		expect(isBlank(' a ')).toBe(false);
	});
});

describe('parseNumber', () => {
	it('parses a decimal point', () => {
		expect(parseNumber('1.5')).toBe(1.5);
	});

	it('parses a German decimal comma', () => {
		expect(parseNumber('1,5')).toBe(1.5);
	});

	it('ignores surrounding whitespace', () => {
		expect(parseNumber('  42  ')).toBe(42);
	});

	it('returns null for blank input', () => {
		expect(parseNumber('')).toBeNull();
		expect(parseNumber('   ')).toBeNull();
	});

	it('returns null for anything that is not a number', () => {
		expect(parseNumber('abc')).toBeNull();
		expect(parseNumber('1,5,5')).toBeNull();
		expect(parseNumber('--1')).toBeNull();
	});

	it('keeps the sign so validation can reject negatives', () => {
		expect(parseNumber('-3')).toBe(-3);
	});

	it('survives the number and null a number input can bind back', () => {
		expect(parseNumber(4)).toBe(4);
		expect(parseNumber(1.5)).toBe(1.5);
		expect(parseNumber(Number.NaN)).toBeNull();
		expect(parseNumber(null)).toBeNull();
	});
});

describe('normaliseTag', () => {
	it('trims and lower-cases', () => {
		expect(normaliseTag('  Dessert ')).toBe('dessert');
		expect(normaliseTag('BACKEN')).toBe('backen');
	});
});

describe('fromRecipe', () => {
	it('keeps the scalar fields, turning numbers into editable text', () => {
		const form = fromRecipe(baseInput());
		expect(form.title).toBe('Zitronen-Tarte');
		expect(form.description).toBe('Frisch und sauer.');
		expect(form.servings).toBe('8');
		expect(form.prepMinutes).toBe('30');
		expect(form.cookMinutes).toBe('45');
		expect(form.sourceUrl).toBe('https://example.test/tarte');
		expect(form.tags).toEqual(['dessert', 'backen']);
	});

	it('renders null scalars as empty strings', () => {
		const form = fromRecipe(baseInput({ prepMinutes: null, cookMinutes: null, sourceUrl: null }));
		expect(form.prepMinutes).toBe('');
		expect(form.cookMinutes).toBe('');
		expect(form.sourceUrl).toBe('');
	});

	it('renders a fractional quantity with a decimal point in English', () => {
		const form = fromRecipe(baseInput());
		expect(form.ingredientGroups[0].ingredients[1].quantity).toBe('1.5');
	});

	it('renders a fractional quantity with a decimal comma in German', () => {
		locale = 'de';
		const form = fromRecipe(baseInput());
		expect(form.ingredientGroups[0].ingredients[1].quantity).toBe('1,5');
	});

	it("renders the stored quantity exactly, without formatQuantity's two-decimal rounding", () => {
		const precise = baseInput({
			ingredientGroups: [
				{
					name: 'Teig',
					ingredients: [{ quantity: 1.333, unit: 'TL', name: 'Salz', note: null }]
				}
			]
		});
		expect(fromRecipe(precise).ingredientGroups[0].ingredients[0].quantity).toBe('1.333');
		locale = 'de';
		expect(fromRecipe(precise).ingredientGroups[0].ingredients[0].quantity).toBe('1,333');
	});

	it('renders null ingredient fields as empty strings', () => {
		const form = fromRecipe(baseInput());
		expect(form.ingredientGroups[0].ingredients[1].note).toBe('');
	});

	it('gives every group, row and step a unique id', () => {
		const form = fromRecipe(baseInput());
		const ids = [
			...form.ingredientGroups.map((group) => group.id),
			...form.ingredientGroups.flatMap((group) => group.ingredients.map((row) => row.id)),
			...form.steps.map((step) => step.id)
		];
		expect(new Set(ids).size).toBe(ids.length);
	});

	it('pads empty collections so there is always something to type into', () => {
		const form = fromRecipe(
			baseInput({
				ingredientGroups: [{ name: null, ingredients: [] }],
				steps: []
			})
		);
		expect(form.ingredientGroups[0].ingredients).toHaveLength(1);
		expect(form.ingredientGroups[0].ingredients[0].name).toBe('');
		expect(form.steps).toHaveLength(1);
		expect(form.steps[0].text).toBe('');
	});

	it('accepts a full Recipe, not just a RecipeInput', () => {
		const sam: Person = { id: 'u1', username: 'sam', displayName: 'Sam', color: 'amber' };
		const recipe: Recipe = {
			...baseInput(),
			id: 'r1',
			slug: 'zitronen-tarte',
			coverImageId: null,
			images: [],
			createdAt: '2026-09-17T10:00:00Z',
			createdBy: sam,
			updatedAt: '2026-09-17T10:00:00Z',
			updatedBy: sam,
			favourite: false
		};
		expect(fromRecipe(recipe).title).toBe('Zitronen-Tarte');
	});
});

describe('emptyForm', () => {
	it('starts with one blank row, one blank step and no tags', () => {
		const form = emptyForm();
		expect(form.title).toBe('');
		expect(form.servings).toBe('4');
		expect(form.tags).toEqual([]);
		expect(form.ingredientGroups).toHaveLength(1);
		expect(form.ingredientGroups[0].ingredients).toHaveLength(1);
		expect(form.steps).toHaveLength(1);
	});
});

describe('toInput', () => {
	it('round-trips a recipe unchanged', () => {
		const input = baseInput();
		expect(toInput(fromRecipe(input))).toEqual(input);
	});

	it('strips the client-side ids', () => {
		const result = toInput(validForm());
		expect(JSON.stringify(result)).not.toContain('"id"');
	});

	it('trims every string', () => {
		const form = validForm({ title: '  Tarte  ', description: '  kurz  ' });
		form.ingredientGroups[0].name = '  Teig  ';
		form.ingredientGroups[0].ingredients[0].name = '  Mehl  ';
		const result = toInput(form);
		expect(result.title).toBe('Tarte');
		expect(result.description).toBe('kurz');
		expect(result.ingredientGroups[0].name).toBe('Teig');
		expect(result.ingredientGroups[0].ingredients[0].name).toBe('Mehl');
	});

	it('converts blank optional strings to null', () => {
		const form = validForm({ sourceUrl: '   ' });
		form.ingredientGroups[0].name = '';
		form.ingredientGroups[0].ingredients[0].unit = '';
		form.ingredientGroups[0].ingredients[0].note = '  ';
		const result = toInput(form);
		expect(result.sourceUrl).toBeNull();
		expect(result.ingredientGroups[0].name).toBeNull();
		expect(result.ingredientGroups[0].ingredients[0].unit).toBeNull();
		expect(result.ingredientGroups[0].ingredients[0].note).toBeNull();
	});

	it('parses a comma quantity', () => {
		const form = validForm();
		form.ingredientGroups[0].ingredients[0].quantity = '2,25';
		expect(toInput(form).ingredientGroups[0].ingredients[0].quantity).toBe(2.25);
	});

	it('leaves a blank quantity as null', () => {
		const form = validForm();
		form.ingredientGroups[0].ingredients[0].quantity = '';
		expect(toInput(form).ingredientGroups[0].ingredients[0].quantity).toBeNull();
	});

	it('rounds minutes to whole numbers', () => {
		const result = toInput(validForm({ prepMinutes: '30,4', cookMinutes: '45,6' }));
		expect(result.prepMinutes).toBe(30);
		expect(result.cookMinutes).toBe(46);
	});

	it('drops fully empty ingredient rows', () => {
		const form = validForm();
		form.ingredientGroups[0].ingredients.push(newIngredient());
		expect(toInput(form).ingredientGroups[0].ingredients).toHaveLength(2);
	});

	it('keeps a row that only has a note', () => {
		const form = validForm();
		const row = newIngredient();
		row.note = 'nach Gefühl';
		form.ingredientGroups[0].ingredients.push(row);
		const rows = toInput(form).ingredientGroups[0].ingredients;
		expect(rows).toHaveLength(3);
		expect(rows[2]).toEqual({ quantity: null, unit: null, name: '', note: 'nach Gefühl' });
	});

	it('drops groups that are neither named nor filled', () => {
		const form = validForm();
		form.ingredientGroups.push(newGroup());
		expect(toInput(form).ingredientGroups).toHaveLength(1);
	});

	it('keeps a named group even when it has no rows left', () => {
		const form = validForm();
		const group = newGroup();
		group.name = 'Füllung';
		form.ingredientGroups.push(group);
		const groups = toInput(form).ingredientGroups;
		expect(groups).toHaveLength(2);
		expect(groups[1]).toEqual({ name: 'Füllung', ingredients: [] });
	});

	it('always emits at least one group, as the API requires', () => {
		const form = validForm({ ingredientGroups: [newGroup()] });
		expect(toInput(form).ingredientGroups).toEqual([{ name: null, ingredients: [] }]);
	});

	it('drops empty steps', () => {
		const form = validForm();
		form.steps.push(newStep());
		form.steps.push({ id: 'x', text: '   ', references: [] });
		expect(toInput(form).steps).toEqual([
			{ text: 'Mehl mischen.', references: [] },
			{ text: 'Backen.', references: [] }
		]);
	});

	it('round-trips a recipe carrying step references', () => {
		const input = linkedInput();
		expect(toInput(fromRecipe(input)).steps).toEqual(input.steps);
	});

	it('drops blank tags', () => {
		expect(toInput(validForm({ tags: ['dessert', '  ', 'backen'] })).tags).toEqual([
			'dessert',
			'backen'
		]);
	});

	it('truncates servings and falls back to 0 when blank', () => {
		expect(toInput(validForm({ servings: '8,9' })).servings).toBe(8);
		expect(toInput(validForm({ servings: '' })).servings).toBe(0);
	});
});

/**
 * A reference in the form is anchored to the ingredient ROW, so a rename moves
 * the link with it instead of breaking it. These are the cases that broke
 * before: every one of them used to send the names the reference was made with
 * and earn a 422 from the API, with every link in the recipe drawn as broken.
 */
describe('toInput with renamed ingredients', () => {
	function refs(form: RecipeForm) {
		return toInput(form).steps[0].references;
	}

	it('sends the group’s new name after the group is renamed', () => {
		const form = fromRecipe(linkedInput());
		form.ingredientGroups[0].name = 'Boden';
		expect(refs(form)).toEqual([{ word: 'Mehl', groupName: 'Boden', ingredientName: 'Mehl' }]);
	});

	it('sends the group’s name once the unnamed group is given one', () => {
		// The reported bug, exactly: one group, no name, links into it. Naming
		// it used to make every link in the recipe unresolvable.
		const form = fromRecipe(
			baseInput({
				ingredientGroups: [
					{ name: null, ingredients: [{ quantity: 250, unit: 'g', name: 'Mehl', note: null }] }
				],
				steps: [
					{
						text: 'Mehl mischen.',
						references: [{ word: 'Mehl', groupName: null, ingredientName: 'Mehl' }]
					}
				]
			})
		);
		form.ingredientGroups[0].name = 'Teig';
		expect(refs(form)).toEqual([{ word: 'Mehl', groupName: 'Teig', ingredientName: 'Mehl' }]);
	});

	it('sends the row’s new name after the ingredient is renamed', () => {
		const form = fromRecipe(linkedInput());
		form.ingredientGroups[0].ingredients[0].name = 'Dinkelmehl';
		// The word is the author’s sentence and does not move; only the name of
		// the row it points at does.
		expect(refs(form)).toEqual([{ word: 'Mehl', groupName: 'Teig', ingredientName: 'Dinkelmehl' }]);
	});

	it('follows a row moved into another group', () => {
		const form = fromRecipe(linkedInput());
		const [row] = form.ingredientGroups[0].ingredients.splice(0, 1);
		form.ingredientGroups.push({ id: 'g-belag', name: 'Belag', ingredients: [row] });
		expect(refs(form)).toEqual([{ word: 'Mehl', groupName: 'Belag', ingredientName: 'Mehl' }]);
	});

	it('sends a reference naming a row the recipe no longer has back unchanged', () => {
		// Nothing to anchor to, so the names it arrived with are what goes out:
		// the API answers with a 422 naming the reference and the editor has
		// already drawn it as broken. Dropping it instead would lose a link the
		// author can still see and fix.
		const input = baseInput({
			steps: [
				{
					text: 'Zucker aufkochen.',
					references: [{ word: 'Zucker', groupName: 'Grütze', ingredientName: 'Zucker' }]
				}
			]
		});
		const form = fromRecipe(input);
		expect(form.steps[0].references[0].ingredientId).toBeNull();
		expect(refs(form)).toEqual(input.steps[0].references);
	});
});

/**
 * A link whose row is deleted while the recipe is open is its own state, and
 * the one the feature must never get wrong. The names it remembers may since
 * have come to describe a DIFFERENT row, so sending them would resolve the
 * link onto a quantity the author never chose - and a wrong quantity at the
 * stove is the single outcome this whole feature exists to rule out.
 */
describe('presentReferences', () => {
	const refs = [{ word: 'Mehl', ingredientId: 'r1', groupName: 'Teig', ingredientName: 'Mehl' }];

	it('keeps a reference whose word is still there', () => {
		expect(presentReferences(refs, 'Das Mehl sieben.')).toEqual(refs);
	});

	it('leaves one out once the word is typed over', () => {
		expect(presentReferences(refs, 'Den Zucker sieben.')).toEqual([]);
	});

	it('does not count the word inside a longer one', () => {
		expect(presentReferences(refs, 'Mehlschwitze anrühren.')).toEqual([]);
	});
});

/**
 * Editing a step keeps a link whose word it removed, so undo can bring the
 * link back with the word. Only the save leaves it out: the API refuses a
 * reference to a word the text does not hold.
 */
describe('a reference whose word has been edited away', () => {
	it('stays on the form while the word is gone', () => {
		const form = fromRecipe(linkedInput());
		form.steps[0].text = 'Alles mischen.';
		expect(form.steps[0].references).toHaveLength(1);
		expect(toInput(form).steps[0].references).toEqual([]);
	});

	it('is sent again once the word is back', () => {
		const form = fromRecipe(linkedInput());
		form.steps[0].text = 'Alles mischen.';
		form.steps[0].text = 'Mehl mischen.';
		expect(toInput(form)).toEqual(linkedInput());
	});

	it('does not block a save when its row was deleted as well', () => {
		// Nothing of the link is left on screen, so there is nothing for the
		// author to take off; it is simply not sent.
		const form = fromRecipe(linkedInput());
		form.ingredientGroups[0].ingredients.splice(0, 1);
		form.steps[0].text = 'Salz mischen.';
		expect(validate(form)[`step:${form.steps[0].id}`]).toBeUndefined();
		expect(toInput(form).steps[0].references).toEqual([]);
	});
});

describe('a reference whose row is deleted while the recipe is open', () => {
	/** Deletes the custard and gives its name to the compote. */
	function stealTheName(form: RecipeForm): RecipeForm {
		form.ingredientGroups.splice(1, 1);
		form.ingredientGroups[0].name = 'Vanillesoße';
		return form;
	}

	it('never sends the names it remembers', () => {
		// Those names now describe the 100 g row. Emitting them would have the
		// server resolve the link happily onto a quantity twice the one the
		// author picked, with nothing anywhere saying so.
		const form = stealTheName(fromRecipe(ambiguousInput()));
		expect(toInput(form).steps[0].references).toEqual([]);
	});

	it('is reported by validate against the step it sits in', () => {
		const form = stealTheName(fromRecipe(ambiguousInput()));
		const errors = validate(form);
		// Keyed to the step, so the editor scrolls to it and the author can
		// take the link off deliberately. `toInput` dropping it is only the
		// second line: no save gets that far while this stands.
		expect(errors[`step:${form.steps[0].id}`]).toBe('This link points at a deleted ingredient');
		expect(firstErrorField(errors)).toBe(`step:${form.steps[0].id}`);
		expect(anchorId(`step:${form.steps[0].id}`)).toBe(`step-${form.steps[0].id}`);
	});

	it('is reported when only the row is deleted', () => {
		const form = fromRecipe(ambiguousInput());
		form.ingredientGroups[1].ingredients = [];
		expect(validate(form)[`step:${form.steps[0].id}`]).toBeDefined();
	});

	it('is reported when the row is emptied rather than deleted', () => {
		// The row is still in the form, but `toInput` drops it; sending the
		// link would name an ingredient of "".
		const form = fromRecipe(ambiguousInput());
		const row = form.ingredientGroups[1].ingredients[0];
		Object.assign(row, { quantity: '', unit: '', name: '', note: '' });
		expect(validate(form)[`step:${form.steps[0].id}`]).toBeDefined();
		expect(toInput(form).steps[0].references).toEqual([]);
	});

	it('follows the row again once its name is typed back in', () => {
		const form = fromRecipe(ambiguousInput());
		const row = form.ingredientGroups[1].ingredients[0];
		row.name = '';
		expect(validate(form)[`step:${form.steps[0].id}`]).toBeDefined();
		row.name = 'Rohrzucker';
		expect(validate(form)[`step:${form.steps[0].id}`]).toBeUndefined();
		expect(toInput(form).steps[0].references[0].ingredientName).toBe('Rohrzucker');
	});

	it('leaves a reference that never found a row to the API', () => {
		// The other unresolved state, and it keeps the loud path: nothing here
		// can tell a stale name from a name that was always wrong, so the
		// names go out and the server answers with a 422 naming the reference.
		const input = baseInput({
			steps: [
				{
					text: 'Zucker aufkochen.',
					references: [{ word: 'Zucker', groupName: 'Grütze', ingredientName: 'Zucker' }]
				}
			]
		});
		const form = fromRecipe(input);
		expect(validate(form)[`step:${form.steps[0].id}`]).toBeUndefined();
		expect(toInput(form).steps[0].references).toEqual(input.steps[0].references);
	});

	it('does not report one inside a blank step, which is not submitted', () => {
		const form = stealTheName(fromRecipe(ambiguousInput()));
		form.steps[0].text = '   ';
		expect(validate(form)[`step:${form.steps[0].id}`]).toBeUndefined();
	});
});

describe('validate', () => {
	it('accepts a complete form', () => {
		expect(validate(validForm())).toEqual({});
	});

	it('requires a title', () => {
		expect(validate(validForm({ title: '   ' })).title).toBe('Please enter a title');
	});

	it('rejects servings outside 1-99', () => {
		expect(validate(validForm({ servings: '0' }))).toHaveProperty('servings');
		expect(validate(validForm({ servings: '100' }))).toHaveProperty('servings');
		expect(validate(validForm({ servings: '' }))).toHaveProperty('servings');
		expect(validate(validForm({ servings: 'acht' }))).toHaveProperty('servings');
		expect(validate(validForm({ servings: '2,5' }))).toHaveProperty('servings');
	});

	it('accepts servings at both ends of the range', () => {
		expect(validate(validForm({ servings: '1' }))).toEqual({});
		expect(validate(validForm({ servings: '99' }))).toEqual({});
	});

	it('rejects non-numeric minutes', () => {
		expect(validate(validForm({ cookMinutes: 'abc' })).cookMinutes).toBe('Please enter a number');
	});

	it('rejects negative minutes', () => {
		expect(validate(validForm({ prepMinutes: '-5' })).prepMinutes).toBe(
			'Please do not enter a negative number'
		);
	});

	it('accepts blank minutes', () => {
		expect(validate(validForm({ prepMinutes: '', cookMinutes: '' }))).toEqual({});
	});

	it('accepts a blank source URL', () => {
		expect(validate(validForm({ sourceUrl: '  ' }))).toEqual({});
	});

	it('accepts http and https source URLs', () => {
		expect(validate(validForm({ sourceUrl: 'http://example.test/rezept' }))).toEqual({});
		expect(validate(validForm({ sourceUrl: ' https://example.test/rezept ' }))).toEqual({});
	});

	it('rejects a source URL that is not http(s)', () => {
		const message = 'Please enter an address starting with http:// or https://';
		expect(validate(validForm({ sourceUrl: 'javascript:alert(1)' })).sourceUrl).toBe(message);
		expect(validate(validForm({ sourceUrl: 'data:text/html,<b>x' })).sourceUrl).toBe(message);
		expect(validate(validForm({ sourceUrl: 'example.test/rezept' })).sourceUrl).toBe(message);
		expect(validate(validForm({ sourceUrl: 'ftp://example.test' })).sourceUrl).toBe(message);
	});

	it('rejects a negative quantity, keyed by the row id', () => {
		const form = validForm();
		const row = form.ingredientGroups[0].ingredients[0];
		row.quantity = '-1';
		expect(validate(form)[`quantity:${row.id}`]).toBe('Please do not enter a negative number');
	});

	it('requires a name on a row that has other content', () => {
		const form = validForm();
		const row = newIngredient();
		row.quantity = '2';
		form.ingredientGroups[0].ingredients.push(row);
		expect(validate(form)[`name:${row.id}`]).toBe('Please enter a name');
	});

	it('ignores fully empty rows', () => {
		const form = validForm();
		form.ingredientGroups[0].ingredients.push(newIngredient());
		expect(validate(form)).toEqual({});
	});

	it('requires at least one named ingredient', () => {
		const form = validForm({ ingredientGroups: [newGroup()] });
		expect(validate(form).ingredientGroups).toBe('Please add at least one ingredient with a name');
	});
});

describe('applyServerErrors', () => {
	it('maps a top-level location onto its form field', () => {
		expect(
			applyServerErrors([{ location: 'body.title', message: 'expected minLength 1' }], [])
		).toEqual({ title: 'expected minLength 1' });
	});

	it('collapses a nested location onto its top-level field', () => {
		expect(
			applyServerErrors(
				[{ location: 'body.ingredientGroups[0].ingredients[2].name', message: 'too short' }],
				[]
			)
		).toEqual({ ingredientGroups: 'too short' });
	});

	it('keys a problem inside one step by that step’s id, not the whole section', () => {
		const steps = validForm().steps;
		expect(
			applyServerErrors([{ location: 'body.steps[1].text', message: 'too long' }], steps)
		).toEqual({ [`step:${steps[1].id}`]: 'too long' });
	});

	it('indexes against the steps the server actually saw, skipping blanks', () => {
		// A blank step in the form shifts every index after it - toInput drops
		// it, so the server's steps[1] is the form's third step, not its second.
		const form = validForm();
		form.steps.unshift(newStep());
		const [, first, second] = form.steps;
		expect(
			applyServerErrors([{ location: 'body.steps[1].text', message: 'too long' }], form.steps)
		).toEqual({ [`step:${second.id}`]: 'too long' });
		expect(anchorId(`step:${first.id}`)).not.toBe(anchorId(`step:${second.id}`));
	});

	it('falls back to the section when the step index no longer exists', () => {
		expect(
			applyServerErrors(
				[{ location: 'body.steps[5].text', message: 'too long' }],
				validForm().steps
			)
		).toEqual({ steps: 'too long' });
	});

	it('collapses a bare steps location onto the section', () => {
		expect(applyServerErrors([{ location: 'body.steps', message: 'too many' }], [])).toEqual({
			steps: 'too many'
		});
	});

	it('keeps the first message per field', () => {
		expect(
			applyServerErrors(
				[
					{ location: 'body.title', message: 'first' },
					{ location: 'body.title', message: 'second' }
				],
				[]
			)
		).toEqual({ title: 'first' });
	});

	it('ignores locations outside the body and unknown fields', () => {
		expect(
			applyServerErrors(
				[
					{ location: 'path.id', message: 'nope' },
					{ location: 'body.somethingElse', message: 'nope' },
					{ location: '', message: 'nope' }
				],
				[]
			)
		).toEqual({});
	});
});

describe('firstErrorField', () => {
	it('returns null when there is nothing wrong', () => {
		expect(firstErrorField({})).toBeNull();
	});

	it('picks the field that comes first in the form', () => {
		expect(firstErrorField({ steps: 'a', title: 'b', servings: 'c' })).toBe('title');
	});

	it('ranks row-level errors with the ingredients section', () => {
		expect(firstErrorField({ steps: 'a', 'quantity:f7': 'b' })).toBe('quantity:f7');
		expect(firstErrorField({ 'name:f7': 'a', servings: 'b' })).toBe('servings');
	});

	it('ranks a step-level error with the steps section', () => {
		expect(firstErrorField({ 'step:f7': 'a', tags: 'b' })).toBe('tags');
		expect(firstErrorField({ title: 'a', 'step:f7': 'b' })).toBe('title');
	});

	it('keeps insertion order among equally ranked fields', () => {
		expect(firstErrorField({ 'name:f2': 'a', 'quantity:f9': 'b' })).toBe('name:f2');
	});
});

describe('anchorId', () => {
	it('maps scalar fields to their input', () => {
		expect(anchorId('title')).toBe('editor-title');
		expect(anchorId('prepMinutes')).toBe('editor-prepMinutes');
	});

	it('maps list fields to their section', () => {
		expect(anchorId('ingredientGroups')).toBe('editor-section-ingredients');
		expect(anchorId('steps')).toBe('editor-section-steps');
	});

	it('maps row-level fields to the row input', () => {
		expect(anchorId('quantity:f7')).toBe('ingredient-quantity-f7');
		expect(anchorId('name:f7')).toBe('ingredient-name-f7');
	});

	it('maps a step-level field to that step’s textarea', () => {
		expect(anchorId('step:f7')).toBe('step-f7');
	});
});

describe('isDirty', () => {
	it('is false for an untouched copy', () => {
		const form = validForm();
		expect(isDirty(form, cloneForm(form))).toBe(false);
	});

	it('notices an edited field', () => {
		const initial = validForm();
		const form = cloneForm(initial);
		form.title = 'Anders';
		expect(isDirty(form, initial)).toBe(true);
	});

	it('notices a reordered list', () => {
		const initial = validForm();
		const form = cloneForm(initial);
		form.steps.reverse();
		expect(isDirty(form, initial)).toBe(true);
	});

	it('notices an added blank row', () => {
		const initial = validForm();
		const form = cloneForm(initial);
		form.ingredientGroups[0].ingredients.push(newIngredient());
		expect(isDirty(form, initial)).toBe(true);
	});
});
