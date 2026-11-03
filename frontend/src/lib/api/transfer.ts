// Export and import talk zip, not JSON, so they bypass api(): the export
// streams a download with progress, the import needs XMLHttpRequest because
// fetch cannot report upload progress.
import { ApiError, type Problem } from './client';
import { uploadWithProgress } from './upload';

export type ImportResult = { created: { id: string; slug: string; title: string }[] };

export async function exportRecipes(
	ids: string[],
	onProgress: (loaded: number, total: number | null) => void
): Promise<{ blob: Blob; filename: string }> {
	const res = await fetch('/api/v1/export', {
		method: 'POST',
		credentials: 'same-origin',
		headers: {
			'Content-Type': 'application/json',
			Accept: 'application/zip, application/problem+json'
		},
		body: JSON.stringify({ recipeIds: ids })
	});
	if (!res.ok) {
		const problem = (await res.json().catch(() => ({}))) as Problem;
		throw new ApiError(res.status, problem);
	}
	const total = Number(res.headers.get('Content-Length')) || null;
	const filename =
		/filename="([^"]+)"/.exec(res.headers.get('Content-Disposition') ?? '')?.[1] ?? 'rezepte.zip';
	const reader = res.body!.getReader();
	const chunks: Uint8Array<ArrayBuffer>[] = [];
	let loaded = 0;
	for (;;) {
		const { done, value } = await reader.read();
		if (done) break;
		chunks.push(value);
		loaded += value.length;
		onProgress(loaded, total);
	}
	return { blob: new Blob(chunks, { type: 'application/zip' }), filename };
}

export function importZip(
	zip: Uint8Array,
	onUpload: (fraction: number) => void
): Promise<ImportResult> {
	return uploadWithProgress<ImportResult>('/import', zip, {
		contentType: 'application/zip',
		expect: 201,
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
