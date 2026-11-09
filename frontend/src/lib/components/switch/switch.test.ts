import { describe, expect, it } from 'vitest';
import { firstName, nameSize, seenLabel } from './switch';

// Local dates, so the calendar day is the same in every time zone the suite runs in.
const now = new Date(2026, 10, 8, 9, 30); // Sunday, 8 November 2026
const at = (day: number, hour = 12) => new Date(2026, 10, day, hour).toISOString();
const ago = (days: number) => new Date(now.getTime() - days * 86_400_000).toISOString();

describe('seenLabel', () => {
	it('says today for the same calendar day', () => {
		expect(seenLabel(at(8, 1), now, 'de')).toBe('Heute hier');
		expect(seenLabel(at(8, 1), now, 'en')).toBe('Here today');
	});

	it('says yesterday for the day before, however few hours ago', () => {
		expect(seenLabel(at(7, 23), now, 'de')).toBe('Gestern hier');
		expect(seenLabel(at(7, 23), now, 'en')).toBe('Here yesterday');
	});

	it('names the weekday within a week', () => {
		expect(seenLabel(at(5), now, 'de')).toBe('Am Donnerstag hier');
		expect(seenLabel(at(5), now, 'en')).toBe('Here on Thursday');
	});

	it('names the weekday up to six days back', () => {
		expect(seenLabel(at(2), now, 'de')).toBe('Am Montag hier');
	});

	it('says last week from seven to thirteen days', () => {
		expect(seenLabel(at(1), now, 'de')).toBe('Letzte Woche hier');
		expect(seenLabel(ago(13), now, 'en')).toBe('Here last week');
	});

	it('counts whole weeks from fourteen days on', () => {
		expect(seenLabel(ago(14), now, 'de')).toBe('Vor 2 Wochen hier');
		expect(seenLabel(ago(20), now, 'en')).toBe('Here 2 weeks ago');
		expect(seenLabel(ago(35), now, 'de')).toBe('Vor 5 Wochen hier');
	});
});

describe('nameSize', () => {
	it('steps down over 16 and over 28 characters', () => {
		expect(nameSize('a'.repeat(16))).toBe('lg');
		expect(nameSize('a'.repeat(17))).toBe('md');
		expect(nameSize('a'.repeat(28))).toBe('md');
		expect(nameSize('a'.repeat(29))).toBe('sm');
	});
});

describe('firstName', () => {
	it('takes the first word, a hyphenated one whole', () => {
		expect(firstName('Maximiliane-Friederike Sophie')).toBe('Maximiliane-Friederike');
		expect(firstName('  Jo  ')).toBe('Jo');
	});
});
