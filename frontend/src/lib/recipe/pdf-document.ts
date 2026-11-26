import { m } from '#lib/paraglide/messages.js';
import { getLocale } from '#lib/paraglide/runtime.js';
import type { Recipe } from '../api/recipes';
import { formatMinutes } from './format';
import { segmentStep } from './references';
import type { PrintOptions } from './print-options.svelte';
import { formatQuantityFor, roundScaled, scaleQuantity } from './scale';

/** Mirror of the Go `ClientRecipe` (service/internal/pdf); lists are never null. */
export type ClientRecipe = {
	id: string;
	title: string;
	description: string;
	tags: string[];
	servings: { value: number; original: number; display: string };
	times: {
		prep: Minutes | null;
		cook: Minutes | null;
		total: Minutes | null;
	};
	source: { name: string | null; url: string | null; display: string } | null;
	ingredientGroups: {
		name: string | null;
		ingredients: {
			quantity: { value: number; display: string } | null;
			unit: string | null;
			name: string;
			note: string | null;
			display: string;
		}[];
	}[];
	steps: ClientStep[];
};
export type ClientStep = { number: number; text: string; segments: ClientSegment[] };
type Minutes = { minutes: number; display: string };
export type ClientSegment = {
	kind: 'text' | 'ingredient' | 'time';
	text: string;
	/** Both always sent, null when the kind has none: data.json has no missing keys. */
	amount: string | null;
	minutes: number | null;
};

export type Labels = {
	servings: string;
	prep: string;
	cook: string;
	total: string;
	ingredients: string;
	steps: string;
	source: string;
	addedBy: string;
	contents: string;
	recipeFallback: string;
	continued: string;
	nextPage: string;
	openRecipe: string;
	/** "scaled from {n}": the template puts the recipe's own servings in for `{n}`. */
	scaledFrom: string;
	scaledFromShort: string;
};

export type PdfRequest = {
	origin: string;
	locale: string;
	labels: Labels;
	options: {
		bundle: 'single' | 'book' | 'zip';
		paper: 'a4' | 'a5';
		photo: boolean;
		footer: boolean;
	};
	book: { title: string; date: string; dateDisplay: string; countDisplay: string } | null;
	recipes: ClientRecipe[];
};

/**
 * The body of every PDF request, the PDF tab's and the Export card's: the
 * remembered format, photo and footer, the UI locale and its labels.
 */
export function pdfRequest(
	bundle: PdfRequest['options']['bundle'],
	recipes: ClientRecipe[],
	options: Pick<PrintOptions, 'format' | 'photo' | 'footer'>,
	origin: string,
	book: PdfRequest['book'] = null
): PdfRequest {
	return {
		origin,
		locale: getLocale(),
		labels: pdfLabels(),
		options: {
			bundle,
			paper: options.format,
			photo: options.photo,
			footer: options.footer
		},
		book,
		recipes
	};
}

const minutes = (n: number | null): Minutes | null =>
	n === null ? null : { minutes: n, display: formatMinutes(n) };

// A URL's host without a leading `www.`: how paper names a source.
function host(href: string): string {
	try {
		return new URL(href).host.replace(/^www\./, '');
	} catch {
		return href;
	}
}

/**
 * Text as paper sets it: a number keeps the word, fraction glyph, `°` or `%`
 * after it on its line ("etwa 300 g", "1 ½ TL", "180 °C", "5 %"), by a no-break space in the data, so
 * the PDF never breaks between an amount and its unit.
 */
export const glueAmounts = (text: string) =>
	text.replace(/(\p{N}) (?=[\p{L}\p{No}°%])/gu, '$1\u00a0');
const glued = (text: string | null) => (text === null ? null : glueAmounts(text));

export function buildClientRecipe(recipe: Recipe, servings: number): ClientRecipe {
	const { prepMinutes: prep, cookMinutes: cook } = recipe;
	const source = recipe.sourceName ?? (recipe.sourceUrl && host(recipe.sourceUrl));
	const ingredientGroups: ClientRecipe['ingredientGroups'] = recipe.ingredientGroups.map((g) => ({
		name: g.name,
		ingredients: g.ingredients.map((i) => {
			const text = formatQuantityFor(i.quantity, recipe.servings, servings);
			return {
				quantity:
					i.quantity === null
						? null
						: {
								value:
									servings === recipe.servings
										? i.quantity
										: roundScaled(scaleQuantity(i.quantity, recipe.servings, servings)),
								display: text
							},
				unit: i.unit,
				name: i.name,
				note: glued(i.note),
				display: [text, i.unit, i.name].filter(Boolean).join(' ')
			};
		})
	}));
	const steps: ClientStep[] = recipe.steps.map((s, n) => {
		const segments = segmentStep(s, recipe.ingredientGroups, servings, recipe.servings).map(
			(seg): ClientSegment => {
				const text = glueAmounts(seg.text);
				if (seg.quantity !== undefined) {
					return { kind: 'ingredient', text, amount: glued(seg.quantity || null), minutes: null };
				}
				if (seg.time) {
					const minutes = Math.max(1, Math.round(seg.time.seconds / 60));
					return { kind: 'time', text, amount: null, minutes };
				}
				return { kind: 'text', text, amount: null, minutes: null };
			}
		);
		return { number: n + 1, text: segments.map((seg) => seg.text).join(''), segments };
	});
	return {
		id: recipe.id,
		title: recipe.title,
		description: glueAmounts(recipe.description ?? ''),
		tags: recipe.tags ?? [],
		servings: {
			value: servings,
			original: recipe.servings,
			display: String(servings)
		},
		times: {
			prep: minutes(prep),
			cook: minutes(cook),
			total: prep === null && cook === null ? null : minutes((prep ?? 0) + (cook ?? 0))
		},
		source: source ? { name: recipe.sourceName, url: recipe.sourceUrl, display: source } : null,
		ingredientGroups,
		steps
	};
}

export function pdfLabels(): Labels {
	return {
		servings: m.pdf_label_servings(),
		prep: m.pdf_label_prep(),
		cook: m.pdf_label_cook(),
		total: m.pdf_label_total(),
		ingredients: m.pdf_label_ingredients(),
		steps: m.pdf_label_steps(),
		source: m.pdf_label_source(),
		addedBy: m.pdf_label_added_by(),
		contents: m.pdf_label_contents(),
		recipeFallback: m.pdf_label_recipe_fallback(),
		continued: m.pdf_label_continued(),
		nextPage: m.pdf_label_next_page(),
		openRecipe: m.pdf_label_open_recipe(),
		// `{n}` stays a placeholder: the template fills in each recipe's servings.
		scaledFrom: m.pdf_label_scaled_from({ n: '{n}' }),
		scaledFromShort: m.pdf_label_scaled_from_short({ n: '{n}' })
	};
}
