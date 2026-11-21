<script lang="ts">
	import Check from '@lucide/svelte/icons/check';
	import { toast } from 'svelte-sonner';
	import type { ListPerson } from '#lib/api/shopping.js';
	import { ApiError, isSignedOut } from '#lib/api/client.js';
	import Button from '#lib/components/ui/Button.svelte';
	import PersonMark from '#lib/components/ui/PersonMark.svelte';
	import { m } from '#lib/paraglide/messages.js';

	/**
	 * An invitation to a list or a plan, answered in place: the caller says
	 * what accepting and declining do. A 404 means it was withdrawn, or the
	 * thing deleted, meanwhile: `gone` is said and the card is done.
	 */
	let {
		from,
		title,
		hint,
		gone,
		accept,
		decline,
		ondone
	}: {
		from: ListPerson;
		title: string;
		hint: string;
		/** The toast when the invitation is no longer there. */
		gone: string;
		/** Accepts and moves on (to the list, onto the plan). */
		accept: () => Promise<unknown>;
		decline: () => Promise<unknown>;
		/** The card is done: declined, or the invitation was withdrawn meanwhile. */
		ondone: () => void;
	} = $props();

	let busy = $state(false);

	async function answer(yes: boolean) {
		if (busy) return;
		busy = true;
		try {
			if (yes) await accept();
			else {
				await decline();
				ondone();
			}
		} catch (error) {
			if (error instanceof ApiError && error.status === 404) {
				toast.error(gone);
				ondone();
			} else if (!isSignedOut(error)) toast.error(m.lists_invite_error());
		} finally {
			busy = false;
		}
	}
</script>

<div
	class="flex flex-wrap items-center gap-3.5 rounded-2xl border-[1.5px] border-dashed border-primary bg-surface px-5 py-4"
>
	<PersonMark person={from} size="md" />
	<div class="min-w-0 flex-1 basis-0">
		<h3 class="font-semibold wrap-break-word">{title}</h3>
		<p class="mt-0.5 text-caption text-text-muted">{hint}</p>
	</div>
	<div class="flex w-full flex-wrap gap-2 md:ml-auto md:w-auto">
		<!-- Named with the invitation, so two cards' buttons tell apart. -->
		<Button
			variant="ghost"
			disabled={busy}
			label={m.lists_invite_decline_named({ title })}
			onclick={() => answer(false)}
			class="max-md:h-11 max-md:flex-1"
		>
			{m.lists_invite_decline()}
		</Button>
		<Button
			variant="primary"
			{busy}
			label={m.lists_invite_accept_named({ title })}
			onclick={() => answer(true)}
			class="max-md:h-11 max-md:flex-1"
		>
			<Check class="size-4" aria-hidden="true" />
			{m.lists_invite_accept()}
		</Button>
	</div>
</div>
