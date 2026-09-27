import { describe, expect, it } from 'vitest';
import { render } from 'svelte/server';
import Input from './Input.svelte';
import Textarea from './Textarea.svelte';

describe('Input', () => {
	it('merges a caller-supplied class with the built-in classes', () => {
		const { body } = render(Input, {
			props: { id: 'title', label: 'Titel', class: 'my-custom-class' }
		});
		expect(body).toContain('h-11');
		expect(body).toContain('my-custom-class');
	});
});

describe('Input marked required', () => {
	const required = (props: { value?: string; error?: string | null }) =>
		render(Input, { props: { id: 'title', label: 'Titel', required: true, ...props } }).body;

	it('tells assistive tech without the browser taking over validation', () => {
		const body = required({});
		expect(body).toContain('aria-required="true"');
		expect(body).not.toMatch(/<input[^>]*\srequired[\s>=]/);
	});

	it('hides the star from assistive tech, which already hears aria-required', () => {
		expect(required({})).toMatch(
			/<span[^>]*aria-hidden="true"[^>]*data-required-mark[^>]*>\*<\/span>/
		);
	});

	it('marks an empty field as still to do, star and margin stroke', () => {
		const body = required({ value: '' });
		expect(body).toMatch(/data-required-mark[^>]*data-state="empty"/);
		expect(body).toMatch(/data-required-stroke[^>]*data-state="empty"/);
	});

	it('treats whitespace as empty', () => {
		expect(required({ value: '   ' })).toMatch(/data-required-mark[^>]*data-state="empty"/);
	});

	it('quietens the star and drops the stroke once the field has content', () => {
		const body = required({ value: 'Kürbissuppe' });
		expect(body).toMatch(/data-required-mark[^>]*data-state="filled"/);
		expect(body).toMatch(/data-required-stroke[^>]*data-state="filled"/);
	});

	it('lets the error win over the empty state', () => {
		const body = required({ value: '', error: 'Bitte einen Titel eingeben' });
		expect(body).toMatch(/data-required-mark[^>]*data-state="error"/);
		expect(body).toMatch(/data-required-stroke[^>]*data-state="error"/);
	});

	it('draws neither star nor stroke on an optional field', () => {
		const { body } = render(Input, { props: { id: 'note', label: 'Notiz' } });
		expect(body).not.toContain('data-required-mark');
		expect(body).not.toContain('data-required-stroke');
		expect(body).not.toContain('aria-required');
	});
});

describe('Input with a prefix', () => {
	it('draws the prefix inside the field and describes the field with it', () => {
		const { body } = render(Input, {
			props: { id: 'src', label: 'Source', prefix: 'Adapted from' }
		});
		expect(body).toMatch(/<span[^>]*id="src-prefix"[^>]*>Adapted from<\/span>/);
		expect(body).toMatch(/aria-describedby="[^"]*src-prefix/);
	});

	it('draws no prefix when none is given', () => {
		const { body } = render(Input, { props: { id: 'src', label: 'Source' } });
		expect(body).not.toContain('src-prefix');
	});
});

describe('Textarea', () => {
	it('merges a caller-supplied class with the built-in classes', () => {
		const { body } = render(Textarea, {
			props: { id: 'description', label: 'Beschreibung', class: 'my-custom-class' }
		});
		expect(body).toContain('min-h-11');
		expect(body).toContain('my-custom-class');
	});
});
