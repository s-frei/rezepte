import { m } from '#lib/paraglide/messages.js';
import { getLocale } from '#lib/paraglide/runtime.js';
import type { Recipe } from '../api/recipes';
import { buildClientRecipe, pdfRequest, type PdfRequest } from './pdf-document';
import type { PrintOptions } from './print-options.svelte';

/**
 * The Export card's request: the recipes in the order given, each at the
 * override servings or its own, as one cookbook or a zip of one PDF each.
 */
export function bulkPdfRequest(
	recipes: Recipe[],
	options: PrintOptions,
	origin: string,
	now = new Date()
): PdfRequest {
	// The local day, not UTC's: a cookbook made late at night carries tonight's date.
	const date = [now.getFullYear(), now.getMonth() + 1, now.getDate()]
		.map((n) => String(n).padStart(2, '0'))
		.join('-');
	const n = recipes.length;
	return pdfRequest(
		options.bundle,
		recipes.map((r) => buildClientRecipe(r, options.servings ?? r.servings)),
		options,
		origin,
		options.bundle === 'book'
			? {
					title: m.pdf_label_book_title(),
					date,
					dateDisplay: new Intl.DateTimeFormat(getLocale(), {
						month: 'long',
						year: 'numeric'
					}).format(now),
					countDisplay:
						n === 1
							? m.settings_transfer_pdf_count_one()
							: m.settings_transfer_pdf_count({ count: n })
				}
			: null
	);
}
