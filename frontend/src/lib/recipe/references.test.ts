import { describe, expect, it } from 'vitest';
import { segmentStep, suggestReferences } from './references';
import type { IngredientGroup } from '$lib/api/recipes';

const groups: IngredientGroup[] = [
	{
		name: 'Grütze',
		ingredients: [
			{ quantity: 500, unit: 'g', name: 'Beeren', note: null },
			{ quantity: 100, unit: 'g', name: 'Zucker', note: null }
		]
	},
	{
		name: 'Vanillesoße',
		ingredients: [{ quantity: 50, unit: 'g', name: 'Zucker', note: null }]
	}
];

describe('segmentStep', () => {
	it('splits the text around a referenced word and carries its quantity', () => {
		const out = segmentStep(
			{
				text: 'Saft mit Zucker aufkochen.',
				references: [{ word: 'Zucker', groupName: 'Grütze', ingredientName: 'Zucker' }]
			},
			groups,
			4,
			4
		);
		expect(out).toEqual([
			{ text: 'Saft mit ' },
			{ text: 'Zucker', quantity: '100 g' },
			{ text: ' aufkochen.' }
		]);
	});

	it('scales the quantity with the servings', () => {
		const out = segmentStep(
			{
				text: 'Zucker zugeben.',
				references: [{ word: 'Zucker', groupName: 'Vanillesoße', ingredientName: 'Zucker' }]
			},
			groups,
			8,
			4
		);
		expect(out[0]).toEqual({ text: 'Zucker', quantity: '100 g' });
	});

	it('drops a reference whose ingredient is gone, leaving plain text', () => {
		const out = segmentStep(
			{
				text: 'Mehl zugeben.',
				references: [{ word: 'Mehl', groupName: 'Grütze', ingredientName: 'Mehl' }]
			},
			groups,
			4,
			4
		);
		expect(out).toEqual([{ text: 'Mehl zugeben.' }]);
	});

	it('respects unicode word boundaries', () => {
		const withOil: IngredientGroup[] = [
			{ name: null, ingredients: [{ quantity: 2, unit: 'EL', name: 'Öl', note: null }] }
		];
		const out = segmentStep(
			{
				text: 'In Öl anbraten.',
				references: [{ word: 'Öl', groupName: null, ingredientName: 'Öl' }]
			},
			withOil,
			4,
			4
		);
		expect(out).toEqual([
			{ text: 'In ' },
			{ text: 'Öl', quantity: '2 EL' },
			{ text: ' anbraten.' }
		]);
	});

	it('annotates two references in text order, regardless of reference order', () => {
		const out = segmentStep(
			{
				text: 'Beeren mit Zucker verrühren.',
				references: [
					{ word: 'Zucker', groupName: 'Grütze', ingredientName: 'Zucker' },
					{ word: 'Beeren', groupName: 'Grütze', ingredientName: 'Beeren' }
				]
			},
			groups,
			4,
			4
		);
		expect(out).toEqual([
			{ text: 'Beeren', quantity: '500 g' },
			{ text: ' mit ' },
			{ text: 'Zucker', quantity: '100 g' },
			{ text: ' verrühren.' }
		]);
	});

	it('does not match a word inside a longer word (negative boundary)', () => {
		const withOil: IngredientGroup[] = [
			{ name: null, ingredients: [{ quantity: 2, unit: 'EL', name: 'Öl', note: null }] }
		];
		const out = segmentStep(
			{
				text: 'Ölsardinen in der Pfanne braten.',
				references: [{ word: 'Öl', groupName: null, ingredientName: 'Öl' }]
			},
			withOil,
			4,
			4
		);
		expect(out).toEqual([{ text: 'Ölsardinen in der Pfanne braten.' }]);
	});

	it('annotates only the first occurrence of a word, never a later mention', () => {
		const flour: IngredientGroup[] = [
			{ name: null, ingredients: [{ quantity: 200, unit: 'g', name: 'Mehl', note: null }] }
		];
		const out = segmentStep(
			{
				text: 'Mehl mit Mehl bestäuben.',
				references: [{ word: 'Mehl', groupName: null, ingredientName: 'Mehl' }]
			},
			flour,
			4,
			4
		);
		expect(out).toEqual([{ text: 'Mehl', quantity: '200 g' }, { text: ' mit Mehl bestäuben.' }]);
	});

	it('keeps only the first reference when two share the same word', () => {
		const out = segmentStep(
			{
				text: 'Zucker und Zucker.',
				references: [
					{ word: 'Zucker', groupName: 'Grütze', ingredientName: 'Zucker' },
					{ word: 'Zucker', groupName: 'Vanillesoße', ingredientName: 'Zucker' }
				]
			},
			groups,
			4,
			4
		);
		expect(out).toEqual([{ text: 'Zucker', quantity: '100 g' }, { text: ' und Zucker.' }]);
	});

	it('drops a later duplicate word even when the first reference for it fails to resolve', () => {
		// The first reference claims the word "Zucker" but names an ingredient
		// that does not exist in "Grütze"; the second reference also says
		// "Zucker" and does resolve. Before the seenWords fix, the first
		// reference's failed lookup fell through `if (!found) continue`, so the
		// second one went on to produce a hit - the word rendered with the
		// SECOND reference's quantity. The fix claims the word on sight of the
		// first reference regardless of whether it resolves, so the second one
		// is skipped and the word must stay plain.
		const out = segmentStep(
			{
				text: 'Zucker dazugeben.',
				references: [
					{ word: 'Zucker', groupName: 'Grütze', ingredientName: 'Marzipan' },
					{ word: 'Zucker', groupName: 'Grütze', ingredientName: 'Zucker' }
				]
			},
			groups,
			4,
			4
		);
		expect(out).toEqual([{ text: 'Zucker dazugeben.' }]);
	});

	it('degrades to plain text when the group name is ambiguous', () => {
		const ambiguous: IngredientGroup[] = [
			{ name: 'Suppe', ingredients: [{ quantity: 1, unit: 'L', name: 'Brühe', note: null }] },
			{ name: 'Suppe', ingredients: [{ quantity: 2, unit: 'L', name: 'Brühe', note: null }] }
		];
		const out = segmentStep(
			{
				text: 'Brühe erhitzen.',
				references: [{ word: 'Brühe', groupName: 'Suppe', ingredientName: 'Brühe' }]
			},
			ambiguous,
			4,
			4
		);
		expect(out).toEqual([{ text: 'Brühe erhitzen.' }]);
	});

	it('suppresses the annotation when the ingredient has no quantity', () => {
		const saltless: IngredientGroup[] = [
			{ name: null, ingredients: [{ quantity: null, unit: 'Prise', name: 'Salz', note: null }] }
		];
		const out = segmentStep(
			{
				text: 'Salz dazugeben.',
				references: [{ word: 'Salz', groupName: null, ingredientName: 'Salz' }]
			},
			saltless,
			4,
			4
		);
		expect(out).toEqual([{ text: 'Salz dazugeben.' }]);
	});
});

