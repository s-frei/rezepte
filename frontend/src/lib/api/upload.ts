import { ApiError, redirectToLogin, type Problem } from './client';

const BASE = '/api/v1';

/**
 * POSTs a body with upload progress; fetch cannot report that. Resolves with
 * the parsed JSON on the `expect` status and rejects like `api()` does:
 * `ApiError(status, problem)`, with status 0 for network, abort and timeout.
 */
export function uploadWithProgress<T>(
	path: string,
	body: FormData | Uint8Array,
	opts: { contentType?: string; expect: number; onProgress?: (fraction: number) => void }
): Promise<T> {
	return new Promise((resolve, reject) => {
		const xhr = new XMLHttpRequest();
		xhr.open('POST', BASE + path);
		// FormData sets its own multipart Content-Type with the boundary.
		if (opts.contentType) xhr.setRequestHeader('Content-Type', opts.contentType);
		xhr.setRequestHeader('Accept', 'application/json');
		xhr.upload.onprogress = (e) => {
			if (e.lengthComputable) opts.onProgress?.(e.loaded / e.total);
		};
		// A small body may finish without any progress event; this one always fires.
		xhr.upload.onload = () => opts.onProgress?.(1);
		xhr.onload = () => {
			let parsed: unknown = {};
			try {
				parsed = JSON.parse(xhr.responseText);
			} catch {
				// keep {}
			}
			if (xhr.status === opts.expect) {
				resolve(parsed as T);
				return;
			}
			if (
				xhr.status === 401 &&
				typeof window !== 'undefined' &&
				window.location.pathname !== '/login'
			) {
				redirectToLogin();
			}
			reject(new ApiError(xhr.status, parsed as Problem));
		};
		xhr.onerror = () => reject(new ApiError(0, { title: 'Network error' }));
		xhr.onabort = () => reject(new ApiError(0, { title: 'Upload aborted' }));
		xhr.ontimeout = () => reject(new ApiError(0, { title: 'Upload timed out' }));
		xhr.send(body as FormData | Uint8Array<ArrayBuffer>);
	});
}
