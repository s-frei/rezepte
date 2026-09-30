// Export and import talk zip, not JSON, so they bypass api(): the export
// streams a download with progress, the import needs XMLHttpRequest because
// fetch cannot report upload progress.
import { ApiError, type Problem } from './client';

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
	return new Promise((resolve, reject) => {
		const xhr = new XMLHttpRequest();
		xhr.open('POST', '/api/v1/import');
		xhr.setRequestHeader('Content-Type', 'application/zip');
		xhr.setRequestHeader('Accept', 'application/json');
		xhr.upload.onprogress = (e) => {
			if (e.lengthComputable) onUpload(e.loaded / e.total);
		};
		// A small body may finish without any progress event; this one always fires.
		xhr.upload.onload = () => onUpload(1);
		xhr.onload = () => {
			let body: unknown = {};
			try {
				body = JSON.parse(xhr.responseText);
			} catch {
				// keep {}
			}
			if (xhr.status === 201) resolve(body as ImportResult);
			else reject(new ApiError(xhr.status, body as Problem));
		};
		xhr.onerror = () => reject(new ApiError(0, { title: 'Network error' }));
		xhr.onabort = () => reject(new ApiError(0, { title: 'Upload aborted' }));
		xhr.ontimeout = () => reject(new ApiError(0, { title: 'Upload timed out' }));
		xhr.send(zip as Uint8Array<ArrayBuffer>);
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
