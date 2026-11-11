<script lang="ts">
	import Camera from '@lucide/svelte/icons/camera';
	import { DropdownMenu } from 'bits-ui';
	import { tick } from 'svelte';
	import { toast } from 'svelte-sonner';
	import { ApiError, isSignedOut } from '#lib/api/client.js';
	import { MAX_IMAGE_BYTES } from '#lib/api/recipes.js';
	import PersonMark from '#lib/components/ui/PersonMark.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import type { Crop } from '#lib/user/crop.js';
	import AvatarCropDialog from './AvatarCropDialog.svelte';

	let {
		person,
		upload,
		remove,
		onchange
	}: {
		person: { id: string; displayName: string; color: string; avatarId: string | null };
		/** Sends the file and crop; resolves to the new avatar id. */
		upload: (file: File, crop: Crop) => Promise<{ avatarId: string }>;
		remove: () => Promise<void>;
		onchange: (avatarId: string | null) => void;
	} = $props();

	let input = $state<HTMLInputElement>();
	let file = $state<File | null>(null);
	let cropOpen = $state(false);
	let busy = $state(false);
	let badge = $state<HTMLSpanElement>();

	const item =
		'flex h-10 items-center rounded-sm px-3 text-body-sm text-text outline-none data-highlighted:bg-background';

	function picked(event: Event) {
		const target = event.currentTarget as HTMLInputElement;
		file = target.files?.[0] ?? null;
		// Cleared so picking the same file again still fires `change`.
		target.value = '';
		if (file && file.size > MAX_IMAGE_BYTES) {
			toast.error(m.avatar_error_too_large());
			file = null;
		}
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

	// Adding or removing a photo swaps the badge between a plain button and
	// a menu trigger, and focus would fall to the top of the page with the
	// old one. The crop dialog hands focus back after its close transition,
	// to the button that is gone by then, so the save waits that out.
	async function refocus(wait = 0) {
		await tick();
		// ponytail: waits out the dialog's 200ms close; a dialog close event would be exact.
		if (wait) await new Promise((r) => setTimeout(r, wait));
		badge?.querySelector('button')?.focus();
	}

	async function save(crop: Crop) {
		if (!file) return;
		busy = true;
		try {
			const { avatarId } = await upload(file, crop);
			onchange(avatarId);
			cropOpen = false;
			toast.success(m.avatar_saved());
			void refocus(300);
		} catch (error) {
			if (!isSignedOut(error)) toast.error(message(error));
		} finally {
			busy = false;
		}
	}

	async function clear() {
		if (busy) return;
		busy = true;
		try {
			await remove();
			onchange(null);
			toast.success(m.avatar_removed());
			void refocus();
		} catch (error) {
			if (!isSignedOut(error)) toast.error(m.avatar_error_remove());
		} finally {
			busy = false;
		}
	}
</script>

<!--
	The photo with a camera badge on its edge, 34px drawn and 44px to hit
	(its `::after`). With a photo the badge opens a small menu (change or
	remove); without one there is only one thing to do, so the badge goes
	straight to the file picker.
-->
<div class="relative inline-flex shrink-0">
	<PersonMark {person} size="profile" class="ring-4 ring-surface" />
	<span bind:this={badge} class="absolute -right-0.5 -bottom-0.5">
		{#if person.avatarId}
			<DropdownMenu.Root>
				<DropdownMenu.Trigger
					aria-label={m.avatar_change()}
					title={m.avatar_change()}
					aria-disabled={busy}
					aria-busy={busy}
					class="relative inline-flex size-[34px] items-center justify-center rounded-full bg-surface hover-tint text-text shadow-card ring-1 ring-border transition after:absolute after:-inset-[5px] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary aria-disabled:opacity-50"
				>
					<Camera class="size-4" aria-hidden="true" />
				</DropdownMenu.Trigger>
				<DropdownMenu.Portal>
					<DropdownMenu.Content
						preventScroll={false}
						sideOffset={8}
						collisionPadding={16}
						align="start"
						class="w-48 max-w-[calc(100vw-2rem)] rounded-2xl border border-popover-border bg-popover p-2 shadow-dialog"
					>
						<DropdownMenu.Item onSelect={() => !busy && input?.click()} class={item}>
							{m.avatar_change()}
						</DropdownMenu.Item>
						<DropdownMenu.Item onSelect={clear} class={item}>
							{m.avatar_remove()}
						</DropdownMenu.Item>
					</DropdownMenu.Content>
				</DropdownMenu.Portal>
			</DropdownMenu.Root>
		{:else}
			<button
				type="button"
				aria-label={m.avatar_add()}
				title={m.avatar_add()}
				aria-disabled={busy}
				aria-busy={busy}
				onclick={() => !busy && input?.click()}
				class="relative inline-flex size-[34px] items-center justify-center rounded-full bg-surface hover-tint text-text shadow-card ring-1 ring-border transition after:absolute after:-inset-[5px] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary aria-disabled:opacity-50"
			>
				<Camera class="size-4" aria-hidden="true" />
			</button>
		{/if}
	</span>
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
