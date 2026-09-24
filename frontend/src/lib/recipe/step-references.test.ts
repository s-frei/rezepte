import { describe, expect, it } from 'vitest';
import type { FormGroup, FormRef, FormStep } from './form';
import {
	acceptAll,
	addReference,
	countPending,
	entryLabel,
	firstWordMatch,
	fitsWordLimit,
	isResolved,
	matchEntries,
	pendingFor,
	pickerEntries,
	refLabel,
	refView,
	removeReference,
	toIngredientGroups,
	wordAt,
	wordAtCaret
} from './step-references';

function group(name: string, ...rows: [string, string, string][]): FormGroup {
	return {
		id: `g-${name}`,
		name,
		ingredients: rows.map(([quantity, unit, ingredient], index) => ({
			id: `${name}-${index}`,
			quantity,
			unit,
			name: ingredient,
			note: ''
		}))
	};
}

function step(text: string, references: FormRef[] = []): FormStep {
	return { id: 's1', text, references };
}

/** A reference anchored to the row `group()` gave that id. */
function ref(
	word: string,
	ingredientId: string | null,
	groupName: string | null,
	ingredientName: string
): FormRef {
	return { word, ingredientId, groupName, ingredientName };
}

/** The recipe the whole feature was designed around: one name, two groups. */
const AMBIGUOUS = [
	group('Grütze', ['100', 'g', 'Zucker']),
	group('Vanillesoße', ['50', 'g', 'Zucker'])
];

describe('toIngredientGroups', () => {
	it('shapes the form the way the API will see it', () => {
		const groups = toIngredientGroups([
			{
				id: 'g1',
				name: '  ',
				ingredients: [
					{ id: 'a', quantity: '1,5', unit: ' TL ', name: ' Salz ', note: 'x' },
					{ id: 'b', quantity: '2', unit: '', name: '   ', note: '' }
				]
			}
		]);
		expect(groups).toEqual([
			{ name: null, ingredients: [{ quantity: 1.5, unit: 'TL', name: 'Salz', note: null }] }
		]);
	});
});

describe('pickerEntries', () => {
	it('offers the same name once per group, so the two can be told apart', () => {
		expect(pickerEntries(AMBIGUOUS).map(entryLabel)).toEqual([
			'Zucker · 100 g · Grütze',
			'Zucker · 50 g · Vanillesoße'
		]);
	});

	it('leaves out what the API could not resolve', () => {
		// Two groups of one name, and one name twice inside a group: the API
		// answers both with a 422, so neither may be offered.
		const entries = pickerEntries([
			group('Teig', ['1', '', 'Ei']),
			group('Teig', ['2', '', 'Mehl']),
			group('Füllung', ['1', '', 'Quark'], ['2', '', 'Quark'], ['1', 'Prise', 'Salz'])
		]);
		expect(entries.map((entry) => entry.name)).toEqual(['Salz']);
	});

	it('does not count a freshly added, still empty unnamed group', () => {
		// "Add group" puts one in, and `toInput` drops it again. Counted, it
		// would make the unnamed group above it look ambiguous and take every
		// link into that group down with it.
		const groups = [group('', ['100', 'g', 'Zucker']), group('', ['', '', ''])];
		expect(pickerEntries(groups).map(entryLabel)).toEqual(['Zucker · 100 g']);
		expect(isResolved(ref('Zucker', '-0', null, 'Zucker'), pickerEntries(groups))).toBe(true);
		expect(pendingFor(step('Zucker verrühren.'), groups).map((p) => p.word)).toEqual(['Zucker']);
	});

	it('drops the amount when there is no quantity', () => {
		expect(pickerEntries([group('', ['', '', 'Pfeffer'])]).map(entryLabel)).toEqual(['Pfeffer']);
	});
});

