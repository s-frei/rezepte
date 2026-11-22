/**
 * Whether a pantry ingredient (have) serves a recipe ingredient (need); both
 * are keys. Mirrors `Covers` in service/internal/ingredient/covers.go, which
 * owns the rule and the shared test vectors.
 */
export function covers(have: string, need: string): boolean {
	if (have === need) {
		return true;
	}
	const a = lastWord(have);
	const b = lastWord(need);
	return (
		b === have ||
		a === need ||
		(b.endsWith(have) && oneLongWord(have)) ||
		(a.endsWith(need) && oneLongWord(need))
	);
}

function lastWord(key: string): string {
	return (
		key
			.split(/[ -]/)
			.filter((w) => w !== '')
			.at(-1) ?? ''
	);
}

// Runes, not UTF-16 units, as in the Go original.
function oneLongWord(key: string): boolean {
	return [...key].length >= 4 && !/[ -]/.test(key);
}
