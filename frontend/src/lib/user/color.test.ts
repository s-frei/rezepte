import { describe, expect, it } from 'vitest';
import {
	isUserColor,
	leastUsedColor,
	USER_COLOR_CLASSES,
	USER_COLORS,
	userColorClasses
} from './color';

describe('the user color palette', () => {
	it('holds the eight palette tokens in palette order', () => {
		expect(USER_COLORS).toEqual([
			'amber',
			'clay',
			'rose',
			'plum',
			'sage',
			'olive',
			'teal',
			'slate'
		]);
	});

	it('maps every color to classes, and nothing else', () => {
		expect(Object.keys(USER_COLOR_CLASSES).sort()).toEqual([...USER_COLORS].sort());
		for (const color of USER_COLORS) {
			expect(USER_COLOR_CLASSES[color]).toContain(`bg-user-${color}`);
			expect(USER_COLOR_CLASSES[color]).toContain(`text-user-${color}-foreground`);
		}
	});

	it('renders a known color with its own classes', () => {
		expect(userColorClasses('teal')).toBe(USER_COLOR_CLASSES.teal);
	});

	it('falls back to the neutral accent for anything it does not know', () => {
		const neutral = 'bg-accent text-accent-foreground';
		expect(userColorClasses('chartreuse')).toBe(neutral);
		expect(userColorClasses(undefined)).toBe(neutral);
		expect(userColorClasses(null)).toBe(neutral);
	});

	it('recognizes its own members', () => {
		expect(isUserColor('sage')).toBe(true);
		expect(isUserColor('SAGE')).toBe(false);
	});
});

// Pins the same three cases as TestLeastUsed in
// service/internal/user/profile.go: this is the client's own copy of that
// rule, for the create dialog's preselection alone.
describe('leastUsedColor', () => {
	it('picks the first palette color for an empty instance', () => {
		expect(leastUsedColor([])).toBe('amber');
	});

	it('picks the one free color', () => {
		const usage = USER_COLORS.map((color) => ({ color, count: color === 'rose' ? 0 : 1 }));
		expect(leastUsedColor(usage)).toBe('rose');
	});

	it('breaks a tie by palette order', () => {
		const usage = USER_COLORS.map((color) => ({
			color,
			count: color === 'teal' || color === 'clay' ? 1 : 2
		}));
		expect(leastUsedColor(usage)).toBe('clay');
	});
});
