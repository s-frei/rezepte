<script lang="ts">
	import type { Snippet } from 'svelte';
	import { untrack } from 'svelte';
	import { toast } from 'svelte-sonner';
	import { isSignedOut } from '#lib/api/client.js';
	import { listShoppingLists, type ShoppingSummary } from '#lib/api/shopping.js';
	import Input from '#lib/components/ui/Input.svelte';
	import Select from '#lib/components/ui/Select.svelte';
	import { byListName } from '#lib/shopping/plan-ref.js';
	import { m } from '#lib/paraglide/messages.js';

	/**
	 * "Which list": the reader's lists in a Select, or a name for a new one,
	 * with the "New list"/Cancel toggle, and a retry row when the lists fail.
	 * Loads on mount, which is every opening of the dialog around it.
	 *
	 * The first of `prefer` that is among the choices is picked, else the
	 * first choice; `newUnlessKnown` starts on a new list when none of
	 * `prefer` is. With no choices at all, only the name is asked. A pick or
	 * "New list" made before a remount (the sheet becoming a dialog across
	 * `md`) stays: the dialog clears `listId` and `creating` when it opens.
	 */
	let {
		lists = $bindable([]),
		listId = $bindable(''),
		newName = $bindable(''),
		creating = $bindable(false),
		failed = $bindable(false),
		exclude = [],
		byName = false,
		prefer = [],
		newUnlessKnown = false,
		onname,
		none,
		children
	}: {
		lists?: ShoppingSummary[];
		listId?: string;
		newName?: string;
		/** The name field shows instead of the Select. */
		creating?: boolean;
		/** The lists could not be loaded: no list can be picked. */
		failed?: boolean;
		/** Lists not to offer. */
		exclude?: string[];
		/** Offer the lists by name, not newest first. */
		byName?: boolean;
		prefer?: (string | null | undefined)[];
		newUnlessKnown?: boolean;
		/** The reader typed in the name field. */
		onname?: () => void;
		/** Under the name field when there is no list to offer. */
		none?: Snippet;
		/** Under the field. */
		children?: Snippet;
	} = $props();

	const uid = $props.id();
	const pool = $derived(lists.filter((l) => !exclude.includes(l.id)));
	const choices = $derived(byName ? byListName(pool) : pool);

	// The dialog closed and reopened before an answer: this instance is gone,
	// and only the new one's answer counts; a retry supersedes the one before.
	let req = 0;
	let gone = false;
	$effect(() => {
		untrack(() => void load());
		return () => {
			gone = true;
		};
	});

	async function load() {
		const mine = ++req;
		failed = false;
		try {
			const all = (await listShoppingLists()).items;
			if (gone || mine !== req) return;
			lists = all;
			const offered = (id: string | null | undefined) => pool.some((l) => l.id === id);
			const known = offered(listId) ? listId : prefer.find(offered);
			listId = known ?? pool[0]?.id ?? '';
			creating = creating || pool.length === 0 || (newUnlessKnown && !known);
		} catch (error) {
			if (gone || mine !== req) return;
			failed = true;
			if (!isSignedOut(error)) toast.error(m.lists_load_error());
		}
	}
</script>

<div>
	<div class="mb-2 flex items-baseline justify-between">
		<span class="text-label font-bold tracking-[0.08em] text-text-muted uppercase"
			>{m.add_to_list_list_label()}</span
		>
		{#if choices.length > 0 && !failed}
			<!-- A 44px target that keeps the line's height: padding out, margin back. -->
			<button
				type="button"
				onclick={() => (creating = !creating)}
				class="-mx-2 -mt-4 -mb-2 rounded-md px-2 pt-4 pb-2 text-caption font-semibold text-primary focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
			>
				{creating ? m.common_cancel() : m.add_to_list_new_list()}
			</button>
		{/if}
	</div>
	{#if failed}
		<button
			type="button"
			onclick={load}
			class="flex min-h-11 w-full items-center justify-between gap-2.5 rounded-md border border-border bg-surface-elevated px-4 py-1.5 text-left text-body-sm text-text-muted focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
		>
			<span class="min-w-0 text-caption leading-tight">{m.lists_load_error()}</span>
			<span class="shrink-0 pl-3 font-semibold whitespace-nowrap text-primary"
				>{m.add_to_list_retry()}</span
			>
		</button>
	{:else if creating}
		<Input
			id="{uid}-name"
			label={m.lists_new_name_label()}
			placeholder={m.lists_new_name_placeholder()}
			autocomplete="off"
			maxlength={80}
			bind:value={newName}
			oninput={onname}
		/>
		{#if choices.length === 0}{@render none?.()}{/if}
	{:else}
		<Select
			bind:value={listId}
			options={choices.map((l) => ({ value: l.id, label: l.name }))}
			label={m.add_to_list_list_label()}
			class="w-full justify-between bg-surface-elevated"
		/>
	{/if}
	{@render children?.()}
</div>
