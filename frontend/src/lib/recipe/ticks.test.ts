import { describe, expect, test } from 'vitest';
import { beyondCap, coveredBy, ownTickCount, withTicks } from './ticks.js';

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
