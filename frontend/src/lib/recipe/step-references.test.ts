import { describe, expect, it } from 'vitest';
import type { FormGroup, FormRef, FormStep } from './form';
import { firstWordMatch } from './references';
import {
	acceptAll,
	addReference,
	choicesFor,
	countPending,
	entryLabel,
	findDuplicates,
	fitsWordLimit,
	isResolved,
	matchEntries,
	matchedPart,
	pendingFor,
	pickerEntries,
	refLabel,
	refView,
	removeReference,
	repeatAtCaret,
	toIngredientGroups,
	waitingChoices,
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
	return { id: 's1', text, references, times: [] };
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

describe('choicesFor', () => {
	const SAUCE = [
		group('Klopse', ['500', 'g', 'Hackfleisch'], ['1', 'Prise', 'Salz']),
		group('Kapernsauce', ['200', 'ml', 'Sahne'], ['2', 'EL', 'Kapern'], ['1', 'Prise', 'Salz'])
	];
	const labels = (refs: FormRef[]) => refs.map((ref) => refLabel(ref, pickerEntries(SAUCE)));

	it('guesses the group the rest of the sentence comes from, and Accept all takes it', () => {
		const s = step('Sahne und Kapern einrühren, mit Salz abschmecken.');
		const [choice] = choicesFor(s, SAUCE);
		expect(choice.word).toBe('Salz');
		expect(labels(choice.options)).toEqual([
			'Salz · 1 Prise · Klopse',
			'Salz · 1 Prise · Kapernsauce'
		]);
		expect(labels([choice.guess!])).toEqual(['Salz · 1 Prise · Kapernsauce']);
		expect(choice.because).toEqual(['Sahne', 'Kapern']);
		expect(labels(pendingFor(s, SAUCE))).toContain('Salz · 1 Prise · Kapernsauce');
		expect(waitingChoices([s], SAUCE)).toEqual([]);
	});

	it('counts a stored link as much as a proposal', () => {
		const s = step('Hackfleisch mit Salz würzen.', [
			ref('Hackfleisch', 'Klopse-0', 'Klopse', 'Hackfleisch')
		]);
		expect(labels([choicesFor(s, SAUCE)[0].guess!])).toEqual(['Salz · 1 Prise · Klopse']);
	});

	it('waits for the author when the sentence does not lead to one group', () => {
		for (const text of ['Mit Salz abschmecken.', 'Hackfleisch und Sahne mit Salz verrühren.']) {
			const s = step(text);
			expect(choicesFor(s, SAUCE)[0].guess).toBeNull();
			expect(pendingFor(s, SAUCE).map((p) => p.word)).not.toContain('Salz');
			expect(waitingChoices([step('Zwiebel schneiden.'), s], SAUCE)).toEqual([
				{ word: 'Salz', step: 2 }
			]);
		}
	});

	it('offers nothing for an already linked word', () => {
		const linked = step('Mit Salz abschmecken.', [ref('Salz', 'Klopse-1', 'Klopse', 'Salz')]);
		expect(choicesFor(linked, SAUCE)).toEqual([]);
	});

	it('leaves a word alone when one of its groups cannot be singled out', () => {
		const twice = [...SAUCE, { ...group('Klopse', ['1', '', 'Ei']), id: 'again' }];
		expect(choicesFor(step('Mit Salz abschmecken.'), twice)).toEqual([]);
	});
});

describe('repeatAtCaret', () => {
	const text = 'Quark verrühren. Den Quark kühlen.';
	it('finds the caret in a later occurrence only', () => {
		expect(repeatAtCaret(text, ['Quark'], text.lastIndexOf('Quark') + 2)).toBe('Quark');
		expect(repeatAtCaret(text, ['Quark'], 2)).toBeNull();
		expect(repeatAtCaret(text, ['Quark'], text.lastIndexOf('Quark'))).toBeNull();
	});
});

describe('findDuplicates', () => {
	/** `group()` keys a group by its name; two of one name need ids of their own. */
	const as = (id: string, g: FormGroup): FormGroup => ({ ...g, id });

	it('reports a repeated group name at the later group, and every row of a repeated name', () => {
		const found = findDuplicates([
			group('Teig', ['1', '', 'Ei']),
			as('second', group(' Teig ', ['2', '', 'Mehl'])),
			group('Füllung', ['1', '', 'Quark'], ['2', '', ' Quark '], ['1', 'Prise', 'Salz'])
		]);
		expect([...found.groups]).toEqual([['second', 'Teig']]);
		expect([...found.rows]).toEqual(['Füllung-0', 'Füllung-1']);
	});

	it('names a second unnamed group, but not one still empty', () => {
		// "Add group" puts an empty unnamed one in, and `toInput` drops it: it
		// is no clash until it holds a row.
		const empty = findDuplicates([group('', ['1', '', 'Ei']), as('new', group('', ['', '', '']))]);
		expect(empty.groups.size).toBe(0);
		const filled = findDuplicates([
			group('', ['1', '', 'Ei']),
			as('new', group('', ['', '', 'Mehl']))
		]);
		expect([...filled.groups]).toEqual([['new', null]]);
	});

	it('finds nothing where the picker offers every row', () => {
		const found = findDuplicates(AMBIGUOUS);
		expect(found.groups.size + found.rows.size).toBe(0);
		expect(pickerEntries(AMBIGUOUS)).toHaveLength(2);
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

	it('keeps rows of one name together, where the first of them stands', () => {
		const entries = pickerEntries([
			group('Klopse', ['1', 'TL', 'Salz'], ['1', '', 'Sahne']),
			group('Sauce', ['1', 'Prise', 'Salz'])
		]);
		expect(matchEntries(entries, 'sa').map(entryLabel)).toEqual([
			'Salz · 1 TL · Klopse',
			'Salz · 1 Prise · Sauce',
			'Sahne · 1 · Klopse'
		]);
	});
});

describe('matchedPart', () => {
	it('finds the query in the name, keeping how the name writes it', () => {
		expect(matchedPart('Rohrzucker', 'ZUCK')).toEqual({ before: 'Rohr', hit: 'zuck', after: 'er' });
	});

	it('is null without a query or a match', () => {
		expect(matchedPart('Salz', '')).toBeNull();
		expect(matchedPart('Salz', 'mehl')).toBeNull();
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

	// Guard on the invariant rather than on one implementation of it: "Accept
	// all" turns every open proposal into a real reference, so a proposal the API
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
	it('adds up the open proposals of every step', () => {
		const groups = [group('', ['100', 'g', 'Zucker'], ['200', 'g', 'Mehl'])];
		const steps: FormStep[] = [
			{ id: 'a', text: 'Zucker und Mehl verrühren.', references: [], times: [] },
			{ id: 'b', text: 'Mehl sieben.', references: [], times: [] }
		];
		expect(countPending(steps, groups)).toBe(3);
	});
});

describe('times among the proposals', () => {
	const groups = [group('', ['100', 'g', 'Zwiebel'])];
	it('countPending counts a time beside a link', () => {
		expect(countPending([step('Zwiebel 10 Minuten dünsten.')], groups)).toBe(2);
	});
	it('acceptAll stores the time', () => {
		const steps = [step('Zwiebel 10 Minuten dünsten.')];
		acceptAll(steps, groups);
		expect(steps[0].times).toEqual([{ phrase: '10 Minuten', seconds: 600 }]);
	});
});

describe('acceptAll', () => {
	it('confirms every open proposal', () => {
		const groups = [group('', ['100', 'g', 'Zucker'], ['200', 'g', 'Mehl'])];
		const steps: FormStep[] = [
			{ id: 'a', text: 'Zucker und Mehl verrühren.', references: [], times: [] }
		];
		acceptAll(steps, groups);
		expect(steps[0].references).toEqual([
			ref('Zucker', '-0', null, 'Zucker'),
			ref('Mehl', '-1', null, 'Mehl')
		]);
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
