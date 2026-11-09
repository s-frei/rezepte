<script lang="ts">
	import UserPlus from '@lucide/svelte/icons/user-plus';
	import UsersRound from '@lucide/svelte/icons/users-round';
	import type { DeviceAccount } from '#lib/api/auth.js';
	import { knownAccounts } from '#lib/device-accounts.js';
	import PersonMark from '#lib/components/ui/PersonMark.svelte';
	import { m } from '#lib/paraglide/messages.js';

	/**
	 * The entry row's contents, the way to "Who's cooking?": the desktop menu
	 * (`xs` marks) and the phone's You sheet (`sm`) wrap it in their own row.
	 */
	let { open, markSize }: { open: boolean; markSize: 'xs' | 'sm' } = $props();

	// The device's other people, asked each time the menu or sheet opens.
	let others = $state<DeviceAccount[]>([]);
	$effect(() => {
		if (!open) return;
		void knownAccounts().then((all) => (others = all));
	});

	const menu = $derived(markSize === 'xs');
	const icon = $derived(`${menu ? 'size-4' : 'size-5'} shrink-0 text-text-muted`);
</script>

{#if others.length > 0}
	<UsersRound class={icon} aria-hidden="true" />
	{m.nav_switch()}
	<span class="ml-auto flex {menu ? '-space-x-1.5' : '-space-x-2'}">
		{#each others.slice(0, 3) as person (person.id)}
			<PersonMark
				{person}
				size={markSize}
				class="ring-2 {menu ? 'ring-popover' : 'ring-background'}"
			/>
		{/each}
	</span>
{:else}
	<UserPlus class={icon} aria-hidden="true" />
	{m.nav_switch_add()}
{/if}
