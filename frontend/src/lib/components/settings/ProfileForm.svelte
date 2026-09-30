<script lang="ts">
	import Check from '@lucide/svelte/icons/check';
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
	let email = $state(session.user?.email ?? '');
	let nameError = $state<string | null>(null);
	let emailError = $state<string | null>(null);
	let saving = $state(false);

	// The confirmed line only applies to the address still on the account -
	// the moment somebody edits it, the provider's vouching no longer says
	// anything about what is now typed.
	const emailConfirmed = $derived(
		!!session.user?.emailVerified && email.trim() === (session.user?.email ?? '')
	);

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		nameError = null;
		emailError = null;
		saving = true;
		try {
			// An empty name is not an error: the API falls back to the login
			// name, which is how a person takes a display name back off. An
			// empty email clears it the same way.
			const saved = await updateOwnProfile({
				displayName: displayName.trim(),
				email: email.trim()
			});
			// Straight into the shared session, so the top bar, the recipe
			// cards and this page's own avatar all change at once.
			session.user = saved;
			displayName = saved.displayName;
			email = saved.email;
			toast.success(m.settings_profile_saved());
		} catch (failure) {
			// The name has two failure modes: too long, or a control character
			// (a pasted newline reaches this one; the length one is mostly
			// caught client-side by `maxlength`) - both located at
			// `body.displayName`, so the message is what tells them apart. The
			// email has one, located at `body.email`.
			if (failure instanceof ApiError && failure.status === 422) {
				const nameIssue = failure.errors.find((e) => e.location === 'body.displayName');
				if (nameIssue) {
					nameError = nameIssue.message.includes('control character')
						? m.settings_profile_name_control_char()
						: m.settings_profile_name_invalid();
				}
				const emailIssue = failure.errors.some((e) => e.location === 'body.email');
				if (emailIssue) {
					emailError = m.settings_profile_email_invalid();
				}
				if (!nameIssue && !emailIssue) {
					toast.error(m.settings_profile_error());
				}
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
		oninput={() => (nameError = null)}
		error={nameError}
	/>
	<div class="space-y-1.5">
		<Input
			id="email"
			label={m.settings_profile_email_label()}
			type="email"
			autocomplete="email"
			maxlength={254}
			bind:value={email}
			oninput={() => (emailError = null)}
			error={emailError}
			hint={emailConfirmed ? undefined : m.settings_profile_email_hint()}
		/>
		{#if emailConfirmed}
			<p class="flex items-center gap-1 text-micro text-text-muted">
				<Check class="size-3.5" aria-hidden="true" />
				{m.settings_profile_email_verified()}
			</p>
		{/if}
	</div>
	<!-- Beside the field's right edge at every width, because the password
	     form in the next card down ends exactly there: two settings cards
	     stacked on one page whose buttons are shaped differently read as an
	     oversight, not as a decision about thumbs. -->
	<div class="flex justify-end">
		<Button type="submit" disabled={saving}>{m.settings_profile_name_save()}</Button>
	</div>
</form>