describe('matchEntries', () => {
	it('puts what starts with the query before what merely contains it', () => {
		const entries = pickerEntries([group('', ['1', '', 'Rohrzucker'], ['2', '', 'Zucker'])]);
		expect(matchEntries(entries, 'zuck').map((entry) => entry.name)).toEqual([
			'Zucker',
			'Rohrzucker'
		]);
	});

	it('returns everything for an empty query', () => {
		const entries = pickerEntries(AMBIGUOUS);
		expect(matchEntries(entries, '')).toHaveLength(2);
	});
});

describe('isResolved', () => {
	const entries = pickerEntries(AMBIGUOUS);

	it('is true while the row is still there', () => {
		expect(isResolved(ref('Zucker', 'Grütze-0', 'Grütze', 'Zucker'), entries)).toBe(true);
	});

	it('stays true after the row and its group have been renamed', () => {
		// The anchor is the row, not the names it carried when the link was
		// made. Both names are stale here and the link still resolves, which is
		// what the client id is for.
		const renamed = pickerEntries([group('Kompott', ['100', 'g', 'Rohrzucker'])]);
		expect(isResolved(ref('Zucker', 'Kompott-0', 'Grütze', 'Zucker'), renamed)).toBe(true);
	});

	it('is false once the row has been deleted', () => {
		// The editor marks this one as broken instead of dropping it: the API
		// answers it with a 422, and a link that vanishes on its own is worse
		// than one the author is shown and can fix.
		expect(isResolved(ref('Zucker', 'Sirup-0', 'Sirup', 'Zucker'), entries)).toBe(false);
	});

	it('is false for a reference that never found a row', () => {
		// What `fromRecipe` produces for a stored reference naming a row the
		// recipe no longer has.
		expect(isResolved(ref('Zucker', null, 'Sirup', 'Zucker'), entries)).toBe(false);
	});

	it('is false once the row it names has become ambiguous', () => {
		// Two groups of one name: `pickerEntries` leaves both out, so an older
		// reference into either one no longer resolves - which is exactly what
		// the API will say.
		const ambiguous = pickerEntries([
			group('Teig', ['1', '', 'Ei']),
			group('Teig', ['2', '', 'Mehl'])
		]);
		expect(isResolved(ref('Ei', 'Teig-0', 'Teig', 'Ei'), ambiguous)).toBe(false);
	});
});

describe('refLabel', () => {
	it('names a reference whose row has been deleted', () => {
		const entries = pickerEntries(AMBIGUOUS);
		expect(refLabel(ref('Zucker', null, 'Sirup', 'Zucker'), entries)).toBe('Zucker · Sirup');
	});
});

describe('refView', () => {
	it('leaves the group out where the name is unique', () => {
		const entries = pickerEntries([group('Klopse', ['1', 'Stück', 'Zwiebel'])]);
		expect(refView(ref('Zwiebel', 'Klopse-0', 'Klopse', 'Zwiebel'), entries)).toEqual({
			name: 'Zwiebel',
			amount: '1 Stück',
			group: null,
			resolved: true
		});
	});

	it('keeps it where the same name is in two groups', () => {
		const entries = pickerEntries(AMBIGUOUS);
		expect(refView(ref('Zucker', 'Vanillesoße-0', 'Vanillesoße', 'Zucker'), entries)).toEqual({
			name: 'Zucker',
			amount: '50 g',
			group: 'Vanillesoße',
			resolved: true
		});
	});

	it('keeps it, from the reference, where the row is gone', () => {
		const entries = pickerEntries(AMBIGUOUS);
		expect(refView(ref('Saft', null, 'Grütze', 'Saft'), entries)).toEqual({
			name: 'Saft',
			amount: '',
			group: 'Grütze',
			resolved: false
		});
	});
});

describe('wordAtCaret', () => {
	const text = 'Zucker und Mehl verrühren.';

	it('finds the word the caret stands inside', () => {
		expect(wordAtCaret(text, ['Zucker', 'Mehl'], 13)).toBe('Mehl');
	});

	it('ignores a caret at either edge of the word', () => {
		// Where somebody who just typed the word, or is about to type in front
		// of it, has their caret.
		expect(wordAtCaret(text, ['Mehl'], 11)).toBeNull();
		expect(wordAtCaret(text, ['Mehl'], 15)).toBeNull();
	});

	it('counts only the first occurrence, the underlined one', () => {
		expect(wordAtCaret('Mehl sieben, Mehl wiegen.', ['Mehl'], 15)).toBeNull();
	});
});

