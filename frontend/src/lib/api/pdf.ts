import type { PdfRequest } from '../recipe/pdf-document';
import { postForFile } from './transfer';

/**
 * POSTs the formatted recipes and streams the rendered PDF (or zip) back.
 * An aborted `signal` cancels the render on the server too.
 */
export function requestPdf(
	body: PdfRequest,
	onProgress?: (loaded: number, total: number | null) => void,
	signal?: AbortSignal
): Promise<{ blob: Blob; filename: string }> {
	return postForFile(
		'/api/v1/pdf',
		body,
		'application/pdf, application/zip',
		'rezepte.pdf',
		onProgress,
		signal
	);
}

/** Page 1 of a single recipe's PDF as a PNG, for the PDF tab's preview. */
export async function requestPreview(body: PdfRequest, signal?: AbortSignal): Promise<Blob> {
	const { blob } = await postForFile(
		'/api/v1/pdf/preview',
		body,
		'image/png',
		'preview.png',
		undefined,
		signal
	);
	return blob;
}
