const BASE = '/api/v1';

export type FieldError = { location: string; message: string };

type Problem = {
	title?: string;
	detail?: string;
	status?: number;
	errors?: Array<{ location?: string; message?: string }>;
};

export class ApiError extends Error {
	readonly status: number;
	readonly title: string;
	readonly detail?: string;
	readonly errors: FieldError[];

	constructor(status: number, problem: Problem) {
		super(problem.detail ?? problem.title ?? `HTTP ${status}`);
		this.name = 'ApiError';
		this.status = status;
		this.title = problem.title ?? `HTTP ${status}`;
		this.detail = problem.detail;
		this.errors = (problem.errors ?? []).map((e) => ({
			location: e.location ?? '',
			message: e.message ?? ''
		}));
	}
}

/** Calls the JSON API. Resolves with the parsed body, or undefined for empty responses. */
export async function api<T>(path: string, init: RequestInit = {}): Promise<T> {
	const headers = new Headers(init.headers);
	headers.set('Accept', 'application/json');
	if (init.body !== undefined && !headers.has('Content-Type')) {
		headers.set('Content-Type', 'application/json');
	}
	const res = await fetch(BASE + path, { ...init, headers, credentials: 'same-origin' });
	if (!res.ok) {
		const problem = await res.json().catch(() => ({}) as Problem);
		throw new ApiError(res.status, problem);
	}
	if (res.status === 204) {
		return undefined as T;
	}
	const text = await res.text();
	if (text.length === 0) {
		return undefined as T;
	}
	return JSON.parse(text) as T;
}