describe('pendingFor', () => {
	const groups = [group('', ['100', 'g', 'Zucker'], ['200', 'g', 'Mehl'])];

	it('proposes what the matcher found, anchored to the row it found', () => {
		expect(pendingFor(step('Zucker und Mehl verrühren.'), groups)).toEqual([
			ref('Zucker', '-0', null, 'Zucker'),
			ref('Mehl', '-1', null, 'Mehl')
		]);
	});

	it('does not propose a word that is already linked', () => {
		const linked = step('Zucker und Mehl verrühren.', [ref('Zucker', '-0', null, 'Zucker')]);
		expect(pendingFor(linked, groups).map((proposal) => proposal.word)).toEqual(['Mehl']);
	});

	it('does not propose a word that was turned down', () => {
		expect(
			pendingFor(step('Zucker und Mehl verrühren.'), groups, ['Mehl']).map(
				(proposal) => proposal.word
			)
		).toEqual(['Zucker']);
	});

	// Guard on the invariant rather than on one implementation of it: a save
	// turns every open proposal into a real reference, so a proposal the API
	// cannot resolve would fail the save with an error about the ingredient
	// list, under a step, for a link the author never made. Two unnamed groups
	// is an ordinary state to be in halfway through writing a recipe.
	//
	// Two things enforce this - the matcher declines to propose a row in a
	// group that occurs twice, and `pendingFor` keeps only what it finds in the
	// picker's own list. Removing either one alone leaves this passing;
	// removing both makes it fail. That is the point of having both.
	it('does not propose what the API could not resolve', () => {
		const twoUnnamed = [group('', ['100', 'g', 'Zucker']), group('', ['200', 'g', 'Mehl'])];
		expect(pendingFor(step('Zucker und Mehl verrühren.'), twoUnnamed)).toEqual([]);
		// Nothing offerable, nothing proposed: the two lists agree.
		expect(pickerEntries(twoUnnamed)).toEqual([]);
	});
});

describe('countPending', () => {
	it('adds up the open proposals of every step, minus the dismissed ones', () => {
		const groups = [group('', ['100', 'g', 'Zucker'], ['200', 'g', 'Mehl'])];
		const steps: FormStep[] = [
			{ id: 'a', text: 'Zucker und Mehl verrühren.', references: [] },
			{ id: 'b', text: 'Mehl sieben.', references: [] }
		];
		expect(countPending(steps, groups)).toBe(3);
		expect(countPending(steps, groups, { b: ['Mehl'] })).toBe(2);
	});
});

describe('acceptAll', () => {
	it('confirms every open proposal and leaves the dismissed ones alone', () => {
		const groups = [group('', ['100', 'g', 'Zucker'], ['200', 'g', 'Mehl'])];
		const steps: FormStep[] = [{ id: 'a', text: 'Zucker und Mehl verrühren.', references: [] }];
		acceptAll(steps, groups, { a: ['Mehl'] });
		expect(steps[0].references).toEqual([ref('Zucker', '-0', null, 'Zucker')]);
	});
});

describe('addReference', () => {
	it('replaces a reference naming the same word instead of adding a second', () => {
		// The API rejects a step whose references name one word twice, so this
		// is what stops the picker from writing an unsaveable step.
		const refs = addReference(
			[ref('Zucker', 'Grütze-0', 'Grütze', 'Zucker')],
			ref('Zucker', 'Vanillesoße-0', 'Vanillesoße', 'Zucker')
		);
		expect(refs).toEqual([ref('Zucker', 'Vanillesoße-0', 'Vanillesoße', 'Zucker')]);
	});

	it('appends a reference naming another word', () => {
		const refs = addReference(
			[ref('Zucker', '-0', null, 'Zucker')],
			ref('Mehl', '-1', null, 'Mehl')
		);
		expect(refs.map((entry) => entry.word)).toEqual(['Zucker', 'Mehl']);
	});
});

