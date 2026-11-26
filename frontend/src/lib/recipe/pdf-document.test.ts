import { describe, expect, it } from 'vitest';
import type { Recipe } from '../api/recipes';
import { buildClientRecipe, glueAmounts, pdfLabels } from './pdf-document';

const recipe = {
	id: 'r1',
	title: 'Käsespätzle',
	description: '',
	servings: 4,
	prepMinutes: 20,
	cookMinutes: null,
	sourceUrl: null,
	sourceName: null,
	tags: [],
	ingredientGroups: [
		{
			name: null,
			ingredients: [
				{ quantity: 400, unit: 'g', name: 'Mehl', note: null },
				{ quantity: 1, unit: 'TL', name: 'Salz', note: null },
				{ quantity: null, unit: null, name: 'Muskat', note: 'frisch' }
			]
		}
	],
	steps: [
		{
			text: 'Mehl und Salz mischen, 10 Minuten ruhen lassen.',
			references: [{ word: 'Mehl', groupName: null, ingredientName: 'Mehl' }],
			times: [{ phrase: '10 Minuten', seconds: 600 }]
		}
	]
} as unknown as Recipe;

describe('buildClientRecipe', () => {
	const c = buildClientRecipe(recipe, 2);

	it('scales quantities and formats display', () => {
		const [mehl, salz, muskat] = c.ingredientGroups[0].ingredients;
		expect(mehl.quantity).toEqual({ value: 200, display: '200' });
		expect(mehl.unit).toBe('g');
		expect(mehl.display).toBe('200 g Mehl');
		expect(salz.quantity?.display).toBe('½');
		expect(muskat.quantity).toBeNull();
		expect(c.servings).toEqual({ value: 2, original: 4, display: '2' });
	});

	it('never has null lists and nulls absent times', () => {
		expect(c.tags).toEqual([]);
		expect(c.times.cook).toBeNull();
		expect(c.times.total?.minutes).toBe(20);
		expect(c.source).toBeNull();
	});

	it('names a source by its name, else the host of its URL', () => {
		const url = 'https://www.example.com/kaesespaetzle';
		expect(buildClientRecipe({ ...recipe, sourceUrl: url }, 2).source?.display).toBe('example.com');
		expect(
			buildClientRecipe({ ...recipe, sourceUrl: url, sourceName: 'Omas Buch' }, 2).source?.display
		).toBe('Omas Buch');
	});

	it('maps step segments with the on-screen amount', () => {
		const [step] = c.steps;
		expect(step.number).toBe(1);
		expect(step.segments[0]).toEqual({
			kind: 'ingredient',
			text: 'Mehl',
			amount: '200\u00a0g',
			minutes: null
		});
		expect(step.segments.find((s) => s.kind === 'time')).toMatchObject({
			text: '10\u00a0Minuten',
			minutes: 10
		});
		expect(step.segments.map((s) => s.text).join('')).toBe(step.text);
	});
});

describe('segments', () => {
	it('keeps a referenced ingredient without quantity an ingredient or plain text, and short timers at least 1 minute', () => {
		const r = {
			...recipe,
			steps: [
				{
					text: 'Muskat reiben, 20 Sekunden ruhen.',
					references: [{ word: 'Muskat', groupName: null, ingredientName: 'Muskat' }],
					times: [{ phrase: '20 Sekunden', seconds: 20 }]
				}
			]
		} as unknown as Recipe;
		const segs = buildClientRecipe(r, 4).steps[0].segments;
		expect(segs[0].kind === 'ingredient' ? segs[0].amount : null).toBeNull();
		expect(segs.find((s) => s.kind === 'time')?.minutes).toBe(1);
	});
});

describe('glueAmounts', () => {
	it('keeps a number with the word after it', () => {
		expect(glueAmounts('etwa 300 g, dann 1,5 kg und ½ TL')).toBe(
			'etwa 300\u00a0g, dann 1,5\u00a0kg und ½\u00a0TL'
		);
		expect(glueAmounts('3 - 4 Minuten')).toBe('3 - 4\u00a0Minuten');
		expect(glueAmounts('1 ½ TL')).toBe('1\u00a0½\u00a0TL');
		expect(glueAmounts('bei 180 °C backen')).toBe('bei 180\u00a0°C backen');
		expect(glueAmounts('5 % Fett')).toBe('5\u00a0% Fett');
		// A slash is no unit: "1 / 2" stays breakable.
		expect(glueAmounts('1 / 2')).toBe('1 / 2');
	});

	it('glues notes, amounts, step text and the description', () => {
		const c = buildClientRecipe(
			{
				...recipe,
				description: 'Reicht für 4 Personen.',
				ingredientGroups: [
					{
						name: null,
						ingredients: [{ quantity: 4, unit: 'Stück', name: 'Reis', note: 'etwa 300 g' }]
					}
				],
				steps: [
					{
						text: 'Reis 20 Minuten quellen lassen.',
						references: [{ word: 'Reis', groupName: null, ingredientName: 'Reis' }],
						times: []
					}
				]
			} as unknown as Recipe,
			4
		);
		expect(c.description).toBe('Reicht für 4\u00a0Personen.');
		expect(c.ingredientGroups[0].ingredients[0].note).toBe('etwa 300\u00a0g');
		const [ref, rest] = c.steps[0].segments;
		expect(ref.amount).toBe('4\u00a0Stück');
		expect(rest.text).toContain('20\u00a0Minuten');
	});
});

describe('pdfLabels', () => {
	it('leaves {n} in the scaled-from notes for the template to fill', () => {
		const labels = pdfLabels();
		expect(labels.scaledFrom).toContain('{n}');
		expect(labels.scaledFromShort).toContain('{n}');
	});
});
