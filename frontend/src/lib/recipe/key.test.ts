import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';
import { ingredientKey } from './key';

type Row = { have: string; haveKey: string; need: string; needKey: string };

const rows: Row[] = JSON.parse(
	readFileSync(
		new URL('../../../../service/internal/ingredient/testdata/covers.json', import.meta.url),
		'utf8'
	)
);

describe('ingredientKey', () => {
	it.each(rows)('$have / $need', (row) => {
		expect(ingredientKey(row.have)).toBe(row.haveKey);
		expect(ingredientKey(row.need)).toBe(row.needKey);
	});
});
