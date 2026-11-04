<script lang="ts">
	import Check from '@lucide/svelte/icons/check';
	import { toast } from 'svelte-sonner';
	import { goto } from '$app/navigation';
	import { acceptListInvitation, leaveList, type ShoppingInvitation } from '#lib/api/shopping.js';
	import { ApiError, isSignedOut } from '#lib/api/client.js';
	import Button from '#lib/components/ui/Button.svelte';
	import PersonMark from '#lib/components/ui/PersonMark.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import { invitations } from '#lib/shopping/invitations.svelte.js';
	import { listHref } from '#lib/shopping/href.js';

	let {
		invitation,
		ondecline
	}: {
		invitation: ShoppingInvitation;
		/** The card is done: declined, or the invitation was withdrawn meanwhile. */
		ondecline: (listId: string) => void;
	} = $props();

	let busy = $state(false);

	async function answer(accept: boolean) {
		if (busy) return;
		busy = true;
		try {
			if (accept) {
				await acceptListInvitation(invitation.listId);
				invitations.count--;
				await goto(listHref(invitation.listId));
			} else {
				await leaveList(invitation.listId);
				ondecline(invitation.listId);
			}
		} catch (error) {
			if (error instanceof ApiError && error.status === 404) {
				toast.error(m.list_gone());
				ondecline(invitation.listId);
			} else if (!isSignedOut(error)) toast.error(m.lists_invite_error());
		} finally {
			busy = false;
		}
	}
</script>

<div
	class="flex flex-wrap items-center gap-3.5 rounded-2xl border-[1.5px] border-dashed border-primary bg-surface px-5 py-4"
>
	<PersonMark person={invitation.from} size="md" />
	<div class="min-w-0 flex-1 basis-0">
		<p class="font-semibold">
			{m.lists_invite_title({ name: invitation.from.displayName, list: invitation.listName })}
		</p>
		<p class="mt-0.5 text-caption text-text-muted">
			{m.lists_invite_hint({ count: invitation.openCount })}
		</p>
	</div>
	<div class="flex w-full flex-wrap gap-2 md:ml-auto md:w-auto">
		<Button variant="ghost" disabled={busy} onclick={() => answer(false)} class="max-md:flex-1">
			{m.lists_invite_decline()}
		</Button>
		<Button variant="primary" {busy} onclick={() => answer(true)} class="max-md:flex-1">
			<Check class="size-4" aria-hidden="true" />
			{m.lists_invite_accept()}
		</Button>
	</div>
</div>
