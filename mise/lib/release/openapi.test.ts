import { describe, expect, test } from 'bun:test';
import { checkSpec, specVersion, stampSpec } from './openapi';

const committed = `${JSON.stringify({ openapi: '3.1.0', info: { title: 'Rezepte API', version: 'dev' }, paths: {} }, null, 2)}\n`;

describe('specVersion', () => {
	test('drops the tag prefix, as the build does', () => {
		expect(specVersion('v1.2.0')).toBe('1.2.0');
		expect(specVersion('v1.5.0-rc1')).toBe('1.5.0-rc1');
	});
});

describe('stampSpec', () => {
	test('sets info.version and changes nothing else', () => {
		const stamped = stampSpec(committed, 'v1.2.0');
		expect(stamped).toBe(committed.replace('"version": "dev"', '"version": "1.2.0"'));
	});

	test('is idempotent', () => {
		const once = stampSpec(committed, 'v1.2.0');
		expect(stampSpec(once, 'v1.2.0')).toBe(once);
	});

	test('refuses a document without info', () => {
		expect(() => stampSpec('{"paths":{}}', 'v1.2.0')).toThrow('no info object');
	});
});

describe('checkSpec', () => {
	test('accepts the stamped document', () => {
		expect(checkSpec(stampSpec(committed, 'v1.5.0-rc1'), 'v1.5.0-rc1')).toBeNull();
	});

	test('names the version it found and the one it wants', () => {
		expect(checkSpec(committed, 'v1.2.0')).toContain('info.version "dev", not "1.2.0"');
	});

	test('reports a document that is not JSON', () => {
		expect(checkSpec('<html>', 'v1.2.0')).toContain('is not JSON');
	});
});
