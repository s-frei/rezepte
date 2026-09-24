import { expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import { COPIES } from './brand-assets';

const root = new URL('../../../', import.meta.url);
const docs = new URL('../', import.meta.url);

// The docs site cannot reach outside its own folder at build time, so it
// carries copies of the logo files; they must never drift from the sources.
test.each(COPIES)('%s is an exact copy of %s', (target, source) => {
	expect(readFileSync(new URL(target, docs))).toEqual(readFileSync(new URL(source, root)));
});

test('the navigation and the docs home show the lockups', () => {
	expect(readFileSync(new URL('lib/layout.shared.tsx', docs), 'utf8')).toContain('<Lockup variant="compact"');
	expect(readFileSync(new URL('content/index.mdx', docs), 'utf8')).toContain('<Lockup variant="horizontal"');
});
