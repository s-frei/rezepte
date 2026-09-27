/**
 * The errors minus `keys`, for clearing a field's error as soon as someone
 * types in it: an error is the answer to the last save, and once the field
 * has changed it no longer describes what is there. New errors still only
 * come from the next save.
 *
 * Returns the same object when there is nothing to drop, so a keystroke in
 * a field without an error does not wake every reader of the record.
 */
export function withoutErrors<T extends Partial<Record<string, string>>>(
	errors: T,
	keys: (keyof T & string)[]
): T {
	if (!keys.some((key) => errors[key] !== undefined)) {
		return errors;
	}
	const rest = { ...errors };
	for (const key of keys) {
		delete rest[key];
	}
	return rest;
}
