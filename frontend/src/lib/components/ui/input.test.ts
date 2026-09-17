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

describe('Textarea', () => {
	it('merges a caller-supplied class with the built-in classes', () => {
		const { body } = render(Textarea, {
			props: { id: 'description', label: 'Beschreibung', class: 'my-custom-class' }
		});
		expect(body).toContain('min-h-11');
		expect(body).toContain('my-custom-class');
	});
});
