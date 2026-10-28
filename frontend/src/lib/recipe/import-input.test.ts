import { describe, expect, it } from 'vitest';
import { detectInput, sharedInput } from './import-input';

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

describe('sharedInput', () => {
	const shared = (query: string) => sharedInput(new URLSearchParams(query));
	it('a shared url wins', () => {
		expect(shared('url=https://ex.com/r&text=Look')).toBe('https://ex.com/r');
	});
	it('a link inside text is taken out of it', () => {
		expect(shared('text=Carbonara https://ex.com/c so good')).toBe('https://ex.com/c');
	});
	it('trailing punctuation is not part of a link in text', () => {
		expect(shared('text=Try this (https://ex.com/c).')).toBe('https://ex.com/c');
	});
	it('a url that is not http falls back to the text', () => {
		expect(shared('url=intent://x&text=https://ex.com/a')).toBe('https://ex.com/a');
	});
	it('plain text stays text', () => {
		const text = '200 g flour\n2 eggs';
		expect(sharedInput(new URLSearchParams({ text }))).toBe(text);
	});
	it('title alone is not a recipe', () => {
		expect(shared('title=Hello')).toBe('');
	});
	it('nothing shared is empty', () => {
		expect(shared('')).toBe('');
	});
});
