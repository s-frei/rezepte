<script lang="ts">
	import { Dialog } from 'bits-ui';
	import { cubicOut } from 'svelte/easing';
	import { prefersReducedMotion } from 'svelte/motion';
	import { MediaQuery } from 'svelte/reactivity';
	import { slide } from 'svelte/transition';
	import { ApiError } from '#lib/api/client.js';
	import {
		importIntoShoppingList,
		importShoppingList,
		type ShoppingList
	} from '#lib/api/shopping.js';
	import ChevronDown from '@lucide/svelte/icons/chevron-down';
	import { tick } from 'svelte';
	import { TOGGLE } from '#lib/components/editor/link-list.js';
	import AiInstruction from '#lib/components/import/AiInstruction.svelte';
	import BaseDialog from '#lib/components/ui/BaseDialog.svelte';
	import BottomSheet from '#lib/components/ui/BottomSheet.svelte';
	import Button from '#lib/components/ui/Button.svelte';
	import Input from '#lib/components/ui/Input.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import { session } from '#lib/auth.svelte.js';
	import { addedIds } from '#lib/shopping/added.js';
	import { failList } from '#lib/shopping/fail.js';

	// One dialog for both ways text reaches a list: a new list from the
	// overview, or more items for the open list from its menu.
	let {
		open = $bindable(false),
		mode,
		ondone
	}: {
		open?: boolean;
		mode: { kind: 'new' } | { kind: 'add'; list: ShoppingList };
		/** The list as the server answered, and the ids of the items this paste added. */
		ondone: (list: ShoppingList, addedIds: string[]) => void;
	} = $props();

	// The same breakpoint as the bottom nav: a phone gets a sheet.
	const phone = new MediaQuery('max-width: 767.98px');
	const DURATION = $derived(prefersReducedMotion.current ? 0 : 180);

	let text = $state('');
	let name = $state('');
	// The name is an override: the paste's "List:" line or the default name
	// fits most lists, so its field waits behind a toggle, and a name left in
	// it after closing the toggle again does not count.
	let naming = $state(false);
	async function openNaming() {
		naming = !naming;
		if (!naming) return;
		await tick();
		document.getElementById('list-import-name')?.focus();
	}
	let sending = $state(false);
	let error = $state<string | null>(null);

	// Closed, it forgets the paste, as the recipe import does: the new-list
	// one lives in the layout for the whole visit.
	$effect(() => {
		if (open) return;
		text = '';
		name = '';
		naming = false;
		error = null;
	});

	/** The service refuses a paste with a code at `body.text`; the catalog words it. */
	function refusal(err: unknown): string | null {
		if (!(err instanceof ApiError) || err.status !== 422) return null;
		const code = err.errors[0]?.message ?? '';
		if (code === 'too-many-items') return m.list_import_too_many();
		if (code === 'no-items') return m.list_import_no_items();
		const line = /^line-too-long:(\d+)$/.exec(code)?.[1];
		if (line) return m.list_import_line_too_long({ line });
		// The API's own length limit on the text answers without a code.
		return err.errors[0]?.location === 'body.text' ? m.list_import_too_long() : null;
	}

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		if (!text.trim() || sending) return;
		sending = true;
		error = null;
		// Before the await: a live refetch must not hide what the import added.
		const before = mode.kind === 'add' ? { ...mode.list, items: [...mode.list.items] } : null;
		try {
			const list =
				mode.kind === 'add'
					? await importIntoShoppingList(mode.list.id, text)
					: await importShoppingList(
							text,
							(naming && name.trim()) || undefined,
							m.list_import_default_name()
						);
			open = false;
			ondone(list, before ? addedIds(before, list, session.user?.id) : []);
		} catch (err) {
			error = refusal(err);
			if (!error)
				failList(err, mode.kind === 'add' ? m.list_action_error() : m.lists_create_error());
		} finally {
			sending = false;
		}
	}

	// The sheet would focus its handle; the field is what it opens for.
	function focusField(event: Event) {
		event.preventDefault();
		document.getElementById('list-import-field')?.focus();
	}
