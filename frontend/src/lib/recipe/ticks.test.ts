import { describe, expect, test } from 'vitest';
import { beyondCap, coveredBy, notChips, ownTickCount, tickCovers, withTicks } from './ticks.js';

const item = (name: string, key = name.toLowerCase()) => ({ name, key });

describe('coveredBy', () => {
	test('names the tick that covers an ingredient', () => {
		expect(coveredBy(item('Rotweinessig'), [item('Essig')])).toEqual(item('Essig'));
	});

	test('an own tick is not covered by itself', () => {
		expect(coveredBy(item('Essig'), [item('Essig')])).toBeNull();
	});

	test('nothing covers an unticked ingredient', () => {
		expect(coveredBy(item('Salz'), [item('Essig')])).toBeNull();
	});
});

describe('ownTickCount', () => {
	test('counts each own tick', () => {
		expect(ownTickCount([item('Essig'), item('Salz')])).toBe(2);
	});

	test('counts a key once', () => {
		expect(ownTickCount([item('Zwiebel', 'zwiebel'), item('Zwiebeln', 'zwiebel')])).toBe(1);
	});
});

describe('withTicks', () => {
	test('appends a tick that covers nothing in the row', () => {
		const row = [item('Salz'), item('Rotweinessig')];
		expect(withTicks(row, [item('Essig'), item('Mehl')])).toEqual([...row, item('Mehl')]);
	});
});

describe('beyondCap', () => {
	test('hides the untouched pills past the cap and keeps every ticked one', () => {
		const row = ['A', 'B', 'C', 'D'].map((n) => item(n));
		expect(beyondCap(row, [item('D')], 2)).toEqual([item('C')]);
	});
});

describe('tickCovers', () => {
	test('compares by key, not by the ticked name', () => {
		// "Tomaten" ticked is the key "tomate", which its lower case is not.
		expect(tickCovers([item('Tomaten', 'tomate')], 'tomate')).toBe(true);
		expect(tickCovers([item('Essig')], 'rotweinessig')).toBe(true);
		expect(tickCovers([item('Salz')], 'tomate')).toBe(false);
	});
});

describe('notChips', () => {
	const suggestion = (name: string, also: string[] = []) => ({
		name,
		key: name.toLowerCase(),
		count: 1,
		also
	});

	test('leaves out a suggestion that is a chip, by key or name', () => {
		expect(notChips([suggestion('Zwiebel'), suggestion('Zwieback')], ['ZWIEBEL'])).toEqual([
			suggestion('Zwieback')
		]);
	});

	test('keeps a suggestion that only covers a chip', () => {
		const zwiebel = suggestion('Zwiebel', ['Frühlingszwiebeln']);
		expect(notChips([zwiebel], ['Frühlingszwiebeln'])).toEqual([zwiebel]);
	});
});
