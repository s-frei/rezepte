import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';
import { covers } from './covers';

type Row = { have: string; haveKey: string; need: string; needKey: string; covers: boolean };

const rows: Row[] = JSON.parse(
	readFileSync(
		new URL('../../../../service/internal/ingredient/testdata/covers.json', import.meta.url),
		'utf8'
	)
);

describe('covers', () => {
	it.each(rows)('$have / $need', (row) => {
		expect(covers(row.haveKey, row.needKey)).toBe(row.covers);
	});
});
