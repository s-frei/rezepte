import { describe, expect, test } from 'vitest';
import {
	beyondCap,
	coveredBy,
	notChips,
	ownTickCount,
	tickCovers,
	ticksNamed,
	withTicks
} from './ticks.js';

const item = (name: string, key = name.toLowerCase()) => ({ name, key });

describe('ticksNamed', () => {
	// A plural tick from a link is keyed like the server's row pill from
	// the first render: a lower-cased stand-in ("kartoffeln") made a
	// second, unticked pill beside the row's "Kartoffeln" (`kartoffel`).
	test('keys the names of a link as the server does', () => {
		expect(ticksNamed(['Kartoffeln', 'Tomaten', 'Crème fraîche'])).toEqual([
			{ name: 'Kartoffeln', key: 'kartoffel' },
			{ name: 'Tomaten', key: 'tomate' },
			{ name: 'Crème fraîche', key: 'creme fraiche' }
		]);
	});
});

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

	test('leaves out a suggestion that is a chip, by key', () => {
		expect(notChips([suggestion('Zwiebel'), suggestion('Zwieback')], ['ZWIEBEL'])).toEqual([
			suggestion('Zwieback')
		]);
	});

	test('leaves out the singular of a plural chip', () => {
		expect(notChips([suggestion('Zwiebel')], ['Zwiebeln'])).toEqual([]);
	});

	test('keeps a suggestion that only covers a chip', () => {
		const zwiebel = suggestion('Zwiebel', ['Frühlingszwiebeln']);
		expect(notChips([zwiebel], ['Frühlingszwiebeln'])).toEqual([zwiebel]);
	});
});
