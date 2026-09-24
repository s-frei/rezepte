const TINTS = ['tint-1', 'tint-2', 'tint-3'] as const;

export type Tint = (typeof TINTS)[number];

/**
 * Deterministically maps a recipe id to one of the three placeholder tint
 * tokens (`tint-1` / `tint-2` / `tint-3`).
 *
 * Used to give a recipe without a cover photo a stable background color
 * (the same id always renders in the same tint), while spreading recipes
 * roughly evenly across the three tones. The hash itself has no meaning
 * beyond that - it's not a content fingerprint.
 */
export function tintFor(id: string): Tint {
	let hash = 0;
	for (let i = 0; i < id.length; i++) {
		// `| 0` keeps the running hash a 32-bit signed integer, both to match
		// common string-hash implementations and to keep `Math.abs` below safe
		// (its largest possible input is 2^31, well within Number.MAX_SAFE_INTEGER).
		hash = (hash * 31 + id.charCodeAt(i)) | 0;
	}
	return TINTS[Math.abs(hash) % TINTS.length];
}
