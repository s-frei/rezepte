import { describe, expect, it } from 'vitest';
import { render } from 'svelte/server';
import AiInstruction from './AiInstruction.svelte';

describe('AiInstruction', () => {
	it('says what the instruction is and ties the sentence to its button', () => {
		const { body } = render(AiInstruction, { props: { kind: 'list' } });
		expect(body).toContain('List from an AI?');
		expect(body).toContain('Copy AI instruction');
		const id = body.match(/aria-describedby="([^"]+)"/)?.[1];
		expect(id).toBeTruthy();
		expect(body).toMatch(new RegExp(`<p id="${id}"[^>]*>A text for ChatGPT[^<]*shopping lists`));
	});
});
