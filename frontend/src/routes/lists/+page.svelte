<script lang="ts">
	import Plus from '@lucide/svelte/icons/plus';
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { toast } from 'svelte-sonner';
	import { isSignedOut } from '#lib/api/client.js';
	import { holdKeyboard } from '#lib/keyboard.js';
	import {
		listShoppingLists,
		type ShoppingInvitation,
		type ShoppingSummary
	} from '#lib/api/shopping.js';
	import { session } from '#lib/auth.svelte.js';
	import InvitationCard from '#lib/components/shopping/InvitationCard.svelte';
	import { invitations as nav } from '#lib/shopping/invitations.svelte.js';
	import ListCard from '#lib/components/shopping/ListCard.svelte';
	import Button from '#lib/components/ui/Button.svelte';
	import EmptyState from '#lib/components/ui/EmptyState.svelte';
	import NoteEmptyState from '#lib/components/ui/NoteEmptyState.svelte';
	import IconButton from '#lib/components/ui/IconButton.svelte';
	import Skeleton from '#lib/components/ui/Skeleton.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import shoppingArt from '$brand/lockups/rezepte-mascot-shopping.webp?url';
	import { shell } from '#lib/shell.svelte.js';

	let lists = $state<ShoppingSummary[]>();
	let invited = $state<ShoppingInvitation[]>([]);
	const mine = $derived(lists?.filter((l) => l.owner.id === session.user?.id) ?? []);
	const shared = $derived(lists?.filter((l) => l.owner.id !== session.user?.id) ?? []);
	let failed = $state(false);

	// The draft of a list opens as a page, its name field at the top. The
	// keyboard is raised inside this tap, which iOS insists on.
	function newList() {
		holdKeyboard();
		void goto(resolve('/lists/new'));
	}

	async function load() {
		failed = false;
		try {
			const body = await listShoppingLists();
			lists = body.items;
			invited = body.invitations;
			nav.count = invited.length;
		} catch (error) {
			if (!isSignedOut(error)) toast.error(m.lists_load_error());
			failed = true;
		}
	}

	onMount(load);

	function declined(listId: string) {
		invited = invited.filter((i) => i.listId !== listId);
		nav.count = invited.length;
	}

	$effect(() => {
		shell.breadcrumb = m.lists_title();
		return () => (shell.breadcrumb = undefined);
	});
</script>

<svelte:head><title>{m.lists_title()} · {m.app_name()}</title></svelte:head>

<div class="pt-6 pb-8 md:pt-4">
	<div class="mb-6 flex items-end justify-between gap-4">
		<h1
			class="min-w-0 font-display text-display-md font-medium hyphens-auto max-[359px]:text-display-sm md:text-display-lg"
		>
			{m.lists_title()}
		</h1>
		{#if !failed && (lists === undefined || lists.length > 0 || invited.length > 0)}
			<Button variant="primary" onclick={newList} class="max-md:hidden">
				<Plus class="size-4" aria-hidden="true" />
				{m.lists_new()}
			</Button>
			<IconButton
				label={m.lists_new()}
				onclick={newList}
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
	{:else if lists.length === 0 && invited.length === 0}
		<NoteEmptyState
			art={shoppingArt}
			title={m.lists_empty_title()}
			steps={[
				[m.lists_empty_step_create(), m.lists_empty_step_create_hint()],
				[m.lists_empty_step_recipes(), m.lists_empty_step_recipes_hint()],
				[m.lists_empty_step_share(), m.lists_empty_step_share_hint()]
			]}
		>
			<Button variant="primary" size="lg" onclick={newList} class="group shadow-cta md:h-12">
				<Plus
					aria-hidden="true"
					strokeWidth={2.5}
					class="size-5 transition-transform duration-300 ease-spring group-hover:rotate-90 motion-reduce:transition-none"
				/>
				{m.lists_new()}
			</Button>
		</NoteEmptyState>
	{:else}
		{#if invited.length > 0}
			<div class="flex flex-col gap-3">
				{#each invited as invitation (invitation.listId)}
					<InvitationCard {invitation} ondecline={declined} />
				{/each}
			</div>
		{/if}
		{#each [{ title: m.lists_mine(), items: mine }, { title: m.lists_shared_with_me(), items: shared }] as section (section.title)}
			{#if section.items.length > 0}
				<div class="mt-8 mb-3.5 flex items-center gap-3">
					<h2 class="font-display text-heading font-medium">{section.title}</h2>
					<span class="text-caption font-semibold text-text-muted">{section.items.length}</span>
					<span aria-hidden="true" class="min-w-6 flex-1 border-b-2 border-dotted border-handle"
					></span>
				</div>
				<div class="grid grid-cols-[repeat(auto-fill,minmax(min(100%,320px),1fr))] gap-4">
					{#each section.items as list (list.id)}
						<ListCard {list} />
					{/each}
				</div>
			{/if}
		{/each}
	{/if}
</div>
