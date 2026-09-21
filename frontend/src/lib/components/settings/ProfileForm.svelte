<script lang="ts">
	import { toast } from 'svelte-sonner';
	import { updateOwnProfile } from '$lib/api/auth';
	import { ApiError, isSignedOut } from '$lib/api/client';
	import { session } from '$lib/auth.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import { m } from '$lib/paraglide/messages';

	// Seeded once, not derived: the root layout's `load` puts the session in
	// place before any page renders, and from then on the field belongs to
	// whoever is typing in it.
	let displayName = $state(session.user?.displayName ?? '');
	let error = $state<string | null>(null);
	let saving = $state(false);

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		error = null;
		saving = true;
		try {
			// An empty name is not an error: the API falls back to the login
			// name, which is how a person takes a display name back off.
			const saved = await updateOwnProfile({ displayName: displayName.trim() });
			// Straight into the shared session, so the top bar, the recipe
			// cards and this page's own avatar all change at once.
			session.user = saved;
			displayName = saved.displayName;
			toast.success(m.settings_profile_saved());
		} catch (failure) {
			// The only field this form sends is the name, so a 422 is about it -
			// but the name has two failure modes: too long, or a control
			// character (a pasted newline reaches this one; the length one is
			// mostly caught client-side by `maxlength`). The location is
			// `body.displayName` for both, so the message is what tells them
			// apart.
			if (failure instanceof ApiError && failure.status === 422) {
				const controlChar = failure.errors.some(
					(e) => e.location === 'body.displayName' && e.message.includes('control character')
				);
				error = controlChar
					? m.settings_profile_name_control_char()
					: m.settings_profile_name_invalid();
			} else if (!isSignedOut(failure)) {
				// 401 already redirects to the login page (see $lib/api/client).
				toast.error(m.settings_profile_error());
			}
		} finally {
			saving = false;
		}
	}
</script>

<form onsubmit={submit} class="space-y-4">
	<!-- `maxlength` mirrors the API's 64-rune limit, so the usual way to
	     overrun it - pasting - is caught before the request. `counter` is the
	     same number again because it counts towards that same limit. -->
	<Input
		id="display-name"
		label={m.settings_profile_name_label()}
		autocomplete="nickname"
		maxlength={64}
		counter={64}
		bind:value={displayName}
		{error}
	/>
	<!-- Beside the field's right edge at every width, because the password
	     form in the next card down ends exactly there: two settings cards
	     stacked on one page whose buttons are shaped differently read as an
	     oversight, not as a decision about thumbs. -->
	<div class="flex justify-end">
		<Button type="submit" disabled={saving}>{m.settings_profile_name_save()}</Button>
	</div>
</form>
