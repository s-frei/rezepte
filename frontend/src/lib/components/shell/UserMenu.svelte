<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { DropdownMenu } from 'bits-ui';
	import { session } from '$lib/auth.svelte';
	import { m } from '$lib/paraglide/messages';
	import { signOut } from '$lib/sign-out';

	/** The desktop top bar's avatar menu; a phone reaches the same through the bottom nav's "You". */
	let { open = $bindable(false) }: { open?: boolean } = $props();

	const initial = $derived(session.user?.displayName.charAt(0).toUpperCase() ?? '');
</script>

<DropdownMenu.Root bind:open>
	<DropdownMenu.Trigger
		aria-label={m.nav_user_menu()}
		class="flex size-9 items-center justify-center rounded-full bg-accent initial-centered font-display font-semibold text-accent-foreground"
	>
		{initial}
	</DropdownMenu.Trigger>
	<DropdownMenu.Portal>
		<DropdownMenu.Content
			preventScroll={false}
			sideOffset={8}
			align="end"
			class="w-48 rounded-2xl bg-surface p-2 shadow-dialog"
		>
			<DropdownMenu.Item
				onSelect={() => void goto(resolve('/settings'))}
				class="flex h-10 items-center rounded-sm px-3 text-body-sm text-text transition hover:bg-background"
			>
				{m.nav_settings()}
			</DropdownMenu.Item>
			<DropdownMenu.Item
				onSelect={() => {
					open = false;
					void signOut();
				}}
				class="flex h-10 items-center rounded-sm px-3 text-body-sm text-destructive transition hover:bg-background"
			>
				{m.logout()}
			</DropdownMenu.Item>
		</DropdownMenu.Content>
	</DropdownMenu.Portal>
</DropdownMenu.Root>
