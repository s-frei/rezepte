import { describe, expect, it, vi } from 'vitest';
import { ApiError } from '$lib/api/client';
import { lockNotice, policyHint, refusalOr } from './access';

// Pinned to English, the same mock shape form.test.ts uses, so the expected
// sentences do not depend on the locale the test run happens to resolve.
vi.mock('$lib/paraglide/runtime', () => ({
	getLocale: () => 'en',
	experimentalStaticLocale: undefined
}));

const base = {
	locked: true,
	createdBy: { id: 'a', username: 'anna', displayName: 'Anna', color: 'amber' as const }
};

describe('lockNotice', () => {
	it('is null for an open recipe', () => {
		expect(lockNotice({ ...base, locked: false }, 'b')).toBeNull();
	});
	it('names the author for everyone else', () => {
		expect(lockNotice(base, 'b')).toBe('Only Anna and admins can edit this recipe.');
	});
	it('addresses the author directly', () => {
		expect(lockNotice(base, 'a')).toBe('Only you and admins can edit this recipe.');
	});
});

describe('policyHint', () => {
	const author = { isAuthor: true, name: 'Anna' };
	const other = { isAuthor: false, name: 'Anna' };

	it('reads the household state for Default', () => {
		expect(policyHint('default', false, other)).toBe(
			'Follows the household setting: right now everyone may edit this recipe.'
		);
		expect(policyHint('default', true, author)).toBe(
			'Follows the household setting: right now only you and admins may edit this recipe.'
		);
		expect(policyHint('default', true, other)).toBe(
			'Follows the household setting: right now only Anna and admins may edit this recipe.'
		);
	});
	it('says only that Default follows the household while its state is unknown', () => {
		expect(policyHint('default', null, other)).toBe('Follows the household setting.');
	});
	it('addresses the author and names them to everyone else', () => {
		expect(policyHint('open', false, author)).toBe(
			'Everyone in the household may edit this recipe. Only you and admins can delete it.'
		);
		expect(policyHint('open', false, other)).toBe(
			'Everyone in the household may edit this recipe. Only Anna and admins can delete it.'
		);
		expect(policyHint('locked', false, author)).toBe('Only you and admins may edit this recipe.');
		expect(policyHint('locked', false, other)).toBe('Only Anna and admins may edit this recipe.');
	});
});

describe('refusalOr', () => {
	it('names the lost right for a 403', () => {
		expect(refusalOr(new ApiError(403, { title: 'Forbidden' }), 'Recipe could not be saved.')).toBe(
			'You can no longer edit this recipe.'
		);
	});
	it('falls back for any other failure', () => {
		expect(refusalOr(new ApiError(500, { title: 'Oops' }), 'Recipe could not be saved.')).toBe(
			'Recipe could not be saved.'
		);
		expect(refusalOr(new TypeError('offline'), 'Cover could not be set.')).toBe(
			'Cover could not be set.'
		);
	});
});
