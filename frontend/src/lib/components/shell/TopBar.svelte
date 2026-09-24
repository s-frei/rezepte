<script lang="ts">
	import Plus from '@lucide/svelte/icons/plus';
	import { resolve } from '$app/paths';
	import Lockup from '$lib/components/brand/Lockup.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import { m } from '$lib/paraglide/messages';
	import { shell } from '$lib/shell.svelte';
	import UserMenu from './UserMenu.svelte';
</script>

<header class="mx-auto hidden h-16 max-w-[1280px] items-center justify-between px-8 md:flex">
	<div class="flex items-center gap-4">
		<a href={resolve('/')} aria-label={m.app_name()} class="flex items-center">
			<Lockup variant="compact" class="h-8" />
		</a>
		{#if shell.breadcrumb}
			<span class="text-body-sm text-text-muted">{shell.breadcrumb}</span>
		{/if}
	</div>
	<div class="flex items-center gap-3">
		{#if shell.actions}
			{@render shell.actions()}
		{:else}
			<Button variant="secondary" disabled title={m.common_coming_soon()}>
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
