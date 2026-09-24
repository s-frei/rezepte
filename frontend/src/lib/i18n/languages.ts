import { getLocale, locales, type Locale } from '$lib/paraglide/runtime';

/**
 * One entry of the language picker: the language named in the interface's
 * own language, plus its endonym - what its speakers call it.
 */
export type LanguageOption = {
	value: Locale;
	/** "German" in an English interface, "Deutsch" in a German one. */
	label: string;
	/**
	 * The language's own name, omitted when it equals `label` - so the entry
	 * for the language you are already reading shows one word, not the same
	 * word twice. Named `hint` because that is the field `Select` renders it
	 * in; calling it `endonym` here would only mean mapping it at every call
	 * site.
	 */
	hint?: string;
};

/**
 * The names come from `Intl.DisplayNames` rather than from the message
 * catalogs. A catalog entry per language would mean every locale
 * naming every other one - four strings for two languages, nine for
 * three - and every one of them a chance to forget a file. The browser
 * already knows them all, in every locale it supports.
 *
 * Both names are shown because they answer different questions. The label
 * is for someone reading the interface they are already in; the endonym is
 * for someone who cannot read it and is looking for the word they know.
 * That second case is the whole reason a language picker exists.
 */
function displayName(of: Locale, inLocale: string): string {
	// `of` returns undefined for a tag it cannot name; the tag itself is a
	// poor label but a better one than an empty row.
	return new Intl.DisplayNames([inLocale], { type: 'language' }).of(of) ?? of;
}

/**
 * Every configured language, in the order `service/internal/i18n/locales.json`
 * lists them. Derived from Paraglide's generated
 * `locales`, so a language added there appears here without anyone
 * editing a list.
 */
export function languageOptions(): LanguageOption[] {
	const ui = getLocale();
	return locales.map((value) => {
		const label = displayName(value, ui);
		const endonym = displayName(value, value);
		return { value, label, hint: endonym === label ? undefined : endonym };
	});
}
