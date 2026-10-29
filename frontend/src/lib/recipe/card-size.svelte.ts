import { browser } from '$app/environment';

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

function read(key: string): string | null {
	if (!browser) {
		return null;
	}
	try {
		return window.localStorage.getItem(key);
	} catch {
		// `window.localStorage` itself throws when site data is blocked.
		return null;
	}
}

function write(key: string, value: string): void {
	try {
		window.localStorage.setItem(key, value);
	} catch {
		// Storage blocked or full: the choice still holds for this page view.
	}
}

const storedSize = read(CARD_SIZE_STORAGE_KEY);

export const cardSize = $state({
	desktop: (CARD_SIZES as readonly string[]).includes(storedSize ?? '')
		? (storedSize as CardSize)
		: 'medium',
	phone: (read(PHONE_COLUMNS_STORAGE_KEY) === '1' ? 1 : 2) as PhoneColumns
});

export function setCardSize(size: CardSize): void {
	cardSize.desktop = size;
	write(CARD_SIZE_STORAGE_KEY, size);
}

export function setPhoneColumns(columns: PhoneColumns): void {
	cardSize.phone = columns;
	write(PHONE_COLUMNS_STORAGE_KEY, String(columns));
}