describe('removeReference', () => {
	it('drops the reference naming the word', () => {
		const refs = removeReference(
			[ref('Zucker', '-0', null, 'Zucker'), ref('Mehl', '-1', null, 'Mehl')],
			'Zucker'
		);
		expect(refs.map((entry) => entry.word)).toEqual(['Mehl']);
	});
});

describe('fitsWordLimit', () => {
	it('takes a word as long as the longest ingredient name', () => {
		expect(fitsWordLimit('ä'.repeat(120))).toBe(true);
	});

	it('refuses a longer one, counting characters rather than UTF-16 units', () => {
		expect(fitsWordLimit('ä'.repeat(121))).toBe(false);
		// Four bytes each in UTF-8, two units each in JavaScript: still 120.
		expect(fitsWordLimit('𝔄'.repeat(120))).toBe(true);
	});
});

describe('firstWordMatch', () => {
	it('finds the first whole-word occurrence', () => {
		expect(firstWordMatch('Zucker zum Zucker geben', 'Zucker')).toEqual({ from: 0, to: 6 });
	});

	it('skips an occurrence inside another word', () => {
		// ASCII `\b` would match at index 5 of "Bratäpfel" and underline
		// "äpfel"; the Unicode boundary does not.
		expect(firstWordMatch('Bratäpfel und Äpfel', 'Äpfel')).toEqual({ from: 14, to: 19 });
	});

	it('is null when the word is gone', () => {
		expect(firstWordMatch('Honig aufkochen', 'Zucker')).toBeNull();
	});
});

describe('wordAt', () => {
	// Offsets: 0-6 Fleisch, 7 space, 8-9 in, 10 space, 11-12 Öl, 13 space,
	// 14-19 scharf, 20 space, 21-28 anbraten, 29 full stop.
	const text = 'Fleisch in Öl scharf anbraten.';

	// Guard: a caret is the common case - the author clicks into the word they
	// mean and never selects anything. Both edges of the word count as being
	// in it, which is what makes clicking feel like it worked.
	it('takes the whole word the caret stands in', () => {
		expect(wordAt(text, 3, 3)).toBe('Fleisch');
		expect(wordAt(text, 0, 0)).toBe('Fleisch');
		expect(wordAt(text, 7, 7)).toBe('Fleisch');
	});

	// Guard: the API anchors a reference to a whole word and refuses half of
	// one, so a selection stopping inside a word is grown out to it. "Öl"
	// also pins the Unicode boundary: an ASCII notion of a word character
	// would cut it in half.
	it('grows a partial selection out to whole words', () => {
		expect(wordAt(text, 2, 5)).toBe('Fleisch');
		expect(wordAt(text, 11, 12)).toBe('Öl');
		expect(wordAt(text, 22, 24)).toBe('anbraten');
	});

	// Guard: "Crème fraîche" has to be as linkable as "Fleisch", so a
	// selection spanning words keeps them - and drops the full stop, which
	// belongs to no word and would make the reference unresolvable.
	it('keeps a selection spanning several words and drops the punctuation', () => {
		expect(wordAt(text, 11, 30)).toBe('Öl scharf anbraten');
	});

	// Guard: with nothing to anchor to the editor must offer no link at all
	// rather than one over an empty word, which the API answers with a 422.
	it('is null where there is no word', () => {
		expect(wordAt(text, 30, 30)).toBeNull();
		expect(wordAt('Salz  Pfeffer', 5, 5)).toBeNull();
		expect(wordAt('', 0, 0)).toBeNull();
	});

	it('clamps a range that runs past the text', () => {
		expect(wordAt(text, 500, 500)).toBeNull();
		expect(wordAt(text, 0, 500)).toBe('Fleisch in Öl scharf anbraten');
	});
});
