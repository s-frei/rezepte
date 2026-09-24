import type { ZxcvbnFactory } from '@zxcvbn-ts/core';
import { m } from '$lib/paraglide/messages';

/** zxcvbn's score: 0 is guessed within seconds, 4 holds out for centuries. */
export type Strength = 0 | 1 | 2 | 3 | 4;

// Words every password on this instance is near, whoever sets it.
const INSTANCE_WORDS = ['rezepte', 'recipes'];

let estimator: Promise<ZxcvbnFactory> | null = null;

// Loaded on the first estimate, not with the module: the common-password list
// is some 230 KB gzipped, and only the three forms that set a password need it.
// The language packs are left out on purpose - German and English together add
// another megabyte for names and Wikipedia words the leaked-password list
// mostly covers already.
function loadEstimator(): Promise<ZxcvbnFactory> {
	estimator ??= Promise.all([import('@zxcvbn-ts/core'), import('@zxcvbn-ts/language-common')]).then(
		([core, common]) =>
			new core.ZxcvbnFactory({
				dictionary: common.dictionary,
				graphs: common.adjacencyGraphs
			})
	);
	return estimator;
}

/**
 * Scores a password against the common-password list, keyboard patterns and
 * `userInputs` - the account's own names, which make a password that contains
 * them weak for exactly that account. `null` for an empty password.
 */
export async function estimateStrength(
	password: string,
	userInputs: string[] = []
): Promise<Strength | null> {
	if (password === '') {
		return null;
	}
	const zxcvbn = await loadEstimator();
	return zxcvbn.check(password, [...INSTANCE_WORDS, ...words(userInputs)]).score as Strength;
}

// Each input whole and word by word: a display name like "Anna Berg" is
// matched as "annaberg" only when whole, and "berg1990" should be weak too.
function words(inputs: string[]): string[] {
	return inputs.flatMap((input) => {
		const parts = input.split(/\s+/).filter((part) => part !== '');
		return parts.length > 1 ? [parts.join(''), ...parts] : parts;
	});
}

export function strengthLabel(strength: Strength): string {
	return [
		m.settings_password_strength_very_weak,
		m.settings_password_strength_weak,
		m.settings_password_strength_fair,
		m.settings_password_strength_good,
		m.settings_password_strength_strong
	][strength]();
}
