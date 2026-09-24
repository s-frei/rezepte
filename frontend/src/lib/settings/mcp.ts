import type { TokenScope } from '$lib/api/tokens';

/** The MCP endpoint of the instance the browser is talking to. */
export function mcpUrl(origin: string): string {
	return `${origin}/mcp`;
}

/** A `claude mcp add` command that registers this instance with Claude Code. */
export function claudeCodeSnippet(url: string, token: string): string {
	return `claude mcp add --transport http rezepte ${url} --header "Authorization: Bearer ${token}"`;
}

/**
 * The `mcpServers` block Cursor and Claude Code's project `.mcp.json` read.
 * VS Code's `mcp.json` expects the same entry under a top-level `servers`
 * key instead.
 */
export function jsonSnippet(url: string, token: string): string {
	return JSON.stringify(
		{
			mcpServers: { rezepte: { type: 'http', url, headers: { Authorization: `Bearer ${token}` } } }
		},
		null,
		2
	);
}

/** Every MCP tool needs recipes:read, so only such a token gets a snippet. */
export function offersMcp(scopes: readonly TokenScope[]): boolean {
	return scopes.includes('recipes:read');
}
