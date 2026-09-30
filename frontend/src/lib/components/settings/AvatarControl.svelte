<script lang="ts">
	import type { Snippet } from 'svelte';
	import { toast } from 'svelte-sonner';
	import { ApiError, isSignedOut } from '$lib/api/client';
	import Button from '$lib/components/ui/Button.svelte';
	import PersonMark from '$lib/components/ui/PersonMark.svelte';
	import { m } from '$lib/paraglide/messages';
	import type { Crop } from '$lib/user/crop';
	import AvatarCropDialog from './AvatarCropDialog.svelte';

	let {
		person,
		upload,
		remove,
		onchange,
		children
	}: {
		person: { id: string; displayName: string; color: string; avatarId: string | null };
		/** Sends the file and crop; resolves to the new avatar id. */
		upload: (file: File, crop: Crop) => Promise<{ avatarId: string }>;
		remove: () => Promise<void>;
		onchange: (avatarId: string | null) => void;
		/** Name and role, set between the picture and its buttons. */
		children?: Snippet;
	} = $props();

	let input = $state<HTMLInputElement>();
	let file = $state<File | null>(null);
	let cropOpen = $state(false);
	let busy = $state(false);

	function picked(event: Event) {
		const target = event.currentTarget as HTMLInputElement;
		file = target.files?.[0] ?? null;
		// Cleared so picking the same file again still fires `change`.
		target.value = '';
		if (file) cropOpen = true;
	}

	function message(error: unknown): string {
		if (error instanceof ApiError) {
			if (error.status === 413) return m.avatar_error_too_large();
			if (error.status === 415) return m.avatar_error_format();
			if (error.status === 422) return m.avatar_error_invalid();
		}
		return m.avatar_error();
	}

	async function save(crop: Crop) {
		if (!file) return;
		busy = true;
		try {
			const { avatarId } = await upload(file, crop);
			onchange(avatarId);
			cropOpen = false;
			toast.success(m.avatar_saved());
		} catch (error) {
			if (!isSignedOut(error)) toast.error(message(error));
		} finally {
			busy = false;
		}
	}

	async function clear() {
		busy = true;
		try {
			await remove();
			onchange(null);
			toast.success(m.avatar_removed());
		} catch (error) {
			if (!isSignedOut(error)) toast.error(m.avatar_error());
		} finally {
			busy = false;
		}
	}
</script>

<div class="flex flex-col items-center gap-3 text-center">
	<PersonMark {person} size="profile" />
	{@render children?.()}
	<div class="flex flex-wrap justify-center gap-2">
		<Button
			variant="secondary"
			class="whitespace-nowrap"
			disabled={busy}
			onclick={() => input?.click()}
		>
			{person.avatarId ? m.avatar_change() : m.avatar_add()}
		</Button>
		{#if person.avatarId}
			<Button variant="ghost" disabled={busy} onclick={clear}>{m.avatar_remove()}</Button>
		{/if}
	</div>
	<input
		bind:this={input}
		type="file"
		accept="image/jpeg,image/png,image/webp"
		class="sr-only"
		tabindex="-1"
		aria-hidden="true"
		onchange={picked}
	/>
</div>
<AvatarCropDialog bind:open={cropOpen} {file} {busy} onconfirm={save} />
