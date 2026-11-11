<script lang="ts">
	import Pencil from '@lucide/svelte/icons/pencil';
	import type { Snippet } from 'svelte';
	import { toast } from 'svelte-sonner';
	import { ApiError, isSignedOut } from '#lib/api/client.js';
	import Button from '#lib/components/ui/Button.svelte';
	import Input from '#lib/components/ui/Input.svelte';
	import PersonMark from '#lib/components/ui/PersonMark.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import { userColorLabel } from '#lib/user/color-label.js';
	import { userColorClasses } from '#lib/user/color.js';
	import type { Crop } from '#lib/user/crop.js';
	import AvatarControl from './AvatarControl.svelte';

	let {
		person,
		roleLabel,
		editable,
		colors,
		upload,
		removeAvatar,
		onavatar,
		onrename,
		onerror
	}: {
		person: {
			id: string;
			displayName: string;
			username: string;
			color: string;
			avatarId: string | null;
		};
		roleLabel: string;
		/** Camera, pencil and the `colors` snippet; without it one read-only swatch. */
		editable: boolean;
		/** The color picker, set under the name. */
		colors?: Snippet;
		upload: (file: File, crop: Crop) => Promise<{ avatarId: string }>;
		removeAvatar: () => Promise<void>;
		onavatar: (avatarId: string | null) => void;
		/**
		 * Saves the trimmed name and says so; a rejection keeps the field
		 * open. A refused name shows under the field.
		 */
		onrename: (name: string) => Promise<void>;
		/** Toasts any other failure; by default "could not be saved". */
		onerror?: (error: unknown) => void;
	} = $props();

	const uid = $props.id();

	let editing = $state(false);
	let name = $state('');
	let nameError = $state<string | null>(null);
	let saving = $state(false);
	let pencil = $state<HTMLButtonElement>();

	// Translated whole and split where the bold login name goes, so each
	// language keeps its own order.
	const subtitle = $derived(
		m.profile_head_signs_in_as({ role: roleLabel, username: '\u0000' }).split('\u0000')
	);

	function startRename() {
		name = person.displayName;
		nameError = null;
		editing = true;
	}

	function stopRename() {
		editing = false;
		// Back to the pencil, so the keyboard does not fall to the page's top.
		queueMicrotask(() => pencil?.focus());
	}

	async function rename(event: SubmitEvent) {
		event.preventDefault();
		if (saving) return;
		nameError = null;
		saving = true;
		try {
			// An empty name is not an error: the API falls back to the login name.
			await onrename(name.trim());
			stopRename();
		} catch (failure) {
			// Too long or a control character (a pasted newline), both located at
			// `body.displayName`; the name typed tells them apart (Go's
			// unicode.IsControl is the Cc category).
			const issue =
				failure instanceof ApiError && failure.status === 422
					? failure.errors.find((e) => e.location === 'body.displayName')
					: undefined;
			if (issue) {
				nameError = /\p{Cc}/u.test(name)
					? m.settings_profile_name_control_char()
					: m.settings_profile_name_invalid();
			} else if (onerror) {
				onerror(failure);
			} else if (!isSignedOut(failure)) {
				toast.error(m.settings_profile_error());
			}
		} finally {
			saving = false;
		}
	}
</script>

<!--
	Head B: a flat band in the person's color with their initial set into it
	the way a recipe text card sets its own, the photo on the band's lower
	edge, and everything else on the paper below. It sits flush at the top of
	its card, which clips the band's corners.
-->
<div>
	<div
		class="relative isolate h-26 overflow-hidden md:h-28 {userColorClasses(person.color)}"
		aria-hidden="true"
	>
		<span
			class="pointer-events-none absolute -top-3 right-6 -z-10 font-display text-initial-sm leading-none font-medium italic opacity-14 md:text-initial"
		>
			{person.displayName.charAt(0).toUpperCase()}
		</span>
	</div>
	<div class="-mt-13 flex flex-col items-center gap-3.5 px-5 text-center md:-mt-14 md:px-7">
		{#if editable}
			<AvatarControl {person} {upload} remove={removeAvatar} onchange={onavatar} />
		{:else}
			<PersonMark {person} size="profile" class="ring-4 ring-surface" />
		{/if}

		<div class="grid w-full justify-items-center gap-1">
			{#if editing}
				<form onsubmit={rename} class="grid w-full max-w-sm gap-3 text-left">
					<!-- `maxlength` mirrors the API's 64-rune limit. -->
					<Input
						id="{uid}-name"
						label={m.settings_profile_name_label()}
						autocomplete="nickname"
						maxlength={64}
						counter={64}
						bind:value={name}
						oninput={() => (nameError = null)}
						error={nameError}
						autofocus
						onkeydown={(event) => {
							if (event.key === 'Escape') stopRename();
						}}
					/>
					<div class="flex justify-end gap-2">
						<Button variant="ghost" size="sm" onclick={stopRename}>{m.common_cancel()}</Button>
						<Button type="submit" size="sm" busy={saving}>{m.common_save()}</Button>
					</div>
				</form>
			{:else}
				<!-- The pencil rides on the last line of the name, so a long name
				     wraps around it instead of pushing it out of the card. Its
				     32px circle hits on 44px, through the `::after`. -->
				<h2
					class="max-w-full font-display text-display-sm font-medium tracking-tight break-words hyphens-auto"
				>
					{person.displayName}
					{#if editable}
						<button
							bind:this={pencil}
							type="button"
							aria-label={m.profile_head_rename()}
							title={m.profile_head_rename()}
							onclick={startRename}
							class="relative ml-1 inline-flex size-8 items-center justify-center rounded-full align-middle text-text-muted transition after:absolute after:-inset-1.5 hover:bg-background hover:text-text focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
						>
							<Pencil class="size-4" aria-hidden="true" />
						</button>
					{/if}
				</h2>
			{/if}
			<p class="text-caption text-text-muted">
				{subtitle[0]}<b class="font-semibold text-text">{person.username}</b>{subtitle[1]}
			</p>
		</div>

		{#if editable}
			{@render colors?.()}
		{:else}
			<p class="flex items-center gap-2 text-caption font-semibold">
				<span
					aria-hidden="true"
					class="size-8 shrink-0 rounded-full ring-1 ring-current/30 ring-inset {userColorClasses(
						person.color
					)}"
				></span>
				{userColorLabel(person.color)}
			</p>
		{/if}
	</div>
</div>
