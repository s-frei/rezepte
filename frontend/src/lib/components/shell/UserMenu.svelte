<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { Dialog, DropdownMenu } from 'bits-ui';
	import { fade, fly } from 'svelte/transition';
	import { logout } from '$lib/api/auth';
	import { session } from '$lib/auth.svelte';
	import { m } from '$lib/paraglide/messages';

	let {
		variant = 'dropdown',
		open = $bindable(false)
	}: {
		/** `dropdown` renders its own avatar trigger (desktop top bar); `sheet`
		 * is a bottom sheet controlled by the caller (mobile "Mehr" nav item). */
		variant?: 'dropdown' | 'sheet';
		open?: boolean;
	} = $props();

	const initial = $derived(session.user?.username.charAt(0).toUpperCase() ?? '');

	async function signOut() {
		try {
			await logout();
		} catch {
			// The server call failed (e.g. the session was already gone) - the
			// user still expects to end up signed out and on the login page.
		}
		session.user = null;
		open = false;
		await goto(resolve('/login'));
	}
</script>

{#if variant === 'dropdown'}
	<DropdownMenu.Root bind:open>
		<DropdownMenu.Trigger
			aria-label={m.nav_user_menu()}
			class="flex size-9 items-center justify-center rounded-full bg-accent font-display font-semibold text-accent-foreground"
		>
			{initial}
		</DropdownMenu.Trigger>
		<DropdownMenu.Portal>
			<DropdownMenu.Content
				sideOffset={8}
				align="end"
				class="w-48 rounded-2xl bg-surface p-2 shadow-dialog"
			>
				<DropdownMenu.Item
					disabled
					class="flex h-10 items-center rounded-sm px-3 text-body-sm text-text data-[disabled]:pointer-events-none data-[disabled]:text-text-muted data-[disabled]:opacity-60"
				>
					{m.nav_settings()}
				</DropdownMenu.Item>
				<DropdownMenu.Item
					onSelect={signOut}
					class="flex h-10 items-center rounded-sm px-3 text-body-sm text-text transition hover:bg-background"
				>
					{m.logout()}
				</DropdownMenu.Item>
			</DropdownMenu.Content>
		</DropdownMenu.Portal>
	</DropdownMenu.Root>
{:else}
	<Dialog.Root bind:open>
		<Dialog.Portal>
			<Dialog.Overlay forceMount>
				{#snippet child({ props, open: isOpen })}
					{#if isOpen}
						<div
							{...props}
							class="fixed inset-0 z-40 bg-overlay"
							transition:fade={{ duration: 200 }}
						></div>
					{/if}
				{/snippet}
			</Dialog.Overlay>
			<Dialog.Content forceMount>
				{#snippet child({ props, open: isOpen })}
					{#if isOpen}
						<div
							{...props}
							class="fixed inset-x-0 bottom-0 z-50 rounded-t-3xl bg-background p-5 pb-8 shadow-sheet"
							transition:fly={{ duration: 250, y: 200 }}
						>
							<div class="mx-auto mb-4 h-1 w-9 rounded-pill bg-handle" aria-hidden="true"></div>
							<Dialog.Title class="sr-only">{m.nav_more()}</Dialog.Title>
							<button
								type="button"
								disabled
								class="flex h-12 w-full items-center rounded-sm px-3 text-left text-body text-text-muted disabled:opacity-60"
							>
								{m.nav_settings()}
							</button>
							<button
								type="button"
								onclick={signOut}
								class="flex h-12 w-full items-center rounded-sm px-3 text-left text-body text-text transition hover:bg-surface"
							>
								{m.logout()}
							</button>
						</div>
					{/if}
				{/snippet}
			</Dialog.Content>
		</Dialog.Portal>
	</Dialog.Root>
{/if}
