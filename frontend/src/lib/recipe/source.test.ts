import { describe, expect, it } from 'vitest';
import { sourceLabel } from './source';

describe('sourceLabel', () => {
	it('prefers the name', () => {
		expect(sourceLabel('Chefkoch', 'https://www.chefkoch.de/r/1')).toBe('Chefkoch');
	});

	it('trims the name', () => {
		expect(sourceLabel('  Tante Erika ', null)).toBe('Tante Erika');
	});

	it('falls back to the hostname without www', () => {
		expect(sourceLabel(null, 'https://www.chefkoch.de/r/1')).toBe('chefkoch.de');
		expect(sourceLabel('  ', 'https://localhost:8080/x')).toBe('localhost');
	});

	it('shows a host with umlauts as it was typed, not as punycode', () => {
		expect(sourceLabel(null, 'https://www.müller-küche.de/rezept')).toBe('müller-küche.de');
		expect(sourceLabel(null, 'https://WWW.Müller.de:8080/x')).toBe('müller.de');
	});

	it('still credits a link whose host is an IPv6 address', () => {
		expect(sourceLabel(null, 'https://[::1]:8080/x')).toBe('[::1]');
	});

	it('keeps a host stored as punycode as it is', () => {
		expect(sourceLabel(null, 'https://xn--mller-kva.de/x')).toBe('xn--mller-kva.de');
	});

	it('is null when there is nothing to credit', () => {
		expect(sourceLabel(null, null)).toBeNull();
		expect(sourceLabel('', 'not a url')).toBeNull();
	});
});