const klopse: IngredientGroup[] = [
	{
		name: 'Klopse',
		ingredients: [
			{ quantity: 500, unit: 'g', name: 'Rinderhackfleisch', note: null },
			{ quantity: 1, unit: 'Stück', name: 'Zwiebel', note: null },
			{ quantity: 1, unit: 'Prise', name: 'Pfeffer', note: null }
		]
	},
	{
		name: 'Kapernsauce',
		ingredients: [{ quantity: 500, unit: 'ml', name: 'Rinderbrühe', note: null }]
	}
];

describe('suggestReferences', () => {
	// Guard: exercises the exact-head-noun branch on its own, with several
	// unrelated words in the sentence that must not spuriously overlap.
	it('matches an exact word', () => {
		expect(suggestReferences('Zwiebel fein würfeln.', klopse)).toEqual([
			{ word: 'Zwiebel', groupName: 'Klopse', ingredientName: 'Zwiebel' }
		]);
	});

	// Guard: exercises the compound-overlap branch (suffix of a longer
	// ingredient name), for two independent ingredients/groups.
	it('matches a compound tail', () => {
		expect(suggestReferences('Mit Hackfleisch vermengen.', klopse)).toEqual([
			{ word: 'Hackfleisch', groupName: 'Klopse', ingredientName: 'Rinderhackfleisch' }
		]);
		expect(suggestReferences('Brühe erhitzen.', klopse)).toEqual([
			{ word: 'Brühe', groupName: 'Kapernsauce', ingredientName: 'Rinderbrühe' }
		]);
	});

	// Documentation, not a guard: a verb formed by appending "-en"/"-n" to an
	// ingredient's head noun ("pfeffern", "salzen") never satisfies the
	// compound-overlap check in the first place, because appending letters
	// at the END breaks a *suffix* overlap in both directions -
	// "pfeffern".endsWith("pfeffer") is false, its last 7 characters are
	// "feffern". No verb-specific check exists or is needed; this test
	// exists so a future reader does not add one believing it is missing.
	it('does not match a word that only appends letters to an ingredient', () => {
		expect(suggestReferences('Salzen und pfeffern.', klopse)).toEqual([]);
	});

	// Guard: without NOT_INGREDIENTS's prefix arm this compound-overlaps
	// ("Bratkartoffeln" ends with "kartoffeln") and would wrongly propose
	// Kartoffeln for the finished dish rather than the raw ingredient.
	it('does not match an intermediate product', () => {
		const kartoffeln: IngredientGroup[] = [
			{ name: null, ingredients: [{ quantity: 1, unit: 'kg', name: 'Kartoffeln', note: null }] }
		];
		expect(suggestReferences('Bratkartoffeln anrichten.', kartoffeln)).toEqual([]);
	});

	// Guard: without the "cooking medium" suffix arm this compound-overlaps
	// with both Wasser and Salz - a case the design brief calls out
	// separately from the intermediate-product case above.
	it('does not match a cooking medium naming two ingredients at once', () => {
		const brine: IngredientGroup[] = [
			{
				name: null,
				ingredients: [
					{ quantity: 1, unit: 'L', name: 'Wasser', note: null },
					{ quantity: 20, unit: 'g', name: 'Salz', note: null }
				]
			}
		];
		expect(suggestReferences('Salzwasser aufsetzen.', brine)).toEqual([]);
	});

	// Guard: exercises the ambiguity rule on the EXACT branch - "Zucker" has
	// an exact head-noun match in both groups, so it must stay silent.
	it('stays silent when the name is ambiguous in the recipe', () => {
		const zucker: IngredientGroup[] = [
			{ name: 'Grütze', ingredients: [{ quantity: 100, unit: 'g', name: 'Zucker', note: null }] },
			{ name: 'Soße', ingredients: [{ quantity: 50, unit: 'g', name: 'Zucker', note: null }] }
		];
		expect(suggestReferences('Zucker aufkochen.', zucker)).toEqual([]);
	});

	// Guard: the GROUP can be ambiguous just as the name can. `findGroup` in
	// `service/internal/recipe/references.go` refuses a reference whose group
	// name occurs twice - two unnamed groups included - with a 422, so a
	// proposal into one of them is a link the author could never save.
	// `pickerEntries` already leaves those rows out of the picker; this keeps
	// the matcher in step with it.
	it('stays silent when the group is ambiguous', () => {
		const unnamedTwice: IngredientGroup[] = [
			{ name: null, ingredients: [{ quantity: 500, unit: 'g', name: 'Mehl', note: null }] },
			{ name: null, ingredients: [{ quantity: 1, unit: 'Prise', name: 'Salz', note: null }] }
		];
		expect(suggestReferences('Mehl und Salz mischen.', unnamedTwice)).toEqual([]);

		const namedTwice: IngredientGroup[] = [
			{ name: 'Teig', ingredients: [{ quantity: 1, unit: 'Stück', name: 'Ei', note: null }] },
			{ name: 'Teig', ingredients: [{ quantity: 200, unit: 'g', name: 'Butter', note: null }] }
		];
		expect(suggestReferences('Butter schmelzen.', namedTwice)).toEqual([]);
	});

	// R4 guard: proves the exclusion list no longer swallows the bare
	// ingredient name. Against the brief's original regex - tested against
	// every word rather than gated to the compound branch - "Salz" itself
	// matches `^(brat|röst|salz)` and is skipped, so this ingredient could
	// never match its own name; this test fails under that implementation.
	it('matches the bare ingredient name Salz even though its compounds are excluded', () => {
		const salt: IngredientGroup[] = [
			{ name: null, ingredients: [{ quantity: 20, unit: 'g', name: 'Salz', note: null }] }
		];
		expect(suggestReferences('Salz und Pfeffer.', salt)).toEqual([
			{ word: 'Salz', groupName: null, ingredientName: 'Salz' }
		]);
	});

	// Boundary guard: an ASCII-only tokenizer (`\w`, or `\b`-based scanning)
	// does not treat "Ä" as a word character, so it would split "Äpfel" into
	// "" and "pfel". "pfel" then still compound-overlaps "äpfel" (both >= 4
	// chars, and "äpfel".endsWith("pfel")), so the broken implementation
	// does not go silent - it proposes a hit, but with `word: 'pfel'`
	// instead of `word: 'Äpfel'`, which fails this exact assertion. A
	// correct Unicode-aware tokenizer keeps "Äpfel" as one token and takes
	// the exact-match branch instead.
	it('tokenizes a leading umlaut as part of the word, not a boundary', () => {
		const apples: IngredientGroup[] = [
			{ name: null, ingredients: [{ quantity: 4, unit: 'Stück', name: 'Äpfel', note: null }] }
		];
		expect(suggestReferences('Äpfel schälen und vierteln.', apples)).toEqual([
			{ word: 'Äpfel', groupName: null, ingredientName: 'Äpfel' }
		]);
	});

	// Guard: "kreis".endsWith("reis") is true and both words clear the
	// 4-character floor, so without a minimum non-overlapping remainder this
	// would wrongly propose Reis for a word that only shares a tail with it.
	it('does not match a word that only coincidentally ends with an ingredient', () => {
		const reis: IngredientGroup[] = [
			{ name: null, ingredients: [{ quantity: 200, unit: 'g', name: 'Reis', note: null }] }
		];
		expect(suggestReferences('In einem Kreis anordnen.', reis)).toEqual([]);
	});

	// Guard: exercises the ambiguity rule on the COMPOUND branch - "Schnitzel"
	// compound-overlaps two different ingredients in the same recipe, so it
	// must stay silent even though neither exact-matches on its own.
	it('stays silent when a compound tail overlaps two different ingredients', () => {
		const schnitzel: IngredientGroup[] = [
			{
				name: null,
				ingredients: [
					{ quantity: 2, unit: 'Stück', name: 'Schweineschnitzel', note: null },
					{ quantity: 2, unit: 'Stück', name: 'Kalbsschnitzel', note: null }
				]
			}
		];
		expect(suggestReferences('Schnitzel klopfen.', schnitzel)).toEqual([]);
	});
});
