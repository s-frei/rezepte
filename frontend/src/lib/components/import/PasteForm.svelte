<script lang="ts" module>
	/**
	 * For the sheet's `onOpenAutoFocus`: the sheet would focus its handle,
	 * its first control, while the field is what someone opens it for; the
	 * dialog reaches the field on its own.
	 */
	export function focusField(id: string) {
		return (event: Event) => {
			event.preventDefault();
			document.getElementById(id)?.focus();
		};
	}
</script>

<script lang="ts">
	import { Dialog } from 'bits-ui';
	import type { Snippet } from 'svelte';
	import { cubicOut } from 'svelte/easing';
	import { prefersReducedMotion } from 'svelte/motion';
	import { slide } from 'svelte/transition';
	import Button from '#lib/components/ui/Button.svelte';
	import { narrow } from '#lib/ui/narrow.js';

	// The paste form both imports share, the recipe's and the shopping
	// list's, inside a `ResponsiveDialog`: the title, a refusal in place of
	// the lead, the dashed field and the submit row. In the phone's sheet
	// the body scrolls and the row stays at its foot; in the dialog the
	// caller places the row (`children`'s second snippet).
	let {
		value = $bindable(''),
		onsubmit,
		title,
		lead,
		error,
		fieldId,
		fieldLabel,
		placeholder,
		rows,
		disabled,
		cancelLabel,
		oncancel,
		submitLabel,
		canSubmit,
		busy = false,
		actions = true,
		children
	}: {
		value?: string;
		onsubmit: (event: SubmitEvent) => void;
		title: string;
		lead: string;
		/** Shown in place of the lead. */
		error: string | null;
		fieldId: string;
		fieldLabel: string;
		placeholder: string;
		rows: number;
		disabled: boolean;
		cancelLabel: string;
		oncancel: () => void;
		submitLabel: string;
		canSubmit: boolean;
		busy?: boolean;
		/** False hides the submit row (the recipe import's duplicate notice has its own). */
		actions?: boolean;
		/** Everything below the lead, given the field and the dialog's submit row. */
		children: Snippet<[field: Snippet, row: Snippet]>;
	} = $props();

	const sheet = $derived(narrow.current);
	const DURATION = $derived(prefersReducedMotion.current ? 0 : 180);
</script>

{#snippet field()}
	<label for={fieldId} class="sr-only">{fieldLabel}</label>
	<!-- Dashed like a drop area: a link dragged from the browser lands in it.
	     In the sheet, text makes it grow with its lines instead of scrolling
	     inside the sheet's scrolling body: two scrollers in one another move
	     whichever the finger lands on. Empty, it is as tall as its rows. -->
	<textarea
		id={fieldId}
		bind:value
		{rows}
		{placeholder}
		{disabled}
		style:min-height={sheet && value ? `calc(${rows}lh + 1.5rem + 4px)` : undefined}
		class="{sheet && value
			? 'field-sizing-content resize-none'
			: 'resize-y'} mt-4 block w-full rounded-md border-2 border-dashed border-handle bg-surface-elevated px-3.5 py-3 text-body-sm transition outline-none focus:border-primary disabled:opacity-60"
	></textarea>
{/snippet}

<!-- On a phone the sheet's handle closes it, so the submit stands alone and
     spans the sheet. -->
{#snippet buttons()}
	<div class="flex justify-end gap-2 {sheet ? '' : 'mt-4'}">
		<Button variant="ghost" onclick={oncancel} class="max-md:hidden">{cancelLabel}</Button>
		<Button type="submit" {busy} disabled={!canSubmit} class="max-md:h-12 max-md:w-full">
			{submitLabel}
		</Button>
	</div>
{/snippet}

{#snippet row()}
	{#if !sheet && actions}{@render buttons()}{/if}
{/snippet}

<form {onsubmit} class={sheet ? 'flex min-h-0 flex-1 flex-col' : ''}>
	<div
		data-sheet-body={sheet || undefined}
		class={sheet ? 'min-h-0 flex-1 overflow-y-auto px-5 pb-4' : ''}
	>
		<Dialog.Title class="font-display text-heading font-medium">{title}</Dialog.Title>
		{#if error}
			<p
				in:slide={{ duration: DURATION, easing: cubicOut }}
				role="alert"
				class="mt-3 rounded-md bg-destructive-soft px-3 py-2.5 text-body-sm text-destructive"
			>
				{error}
			</p>
		{:else}
			<Dialog.Description class="mt-1 text-body-sm text-text-muted">{lead}</Dialog.Description>
		{/if}
		{@render children(field, row)}
	</div>
	{#if sheet && actions}
		<div class="shrink-0 border-t border-border px-5 pt-3 pb-5">{@render buttons()}</div>
	{/if}
</form>
