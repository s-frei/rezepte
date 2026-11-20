import { describe, expect, it } from 'vitest';
import {
	dayNames,
	dayRange,
	mondayOf,
	pastDays,
	shiftWeek,
	spanLabel,
	weekDays,
	weekRange
} from './week';

describe('week', () => {
	it('finds the Monday of a day', () => {
		expect(mondayOf('2026-11-22')).toBe('2026-11-16');
		expect(mondayOf('2026-11-16')).toBe('2026-11-16');
		expect(mondayOf('2027-01-01')).toBe('2026-12-28');
	});

	it('lists the seven days from Monday', () => {
		expect(weekDays('2026-11-30')).toEqual([
			'2026-11-30',
			'2026-12-01',
			'2026-12-02',
			'2026-12-03',
			'2026-12-04',
			'2026-12-05',
			'2026-12-06'
		]);
	});

	it('ranges a week', () => {
		expect(weekRange('2026-11-16', 'de')).toBe('16.–22. November');
		expect(weekRange('2026-11-16', 'en')).toBe('November 16–22');
	});

	// No-break spaces everywhere but behind a dash: written as \u00A0 below.
	it('names a span of days', () => {
		expect(spanLabel('2026-11-23', '2026-11-29', 'de')).toBe('23.–29.\u00A0Nov.');
		expect(spanLabel('2026-11-30', '2026-12-06', 'en')).toBe('Nov\u00A030\u00A0– Dec\u00A06');
		expect(spanLabel('2026-11-23', '2026-11-23', 'de')).toBe('Mo\u00A023.\u00A0Nov.');
		expect(spanLabel('2026-11-23', '2026-11-23', 'en')).toBe('Mon\u00A0Nov\u00A023');
	});

	it('ranges any two days, not only a week', () => {
		expect(dayRange('2026-11-18', '2026-11-20', 'de', 'short')).toBe('18.–20. Nov.');
		expect(dayRange('2026-11-25', '2026-12-03', 'de', 'short')).toBe('25. Nov. – 3. Dez.');
	});

	it('ranges a week across months', () => {
		expect(weekRange('2026-11-30', 'de')).toBe('30. November – 6. Dezember');
		expect(weekRange('2026-11-30', 'en')).toBe('November 30 – December 6');
	});

	it('shortens the months for a phone', () => {
		expect(weekRange('2026-11-16', 'de', 'short')).toBe('16.–22. Nov.');
		expect(weekRange('2026-11-30', 'de', 'short')).toBe('30. Nov. – 6. Dez.');
	});

	it('gives the past days of the current week only', () => {
		expect(pastDays('2026-11-16', '2026-11-20')).toEqual([
			'2026-11-16',
			'2026-11-17',
			'2026-11-18',
			'2026-11-19'
		]);
		expect(pastDays('2026-11-16', '2026-11-16')).toEqual([]);
		expect(pastDays('2026-11-23', '2026-11-20')).toEqual([]);
		expect(pastDays('2026-11-09', '2026-11-20')).toEqual([]);
	});

	it('shifts by weeks', () => {
		expect(shiftWeek('2026-11-30', 1)).toBe('2026-12-07');
		expect(shiftWeek('2026-01-05', -1)).toBe('2025-12-29');
	});
});

describe('dayNames', () => {
	it('names a day in both languages', () => {
		expect(dayNames('2026-11-20', 'de')).toEqual({
			weekday: 'Fr',
			long: 'Freitag',
			date: '20. Nov.',
			number: '20.',
			full: 'Freitag, 20. November'
		});
		expect(dayNames('2026-11-20', 'en')).toEqual({
			weekday: 'Fri',
			long: 'Friday',
			date: 'Nov 20',
			number: '20',
			full: 'Friday, November 20'
		});
	});
});
