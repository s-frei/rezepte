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
			/<span[^>]*aria-hidden="true"[^>]*data-required-mark[^>]*>(<!--[^>]*-->)*<span[^>]*>\*<\/span>/
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

	it('turns the star into a check and drops the stroke once the field has content', () => {
		const body = required({ value: 'Kürbissuppe' });
		expect(body).toMatch(/data-required-mark[^>]*data-state="filled"/);
		expect(body).toMatch(
			/data-required-mark[^>]*>(<!--[^>]*-->)*<span[^>]*>(<!--[^>]*-->)*<svg[\s\S]*<span class="invisible">\*<\/span>/
		);
		expect(body).toMatch(/data-required-stroke[^>]*data-state="filled"/);
	});

	it('lets the form decide when a typed value is enough', () => {
		const { body } = render(Input, {
			props: { id: 's', label: 'Portionen', required: true, value: '0', satisfied: false }
		});
		expect(body).toMatch(/data-required-mark[^>]*data-state="empty"/);
		expect(body).not.toContain('<svg');
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

describe('Input with a counter', () => {
	const counting = (value: string, hint?: string) =>
		render(Input, { props: { id: 'n', label: 'Name', counter: 10, value, hint } }).body;

	it('reserves no padding for the counter and shows none while the value is short', () => {
		const body = counting('abc');
		expect(body).not.toContain('pr-16');
		expect(body).not.toContain('data-slot="counter"');
	});

	it('puts the counter on the line below the field, beside the hint', () => {
		const body = counting('abcdefgh', 'Hint');
		expect(body).toMatch(/<p id="n-hint"[^>]*>Hint<\/p>[\s\S]*data-slot="counter"/);
		expect(body.indexOf('data-slot="counter"')).toBeGreaterThan(body.indexOf('</div>'));
	});
});

describe('Input with an error', () => {
	it('stays red while focused', () => {
		const { body } = render(Input, { props: { id: 't', label: 'T', error: 'Nope' } });
		expect(body).toContain('focus:border-destructive');
		expect(body).toContain('focus-visible:outline-destructive');
		expect(body).not.toContain('focus:border-primary');
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
