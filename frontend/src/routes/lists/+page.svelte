<script lang="ts">
	import Plus from '@lucide/svelte/icons/plus';
	import { onMount } from 'svelte';
	import { toast } from 'svelte-sonner';
	import { isSignedOut } from '#lib/api/client.js';
	import { listShoppingLists, type ShoppingSummary } from '#lib/api/shopping.js';
	import ListCard from '#lib/components/shopping/ListCard.svelte';
	import NewListDialog from '#lib/components/shopping/NewListDialog.svelte';
	import Button from '#lib/components/ui/Button.svelte';
	import EmptyState from '#lib/components/ui/EmptyState.svelte';
	import IconButton from '#lib/components/ui/IconButton.svelte';
	import Skeleton from '#lib/components/ui/Skeleton.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import { shell } from '#lib/shell.svelte.js';

	let lists = $state<ShoppingSummary[]>();
	let failed = $state(false);
	let newOpen = $state(false);

	async function load() {
		failed = false;
		try {
			lists = await listShoppingLists();
		} catch (error) {
			if (!isSignedOut(error)) toast.error(m.lists_load_error());
			failed = true;
		}
	}

	onMount(load);

	$effect(() => {
		shell.breadcrumb = m.lists_title();
		return () => (shell.breadcrumb = undefined);
	});
</script>

<svelte:head><title>{m.lists_title()} · {m.app_name()}</title></svelte:head>

<div class="pt-6 pb-8 md:pt-4">
	<div class="mb-6 flex items-end justify-between gap-4">
		<h1 class="font-display text-display-md font-medium md:text-display-lg">{m.lists_title()}</h1>
		{#if !failed && (lists === undefined || lists.length > 0)}
			<Button variant="primary" onclick={() => (newOpen = true)} class="max-md:hidden">
				<Plus class="size-4" aria-hidden="true" />
				{m.lists_new()}
			</Button>
			<IconButton
				label={m.lists_new()}
				onclick={() => (newOpen = true)}
				class="shrink-0 bg-primary text-primary-foreground md:hidden"
			>
				<Plus class="size-5" aria-hidden="true" />
			</IconButton>
		{/if}
	</div>

	{#if failed}
		<EmptyState title={m.lists_load_error()} text="">
			<Button variant="primary" onclick={load}>{m.common_retry()}</Button>
		</EmptyState>
	{:else if lists === undefined}
		<div class="grid grid-cols-[repeat(auto-fill,minmax(min(100%,320px),1fr))] gap-4">
			<Skeleton class="h-36 rounded-2xl" />
			<Skeleton class="h-36 rounded-2xl" />
		</div>
	{:else if lists.length === 0}
		<EmptyState title={m.lists_empty_title()} text={m.lists_empty_text()}>
			<Button variant="primary" onclick={() => (newOpen = true)}>
				<Plus class="size-4" aria-hidden="true" />
				{m.lists_new()}
			</Button>
		</EmptyState>
	{:else}
		<div class="mt-8 mb-3.5 flex items-center gap-3">
			<h2 class="font-display text-heading font-medium">{m.lists_mine()}</h2>
			<span class="text-caption font-semibold text-text-muted">{lists.length}</span>
			<span aria-hidden="true" class="min-w-6 flex-1 border-b-2 border-dotted border-handle"></span>
		</div>
		<div class="grid grid-cols-[repeat(auto-fill,minmax(min(100%,320px),1fr))] gap-4">
			{#each lists as list (list.id)}
				<ListCard {list} />
			{/each}
		</div>
	{/if}
</div>

<NewListDialog bind:open={newOpen} />
