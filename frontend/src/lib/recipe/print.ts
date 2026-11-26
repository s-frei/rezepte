// Printing a recipe is printing its PDF: the server's Typst rendering is the
// only design, so print, download, share and the PDF tab's preview agree.
import { toast } from 'svelte-sonner';
import { isSignedOut } from '../api/client';
import { requestPdf } from '../api/pdf';
import type { Recipe } from '../api/recipes';
import { saveBlob } from '../api/transfer';
import { m } from '#lib/paraglide/messages.js';
import { buildClientRecipe, pdfRequest, type PdfRequest } from './pdf-document';
import type { PrintOptions } from './print-options.svelte';

/**
 * The request for one recipe at `servings` with `options`, for its PDF and
 * its preview alike. Pure, so an effect that builds it tracks every input it
 * reads. The server puts `REZEPTE_PUBLIC_URL` in for the origin when it is set.
 */
export function recipeRequest(
	recipe: Recipe,
	servings: number,
	options: Pick<PrintOptions, 'format' | 'photo' | 'footer'>
): PdfRequest {
	return pdfRequest('single', [buildClientRecipe(recipe, servings)], options, location.origin);
}

export type PrintRoute = 'frame' | 'tab' | 'download';

/**
 * How this browser prints a PDF. Desktop Chromium (Chrome, Edge) prints a
 * PDF loaded in a same-origin iframe through its `contentWindow.print()`,
 * the print dialog showing the PDF. Firefox's PDF viewer in a frame is a
 * cross-origin window, so `print` on it throws; Safari prints the page
 * around such a frame; on iOS and Android a frame shows no PDF at all. There,
 * with a PDF viewer, the PDF opens in a tab, whose viewer prints or shares
 * it; without one (`pdfViewerEnabled` false: Android Chrome, a viewer
 * turned off) it downloads. The user agent decides because no capability
 * tells whether `print()` on a PDF frame works, and `userAgentData` exists
 * only in secure contexts: an instance on plain http in the LAN would lose
 * the frame route.
 */
export function printRoute(
	nav: Pick<Navigator, 'userAgent' | 'pdfViewerEnabled'> = navigator
): PrintRoute {
	if (!nav.pdfViewerEnabled) return 'download';
	const desktop = !/Android|Mobile/.test(nav.userAgent);
	return desktop && /Chrome\//.test(nav.userAgent) ? 'frame' : 'tab';
}

// Removes the last print's frame and frees its blob URL. Run after its print
// or by the next one: a timeout could pull it while its dialog is still open.
let cleanup: (() => void) | null = null;

/** Resolves once the frame's `print()` ran, so the caller stays busy until the dialog. */
function printInFrame({ blob, filename }: { blob: Blob; filename: string }) {
	cleanup?.();
	const url = URL.createObjectURL(blob);
	const f = document.createElement('iframe');
	// Out of sight but laid out: a `display: none` frame loads no PDF viewer.
	f.style.cssText =
		'position:fixed;inset:auto 0 0 auto;width:0;height:0;border:0;visibility:hidden';
	f.setAttribute('aria-hidden', 'true');
	f.tabIndex = -1;
	const done = () => {
		f.remove();
		URL.revokeObjectURL(url);
		if (cleanup === done) cleanup = null;
	};
	cleanup = done;
	return new Promise<void>((resolve) => {
		f.addEventListener(
			'load',
			() => {
				// A frame the browser will not print (a PDF viewer turned into a
				// download, a policy) still leaves the user the file.
				try {
					const win = f.contentWindow!;
					win.addEventListener('afterprint', done, { once: true });
					win.focus();
					win.print();
				} catch {
					done();
					saveBlob(blob, filename);
				}
				resolve();
			},
			{ once: true }
		);
		f.src = url;
		document.body.append(f);
	});
}

// The print in flight: a second Print, tap or Ctrl+P meanwhile joins it.
let printing: Promise<void> | null = null;

/**
 * Renders `body`'s PDF and hands it to the browser's print, by `printRoute`.
 * Never rejects: a failure is the `share_pdf_failed` toast, once per print.
 */
export function printRecipe(body: PdfRequest): Promise<void> {
	printing ??= print(body)
		.catch((error) => {
			if (!isSignedOut(error)) toast.error(m.share_pdf_failed());
		})
		.finally(() => (printing = null));
	return printing;
}

async function print(body: PdfRequest) {
	const route = printRoute();
	// Opened inside the tap or key press: after the request a popup blocker
	// refuses it. Blocked anyway, the PDF downloads.
	const tab = route === 'tab' ? window.open('', '_blank') : null;
	if (tab) tab.document.title = m.share_pdf_creating();
	let file;
	try {
		file = await requestPdf(body);
	} catch (error) {
		tab?.close();
		throw error;
	}
	if (route === 'frame') return printInFrame(file);
	// Closed while it rendered: the PDF still reaches the reader.
	if (!tab || tab.closed) return saveBlob(file.blob, file.filename);
	// Never revoked: a viewer may fetch it again to share or save; it goes
	// with the page.
	tab.location.href = URL.createObjectURL(file.blob);
}
