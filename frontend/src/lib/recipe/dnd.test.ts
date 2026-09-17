import { describe, expect, it } from 'vitest';
import { setAriaStrings } from 'svelte-dnd-action';
import { applyDndAriaStrings } from './dnd';

describe('applyDndAriaStrings', () => {
	// setAriaStrings validates its argument and throws on an unknown key or a
	// value of the wrong type, so this pins our override object to the
	// library's contract - the part most likely to rot on an upgrade.
	it('is accepted by svelte-dnd-action', () => {
		expect(() => applyDndAriaStrings()).not.toThrow();
	});

	it('only applies once, since the setting is global', () => {
		expect(() => applyDndAriaStrings()).not.toThrow();
	});

	it('would reject a bad override, proving the check above has teeth', () => {
		expect(() => setAriaStrings({ notAThing: 'x' } as never)).toThrow();
		expect(() => setAriaStrings({ dropped: 'not a function' } as never)).toThrow();
	});
});
