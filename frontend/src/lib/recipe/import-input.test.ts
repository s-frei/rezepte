import { describe, expect, it } from 'vitest';
import { detectInput } from './import-input';

describe('detectInput', () => {
	it('is empty for blank input', () => {
		expect(detectInput('  \n ')).toEqual({ kind: 'empty' });
	});
	it('recognizes a single http(s) link and names its host without www', () => {
		expect(detectInput(' https://www.chefkoch.de/rezepte/1/a.html \n')).toEqual({
			kind: 'link',
			url: 'https://www.chefkoch.de/rezepte/1/a.html',
			host: 'chefkoch.de'
		});
	});
	it('treats other schemes, several words and plain text as text', () => {
		expect(detectInput('javascript:alert(1)').kind).toBe('text');
		expect(detectInput('https://a.example/x und noch was').kind).toBe('text');
		expect(detectInput('Käsespätzle\n400 g Mehl').kind).toBe('text');
	});
});
