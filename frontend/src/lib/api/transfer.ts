// Export and import talk zip, not JSON, so they bypass api(). The export and
// the PDF endpoint stream a download with progress through postForFile; the
// import needs XMLHttpRequest because fetch cannot report upload progress.
import { ApiError, type Problem } from './client';
import { uploadWithProgress } from './upload';

export type ImportResult = { created: { id: string; slug: string; title: string }[] };

/**
 * POSTs body as JSON and reads the file it answers with: a problem becomes an
 * ApiError, progress counts against Content-Length, and the name comes from
 * `filename*` (RFC 6266) before `filename`.
 */
export async function postForFile(
	path: string,
	body: unknown,
	accept: string,
	fallbackName: string,
	onProgress?: (loaded: number, total: number | null) => void,
	signal?: AbortSignal
): Promise<{ blob: Blob; filename: string }> {
	const res = await fetch(path, {
		method: 'POST',
		signal,
		credentials: 'same-origin',
		headers: { 'Content-Type': 'application/json', Accept: `${accept}, application/problem+json` },
		body: JSON.stringify(body)
	});
	if (!res.ok) {
		const problem = (await res.json().catch(() => ({}))) as Problem;
		throw new ApiError(res.status, problem);
	}
	const total = Number(res.headers.get('Content-Length')) || null;
	const disposition = res.headers.get('Content-Disposition') ?? '';
	const star = /filename\*\s*=\s*utf-8''([^;]+)/i.exec(disposition)?.[1];
	let filename = /filename="([^"]+)"/.exec(disposition)?.[1] ?? fallbackName;
	if (star) {
		try {
			filename = decodeURIComponent(star.trim());
		} catch {
			// Malformed escape: keep the plain filename.
		}
	}
	const type = res.headers.get('Content-Type') ?? accept.split(',')[0];
	if (!res.body) return { blob: await res.blob(), filename };
	const reader = res.body.getReader();
	const chunks: Uint8Array<ArrayBuffer>[] = [];
	let loaded = 0;
	for (;;) {
		const { done, value } = await reader.read();
		if (done) break;
		chunks.push(value);
		loaded += value.length;
		onProgress?.(loaded, total);
	}
	return { blob: new Blob(chunks, { type }), filename };
}

export function exportRecipes(
	ids: string[],
	onProgress: (loaded: number, total: number | null) => void
): Promise<{ blob: Blob; filename: string }> {
	return postForFile(
		'/api/v1/export',
		{ recipeIds: ids },
		'application/zip',
		'rezepte.zip',
		onProgress
	);
}

export function importZip(
	zip: Uint8Array,
	onUpload: (fraction: number) => void
): Promise<ImportResult> {
	return uploadWithProgress<ImportResult>('/import', zip, {
		contentType: 'application/zip',
		onProgress: onUpload
	});
}

export function saveBlob(blob: Blob, filename: string) {
	const url = URL.createObjectURL(blob);
	const a = document.createElement('a');
	a.href = url;
	a.download = filename;
	// Firefox only follows an anchor that is in the document, and revoking the
	// URL right after the click can cancel the download before it starts.
	document.body.append(a);
	a.click();
	a.remove();
	setTimeout(() => URL.revokeObjectURL(url), 60_000);
}
