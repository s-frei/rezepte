import { describe, expect, it } from 'vitest';
import { claudeCodeSnippet, jsonSnippet, mcpUrl, offersMcp } from './mcp';

describe('mcp snippets', () => {
	const url = mcpUrl('https://rezepte.example');

	it('url', () => expect(url).toBe('https://rezepte.example/mcp'));

	it('claude code', () =>
		expect(claudeCodeSnippet(url, 'rzp_x')).toBe(
			'claude mcp add --transport http rezepte https://rezepte.example/mcp --header "Authorization: Bearer rzp_x"'
		));

	it('json', () =>
		expect(JSON.parse(jsonSnippet(url, 'rzp_x'))).toEqual({
			mcpServers: {
				rezepte: {
					type: 'http',
					url: 'https://rezepte.example/mcp',
					headers: { Authorization: 'Bearer rzp_x' }
				}
			}
		}));

	it('only recipe tokens get a snippet', () => {
		expect(offersMcp(['recipes:read'])).toBe(true);
		expect(offersMcp(['users:read', 'users:write'])).toBe(false);
	});
});
