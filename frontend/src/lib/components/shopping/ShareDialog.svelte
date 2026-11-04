<script lang="ts">
	import X from '@lucide/svelte/icons/x';
	import { Dialog } from 'bits-ui';
	import { toast } from 'svelte-sonner';
	import { MediaQuery } from 'svelte/reactivity';
	import { isSignedOut } from '#lib/api/client.js';
	import {
		inviteToList,
		removeListMember,
		type ListPerson,
		type ShoppingList
	} from '#lib/api/shopping.js';
	import { listPeople, type PersonEntry } from '#lib/api/users.js';
	import { session } from '#lib/auth.svelte.js';
	import BaseDialog from '#lib/components/ui/BaseDialog.svelte';
	import BottomSheet from '#lib/components/ui/BottomSheet.svelte';
	import Button from '#lib/components/ui/Button.svelte';
	import PersonMark from '#lib/components/ui/PersonMark.svelte';
	import Select from '#lib/components/ui/Select.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import { failList } from '#lib/shopping/fail.js';

	let {
		open = $bindable(false),
		list,
		apply
	}: {
		open?: boolean;
		list: ShoppingList;
		/** The page's guard for racing answers: takes the returned list, false when the request failed. */
		apply: (request: Promise<ShoppingList>, onfail?: (error: unknown) => void) => Promise<boolean>;
	} = $props();

	const phone = new MediaQuery('max-width: 767.98px');

	let people = $state<PersonEntry[]>([]);
	let picked = $state('');

	// Loaded each time the dialog opens, so accounts added meanwhile show up.
	$effect(() => {
		if (!open) return;
		listPeople().then(
			(all) => (people = all),
			(error) => {
				if (!isSignedOut(error)) toast.error(m.users_load_error());
			}
		);
	});

	const options = $derived.by(() => {
		const taken = new Set(
			[list.owner, ...list.members, ...list.pending].map((p) => p.id).concat(session.user?.id ?? '')
		);
		return people
			.filter((p) => !taken.has(p.id))
			.map((p) => ({ value: p.id, label: p.displayName }));
	});

	async function invite(userId: string) {
		if (!userId) return;
		await apply(inviteToList(list.id, userId), (error) => failList(error, m.list_share_error()));
		picked = '';
	}

	const remove = (person: ListPerson) => apply(removeListMember(list.id, person.id));

	const mine = $derived(session.user?.id);
</script>

{#snippet body()}
	<Dialog.Title class="font-display text-heading-lg font-medium text-pretty">
		{m.list_share_title({ name: list.name })}
	</Dialog.Title>
	<Dialog.Description class="mt-2 text-caption text-text-muted">
		{m.list_share_text()}
	</Dialog.Description>
	<p class="mt-5 mb-2 text-label font-bold tracking-wide text-text-muted uppercase">
		{m.list_share_pick()}
	</p>
	<Select
		bind:value={picked}
		{options}
		label={m.list_share_pick()}
		placeholder={m.list_share_pick_placeholder()}
		onchange={invite}
		class="w-full justify-between bg-surface-elevated"
	/>
	<p class="mt-5 mb-2 text-label font-bold tracking-wide text-text-muted uppercase">
		{m.list_share_access()}
	</p>
	<ul class="rounded-2xl border border-border bg-surface-elevated px-4">
		<li class="flex items-center gap-3 border-b border-dashed border-border py-2.5 last:border-b-0">
			<PersonMark person={list.owner} size="sm" />
			<div class="min-w-0 flex-1">
				<p class="text-body font-semibold wrap-break-word">
					{list.owner.displayName}
					{#if list.owner.id === mine}<span class="text-caption font-normal text-text-muted"
							>{m.list_share_you()}</span
						>{/if}
				</p>
				<p class="text-caption text-text-muted">{m.list_share_created()}</p>
			</div>
		</li>
		{#each list.members as person (person.id)}
			{@render row(person, false)}
		{/each}
		{#each list.pending as person (person.id)}
			{@render row(person, true)}
		{/each}
	</ul>
{/snippet}

{#snippet row(person: ListPerson, pending: boolean)}
	<li class="flex items-center gap-3 border-b border-dashed border-border py-2.5 last:border-b-0">
		<PersonMark {person} size="sm" />
		<div class="min-w-0 flex-1">
			<p class="text-body font-semibold wrap-break-word">{person.displayName}</p>
			<p class="text-caption text-text-muted">
				{#if pending}
					<span class="inline-block rounded-pill bg-accent px-2.5 py-0.5 text-accent-foreground"
						>{m.list_share_pending()}</span
					>
				{:else}
					{m.list_share_accepted()}
				{/if}
			</p>
		</div>
		<button
			type="button"
			aria-label={m.list_share_remove({ name: person.displayName })}
			onclick={() => remove(person)}
			class="-mr-2 inline-flex size-10 shrink-0 items-center justify-center rounded-full hover-tint text-text-muted transition focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
		>
			<X class="size-[18px]" aria-hidden="true" />
		</button>
	</li>
{/snippet}

{#if phone.current}
	<BottomSheet bind:open closeLabel={m.list_share_done()}>
		<div class="min-h-0 flex-1 overflow-y-auto px-5 pb-4">{@render body()}</div>
		<div class="flex justify-end border-t border-border px-5 pt-3.5 pb-5">
			<Button onclick={() => (open = false)} class="grow">{m.list_share_done()}</Button>
		</div>
	</BottomSheet>
{:else}
	<BaseDialog bind:open>
		{@render body()}
		<div class="mt-6 flex justify-end border-t border-border pt-4">
			<Button onclick={() => (open = false)}>{m.list_share_done()}</Button>
		</div>
	</BaseDialog>
{/if}
