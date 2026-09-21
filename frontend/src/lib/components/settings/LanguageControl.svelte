<script lang="ts">
	import Languages from 'lucide-svelte/icons/languages';
	import { toast } from 'svelte-sonner';
	import { updateOwnProfile, type Locale } from '$lib/api/auth';
	import { ApiError, isSignedOut } from '$lib/api/client';
	import SegmentedControl from '$lib/components/ui/SegmentedControl.svelte';
	import { m } from '$lib/paraglide/messages';
	import { getLocale, setLocale } from '$lib/paraglide/runtime';

	let saving = $state(false);
	// The highlighted segment, not `getLocale()` directly: that is a snapshot
	// read once, and SegmentedControl flips its own value on click. Without a
	// reactive value here, a rejected save would leave the control showing the
	// language the account does not have, with no way back - the guard in
	// choose() would read the genuinely active one as "already selected".
	let current = $state(getLocale());

	const options = [
		{ value: 'en' as Locale, label: m.settings_language_english(), icon: Languages },
		{ value: 'de' as Locale, label: m.settings_language_german(), icon: Languages }
	];

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
	// doing the cookie bookkeeping the Paraglide API expects; the unconditional
	// reload below is what actually shows the new language.
	async function choose(next: Locale) {
		if (next === current || saving) {
			return;
		}
		saving = true;
		// Follow the control's own optimistic flip, so the reset below is a
		// change the control sees and not a write of the value it already has.
		current = next;
		try {
			await updateOwnProfile({ locale: next });
			setLocale(next, { reload: false });
			window.location.reload();
		} catch (error) {
			// Nothing was stored, so the rendered language is still the right
			// answer: put the highlight back on it.
			current = getLocale();
			if (!(error instanceof ApiError) || !isSignedOut(error)) {
				toast.error(m.settings_language_error());
			}
			saving = false;
		}
	}
</script>

<p class="mb-3 text-body-sm text-text-muted">{m.settings_language_hint()}</p>
<SegmentedControl value={current} {options} label={m.settings_language_label()} onchange={choose} />
