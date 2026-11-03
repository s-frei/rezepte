<script lang="ts">
	import Import from '@lucide/svelte/icons/import';
	import ShoppingBasket from '@lucide/svelte/icons/shopping-basket';
	import Plus from '@lucide/svelte/icons/plus';
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import Lockup from '#lib/components/brand/Lockup.svelte';
	import Button from '#lib/components/ui/Button.svelte';
	import { importDialog } from '#lib/import.svelte.js';
	import { m } from '#lib/paraglide/messages.js';
	import { shell } from '#lib/shell.svelte.js';
	import UserMenu from './UserMenu.svelte';
	const onLists = $derived(page.route.id?.startsWith('/lists') ?? false);
</script>

<header class="mx-auto hidden h-16 max-w-[1280px] items-center justify-between px-8 md:flex">
	<div class="flex items-center gap-4">
		<a href={resolve('/')} aria-label={m.app_name()} class="flex items-center">
			<Lockup variant="horizontal" class="h-10" />
		</a>
		{#if shell.breadcrumb}
			<span class="text-body-sm text-text-muted">{shell.breadcrumb}</span>
		{/if}
	</div>
	<div class="flex items-center gap-3">
		{#if shell.actions}
			{@render shell.actions()}
		{:else}
			<Button variant={onLists ? 'accent' : 'ghost'} href={resolve('/lists')} current={onLists}>
				<ShoppingBasket class="size-4" aria-hidden="true" />
				{m.nav_lists()}
			</Button>
			<!-- Ghost, not secondary: importing is reached for now and then,
			     while "New recipe" beside it is what the bar is for. -->
			<Button variant="ghost" onclick={() => (importDialog.open = true)}>
				<Import class="size-4" aria-hidden="true" />
				{m.overview_import()}
			</Button>
			<Button variant="primary" href={resolve('/recipes/new')}>
				<Plus class="size-4" aria-hidden="true" />
				{m.overview_new_recipe()}
			</Button>
		{/if}
		<UserMenu />
	</div>
</header>
