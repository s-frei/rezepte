<script lang="ts">
	import DialogActions from '#lib/components/ui/DialogActions.svelte';
	import X from '@lucide/svelte/icons/x';
	import { Dialog } from 'bits-ui';
	import { toast } from 'svelte-sonner';
	import { MediaQuery } from 'svelte/reactivity';
	import { ApiError, isSignedOut } from '#lib/api/client.js';
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
	import { prefersReducedMotion } from 'svelte/motion';
	import { slide } from 'svelte/transition';
	import CircleCheckIcon from '#lib/components/icons/CircleCheckIcon.svelte';
	import InvitePicker from '#lib/components/shopping/InvitePicker.svelte';
	import PersonMark from '#lib/components/ui/PersonMark.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import { failList } from '#lib/shopping/fail.js';
	import { joinNames } from '#lib/user/holders.js';

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
	let picked = $state<string[]>([]);
	let sending = $state(false);
	/** The line under the picker after a send: who was invited, or who was not. */
	let notice = $state<{ ok: boolean; text: string } | null>(null);
	/** Ids whose invitation failed; they stay picked. */
	let failed = $state(new Set<string>());
	/** Ids just invited: their rows under "Has access" light up for a moment. */
	let fresh = $state(new Set<string>());
	let freshTimer: ReturnType<typeof setTimeout> | undefined;
	/** Counts openings, so a send still running from an earlier one writes nothing back. */
	let run = 0;

	// Loaded each time the dialog opens, so accounts added meanwhile show up.
	// A pick left from the last time is dropped: closing never invites.
	$effect(() => {
		if (!open) return;
		run++;
		sending = false;
		picked = [];
		notice = null;
		failed = new Set();
		fresh = new Set();
		listPeople().then(
			(all) => (people = all),
			(error) => {
				if (!isSignedOut(error)) toast.error(m.users_load_error());
			}
		);
	});

	const candidates = $derived.by(() => {
		const taken = new Set(
			[list.owner, ...list.members, ...list.pending].map((p) => p.id).concat(session.user?.id ?? '')
		);
		return people.filter((p) => !taken.has(p.id));
	});
	// Only picks still to invite count: someone invited meanwhile (another
	// device, or an invite that timed out after the server sent it) drops out.
	const live = $derived(picked.filter((id) => candidates.some((c) => c.id === id)));

	const nameOf = (id: string) => people.find((p) => p.id === id)?.displayName.trim() ?? '';

	/**
	 * One request per person, one after the other: each answers with the
	 * whole list, and the page keeps only the newest answer, so parallel ones
	 * could land out of order and drop an invitee until the next refetch.
	 */
	async function invite() {
		if (sending || live.length === 0) return;
		const opening = run;
		sending = true;
		notice = null;
		const ok: string[] = [];
		const bad: string[] = [];
		let gone = false;
		for (const id of live) {
			const sent = await apply(inviteToList(list.id, id), (error) => {
				// A 404 means the list is gone: failList leaves the page.
				if (error instanceof ApiError && error.status === 404) {
					gone = true;
					failList(error);
				}
			});
			// Closed and opened again meanwhile: that opening owns the dialog now.
			if (opening !== run) return;
			(sent ? ok : bad).push(id);
			if (gone) break;
		}
		sending = false;
		if (gone) return;
		picked = bad;
		failed = new Set(bad);
		if (bad.length > 0) {
			notice = { ok: false, text: m.list_share_error({ names: joinNames(bad.map(nameOf)) }) };
		} else {
			const names = joinNames(ok.map(nameOf));
			notice = {
				ok: true,
				text: ok.length === 1 ? m.list_share_sent_one({ names }) : m.list_share_sent({ names })
			};
		}
		clearTimeout(freshTimer);
		fresh = new Set(ok);
		freshTimer = setTimeout(() => (fresh = new Set()), 2400);
	}

	const action = $derived(
		sending
			? m.list_share_inviting()
			: live.length === 0
				? m.list_share_done()
				: live.length === 1
					? m.list_share_invite_one()
					: m.list_share_invite({ count: live.length })
	);

	function primary() {
		if (live.length === 0) open = false;
		else void invite();
	}

	const remove = (person: ListPerson) => apply(removeListMember(list.id, person.id));

	const mine = $derived(session.user?.id);
	// A row that joins while the dialog is open (an invitation sent, an
	// answer arriving over the stream) opens into the list; the rows there
	// when the dialog opens do not, since Svelte's transitions are local.
	const rowMotion = $derived(prefersReducedMotion.current ? 0 : 200);
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
	<InvitePicker people={candidates} bind:picked {failed} disabled={sending} />
	<!-- There from the start: a live region announces what comes into it.
		Keyed, so a second send drops in and draws its check again. -->
	<div role="status">
		{#key notice}
			{#if notice}
				<p
					class={[
						'mt-3 flex items-start gap-2 rounded-md px-3 py-2.5 text-body-sm motion-safe:animate-error-in',
						notice.ok
							? 'bg-success-soft text-success-foreground'
							: 'bg-destructive-soft text-destructive'
					]}
				>
					{#if notice.ok}<CircleCheckIcon class="mt-0.5 size-4 shrink-0" />{/if}
					<span>{notice.text}</span>
				</p>
			{/if}
		{/key}
	</div>
	<p class="mt-5 mb-2 text-label font-bold tracking-wide text-text-muted uppercase">
		{m.list_share_access()}
	</p>
	<ul class="overflow-hidden rounded-2xl border border-border bg-surface-elevated px-4">
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
	<li
		in:slide={{ duration: rowMotion }}
		class={[
			'flex items-center gap-3 border-b border-dashed border-border py-2.5 transition-colors duration-700 last:border-b-0',
			fresh.has(person.id) && '-mx-4 bg-background px-4'
		]}
	>
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
			<Button onclick={primary} busy={sending} class="grow">{action}</Button>
		</div>
	</BottomSheet>
{:else}
	<BaseDialog bind:open>
		{@render body()}
		<DialogActions class="mt-6 border-t border-border pt-4">
			<Button onclick={primary} busy={sending}>{action}</Button>
		</DialogActions>
	</BaseDialog>
{/if}
