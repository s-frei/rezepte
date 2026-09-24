import { summarizeSpec, type ApiSummary, type SpecDocument } from '$lib/settings/api-summary';
import { api } from './client';

/** The OpenAPI document the instance serves, reduced to its two figures. */
export async function fetchApiSummary(): Promise<ApiSummary> {
	return summarizeSpec(await api<SpecDocument>('/openapi.json'));
}

/**
 * The version of the running instance, from `/healthz`. That route sits
 * outside `/api/v1`, needs no session and is the one place an operator can
 * read what is running, so this deliberately bypasses `api()` and its base
 * path and 401 redirect handling.
 */
export async function fetchVersion(): Promise<string | undefined> {
	const res = await fetch('/healthz', { headers: { Accept: 'application/json' } });
	if (!res.ok) {
		return undefined;
	}
	const body = (await res.json()) as { version?: unknown };
	return typeof body.version === 'string' ? body.version : undefined;
}
