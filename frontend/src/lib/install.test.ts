import { describe, expect, it } from 'vitest';
import { appCardState, installOffer, isIos, writeDismissed } from './install';

const none = { secure: true, standalone: false, installed: false, ios: false, hasPrompt: false };

describe('installOffer', () => {
	it('nothing without a secure context', () => {
		expect(installOffer({ ...none, secure: false, hasPrompt: true })).toBeNull();
	});
	it('nothing inside the installed app', () => {
		expect(installOffer({ ...none, standalone: true, hasPrompt: true })).toBeNull();
	});
	it('nothing once installed', () => {
		expect(installOffer({ ...none, installed: true, hasPrompt: true })).toBeNull();
	});
	it('the captured prompt offers the browser dialog', () => {
		expect(installOffer({ ...none, hasPrompt: true })).toBe('prompt');
	});
	it('an iPhone gets the steps', () => {
		expect(installOffer({ ...none, ios: true })).toBe('ios');
	});
	it('other browsers get nothing', () => {
		expect(installOffer(none)).toBeNull();
	});
});

describe('appCardState', () => {
	it('the offer shows as is', () => {
		expect(appCardState('prompt', { secure: true, standalone: false, owner: false })).toBe(
			'prompt'
		);
	});
	it('the owner learns about HTTPS', () => {
		expect(appCardState(null, { secure: false, standalone: false, owner: true })).toBe('https');
	});
	it('a member on HTTP sees no card', () => {
		expect(appCardState(null, { secure: false, standalone: false, owner: false })).toBeNull();
	});
	it('no card in the app', () => {
		expect(appCardState(null, { secure: true, standalone: true, owner: true })).toBeNull();
	});
});

describe('isIos', () => {
	it('an iPhone', () => {
		const ua =
			'Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1';
		expect(isIos(ua, 'iPhone', 5)).toBe(true);
	});
	it('an iPad in desktop mode', () => {
		const ua =
			'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15';
		expect(isIos(ua, 'MacIntel', 5)).toBe(true);
	});
	it('not a Mac', () => {
		const ua =
			'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15';
		expect(isIos(ua, 'MacIntel', 0)).toBe(false);
	});
	it('not Android', () => {
		const ua =
			'Mozilla/5.0 (Linux; Android 14; Pixel 7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36';
		expect(isIos(ua, 'Linux armv81', 5)).toBe(false);
	});
});

describe('writeDismissed', () => {
	it('dismissal survives storage that throws', () => {
		expect(() =>
			writeDismissed(() => ({
				setItem: () => {
					throw new Error('blocked');
				}
			}))
		).not.toThrow();
	});
});
