<script lang="ts">
	import Languages from '@lucide/svelte/icons/languages';
	import { toast } from 'svelte-sonner';
	import { updateOwnProfile } from '$lib/api/auth';
	import { ApiError, isSignedOut } from '$lib/api/client';
	import Select from '$lib/components/ui/Select.svelte';
	import { languageOptions } from '$lib/i18n/languages';
	import { m } from '$lib/paraglide/messages';
	import { getLocale, setLocale, type Locale } from '$lib/paraglide/runtime';

	let saving = $state(false);
	// The shown language, not `getLocale()` directly: that is a snapshot read
	// once, and the select flips its own value on pick. Without a reactive
	// value here, a rejected save would leave the control showing the
	// language the account does not have, with no way back - the guard in
	// choose() would read the genuinely active one as "already selected".
	let current = $state<Locale>(getLocale());

	const options = languageOptions();

	// The account's row is the source of truth, so it is written first. The
	// service answers with a fresh locale cookie, and only then does the page
	// reload into the new language - a reload before the write would come
	// back in the old one.
	//
	// The reload is triggered here rather than left to setLocale()'s own
	// default: the PATCH response already carries the new PARAGLIDE_LOCALE
	// cookie, and the browser applies a Set-Cookie header before the fetch
	// promise resolves. By the time setLocale(next) runs, it reads that
	// cookie back as the *current* locale, sees no change against `next`,
	// and silently skips its own reload. `{ reload: false }` keeps setLocale
	// doing the cookie bookkeeping the Paraglide API expects; the
	// unconditional reload below is what actually shows the new language.
	async function choose(next: string) {
		const locale = next as Locale;
		if (locale === current || saving) {
			return;
		}
		saving = true;
		// Follow the control's own optimistic flip, so the reset below is a
		// change the control sees and not a write of the value it already has.
		current = locale;
		try {
			await updateOwnProfile({ locale });
			setLocale(locale, { reload: false });
			window.location.reload();
		} catch (error) {
			// Nothing was stored, so the rendered language is still the right
			// answer: put the selection back on it.
			current = getLocale();
			if (!(error instanceof ApiError) || !isSignedOut(error)) {
				toast.error(m.settings_language_error());
			}
			saving = false;
		}
	}
</script>

<!--
	Labeled like the color picker beside it, because it is a field of the
	profile card rather than a card of its own: a short visible caption, the
	control's accessible name carried separately.

	The trigger keeps its intrinsic width instead of filling the column. A
	language name is one word, and a control stretched across the card would
	promise a form field that takes typing.
-->
<div>
	<span class="block text-caption font-semibold">{m.settings_language_title()}</span>
	<p class="mt-1.5 mb-3 text-micro text-text-muted">{m.settings_language_hint()}</p>
	<Select
		value={current}
		{options}
		label={m.settings_language_label()}
		disabled={saving}
		onchange={choose}
		class="bg-surface"
	>
		{#snippet icon()}
			<Languages class="size-4 shrink-0 text-text-muted" aria-hidden="true" />
		{/snippet}
	</Select>
</div>
