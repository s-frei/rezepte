/**
 * The type sizes cook mode can set a step in, smallest first. Named after
 * the hand-set type sizes of the print shop. The names only keep their
 * order; they do not claim the historical point sizes.
 */
export const TYPE_SIZES = ['brevier', 'bourgeois', 'pica', 'great-primer', 'double-pica'] as const;

export type TypeSize = (typeof TYPE_SIZES)[number];

/** The size every visit to cook mode starts at, in the middle of the scale. */
export const DEFAULT_TYPE_SIZE: TypeSize = 'pica';

/**
 * The design token each size sets the step in: one scale for phones and one,
 * from `md`, for tablets and desktops. A phone gets the smaller end (a long
 * step has to fit a small screen), a large screen the larger end (it is read
 * from across the kitchen). The default is the same on both. Spelled out
 * whole so Tailwind's scanner sees every class.
 */
export const TYPE_SIZE_CLASSES: Record<TypeSize, string> = {
	brevier: 'text-card md:text-heading',
	bourgeois: 'text-heading md:text-heading-lg',
	pica: 'text-display-sm',
	'great-primer': 'text-display-md md:text-display-lg',
	'double-pica': 'text-display-lg md:text-display-xl'
};
