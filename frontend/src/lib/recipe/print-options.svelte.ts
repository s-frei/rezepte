import { load, save } from '#lib/storage.js';

export type PrintOptions = {
	format: 'a4' | 'a5';
	photo: boolean;
	footer: boolean;
	bundle: 'book' | 'zip';
	servings: number | null;
};

const PRINT_STORAGE_KEY = 'rezepte-print';

function read(): PrintOptions {
	let raw: Record<string, unknown> = {};
	try {
		const parsed = JSON.parse(load(PRINT_STORAGE_KEY) ?? '');
		if (parsed && typeof parsed === 'object') raw = parsed;
	} catch {
		// Garbage in storage: start from the defaults.
	}
	return {
		format: raw.format === 'a5' ? 'a5' : 'a4',
		photo: typeof raw.photo === 'boolean' ? raw.photo : true,
		footer: typeof raw.footer === 'boolean' ? raw.footer : true,
		bundle: raw.bundle === 'zip' ? 'zip' : 'book',
		servings:
			typeof raw.servings === 'number' && Number.isInteger(raw.servings) && raw.servings > 0
				? raw.servings
				: null
	};
}

export const printOptions: PrintOptions = $state(read());

export function setPrintOption<K extends keyof PrintOptions>(key: K, value: PrintOptions[K]): void {
	printOptions[key] = value;
	save(PRINT_STORAGE_KEY, JSON.stringify($state.snapshot(printOptions)));
}
