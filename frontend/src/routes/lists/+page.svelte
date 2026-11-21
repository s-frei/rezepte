<script lang="ts">
	import Plus from '@lucide/svelte/icons/plus';
	import { untrack } from 'svelte';
	import { toast } from 'svelte-sonner';
	import { goto, invalidateAll } from '$app/navigation';
	import { redirectSignedOut } from '#lib/api/client.js';
	import { acceptListInvitation, leaveList } from '#lib/api/shopping.js';
	import { session } from '#lib/auth.svelte.js';
	import PlaceSwitch from '#lib/components/shell/PlaceSwitch.svelte';
	import InvitationCard from '#lib/components/shopping/InvitationCard.svelte';
	import { invitations as nav } from '#lib/shopping/invitations.svelte.js';
	import { listHref } from '#lib/shopping/href.js';
	import { startList } from '#lib/shopping/new-list.svelte.js';
	import ListCard from '#lib/components/shopping/ListCard.svelte';
	import Button from '#lib/components/ui/Button.svelte';
	import EmptyState from '#lib/components/ui/EmptyState.svelte';
	import NoteEmptyState from '#lib/components/ui/NoteEmptyState.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import shoppingArt from '$brand/lockups/rezepte-mascot-shopping.webp?url';
	import { shell } from '#lib/shell.svelte.js';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	// Answered here: gone at once, before the next read. Each read starts
	// afresh: the server knows the answers then, and a new invitation to
	// the same list or plan counts again.
	let answered = $derived.by(() => {
		void data;
		return [] as string[];
	});
	const lists = $derived(data.items);
	const invited = $derived(data.invitations.filter((i) => !answered.includes(i.listId)));
	const mine = $derived(lists.filter((l) => l.owner.id === session.user?.id));
	const shared = $derived(lists.filter((l) => l.owner.id !== session.user?.id));

	$effect(() => {
		if (data.signedOut) void redirectSignedOut();
		else if (data.failed) untrack(() => toast.error(m.lists_load_error()));
	});
	$effect(() => {
		if (!data.failed) nav.lists = invited.length;
	});
	// The shell's count moved on its own (the tab came back): read the cards it counts.
	$effect(() => {
		if (!data.failed && nav.lists !== untrack(() => invited.length)) void invalidateAll();
	});

	const answer = (listId: string) => (answered = [...answered, listId]);

	$effect(() => {
		shell.breadcrumb = m.lists_title();
		return () => (shell.breadcrumb = undefined);
	});
</script>

<svelte:head><title>{m.lists_title()} · {m.app_name()}</title></svelte:head>

<div class="pt-6 pb-8 md:pt-4">
	<!-- New list and the import are the top bar's and the phone's "+", as on
	     the recipes; the title shares its line only with the place's switch. -->
	<PlaceSwitch side="lists" />

	{#if data.signedOut}
		<!-- Leaving for the sign-in. -->
	{:else if data.failed}
		<EmptyState title={m.lists_load_error()} text="">
			<Button variant="primary" onclick={() => invalidateAll()}>{m.common_retry()}</Button>
		</EmptyState>
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
			<Button variant="primary" size="lg" onclick={startList} class="group shadow-cta md:h-12">
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
			<h2 class="sr-only">{m.invitations_heading()}</h2>
			<div class="flex flex-col gap-3">
				{#each invited as invitation (invitation.listId)}
					<InvitationCard
						from={invitation.from}
						title={m.lists_invite_title({
							name: invitation.from.displayName,
							list: invitation.listName
						})}
						hint={m.lists_invite_hint({ count: invitation.openCount })}
						gone={m.list_gone()}
						accept={async () => {
							await acceptListInvitation(invitation.listId);
							// The list opens in the overview's place: its count drops here, as the card goes.
							answer(invitation.listId);
							await goto(listHref(invitation.listId));
						}}
						decline={() => leaveList(invitation.listId)}
						ondone={() => answer(invitation.listId)}
					/>
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
