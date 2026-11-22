import { describe, expect, it } from 'vitest';
import {
	dayNames,
	aroundToday,
	dayRange,
	dayNumber,
	dayOf,
	daysFrom,
	mondayOf,
	shiftDay,
	pastDays,
	shiftWeek,
	weekDays,
	weekFromNow,
	spanLabel,
	weekRange,
	windowFromNow
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

	// No-break spaces everywhere but behind a dash: written as \u00A0 below.
	it('names a span of days', () => {
		expect(spanLabel('2026-11-23', '2026-11-29', 'de')).toBe('23.–29.\u00A0Nov.');
		expect(spanLabel('2026-11-30', '2026-12-06', 'en')).toBe('Nov\u00A030\u00A0– Dec\u00A06');
		expect(spanLabel('2026-11-23', '2026-11-23', 'de')).toBe('Mo\u00A023.\u00A0Nov.');
		expect(spanLabel('2026-11-23', '2026-11-23', 'en')).toBe('Mon\u00A0Nov\u00A023');
	});

	it('names a span across New Year', () => {
		expect(spanLabel('2026-12-28', '2027-01-03', 'de')).toBe('28.\u00A0Dez.\u00A0– 3.\u00A0Jan.');
		expect(spanLabel('2026-12-28', '2027-01-03', 'en')).toBe('Dec\u00A028\u00A0– Jan\u00A03');
	});

	it('ranges any two days, not only a week', () => {
		expect(dayRange('2026-11-18', '2026-11-20', 'de', 'short')).toBe('18.–20. Nov.');
		expect(dayRange('2026-11-25', '2026-12-03', 'de', 'short')).toBe('25. Nov. – 3. Dez.');
		expect(dayRange('2026-11-01', '2027-01-01', 'en', 'short')).toBe('Nov 1 – Jan 1');
	});

	it('shortens the months for a phone', () => {
		expect(weekRange('2026-11-16', 'de', 'short')).toBe('16.–22. Nov.');
		expect(weekRange('2026-11-30', 'de', 'short')).toBe('30. Nov. – 6. Dez.');
	});

	it('gives a narrow phone numeric months', () => {
		expect(weekRange('2026-11-16', 'de', 'numeric')).toBe('16.–22.11.');
		expect(weekRange('2026-12-07', 'de', 'numeric')).toBe('7.–13.12.');
		expect(weekRange('2026-11-30', 'de', 'numeric')).toBe('30.11.–6.12.');
		expect(weekRange('2026-11-30', 'en', 'numeric')).toBe('11/30–12/6');
	});

	it('never prints the year, not even across New Year', () => {
		expect(weekRange('2026-12-28', 'de')).toBe('28. Dezember – 3. Januar');
		expect(weekRange('2026-12-28', 'de', 'short')).toBe('28. Dez. – 3. Jan.');
		expect(weekRange('2026-12-28', 'en', 'short')).toBe('Dec 28 – Jan 3');
		expect(weekRange('2026-12-28', 'de', 'numeric')).toBe('28.12.–3.1.');
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

	it('says where a week lies from the current one', () => {
		expect(weekFromNow('2026-11-16', '2026-11-16')).toBe('This week');
		expect(weekFromNow('2026-11-23', '2026-11-16')).toBe('Next week');
		expect(weekFromNow('2026-11-09', '2026-11-16')).toBe('Last week');
		expect(weekFromNow('2026-12-07', '2026-11-16')).toBe('In 3 weeks');
		expect(weekFromNow('2026-11-02', '2026-11-16')).toBe('2 weeks ago');
		// Across New Year, both ways (2026 has a week 53).
		expect(weekFromNow('2027-04-05', '2026-12-28')).toBe('In 14 weeks');
		expect(weekFromNow('2026-12-28', '2027-01-04')).toBe('Last week');
		expect(weekFromNow('2026-11-30', '2027-01-11')).toBe('6 weeks ago');
		expect(weekFromNow('2026-11-16', '2026-11-16', 'de')).toBe('Diese Woche');
		expect(weekFromNow('2026-11-23', '2026-11-16', 'de')).toBe('Nächste Woche');
		expect(weekFromNow('2026-11-09', '2026-11-16', 'de')).toBe('Letzte Woche');
		expect(weekFromNow('2026-11-30', '2026-11-16', 'de')).toBe('In 2 Wochen');
		expect(weekFromNow('2026-12-21', '2026-11-16', 'de')).toBe('In 5 Wochen');
		expect(weekFromNow('2026-11-02', '2026-11-16', 'de')).toBe('Vor 2 Wochen');
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

describe('days in view', () => {
	it('counts days on a ruler and back', () => {
		expect(dayNumber('2026-11-23') - dayNumber('2026-11-16')).toBe(7);
		expect(dayOf(dayNumber('2027-01-01'))).toBe('2027-01-01');
		expect(dayOf(dayNumber('2026-11-22') + 0.6)).toBe('2026-11-22');
	});

	it('moves by days and lists a span', () => {
		expect(shiftDay('2026-11-30', 2)).toBe('2026-12-02');
		expect(shiftDay('2027-01-01', -1)).toBe('2026-12-31');
		expect(daysFrom('2026-11-29', '2026-12-01')).toEqual([
			'2026-11-29',
			'2026-11-30',
			'2026-12-01'
		]);
		expect(daysFrom('2026-11-29', '2026-11-29')).toEqual(['2026-11-29']);
	});

	it('puts today in the middle of the days in view', () => {
		expect(aroundToday('2026-11-22', 7)).toBe('2026-11-19');
		expect(aroundToday('2026-11-22', 5)).toBe('2026-11-20');
		expect(aroundToday('2026-11-22', 4)).toBe('2026-11-21');
	});

	it('names today in the middle "around today"', () => {
		expect(windowFromNow('2026-11-19', '2026-11-22', 7, 'de')).toBe('Rund um heute');
		expect(windowFromNow('2026-11-21', '2026-11-22', 4, 'en')).toBe('Around today');
		// Today a Thursday: its Monday week is also around today, and says so.
		expect(windowFromNow('2026-11-16', '2026-11-19', 7, 'en')).toBe('Around today');
	});

	it('names a whole Monday-to-Sunday week as that week, as a phone does', () => {
		expect(windowFromNow('2026-11-16', '2026-11-22', 7, 'de')).toBe('Diese Woche');
		expect(windowFromNow('2026-11-23', '2026-11-22', 7, 'en')).toBe('Next week');
		// Four days from a Monday are no week.
		expect(windowFromNow('2026-11-16', '2026-11-22', 4, 'de')).toBe('Ab Mo 16. Nov.');
	});

	it('names other days by their first day, past and future alike by date', () => {
		expect(windowFromNow('2026-11-22', '2026-11-22', 7, 'de')).toBe('Ab heute');
		expect(windowFromNow('2026-11-22', '2026-11-22', 7, 'en')).toBe('From today');
		expect(windowFromNow('2026-11-24', '2026-11-23', 5, 'de')).toBe('Ab morgen');
		expect(windowFromNow('2026-11-21', '2026-11-22', 7, 'en')).toBe('From yesterday');
		// A past Thursday is never "Thursday", which reads as the coming one.
		expect(windowFromNow('2026-11-18', '2026-11-22', 7, 'de')).toBe('Ab Mi 18. Nov.');
		expect(windowFromNow('2026-11-27', '2026-11-22', 7, 'en')).toBe('From Fri Nov 27');
		expect(windowFromNow('2026-12-03', '2026-11-22', 5, 'de')).toBe('Ab Do 3. Dez.');
	});
});
