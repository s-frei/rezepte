import { describe, expect, test } from 'vitest';
import {
	addTime,
	caretTimePhrases,
	isMarked,
	overlapsAny,
	pendingTimes,
	presentTimes,
	removeTime
} from './step-times';
import type { FormStep } from './form';
import { wordAtCaret } from './step-references';

const step = (text: string, times = [] as FormStep['times']): FormStep => ({
	id: 's1',
	text,
	references: [],
	times
});

describe('pendingTimes', () => {
	test('suggests what is not stored or dismissed', () => {
		const s = step('20 Minuten ruhen, dann 1 Stunde kühlen.', [
			{ phrase: '20 Minuten', seconds: 1200 }
		]);
		expect(pendingTimes(s).map((t) => t.phrase)).toEqual(['1 Stunde']);
		expect(pendingTimes(s, ['1 Stunde'])).toEqual([]);
	});

	test('yields each phrase once, so a keyed list never repeats a key', () => {
		const phrases = pendingTimes(
			step('20 Minuten backen, 1 Stunde 30 Minuten ruhen, 20 Minuten und 30 Minuten backen.')
		).map((t) => t.phrase);
		expect(phrases).toEqual(['20 Minuten', '1 Stunde 30 Minuten']);
		expect(new Set(phrases).size).toBe(phrases.length);
	});

	test('proposes a duration written around a stored one, and accepting it replaces that one', () => {
		const s = step('1 Stunde 20 Minuten backen.', [{ phrase: '20 Minuten', seconds: 1200 }]);
		const [whole] = pendingTimes(s);
		expect(whole).toEqual({ phrase: '1 Stunde 20 Minuten', seconds: 4800 });
		expect(addTime(s.times, whole, s.text)).toEqual([whole]);
	});

	test('a caret in a stored time inside a proposal finds the proposal', () => {
		const s = step('1 Stunde 20 Minuten backen.', [{ phrase: '20 Minuten', seconds: 1200 }]);
		const phrases = caretTimePhrases(s.times, pendingTimes(s));
		expect(wordAtCaret(s.text, phrases, s.text.indexOf('20') + 1)).toBe('1 Stunde 20 Minuten');
	});

	test('skips a proposal that only partly overlaps a stored time or touches a linked word', () => {
		expect(
			pendingTimes(
				step('1 Stunde 20 Minuten backen.', [{ phrase: '20 Minuten backen', seconds: 1200 }])
			)
		).toEqual([]);
		expect(
			pendingTimes({
				...step('20 Minuten backen.'),
				references: [{ word: 'Minuten', ingredientId: 'i1', groupName: null, ingredientName: 'x' }]
			})
		).toEqual([]);
	});
});

test('overlapsAny: a word inside a time overlaps it, one beside it does not', () => {
	expect(overlapsAny('Den Teig 20 Minuten ruhen.', 'Minuten', ['20 Minuten'])).toBe(true);
	expect(overlapsAny('Den Teig 20 Minuten ruhen.', 'Teig', ['20 Minuten'])).toBe(false);
});

describe('isMarked', () => {
	const s: FormStep = {
		...step('2 Eier für 15 Minuten kochen, dann 2 Lieder warten.', [
			{ phrase: '15 Minuten', seconds: 900 }
		]),
		references: [{ word: '2 Eier', ingredientId: 'i1', groupName: null, ingredientName: 'Eier' }]
	};
	test('a stored time, a linked word, or a stretch overlapping one is marked', () => {
		expect(isMarked(s, '15 Minuten')).toBe(true);
		expect(isMarked(s, 'für 15 Minuten')).toBe(true);
		expect(isMarked(s, '2 Eier')).toBe(true);
	});
	test('a stretch clear of every mark is not', () => {
		expect(isMarked(s, '2 Lieder')).toBe(false);
	});
});

describe('presentTimes', () => {
	test('hides a time whose phrase an edit removed, keeps it for undo', () => {
		const times = [{ phrase: '90 Minuten', seconds: 5400 }];
		expect(presentTimes(times, '95 Minuten schmoren.')).toEqual([]);
		expect(presentTimes(times, '90 Minuten schmoren.')).toEqual(times);
	});
});

test('addTime replaces the same phrase, removeTime drops it', () => {
	const a = { phrase: '10 Min.', seconds: 600 };
	const b = { phrase: '10 Min.', seconds: 660 };
	expect(addTime([a], b, 'ca. 10 Min. backen')).toEqual([b]);
	expect(removeTime([a], '10 Min.')).toEqual([]);
});
