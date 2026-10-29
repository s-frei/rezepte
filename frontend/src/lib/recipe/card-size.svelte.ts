import { load, save } from '#lib/storage.js';

/**
 * How big the overview's cards are, remembered per device like the theme
 * and the sort order. A phone chooses between two columns and one, which
 * shows the desktop card; a wider screen between three card sizes, each a
 * minimum column width the grid fills the row with (see RecipeGrid).
 */
export const CARD_SIZES = ['small', 'medium', 'large'] as const;
export type CardSize = (typeof CARD_SIZES)[number];
export type PhoneColumns = 1 | 2;

export const CARD_SIZE_STORAGE_KEY = 'rezepte-card-size';
export const PHONE_COLUMNS_STORAGE_KEY = 'rezepte-phone-columns';

const storedSize = load(CARD_SIZE_STORAGE_KEY);

export const cardSize = $state({
	desktop: (CARD_SIZES as readonly string[]).includes(storedSize ?? '')
		? (storedSize as CardSize)
		: 'medium',
	phone: (load(PHONE_COLUMNS_STORAGE_KEY) === '1' ? 1 : 2) as PhoneColumns
});

export function setCardSize(size: CardSize): void {
	cardSize.desktop = size;
	save(CARD_SIZE_STORAGE_KEY, size);
}

export function setPhoneColumns(columns: PhoneColumns): void {
	cardSize.phone = columns;
	save(PHONE_COLUMNS_STORAGE_KEY, String(columns));
}
