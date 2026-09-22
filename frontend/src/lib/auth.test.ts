import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { User } from './api/auth';

// The catalogue the mocked runtime pretends the browser is rendering. The
// cookie is what Paraglide reads it from, so this stands in for the cookie.
let cookieLocale = 'en';

vi.mock('$lib/paraglide/runtime', () => ({
	getLocale: () => cookieLocale,
	// The real setLocale writes the cookie and then reloads; the write is the
	// half that matters for the loop argument below.
	setLocale: vi.fn((locale: string) => {
		cookieLocale = locale;
	})
}));

const { getLocale, setLocale } = await import('$lib/paraglide/runtime');
// The module is `auth.svelte.ts`; the test file cannot carry the `.svelte`
// infix, because vitest reserves `*.svelte.test.ts` for a different project.
const { adoptAccountLocale } = await import('./auth.svelte');

function userWith(locale: 'en' | 'de'): User {
	return {
		id: 'u1',
		username: 'sam',
		displayName: 'Sam',
		role: 'user',
		color: 'amber',
		locale
	};
}

describe('adoptAccountLocale', () => {
	beforeEach(() => {
		cookieLocale = 'en';
		vi.mocked(setLocale).mockClear();
	});

	it('leaves a browser that already renders the account language alone', () => {
		expect(adoptAccountLocale(userWith('en'))).toBe(false);
		expect(setLocale).not.toHaveBeenCalled();
	});

	it('adopts the account language when the cookie went stale elsewhere', () => {
		expect(adoptAccountLocale(userWith('de'))).toBe(true);
		expect(setLocale).toHaveBeenCalledWith('de');
	});

	it('leaves the reload to Paraglide instead of asking for none', () => {
		adoptAccountLocale(userWith('de'));
		// No `{ reload: false }`: the cookie is stale here, so Paraglide's own
		// guard sees a change and reloads. Passing options would suppress it.
		expect(setLocale).toHaveBeenCalledWith('de');
	});

	it('does not run twice, because the reload lands on a matching cookie', () => {
		expect(adoptAccountLocale(userWith('de'))).toBe(true);
		// What the page looks like after setLocale wrote the cookie and the
		// browser came back: same account row, same answer from /auth/me.
		expect(getLocale()).toBe('de');
		expect(adoptAccountLocale(userWith('de'))).toBe(false);
		expect(setLocale).toHaveBeenCalledTimes(1);
	});
});
