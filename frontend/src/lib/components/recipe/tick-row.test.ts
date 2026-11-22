import { describe, expect, it } from 'vitest';
import { render } from 'svelte/server';
import TickRow from './TickRow.svelte';

const item = (name: string) => ({ name, key: name.toLowerCase() });
const noop = () => {};
const props = { ticked: [], hint: false, ontoggle: noop, onclear: noop, ondone: noop };

// Clicking "+N more" needs a DOM, which the unit project lacks; the page's
// e2e covers it once the row is placed (Task 8).
describe('TickRow', () => {
	it('caps a plain row at eleven pills behind "+N more"', () => {
		const row = Array.from({ length: 14 }, (_, i) => item(`Item${i}`));
		const { body } = render(TickRow, { props: { ...props, row, focus: null } });
		expect(body).toContain('+3 more');
		expect(body.match(/<button[^>]*md:hidden[^>]*>/g)).toHaveLength(3);
	});

	it('says all is there for a focused card that misses nothing, without an empty group', () => {
		const { body } = render(TickRow, {
			props: { ...props, row: [], focus: { title: 'Potato salad', missing: [] } }
		});
		expect(body).toContain('All there for');
		expect(body).toContain('Potato salad');
		expect(body).not.toContain('role="group"');
	});
});
