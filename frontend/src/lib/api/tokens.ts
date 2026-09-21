import { api } from './client';

/** What a token may do. Write always travels together with its read scope. */
export type TokenScope = 'recipes:read' | 'recipes:write' | 'users:read' | 'users:write';

/** The lifetimes the create form offers; `null` means the token never expires. */
export type TokenExpiry = 30 | 90 | 365 | null;

export type ApiToken = {
	id: string;
	name: string;
	/** First characters of the raw token, for matching a row to a client config. */
	prefix: string;
	scopes: TokenScope[];
	ownerId: string;
	ownerName: string;
	createdAt: string;
	/** Absent when the token never expires. */
	expiresAt?: string;
	/** Accurate to the hour. Absent when the token was never used. */
	lastUsedAt?: string;
};

/** Only the create response carries the secret, and only once. */
export type CreatedApiToken = ApiToken & { token: string };

export async function listTokens(): Promise<ApiToken[]> {
	const list = await api<{ items: ApiToken[] }>('/tokens');
	return list.items;
}

export function createToken(input: {
	name: string;
	scopes: TokenScope[];
	expiresInDays: TokenExpiry;
}): Promise<CreatedApiToken> {
	// The key is omitted rather than sent as null: the server's field is an
	// optional integer with an enum, and an explicit null is not one of its
	// allowed values.
	const body: Record<string, unknown> = { name: input.name, scopes: input.scopes };
	if (input.expiresInDays !== null) {
		body.expiresInDays = input.expiresInDays;
	}
	return api<CreatedApiToken>('/tokens', { method: 'POST', body: JSON.stringify(body) });
}

export function deleteToken(id: string): Promise<void> {
	return api<void>(`/tokens/${encodeURIComponent(id)}`, { method: 'DELETE' });
}
