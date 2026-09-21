import { beforeEach, describe, expect, it, vi } from 'vitest';

let locale: 'en' | 'de' = 'en';
vi.mock('$lib/paraglide/runtime', () => ({
	getLocale: () => locale,
	experimentalStaticLocale: undefined
}));

const { formatDate, formatFactor, formatMinutes, formatQuantity, formatServings, servingsUnit } =
	await import('./format');

// Every test starts in English; a test that needs German sets `locale = 'de'`
// itself. Without this reset, a locale left over from an earlier test would
// leak into the next one, since `locale` is shared module state for the
// whole file.
beforeEach(() => {
	locale = 'en';
});

describe('formatQuantity', () => {
	it('renders null as an empty string', () => {
		expect(formatQuantity(null)).toBe('');
	});

	it('renders whole numbers plain', () => {
		expect(formatQuantity(0)).toBe('0');
		expect(formatQuantity(2)).toBe('2');
		expect(formatQuantity(500)).toBe('500');
	});

	it('renders a half as a bare glyph below one', () => {
		expect(formatQuantity(0.5)).toBe('½');
	});

	it('renders a half with a leading integer', () => {
		expect(formatQuantity(1.5)).toBe('1 ½');
	});

	it('renders a quarter', () => {
		expect(formatQuantity(0.25)).toBe('¼');
		expect(formatQuantity(2.25)).toBe('2 ¼');
	});

	it('renders three quarters', () => {
		expect(formatQuantity(0.75)).toBe('¾');
		expect(formatQuantity(3.75)).toBe('3 ¾');
	});

	it('renders a third', () => {
		expect(formatQuantity(0.33)).toBe('⅓');
		expect(formatQuantity(1.33)).toBe('1 ⅓');
	});

	it('renders two thirds', () => {
		expect(formatQuantity(0.67)).toBe('⅔');
		expect(formatQuantity(4.67)).toBe('4 ⅔');
	});

	it('renders fraction glyphs the same in both locales', () => {
		expect(formatQuantity(1.5)).toBe('1 ½');
		locale = 'de';
		expect(formatQuantity(1.5)).toBe('1 ½');
	});

	it('uses a decimal point in English', () => {
		expect(formatQuantity(1.1)).toBe('1.1');
		expect(formatQuantity(0.4)).toBe('0.4');
		expect(formatQuantity(1.45)).toBe('1.45');
	});

	it('uses a decimal comma in German', () => {
		locale = 'de';
		expect(formatQuantity(1.1)).toBe('1,1');
		expect(formatQuantity(0.4)).toBe('0,4');
		expect(formatQuantity(1.45)).toBe('1,45');
	});

	it('rounds to two decimals when given more precision', () => {
		expect(formatQuantity(1.456)).toBe('1.46');
		locale = 'de';
		expect(formatQuantity(1.456)).toBe('1,46');
	});

	it('regression: 1.25 is the quarter glyph, not decimal formatting', () => {
		expect(formatQuantity(1.25)).toBe('1 ¼');
	});

	it('regression: 2.5 is the half glyph, not decimal formatting', () => {
		expect(formatQuantity(2.5)).toBe('2 ½');
	});

	it('preserves the sign for negative quantities', () => {
		expect(formatQuantity(-1.5)).toBe('-1 ½');
		expect(formatQuantity(-1.1)).toBe('-1.1');
		locale = 'de';
		expect(formatQuantity(-1.1)).toBe('-1,1');
	});
});

describe('formatMinutes', () => {
	it('renders null as an empty string', () => {
		expect(formatMinutes(null)).toBe('');
	});

	it('renders under an hour as minutes', () => {
		expect(formatMinutes(0)).toBe('0 min');
		expect(formatMinutes(30)).toBe('30 min');
		expect(formatMinutes(59)).toBe('59 min');
	});

	it('renders exactly one hour without a minutes part', () => {
		expect(formatMinutes(60)).toBe('1 hr');
	});

	it('renders an hour with a remainder', () => {
		expect(formatMinutes(90)).toBe('1 hr 30 min');
	});

	it('renders multiple hours without a remainder', () => {
		expect(formatMinutes(120)).toBe('2 hr');
	});

	it('renders multiple hours with a remainder', () => {
		expect(formatMinutes(125)).toBe('2 hr 5 min');
	});
});

describe('formatFactor', () => {
	it.each([
		[4, 6, '1.5'],
		[4, 8, '2'],
		[4, 2, '0.5'],
		[4, 5, '1.25'],
		[3, 7, '2.33'],
		[4, 4, '1']
	])('renders %d → %d servings as ×%s in English', (from, to, expected) => {
		expect(formatFactor(from, to)).toBe(expected);
	});

	it.each([
		[4, 6, '1,5'],
		[4, 8, '2'],
		[4, 2, '0,5'],
		[4, 5, '1,25'],
		[3, 7, '2,33'],
		[4, 4, '1']
	])('renders %d → %d servings as ×%s in German', (from, to, expected) => {
		locale = 'de';
		expect(formatFactor(from, to)).toBe(expected);
	});
});

describe('servingsUnit and formatServings', () => {
	it('uses the singular for exactly one', () => {
		expect(servingsUnit(1)).toBe('serving');
		expect(formatServings(1)).toBe('1 serving');
	});

	it('uses the plural otherwise', () => {
		expect(servingsUnit(4)).toBe('servings');
		expect(formatServings(4)).toBe('4 servings');
		expect(formatServings(0)).toBe('0 servings');
	});
});

describe('formatDate', () => {
	it('renders an English long date', () => {
		expect(formatDate('2026-03-03T09:15:00Z')).toBe('March 3, 2026');
		expect(formatDate('2026-09-12T07:45:00Z')).toBe('September 12, 2026');
	});

	it('renders a German long date', () => {
		locale = 'de';
		expect(formatDate('2026-03-03T09:15:00Z')).toBe('3. März 2026');
		expect(formatDate('2026-09-12T07:45:00Z')).toBe('12. September 2026');
	});

	it('renders an unusable value as an empty string', () => {
		expect(formatDate('')).toBe('');
		expect(formatDate('not a date')).toBe('');
	});
});
