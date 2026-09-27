import { describe, expect, test } from 'bun:test';
import { keepCommittedVersion } from './openapi-version';

const fetched = { openapi: '3.1.0', info: { title: 'Rezepte API', version: '1.2.0-4-gabc1234' }, paths: {} };

describe('keepCommittedVersion', () => {
	test('replaces the build version with the committed one', () => {
		const committed = JSON.stringify({ info: { title: 'Rezepte API', version: '1.2.0' } });
		expect(keepCommittedVersion(fetched, committed)).toEqual({ ...fetched, info: { title: 'Rezepte API', version: '1.2.0' } });
	});

	test('keeps the key order, so the written file does not reshuffle', () => {
		const committed = JSON.stringify({ info: { version: '1.2.0' } });
		expect(Object.keys(keepCommittedVersion(fetched, committed).info)).toEqual(['title', 'version']);
	});

	test('takes the fetched version when nothing is committed yet', () => {
		expect(keepCommittedVersion(fetched, null)).toBe(fetched);
		expect(keepCommittedVersion(fetched, '{"paths":{}}')).toBe(fetched);
	});
});
