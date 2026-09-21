import { describe, expect, it } from 'vitest';
import type { Recipe, RecipeInput } from '$lib/api/recipes';
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
	toInput,
	validate,
	type RecipeForm
} from './form';

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
		steps: ['Mehl mischen.', 'Backen.'],
		...overrides
	};
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

	it('renders a fractional quantity with a comma', () => {
		const form = fromRecipe(baseInput());
		expect(form.ingredientGroups[0].ingredients[1].quantity).toBe('1,5');
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
		const recipe: Recipe = {
			...baseInput(),
			id: 'r1',
			slug: 'zitronen-tarte',
			coverImageId: null,
			images: [],
			createdBy: 'u1',
			createdAt: '2026-09-17T10:00:00Z',
			updatedBy: 'u1',
			updatedAt: '2026-09-17T10:00:00Z',
			createdByName: 'sam',
			updatedByName: 'sam',
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
		form.steps.push({ id: 'x', text: '   ' });
		expect(toInput(form).steps).toEqual(['Mehl mischen.', 'Backen.']);
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

describe('validate', () => {
	it('accepts a complete form', () => {
		expect(validate(validForm())).toEqual({});
	});

	it('requires a title', () => {
		expect(validate(validForm({ title: '   ' })).title).toBe('Bitte einen Titel eingeben');
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
		expect(validate(validForm({ cookMinutes: 'abc' })).cookMinutes).toBe(
			'Bitte eine Zahl eingeben'
		);
	});

	it('rejects negative minutes', () => {
		expect(validate(validForm({ prepMinutes: '-5' })).prepMinutes).toBe(
			'Bitte keine negative Zahl eingeben'
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
		const message = 'Bitte eine Adresse mit http:// oder https:// angeben';
		expect(validate(validForm({ sourceUrl: 'javascript:alert(1)' })).sourceUrl).toBe(message);
		expect(validate(validForm({ sourceUrl: 'data:text/html,<b>x' })).sourceUrl).toBe(message);
		expect(validate(validForm({ sourceUrl: 'example.test/rezept' })).sourceUrl).toBe(message);
		expect(validate(validForm({ sourceUrl: 'ftp://example.test' })).sourceUrl).toBe(message);
	});

	it('rejects a negative quantity, keyed by the row id', () => {
		const form = validForm();
		const row = form.ingredientGroups[0].ingredients[0];
		row.quantity = '-1';
		expect(validate(form)[`quantity:${row.id}`]).toBe('Bitte keine negative Zahl eingeben');
	});

	it('requires a name on a row that has other content', () => {
		const form = validForm();
		const row = newIngredient();
		row.quantity = '2';
		form.ingredientGroups[0].ingredients.push(row);
		expect(validate(form)[`name:${row.id}`]).toBe('Bitte einen Namen eingeben');
	});

	it('ignores fully empty rows', () => {
		const form = validForm();
		form.ingredientGroups[0].ingredients.push(newIngredient());
		expect(validate(form)).toEqual({});
	});

	it('requires at least one named ingredient', () => {
		const form = validForm({ ingredientGroups: [newGroup()] });
		expect(validate(form).ingredientGroups).toBe('Bitte mindestens eine Zutat mit Namen angeben');
	});
});

describe('applyServerErrors', () => {
	it('maps a top-level location onto its form field', () => {
		expect(
			applyServerErrors([{ location: 'body.title', message: 'expected minLength 1' }])
		).toEqual({ title: 'expected minLength 1' });
	});

	it('collapses a nested location onto its top-level field', () => {
		expect(
			applyServerErrors([
				{ location: 'body.ingredientGroups[0].ingredients[2].name', message: 'too short' }
			])
		).toEqual({ ingredientGroups: 'too short' });
	});

	it('collapses an indexed list location onto its field', () => {
		expect(applyServerErrors([{ location: 'body.steps[1]', message: 'too long' }])).toEqual({
			steps: 'too long'
		});
	});

	it('keeps the first message per field', () => {
		expect(
			applyServerErrors([
				{ location: 'body.title', message: 'first' },
				{ location: 'body.title', message: 'second' }
			])
		).toEqual({ title: 'first' });
	});

	it('ignores locations outside the body and unknown fields', () => {
		expect(
			applyServerErrors([
				{ location: 'path.id', message: 'nope' },
				{ location: 'body.somethingElse', message: 'nope' },
				{ location: '', message: 'nope' }
			])
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