</script>

{#snippet form(sheet: boolean)}
	<form onsubmit={submit} class={sheet ? 'flex min-h-0 flex-1 flex-col' : ''}>
		<div
			data-sheet-body={sheet || undefined}
			class={sheet ? 'min-h-0 flex-1 overflow-y-auto px-5 pb-4' : ''}
		>
			<Dialog.Title class="font-display text-heading font-medium">
				{mode.kind === 'add'
					? m.list_add_text_title({ name: mode.list.name })
					: m.list_import_title()}
			</Dialog.Title>
			{#if error}
				<p
					in:slide={{ duration: DURATION, easing: cubicOut }}
					role="alert"
					class="mt-3 rounded-md bg-destructive-soft px-3 py-2.5 text-body-sm text-destructive"
				>
					{error}
				</p>
			{:else}
				<Dialog.Description class="mt-1 text-body-sm text-text-muted">
					{mode.kind === 'add' ? m.list_add_text_lead() : m.list_import_lead()}
				</Dialog.Description>
			{/if}
			<label for="list-import-field" class="sr-only">{m.list_import_field_label()}</label>
			<!-- Dashed like the recipe import's field, and like it grown with its
			     lines in the sheet rather than a scroller in a scroller. -->
			<textarea
				id="list-import-field"
				bind:value={text}
				rows={5}
				disabled={sending}
				placeholder={mode.kind === 'add' ? m.list_add_text_example() : m.list_import_example()}
				style:min-height={sheet && text ? 'calc(5lh + 1.5rem + 4px)' : undefined}
				class="{sheet && text
					? 'field-sizing-content resize-none'
					: 'resize-y'} mt-4 block w-full rounded-md border-2 border-dashed border-handle bg-surface-elevated px-3.5 py-3 text-body-sm transition outline-none focus:border-primary disabled:opacity-60"
			></textarea>
			{#if mode.kind === 'new'}
				<div class="mt-2 flex justify-end">
					<button
						type="button"
						aria-expanded={naming}
						aria-controls="list-import-naming"
						onclick={openNaming}
						class={TOGGLE}
					>
						{m.list_import_name_toggle()}
						<ChevronDown
							class="size-3.5 transition-transform motion-reduce:transition-none {naming
								? 'rotate-180'
								: ''}"
							aria-hidden="true"
						/>
					</button>
				</div>
				{#if naming}
					<div
						id="list-import-naming"
						transition:slide={{ duration: DURATION, easing: cubicOut }}
						class="pt-1"
					>
						<Input
							id="list-import-name"
							label={m.list_import_name_label()}
							autocomplete="off"
							maxlength={80}
							bind:value={name}
						/>
					</div>
				{/if}
			{/if}
			<div class="mt-4">
				<AiInstruction kind="list" />
			</div>
		</div>
		<!-- On a phone the sheet's handle closes it, so the button stands alone. -->
		<div
			class="flex justify-end gap-2 {sheet
				? 'shrink-0 border-t border-border px-5 pt-3 pb-5'
				: 'mt-4'}"
		>
			<Button variant="ghost" onclick={() => (open = false)} class="max-md:hidden">
				{m.common_cancel()}
			</Button>
			<Button
				type="submit"
				busy={sending}
				disabled={!text.trim() || sending}
				class="max-md:h-12 max-md:w-full"
			>
				{mode.kind === 'add' ? m.list_add_text_submit() : m.import_submit()}
			</Button>
		</div>
	</form>
{/snippet}

{#if phone.current}
	<BottomSheet bind:open closeLabel={m.common_cancel()} onOpenAutoFocus={focusField}>
		{@render form(true)}
	</BottomSheet>
{:else}
	<BaseDialog bind:open>
		{@render form(false)}
	</BaseDialog>
{/if}
