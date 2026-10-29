import { afterEach, describe, expect, it, vi } from 'vitest';

vi.mock('$app/environment', () => ({ browser: true }));

const { load, save } = await import('./storage');

afterEach(() => {
	vi.unstubAllGlobals();
});

describe('storage', () => {
	it('stores, reads and removes', () => {
		const data = new Map<string, string>();
		vi.stubGlobal('window', {
			localStorage: {
				getItem: (k: string) => data.get(k) ?? null,
				setItem: (k: string, v: string) => void data.set(k, v),
				removeItem: (k: string) => void data.delete(k)
			}
		});
		save('k', 'v');
		expect(load('k')).toBe('v');
		save('k', null);
		expect(load('k')).toBeNull();
		expect(data.size).toBe(0);
	});

	it('survives storage that throws on access', () => {
		vi.stubGlobal('window', {
			get localStorage() {
				throw new Error('blocked');
			}
		});
		expect(load('k')).toBeNull();
		expect(() => save('k', 'v')).not.toThrow();
		expect(() => save('k', null)).not.toThrow();
	});
});
