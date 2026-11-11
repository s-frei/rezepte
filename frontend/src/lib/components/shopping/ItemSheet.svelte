<script lang="ts">
	import DialogActions from '#lib/components/ui/DialogActions.svelte';
	import Trash2 from '@lucide/svelte/icons/trash-2';
	import { Dialog, RadioGroup } from 'bits-ui';
	import { MediaQuery } from 'svelte/reactivity';
	import type { Department } from '#lib/api/shopping.js';
	import BaseDialog from '#lib/components/ui/BaseDialog.svelte';
	import BottomSheet from '#lib/components/ui/BottomSheet.svelte';
	import Button from '#lib/components/ui/Button.svelte';
	import { DEPARTMENTS, departmentLabel } from '#lib/shopping/departments.js';
	import type { Row } from '#lib/shopping/rows.js';
	import { m } from '#lib/paraglide/messages.js';
	import { sourceOrigin } from './ShoppingRow.svelte';

	let {
		open = $bindable(false),
		row,
		department,
		onpick,
		onremove
	}: {
		open?: boolean;
		/** The row last tapped; kept while the sheet closes. */
		row: Row | null;
		department: Department;
		/** Moves the name to another department for the whole instance. */
		onpick: (department: Department) => void;
		/** Takes every item behind the row off the list. */
		onremove: () => void;
	} = $props();

	const phone = new MediaQuery('max-width: 767.98px');
</script>

{#snippet body()}
	{#if row}
		<Dialog.Title class="font-display text-heading-lg font-medium text-pretty"
			>{row.name}</Dialog.Title
		>
		<p class="mt-1 flex flex-wrap gap-x-3 text-caption text-text-muted">
			{#each row.sources as source (source.itemId)}
				{@const origin = sourceOrigin(source)}
				{#if source.amount || origin}
					<span
						>{#if source.amount}<b class="font-bold text-primary">{source.amount}</b
							>{/if}{#if source.amount && origin}&nbsp;·&nbsp;{/if}{origin ?? ''}</span
					>
				{/if}
			{/each}
		</p>
		<p
			id="item-sheet-department"
			class="mt-6 mb-2 text-label font-bold tracking-wide text-text-muted uppercase"
		>
			{m.item_sheet_department()}
		</p>
		<RadioGroup.Root
			value={department}
			onValueChange={(value) => onpick(value as Department)}
			aria-labelledby="item-sheet-department"
			class="rounded-2xl border border-border bg-surface-elevated px-4"
		>
			{#each DEPARTMENTS as d (d)}
				<RadioGroup.Item
					value={d}
					class="flex w-full items-center gap-3 border-b border-dashed border-border py-2.5 text-left text-body last:border-b-0 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary data-[state=checked]:font-semibold"
				>
					{#snippet children({ checked })}
						<span
							aria-hidden="true"
							class="size-5 shrink-0 rounded-full {checked
								? 'border-[6px] border-primary'
								: 'border-[1.5px] border-text-muted'}"
						></span>
						{departmentLabel(d)}
					{/snippet}
				</RadioGroup.Item>
			{/each}
		</RadioGroup.Root>
		<p class="mt-3 text-caption text-text-muted">{m.item_sheet_remembered({ name: row.name })}</p>
	{/if}
{/snippet}

{#snippet footer()}
	<!-- Button's secondary look in the destructive ink (its own text-text would compete). -->
	<button
		type="button"
		onclick={onremove}
		class="inline-flex h-10 grow items-center justify-center gap-2 rounded-pill border border-border bg-surface hover-tint px-4 text-body-sm font-semibold whitespace-nowrap text-destructive transition focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary active:scale-[.98] md:grow-0"
	>
		<Trash2 class="size-4 shrink-0" aria-hidden="true" />
		{m.item_sheet_remove()}
	</button>
	<Button onclick={() => (open = false)} class="grow md:grow-0">{m.item_sheet_done()}</Button>
{/snippet}

{#if phone.current}
	<BottomSheet bind:open closeLabel={m.item_sheet_done()}>
		<div data-sheet-body class="min-h-0 flex-1 overflow-y-auto px-5 pb-4">{@render body()}</div>
		<!-- One line where both fit; on the narrowest phones "Fertig" wraps below, never cutting the label. -->
		<div class="flex flex-wrap gap-3 border-t border-border px-5 pt-3.5 pb-5">
			{@render footer()}
		</div>
	</BottomSheet>
{:else}
	<BaseDialog bind:open>
		{@render body()}
		<DialogActions class="mt-6 border-t border-border pt-4">{@render footer()}</DialogActions>
	</BaseDialog>
{/if}
