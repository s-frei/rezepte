import { describe, expect, it } from 'vitest';
import {
	isUserColor,
	leastUsedColor,
	USER_COLOR_CLASSES,
	USER_COLORS,
	userColorClasses,
	userComposerClass,
	userLeaderClass,
	userRingClass,
	type UserColor
} from './color';
import type { PersonEntry } from '#lib/api/users.js';

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

	it('turns the pair over when asked for the strong ground', () => {
		for (const color of USER_COLORS)
			expect(userColorClasses(color, true)).toBe(`bg-user-${color}-foreground text-user-${color}`);
		expect(userColorClasses('chartreuse', true)).toBe('bg-accent text-accent-foreground');
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

describe('userRingClass', () => {
	it('rings in the strong tone, and in the base tone on the inverse ground', () => {
		for (const color of USER_COLORS) {
			expect(userRingClass(color)).toBe(`outline-user-${color}-foreground`);
			expect(userRingClass(color, true)).toBe(`outline-user-${color}`);
		}
	});

	it('falls back to a muted ring for a color it does not know', () => {
		expect(userRingClass('chartreuse')).toBe('outline-text-muted');
		expect(userRingClass(null, true)).toBe('outline-inverse-muted');
	});
});

describe('userLeaderClass', () => {
	it('dots the leader in a calm strong tone, the handle for no one', () => {
		expect(userLeaderClass('sage')).toBe('border-user-sage-foreground/55');
		expect(userLeaderClass(undefined)).toBe('border-handle');
	});
});

describe('userComposerClass', () => {
	it("tints the focused composer in the writer's paper, nothing for an unknown color", () => {
		expect(userComposerClass('plum')).toBe('has-[textarea:focus-visible]:bg-user-plum');
		expect(userComposerClass('chartreuse')).toBe('');
	});
});

// Pins the same three cases as TestLeastUsed in
// service/internal/user/profile.go: this is the client's own copy of that
// rule, for the create dialog's preselection alone.
describe('leastUsedColor', () => {
	const wearing = (...colors: UserColor[]): PersonEntry[] =>
		colors.map((color, i) => ({
			id: String(i),
			username: `u${i}`,
			displayName: `U ${i}`,
			color,
			role: 'user',
			avatarId: null
		}));

	it('picks the first palette color for an empty instance', () => {
		expect(leastUsedColor([])).toBe('amber');
	});

	it('picks the first color nobody wears', () => {
		expect(leastUsedColor(wearing('amber', 'amber', 'clay'))).toBe('rose');
	});

	it('picks the one free color', () => {
		expect(leastUsedColor(wearing(...USER_COLORS.filter((c) => c !== 'rose')))).toBe('rose');
	});

	it('breaks a tie by palette order', () => {
		const twice = USER_COLORS.filter((c) => c !== 'teal' && c !== 'clay');
		expect(leastUsedColor(wearing(...twice, ...twice, 'teal', 'clay'))).toBe('clay');
	});
});
