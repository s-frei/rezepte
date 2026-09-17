import { describe, expect, it } from 'vitest';
import { formatMinutes, formatQuantity } from './format';

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

	it('renders other decimals with up to two places and a German comma', () => {
		expect(formatQuantity(1.1)).toBe('1,1');
		expect(formatQuantity(0.4)).toBe('0,4');
		expect(formatQuantity(1.45)).toBe('1,45');
	});

	it('rounds to two decimals when given more precision', () => {
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
	});
});

describe('formatMinutes', () => {
	it('renders null as an empty string', () => {
		expect(formatMinutes(null)).toBe('');
	});

	it('renders under an hour as minutes', () => {
		expect(formatMinutes(0)).toBe('0 Min');
		expect(formatMinutes(30)).toBe('30 Min');
		expect(formatMinutes(59)).toBe('59 Min');
	});

	it('renders exactly one hour without a minutes part', () => {
		expect(formatMinutes(60)).toBe('1 Std');
	});

	it('renders an hour with a remainder', () => {
		expect(formatMinutes(90)).toBe('1 Std 30 Min');
	});

	it('renders multiple hours without a remainder', () => {
		expect(formatMinutes(120)).toBe('2 Std');
	});

	it('renders multiple hours with a remainder', () => {
		expect(formatMinutes(125)).toBe('2 Std 5 Min');
	});
});
